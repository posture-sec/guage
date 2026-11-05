package run

import (
	"bytes"
	"context"
	"encoding/base32"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/report"
	"github.com/posture-sec/guage/internal/rules"
	"github.com/posture-sec/guage/internal/sanitize"
	"github.com/posture-sec/guage/internal/sep1"
	"github.com/posture-sec/guage/internal/source"
)

// Task 3.6, gate G2: every Appendix B scenario in PRD.md has a fixture under
// testdata/horizon and a pair of golden report files under testdata/golden, so
// a change to a rule's severity, its evidence wording or the shape of a report
// fails the build rather than quietly changing what an operator reads.
//
// A golden is written with -update and then read. Every file in
// testdata/golden was compared against PRD.md 6.1 and Appendix A line by line
// before it was committed, and each case below also names the findings it
// expects, so a golden that drifts into a different meaning fails twice.

var updateGoldens = flag.Bool("update", false, "rewrite testdata/golden from the current build")

const (
	fixtureDir      = "../../testdata/horizon"
	goldenDir       = "../../testdata/golden"
	fixtureRootName = "root-testnet.json"

	// canonicalTestnet replaces the ephemeral 127.0.0.1 port a fixture server
	// binds, because a golden file must not change when the port does. The
	// substitution happens only after the production client has stamped the
	// snapshot with the server's own URL, which decodeFixture asserts, so the
	// fixture is still proven to decode over the wire.
	canonicalTestnet = "https://horizon-testnet.stellar.org"
	testnetLedger    = 5466897

	mainnetPass = "Public Global Stellar Network ; September 2015"
	domainName  = "gauge.example.invalid"
	hostileName = "\u001b[31mgauge\u202e.example.invalid"

	accountNotFound = "GC2HHIM66CBKJFMYOHECBMKD4NWRGWJ3FEXFDPWPT4TLS7OFR43LO2II"
)

// fixtureAddress maps the role a built fixture plays to the address committed
// inside it, so a scenario reads as a shape rather than as 56 characters.
var fixtureAddress = map[string]string{
	"zero":    "GBIY5RL644R6YRLYDZD5Y6VZXOWR3ZO2MTK4TSVTD4MAORSBKMKKO4RP",
	"unreach": "GALGVY52QMWYIRPFP3EBCQZNQZXW4BPG4WWJAQ4KICLFGZH6VLSL6VMI",
	"order":   "GAGCZPLXXENDIGTXUJC4MG7N3NVW5TA37PFEBDQK64KREAGD2HM2LD6C",
	"hashx":   "GBDSJWRDC7Q4QRQ5L3FTX42CRDXEGT5CJRL4ZREVTO3HKIJCM3A3UKDD",
	"preauth": "GBNLMMJ6ZMKSID5OZBZD7B3IPE2WHWOKOETFBZ5M4V563YQRX53CUDWR",
	"domain":  "GBKDXQ7LCO3X23K4HUKSHCA3KMM3364M4UK3526XYMBWTDD4NJCSRNAI",
	"hostile": "GDF6B4EQSNXLTSN4QROIVYLIKDFGUOZLCAGGWSGFAKPY7US72Y4A2IRJ",
}

func built(label string) string {
	id, ok := fixtureAddress[label]
	if !ok {
		panic("no fixture is committed for " + label)
	}
	return id
}

// scenarioAccount is one account in a scenario run: the fixture bytes, the
// address the run asks the endpoint for, and the policy entry carrying the
// declarations the rules compare against.
type scenarioAccount struct {
	file    string
	id      string
	status  int
	account policy.Account
}

// scenario is one Appendix B case. want lists the findings per account in
// report order, as "rule:severity" pairs.
type scenario struct {
	name     string
	identity map[string]sep1.Result
	network  string
	failOn   model.Severity
	accounts []scenarioAccount
	want     []string

	// declared marks a case whose accounts carry a policy file's entries, which is
	// what lets its report print a declaration block. The cases left false are the
	// shape a zero-config check reads: an address, the default role, nothing
	// declared, and every fact still printed.
	declared bool

	// productionIdentity runs the case with the fetcher a real invocation builds
	// instead of a stub. Only a domain the client refuses before it opens a
	// socket may ask for this, which is what the case using it asserts.
	productionIdentity bool
}

func scenarios() []scenario {
	return []scenario{
		{
			// B1: default account, master weight 1, thresholds 0. The single-key
			// control sits at its INFO baseline because an undeclared run lands on
			// the `other` row of Appendix A, which says nothing about it.
			name:    "1-default-account",
			network: testnet, failOn: model.SevHigh,
			accounts: []scenarioAccount{{
				file: "1-default.json", id: accountOne,
				account: policy.Account{ID: accountOne, Role: model.RoleOther},
			}},
			want: []string{"sig-single-key-high:info"},
		},
		{
			// B2: master weight 0 and no other signers. Nothing on the account can
			// authorize anything, and at thresholds 0/0/0 even a tier of zero is
			// unreachable, because the protocol drops zero-weight candidates
			// before any comparison happens.
			name:    "2-master-weight-zero",
			network: testnet, failOn: model.SevHigh,
			accounts: []scenarioAccount{{
				file: "2-master-zero.json", id: built("zero"),
				account: policy.Account{ID: built("zero"), Role: model.RoleOther},
			}},
			want: []string{"sig-no-signing-weight:high sig-threshold-unreachable:high"},
		},
		{
			// B2 again, same bytes, with the role that means it on purpose: an
			// issuer that declares expect_locked gets INFO for the black hole and
			// still HIGH for the tiers, because a role is a floor and Appendix A
			// lists no row that could lower one. The same role is the one Appendix A
			// says publishes a home domain, and this account publishes none, so the
			// WARN rides along: declaring an intended lock does not excuse an issuer
			// from SEP-1.
			name:     "2b-locked-issuer",
			declared: true,
			network:  testnet, failOn: model.SevHigh,
			accounts: []scenarioAccount{{
				file: "2-master-zero.json", id: built("zero"),
				account: policy.Account{
					ID: built("zero"), Role: model.RoleIssuer, ExpectLocked: value(true),
				},
			}},
			want: []string{"sig-threshold-unreachable:high id-no-home-domain:warn sig-no-signing-weight:info"},
		},
		{
			// B2 again with the wrong declaration attached: the same locked account,
			// declared as a treasury that says expect_locked. Appendix A allows that
			// statement for an issuer alone, so the account keeps its HIGH and the report
			// is the only place an operator can learn the line did nothing. This is the
			// fact internal/policy's gate promises, and a check run cannot show it because
			// a check declares nothing.
			name:     "2c-locked-treasury-declaration-ignored",
			declared: true,
			network:  testnet, failOn: model.SevHigh,
			accounts: []scenarioAccount{{
				file: "2-master-zero.json", id: built("zero"),
				account: policy.Account{
					ID: built("zero"), Role: model.RoleTreasury, ExpectLocked: value(true),
				},
			}},
			want: []string{"sig-no-signing-weight:high sig-threshold-unreachable:high"},
		},
		{
			// B3: high threshold above total signer weight. Weights 3 and 4 sum to
			// 7, so tier 10 can never be authorized, and the declared 4 disagrees
			// with the chain's 10, which is the tier-drift half of the family.
			name:     "3-high-threshold-unreachable",
			declared: true,
			network:  testnet, failOn: model.SevHigh,
			accounts: []scenarioAccount{{
				file: "3-unreachable-high.json", id: built("unreach"),
				account: policy.Account{
					ID: built("unreach"), Role: model.RoleOther,
					ExpectedThresholds: &policy.ExpectedThresholds{
						Low: value(1), Medium: value(2), High: value(4),
					},
				},
			}},
			want: []string{"sig-threshold-unreachable:high thr-drift:warn"},
		},
		{
			// B4: low above medium, which the protocol stores without complaint.
			// One signer alone reaches the high tier of 2, so the single-key INFO
			// rides along: a lower tier demanding more than a higher one is not
			// the same finding as one key holding the account.
			name:    "4-thresholds-out-of-order",
			network: testnet, failOn: model.SevWarn,
			accounts: []scenarioAccount{{
				file: "4-thresholds-out-of-order.json", id: built("order"),
				account: policy.Account{ID: built("order"), Role: model.RoleOther},
			}},
			want: []string{"sig-threshold-order:warn sig-single-key-high:info"},
		},
		{
			// B5: a hash(x) signer whose own weight reaches the medium tier, which
			// is the case that raises this rule from WARN to HIGH: spending it puts
			// the preimage in a published envelope.
			name:    "5-hashx-signer-at-medium",
			network: testnet, failOn: model.SevHigh,
			accounts: []scenarioAccount{{
				file: "5-hashx-signer-at-medium.json", id: built("hashx"),
				account: policy.Account{ID: built("hashx"), Role: model.RoleOther},
			}},
			want: []string{"sig-hashx:high"},
		},
		{
			// B6: a pre-authorized transaction signer and a zero-weight master. A T
			// entry is bound to one transaction rather than standing control, so
			// nothing here can sign the next operation and every tier is dark.
			name:    "6-preauth-signer-only",
			network: testnet, failOn: model.SevHigh,
			accounts: []scenarioAccount{{
				file: "6-preauth-signer-only.json", id: built("preauth"),
				account: policy.Account{ID: built("preauth"), Role: model.RoleOther},
			}},
			want: []string{"sig-no-signing-weight:high sig-threshold-unreachable:high"},
		},
		{
			// B7: the real testnet multisig, declared as three signers when the
			// chain carries six. One declared key is at a different weight, one is
			// absent entirely, and four chain entries were never declared.
			name:     "7-extra-and-missing-signers",
			declared: true,
			network:  testnet, failOn: model.SevWarn,
			accounts: []scenarioAccount{{
				file: "7-extra-signer.json", id: accountThree,
				account: policy.Account{
					ID: accountThree, Role: model.RoleOther,
					ExpectedSigners: &[]policy.ExpectedSigner{
						{Key: "GA5J6K77LKAXSRM4VRKBXAOQXVOVYWARIG7NQURWGBXMQWIXS7R5KHRV", Weight: 1},
						{Key: "GABVD2TOKWRPASQUCSOYD6V5GMFOVTSSLMGBGDHQANZPMCKSLGKQU7JQ", Weight: 3},
						{Key: accountOne, Weight: 2},
					},
				},
			}},
			want: []string{
				"sig-missing-expected:high sig-unexpected-signer:high sig-weight-drift:warn",
			},
		},
		{
			// B8: the captured issuer, whose clawback was never declared and whose
			// auth_required disagrees with the declaration. Appendix A's issuer row
			// lifts both flag rules and the single-key control to HIGH. The identity
			// WARN is the outcome a real run recorded in docs/RECORDING.md: HTTP 200
			// with an HTML body, refused at the parse step.
			name:     "8-clawback-undeclared-and-drift",
			declared: true,
			network:  testnet, failOn: model.SevHigh,
			identity: map[string]sep1.Result{"komunitin.org": {
				HomeDomain: "komunitin.org",
				FailedAt:   sep1.StepParse,
				Detail:     "the document is not a configuration file this parser accepts",
				Status:     http.StatusOK,
				// A kept body that the parser refused still counts: internal/sep1
				// leaves Bytes unset only when it kept nothing, so a 200 with zero
				// bytes would describe a fetch that never happened.
				Bytes: 1200,
			}},
			accounts: []scenarioAccount{{
				file: "8-clawback-undeclared.json", id: accountTwo,
				account: policy.Account{
					ID: accountTwo, Role: model.RoleIssuer,
					ExpectedFlags: &policy.ExpectedFlags{AuthRequired: value(false)},
				},
			}},
			want: []string{
				"flag-drift:high flag-undeclared-clawback:high sig-single-key-high:high id-toml-unreachable:warn",
			},
		},
		{
			// B9, first half: an issuer that publishes no home domain, raised by the
			// role alone with nothing declared.
			name:     "9-home-domain-missing-by-role",
			declared: true,
			network:  testnet, failOn: model.SevWarn,
			accounts: []scenarioAccount{{
				file: "1-default.json", id: accountOne,
				account: policy.Account{ID: accountOne, Role: model.RoleIssuer},
			}},
			want: []string{"sig-single-key-high:high id-no-home-domain:warn"},
		},
		{
			// B9, same observation on the other trigger: no role that expects a
			// domain, and a policy that declared one anyway.
			name:     "9-home-domain-missing-by-declaration",
			declared: true,
			network:  testnet, failOn: model.SevWarn,
			accounts: []scenarioAccount{{
				file: "1-default.json", id: accountOne,
				account: policy.Account{ID: accountOne, Role: model.RoleOther, HomeDomain: value(domainName)},
			}},
			want: []string{"id-no-home-domain:warn sig-single-key-high:info"},
		},
		{
			// B9, second half: a published domain the guard refuses to dial. The
			// result is the shape internal/sep1 returns for a loopback answer.
			name:    "9-stellar-toml-unreachable",
			network: testnet, failOn: model.SevWarn,
			identity: map[string]sep1.Result{domainName: blocked(domainName)},
			accounts: []scenarioAccount{{
				file: "9-home-domain-present.json", id: built("domain"),
				account: policy.Account{ID: built("domain"), Role: model.RoleOther},
			}},
			want: []string{"id-toml-unreachable:warn"},
		},
		{
			// B9, third half: a document that parsed and listed a different account.
			name:    "9-stellar-toml-account-unlisted",
			network: testnet, failOn: model.SevWarn,
			identity: map[string]sep1.Result{domainName: {
				HomeDomain: domainName,
				FailedAt:   sep1.StepOK,
				Status:     http.StatusOK,
				Bytes:      128,
				Identity:   sep1.Identity{Accounts: []string{accountOne}},
			}},
			accounts: []scenarioAccount{{
				file: "9-home-domain-present.json", id: built("domain"),
				account: policy.Account{ID: built("domain"), Role: model.RoleOther},
			}},
			want: []string{"id-toml-account-unlisted:warn"},
		},
		{
			// B9, fourth half: a document that parsed, listed this account and named a
			// different network than the run audited. No rule ID covers a passphrase a
			// stellar.toml disagrees about, and inventing one is an owner decision, so the
			// disagreement is printed as a fact and the findings stay empty: a fact cannot
			// fail a run, which is what the exit code in the golden records.
			name:    "9-stellar-toml-network-differs",
			network: testnet, failOn: model.SevWarn,
			identity: map[string]sep1.Result{domainName: {
				HomeDomain:  domainName,
				FailedAt:    sep1.StepOK,
				Status:      http.StatusOK,
				Hops:        1,
				Bytes:       196,
				CORSPresent: true,
				Identity: sep1.Identity{
					NetworkPassphrase: value(mainnetPass),
					SigningKey:        value(accountOne),
					HorizonURL:        value(canonicalTestnet),
					Version:           value("v0.8.3"),
					Accounts:          []string{built("domain"), accountOne},
				},
			}},
			accounts: []scenarioAccount{{
				file: "9-home-domain-present.json", id: built("domain"),
				account: policy.Account{ID: built("domain"), Role: model.RoleOther},
			}},
			want: []string{""},
		},
		{
			// B10, first half: the endpoint serves a 404 problem document for an
			// address that was never funded.
			name:    "10-account-not-found",
			network: testnet, failOn: model.SevHigh,
			accounts: []scenarioAccount{{
				file: "10-not-found.json", id: accountNotFound, status: http.StatusNotFound,
				account: policy.Account{ID: accountNotFound, Role: model.RoleOther},
			}},
			want: []string{"acct-missing:high"},
		},
		{
			// B10, second half: a mainnet run against a testnet endpoint. Nothing is
			// read from an endpoint on the wrong network, which the account-call
			// assertion below proves against a fake that would happily answer.
			name:     "10-network-passphrase-mismatch",
			declared: true,
			network:  mainnetPass, failOn: model.SevHigh,
			accounts: []scenarioAccount{{
				id: accountOne, account: policy.Account{ID: accountOne, Role: model.RoleOther},
			}, {
				id: built("unreach"), account: policy.Account{ID: built("unreach"), Role: model.RoleTreasury},
			}},
			want: []string{"acct-network-mismatch:high", "acct-network-mismatch:high"},
		},
		{
			// B12: an ANSI escape and a right-to-left override inside the field a
			// report prints. The fixture carries the hostile bytes and the source
			// hands them over untouched, so this case asks the production fetcher
			// what it makes of them: sep1 refuses the name at its hostname step and
			// returns the sanitized value, and the refusal happens before a socket
			// opens, so nothing leaves the process. The golden is the proof that no
			// control byte reaches either format, and the test below looks for one
			// rather than trusting the rendering.
			name:    "12-hostile-home-domain",
			network: testnet, failOn: model.SevWarn,
			accounts: []scenarioAccount{{
				file: "12-hostile-home-domain.json", id: built("hostile"),
				account: policy.Account{ID: built("hostile"), Role: model.RoleOther},
			}},
			productionIdentity: true,
			want:               []string{"id-toml-unreachable:warn"},
		},
	}
}

func TestAppendixBScenarios(t *testing.T) {
	for _, sc := range scenarios() {
		t.Run(sc.name, func(t *testing.T) {
			runScenario(t, sc)
		})
	}
}

// reportlessScenarios names the Appendix B cases that cannot produce a report,
// so the completeness check below knows they are covered elsewhere rather than
// assuming every scenario ends in findings.
var reportlessScenarios = map[int]string{
	11: "muxed, contract and secret-seed input is refused before any fetch, by " +
		"TestScenarioElevenRefusesBeforeAnyFetch",
}

// TestAppendixBNumbersAreAllPresent keeps the table honest about what it covers:
// Appendix B has twelve scenarios, and a scenario that fell out of the table
// would otherwise show up as a green run.
func TestAppendixBNumbersAreAllPresent(t *testing.T) {
	present := map[int]bool{}
	for _, sc := range scenarios() {
		digits := strings.TrimLeft(sc.name, "0123456789")
		number, err := strconv.Atoi(sc.name[:len(sc.name)-len(digits)])
		if err != nil || number < 1 || number > 12 {
			t.Fatalf("scenario %q does not start with an Appendix B number", sc.name)
		}
		present[number] = true
	}
	var missing []string
	for number := 1; number <= 12; number++ {
		if present[number] {
			continue
		}
		if _, ok := reportlessScenarios[number]; ok {
			continue
		}
		missing = append(missing, strconv.Itoa(number))
	}
	if len(missing) != 0 {
		t.Fatalf("Appendix B scenarios without a case: %s", strings.Join(missing, ", "))
	}
}

// TestGoldensRenderNoControlBytes is scenario 12's claim read back off the
// files, so the assertion is about bytes rather than about how a Go string
// happened to compare.
func TestGoldensRenderNoControlBytes(t *testing.T) {
	if *updateGoldens {
		t.Skip("the goldens are being rewritten")
	}
	names, err := filepath.Glob(filepath.Join(goldenDir, "*.golden"))
	if err != nil {
		t.Fatalf("globbing goldens: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("no golden files under " + goldenDir)
	}
	sort.Strings(names)
	for _, path := range names {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for i, r := range string(body) {
			if r < 0x20 && r != '\n' && r != '\t' {
				t.Fatalf("%s carries control byte %#x at offset %d", path, r, i)
			}
			if r == 0x202e || r == 0x202d || (r >= 0x2066 && r <= 0x2069) {
				t.Fatalf("%s carries bidi control %#x at offset %d", path, r, i)
			}
		}
	}
}

func runScenario(t *testing.T, sc scenario) {
	t.Helper()
	fs := &fakeSource{
		info:       model.NetworkInfo{Passphrase: testnet, Ledger: testnetLedger},
		snapshots:  map[string]model.AccountSnapshot{},
		accountErr: map[string]error{},
	}
	accounts := make([]policy.Account, 0, len(sc.accounts))
	// A fixture that answers 404 is still a read the run performed, which is the
	// point of scenario 10: a missing account is an observation, not a skip.
	reads := 0
	for _, entry := range sc.accounts {
		accounts = append(accounts, entry.account)
		if entry.file == "" {
			continue
		}
		info, snapshot, err := decodeFixture(t, entry.file, entry.id, entry.status)
		if info.Ledger != testnetLedger || info.Passphrase != testnet {
			t.Fatalf("root fixture read as %q at %d", info.Passphrase, info.Ledger)
		}
		reads++
		if entry.status == http.StatusNotFound {
			if !errors.Is(err, source.ErrNotFound) {
				t.Fatalf("fixture %s: err = %v, want source.ErrNotFound", entry.file, err)
			}
			fs.accountErr[entry.id] = source.ErrNotFound
			continue
		}
		if err != nil {
			t.Fatalf("fixture %s: %v", entry.file, err)
		}
		fs.snapshots[entry.id] = snapshot
	}

	job := Job{
		ToolVersion: "test",
		Network:     sc.network,
		Endpoints:   []string{canonicalTestnet},
		Accounts:    accounts,
		FailOn:      sc.failOn,
		HasPolicy:   sc.declared,
		NewSource:   stubFactory(fs),
		Evaluate:    rules.Evaluate,
	}
	if sc.identity != nil {
		job.NewIdentity = identityStub(&fakeIdentity{results: sc.identity})
	}
	if sc.productionIdentity {
		if sc.identity != nil {
			t.Fatalf("scenario %s asks for both a stub and the production fetcher", sc.name)
		}
		job.NewIdentity = defaultIdentity("test")
	}

	result, err := Run(context.Background(), job)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calls := fs.called(); len(calls) != reads {
		t.Fatalf("the run read %d accounts %v, want %d", len(calls), calls, reads)
	}

	var got []string
	for _, account := range result.Accounts {
		rendered := make([]string, 0, len(account.Findings))
		for _, f := range account.Findings {
			rendered = append(rendered, fmt.Sprintf("%s:%s", f.RuleID, f.Severity))
		}
		got = append(got, strings.Join(rendered, " "))
	}
	if len(got) != len(sc.want) {
		t.Fatalf("accounts = %d, want %d", len(got), len(sc.want))
	}
	for i, want := range sc.want {
		if got[i] != want {
			t.Errorf("account %d findings = %q, want %q", i, got[i], want)
		}
	}

	var text bytes.Buffer
	if err := report.Text(&text, result); err != nil {
		t.Fatalf("report.Text: %v", err)
	}
	var document bytes.Buffer
	if err := report.JSON(&document, result); err != nil {
		t.Fatalf("report.JSON: %v", err)
	}
	checkGolden(t, sc.name+".text.golden", text.String())
	checkGolden(t, sc.name+".json.golden", document.String())
}

// decodeFixture serves one committed fixture over a socket that is not the
// network and reads it back through the production Horizon client, so a golden
// is the report of what the decoder made of the bytes rather than of a shape
// this file typed into a struct.
func decodeFixture(t *testing.T, file, account string, status int) (
	model.NetworkInfo, model.AccountSnapshot, error) {
	t.Helper()
	root := readTreeFixture(t, fixtureRootName)
	body := readTreeFixture(t, file)
	if status == 0 {
		status = http.StatusOK
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "application/hal+json; charset=utf-8")
			_, _ = w.Write(root)
			return
		}
		if want := "/accounts/" + account; r.URL.Path != want {
			t.Errorf("path = %q, want %q", r.URL.Path, want)
		}
		w.Header().Set("Content-Type", "application/hal+json; charset=utf-8")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	client, err := source.NewHorizon(source.Options{Endpoint: srv.URL, UserAgent: "gauge-test"})
	if err != nil {
		t.Fatalf("NewHorizon(%s): %v", srv.URL, err)
	}
	ctx := context.Background()
	info, err := client.Network(ctx)
	if err != nil {
		t.Fatalf("Network: %v", err)
	}
	snapshot, err := client.Account(ctx, account)
	if err != nil {
		return info, model.AccountSnapshot{}, err
	}
	if snapshot.Endpoint != srv.URL {
		t.Fatalf("decoded endpoint = %q, want the server's own %q", snapshot.Endpoint, srv.URL)
	}
	snapshot.Endpoint = canonicalTestnet
	return info, snapshot, nil
}

func readTreeFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(fixtureDir, name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return body
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join(goldenDir, name)
	if *updateGoldens {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden %s: %v", name, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v (a new case needs -update once)", name, err)
	}
	if string(want) != got {
		t.Errorf("golden %s differs\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
	}
}

// TestScenarioElevenRefusesBeforeAnyFetch covers Appendix B scenario 11, whose
// expected result is no report at all. A muxed address, a contract address and
// a secret seed are each a well-formed strkey, and none of them is an account,
// so the run refuses the request and reads nothing from a source that would
// otherwise answer.
func TestScenarioElevenRefusesBeforeAnyFetch(t *testing.T) {
	factory := stubFactory(&fakeSource{
		info:      model.NetworkInfo{Passphrase: testnet, Ledger: testnetLedger},
		snapshots: map[string]model.AccountSnapshot{},
	})
	for _, input := range []struct {
		name  string
		value string
	}{
		{"muxed account", scenarioStrkey(12, 40)},
		{"contract", scenarioStrkey(2, 32)},
		{"secret seed", scenarioStrkey(18, 32)},
	} {
		t.Run(input.name, func(t *testing.T) {
			_, err := Check(context.Background(), CheckRequest{
				ToolVersion: "test",
				Network:     "testnet",
				Endpoints:   []string{canonicalTestnet},
				Accounts:    []string{input.value},
				FailOn:      model.SevHigh,
				NewSource:   factory,
			})
			if !errors.Is(err, ErrUsage) {
				t.Fatalf("Check(%s) err = %v, want ErrUsage", input.name, err)
			}
			if !strings.Contains(err.Error(), "G address") {
				t.Fatalf("Check(%s) says %q, which does not name the rule it applied",
					input.name, err)
			}
		})
	}
}

// TestHostileFixtureCarriesRawControlBytes is the other half of scenario 12. The
// golden shows a domain that looks clean, and this says why: the fixture holds the
// escape and the override, the decoder hands them over untouched, the fetcher
// refuses the name before it dials, and the only place the hostile string is
// altered is the render. Nothing here trusts the golden to prove it.
func TestHostileFixtureCarriesRawControlBytes(t *testing.T) {
	_, snapshot, err := decodeFixture(t, "12-hostile-home-domain.json", built("hostile"), http.StatusOK)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if snapshot.HomeDomain != hostileName {
		t.Fatalf("decoded home domain = %q, want the raw hostile string", snapshot.HomeDomain)
	}
	if _, ok := sep1.HomeDomain(snapshot.HomeDomain); ok {
		t.Fatal("the fetcher would dial the hostile name, so scenario 12 no longer proves a refusal")
	}
	if got := sanitize.Text(snapshot.HomeDomain); got != domainName {
		t.Fatalf("sanitized home domain = %q, want %q", got, domainName)
	}
}

func value[T any](v T) *T { return &v }

// scenarioStrkey builds a valid strkey at test time, the way
// cmd/stellar-gauge/cli_test.go does, so no seed-shaped literal is committed to
// this repository even for a case that only ever gets refused.
func scenarioStrkey(version byte, size int) string {
	raw := make([]byte, 0, size+3)
	raw = append(raw, version<<3)
	for i := 0; i < size; i++ {
		raw = append(raw, byte(i+1))
	}
	var crc uint16
	for _, b := range raw {
		crc ^= uint16(b) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	raw = append(raw, byte(crc), byte(crc>>8))
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
}

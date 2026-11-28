package report

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

type schemaV1Envelope struct {
	SchemaVersion int               `json:"schema_version"`
	ToolVersion   string            `json:"tool_version"`
	Network       string            `json:"network"`
	Ledger        int64             `json:"ledger"`
	Accounts      []schemaV1Account `json:"accounts"`
	ExitCode      int               `json:"exit_code"`
	Limits        string            `json:"limits"`
}

type schemaV1Account struct {
	Snapshot schemaV1Snapshot  `json:"snapshot"`
	Facts    *schemaV1Facts    `json:"facts"`
	Findings []schemaV1Finding `json:"findings"`
	Missing  bool              `json:"missing"`
	Skipped  string            `json:"skipped"`
}

type schemaV1Snapshot struct {
	ID         string             `json:"id"`
	Network    string             `json:"network"`
	Endpoint   string             `json:"endpoint"`
	Ledger     int64              `json:"ledger"`
	Sequence   string             `json:"sequence"`
	Thresholds schemaV1Thresholds `json:"thresholds"`
	Signers    []schemaV1Signer   `json:"signers"`
	Flags      schemaV1Flags      `json:"flags"`
	HomeDomain string             `json:"home_domain"`
	Sponsor    string             `json:"sponsor"`
	Balances   []schemaV1Balance  `json:"balances"`
	Data       map[string]string  `json:"data"`
}

type schemaV1Thresholds struct {
	Low    int `json:"low"`
	Medium int `json:"medium"`
	High   int `json:"high"`
}

type schemaV1Signer struct {
	Key     string `json:"key"`
	Type    string `json:"type"`
	Weight  int    `json:"weight"`
	Sponsor string `json:"sponsor"`
}

type schemaV1Flags struct {
	AuthRequired    bool `json:"auth_required"`
	AuthRevocable   bool `json:"auth_revocable"`
	AuthImmutable   bool `json:"auth_immutable"`
	ClawbackEnabled bool `json:"clawback_enabled"`
}

type schemaV1Balance struct {
	AssetCode       string `json:"asset_code"`
	AssetIssuer     string `json:"asset_issuer"`
	IsAuthorized    bool   `json:"is_authorized"`
	ClawbackEnabled bool   `json:"clawback_enabled"`
}

type schemaV1Facts struct {
	Reachability *schemaV1Reachability `json:"reachability"`
	Declared     *schemaV1Declared     `json:"declared"`
	Identity     *schemaV1Identity     `json:"identity"`
	Coordinator  *schemaV1Coordinator  `json:"coordinator"`
	Trustlines   []schemaV1Trustline   `json:"trustlines"`
}

type schemaV1Coordinator struct {
	DataKey string `json:"data_key"`
	Domain  string `json:"domain"`
	Valid   bool   `json:"valid"`
}

type schemaV1Reachability struct {
	Low    *int `json:"low"`
	Medium *int `json:"medium"`
	High   *int `json:"high"`
}

type schemaV1Declared struct {
	Name                string                      `json:"name"`
	Role                string                      `json:"role"`
	ExpectedSigners     *int                        `json:"expected_signers"`
	ExpectedThresholds  *schemaV1DeclaredThresholds `json:"expected_thresholds"`
	ExpectedFlags       *schemaV1DeclaredFlags      `json:"expected_flags"`
	HomeDomain          *string                     `json:"home_domain"`
	Coordinator         *string                     `json:"coordinator"`
	ExpectLocked        *bool                       `json:"expect_locked"`
	ExpectLockedIgnored bool                        `json:"expect_locked_ignored"`
}

type schemaV1DeclaredThresholds struct {
	Low    *int `json:"low"`
	Medium *int `json:"medium"`
	High   *int `json:"high"`
}

type schemaV1DeclaredFlags struct {
	AuthRequired  *bool `json:"auth_required"`
	AuthRevocable *bool `json:"auth_revocable"`
	AuthImmutable *bool `json:"auth_immutable"`
	Clawback      *bool `json:"clawback_enabled"`
}

type schemaV1Identity struct {
	HomeDomain        string   `json:"home_domain"`
	Reachable         bool     `json:"reachable"`
	Step              string   `json:"step"`
	Detail            string   `json:"detail"`
	Status            int      `json:"status"`
	Hops              int      `json:"hops"`
	Bytes             int      `json:"bytes"`
	CORSPresent       bool     `json:"cors_present"`
	NetworkPassphrase *string  `json:"network_passphrase"`
	NetworkMatches    *bool    `json:"network_matches"`
	SigningKey        *string  `json:"signing_key"`
	HorizonURL        *string  `json:"horizon_url"`
	Version           *string  `json:"version"`
	Accounts          []string `json:"accounts"`
	MalformedAccounts int      `json:"malformed_accounts"`
}

type schemaV1Trustline struct {
	AssetCode    string         `json:"asset_code"`
	AssetIssuer  string         `json:"asset_issuer"`
	Authorized   bool           `json:"authorized"`
	Clawback     bool           `json:"clawback"`
	IssuerFlags  *schemaV1Flags `json:"issuer_flags"`
	IssuerStatus string         `json:"issuer_status"`
}

type schemaV1Finding struct {
	RuleID      string `json:"rule_id"`
	Severity    string `json:"severity"`
	Account     string `json:"account"`
	Message     string `json:"message"`
	Evidence    string `json:"evidence"`
	Fingerprint string `json:"fingerprint"`
	Accepted    bool   `json:"accepted"`
}

func decodeSchemaV1(t *testing.T, data []byte) schemaV1Envelope {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope schemaV1Envelope
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("schema golden does not decode strictly: %v", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatalf("schema golden has trailing JSON: %v", err)
	}
	return envelope
}

func TestJSONSchemaVersion(t *testing.T) {
	t.Parallel()
	golden, err := os.ReadFile(filepath.Join("..", "..", "testdata", "golden", "schema-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	envelope := decodeSchemaV1(t, golden)
	if envelope.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version is %d, want %d", envelope.SchemaVersion, SchemaVersion)
	}
	if SchemaVersion != 1 {
		t.Fatalf("SchemaVersion is %d, want 1", SchemaVersion)
	}
	if envelope.ToolVersion == "" || envelope.Network == "" || envelope.Ledger == 0 || envelope.Limits == "" {
		t.Fatal("envelope is missing a required top-level field")
	}
	if len(envelope.Accounts) != 3 {
		t.Fatalf("got %d accounts, want 3", len(envelope.Accounts))
	}
	account := envelope.Accounts[0]
	if account.Snapshot.ID == "" || account.Snapshot.Network == "" || account.Snapshot.Endpoint == "" || account.Snapshot.Sequence == "" || account.Snapshot.HomeDomain == "" || account.Snapshot.Sponsor == "" {
		t.Fatal("snapshot is missing a required string field")
	}
	if account.Snapshot.Thresholds.Low != 1 || account.Snapshot.Thresholds.Medium != 2 || account.Snapshot.Thresholds.High != 1 {
		t.Fatal("snapshot thresholds did not decode as integers")
	}
	if len(account.Snapshot.Signers) != 1 || account.Snapshot.Signers[0].Type != "ed25519" || account.Snapshot.Signers[0].Weight != 1 {
		t.Fatal("snapshot signers did not decode as key, type and weight")
	}
	if account.Snapshot.Flags.AuthRequired != true || account.Snapshot.Flags.ClawbackEnabled != false {
		t.Fatal("snapshot flags did not decode as booleans")
	}
	if len(account.Snapshot.Balances) != 1 || account.Snapshot.Balances[0].AssetCode != "XLM" || account.Snapshot.Data["memo_required"] != "MQ==" {
		t.Fatal("snapshot balances and data did not decode")
	}
	if account.Facts == nil || account.Facts.Reachability == nil || account.Facts.Declared == nil || account.Facts.Identity == nil {
		t.Fatal("facts did not decode")
	}
	if account.Facts.Reachability.Low == nil || *account.Facts.Reachability.Low != 1 || account.Facts.Reachability.Medium != nil {
		t.Fatal("reachability tiers did not decode as optional integers")
	}
	if account.Facts.Declared.Role != "other" || account.Facts.Declared.ExpectedSigners == nil || *account.Facts.Declared.ExpectedSigners != 1 || account.Facts.Declared.ExpectedFlags == nil || account.Facts.Declared.ExpectedFlags.AuthRequired == nil {
		t.Fatal("declared facts did not decode")
	}
	if account.Facts.Identity.Status != 200 || account.Facts.Identity.NetworkPassphrase == nil || account.Facts.Identity.NetworkMatches == nil || len(account.Facts.Identity.Accounts) != 1 {
		t.Fatal("identity facts did not decode")
	}
	if account.Facts.Coordinator == nil || !account.Facts.Coordinator.Valid || account.Facts.Coordinator.DataKey != "config.multisig.coordinator" || account.Facts.Coordinator.Domain == "" {
		t.Fatal("coordinator facts did not decode")
	}
	if account.Facts.Declared.Coordinator == nil || *account.Facts.Declared.Coordinator == "" {
		t.Fatal("declared coordinator did not decode")
	}
	if len(account.Facts.Trustlines) != 1 || account.Facts.Trustlines[0].IssuerFlags == nil || account.Facts.Trustlines[0].IssuerStatus != "not found" {
		t.Fatal("trustline facts did not decode")
	}
	if len(account.Findings) != 1 || account.Findings[0].Severity != "warn" || account.Findings[0].RuleID != "flag-drift" || account.Findings[0].Fingerprint == "" {
		t.Fatal("findings did not decode")
	}
	if !envelope.Accounts[1].Missing || envelope.Accounts[2].Skipped == "" {
		t.Fatal("missing and skipped account flags did not decode")
	}

	unknown := strings.Replace(string(golden), "{", "{\n  \"future_field\": 1,", 1)
	strict := json.NewDecoder(strings.NewReader(unknown))
	strict.DisallowUnknownFields()
	var rejected schemaV1Envelope
	err = strict.Decode(&rejected)
	if err == nil || !strings.Contains(err.Error(), "future_field") {
		t.Fatalf("unknown top-level key was not rejected: %v", err)
	}

	var buf bytes.Buffer
	if err := JSON(&buf, model.Report{}); err != nil {
		t.Fatal(err)
	}
	prefix := "{\n  \"schema_version\": " + strconv.Itoa(SchemaVersion) + ",\n"
	if !strings.HasPrefix(buf.String(), prefix) {
		t.Fatalf("schema_version is not the first key:\n%s", buf.String())
	}
}

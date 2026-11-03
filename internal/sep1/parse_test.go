package sep1

import (
	"strings"
	"testing"
)

const (
	validAccount = "GBQHXYDEJD6NEIUQ5NYDBVNTRCACJXMLO32JUQHBGO524NMVYOMIN5CP"
	otherAccount = "GDSX2WUFUT7KCMTUNG3IUQWIUQSC4XNKQ5NTGEZRKLTMLCSJTCUVBEVG"
	muxedKey     = "MAD6CREDL6XRSLWLT4NNUQIPF4N6F2YTPVQ6BQXQZQGFNNEJ3PLHBSWUB"
	badChecksum  = "GBQHXYDEJD6NEIUQ5NYDBVNTRCACJXMLO32JUQHBGO524NMVYOMIN5CQ"
	testnetPass  = "Stellar Testnet Sandbox (not real)"
	mainnetPass  = "Public Global Stellar Network ; September 2015"
)

func TestParseReadsTheIdentityKeys(t *testing.T) {
	t.Parallel()
	body := []string{
		"NETWORK_PASSPHRASE = \"" + mainnetPass + "\"",
		"SIGNING_KEY = \"" + validAccount + "\"",
		"HORIZON_URL = \"https://horizon.stellar.org\"",
		"VERSION = \"1.0\"",
		"ACCOUNTS = [\"" + validAccount + "\", \"" + otherAccount + "\"]",
	}
	id, step, detail := Parse([]byte(strings.Join(body, "\n")))
	if step != StepOK {
		t.Fatalf("Parse step = %v (%q), want ok", step, detail)
	}
	if id.NetworkPassphrase == nil || *id.NetworkPassphrase != mainnetPass {
		t.Fatalf("NETWORK_PASSPHRASE = %v, want it read back", id.NetworkPassphrase)
	}
	if id.SigningKey == nil || *id.SigningKey != validAccount {
		t.Fatalf("SIGNING_KEY = %v, want it read back", id.SigningKey)
	}
	if id.HorizonURL == nil || *id.HorizonURL != "https://horizon.stellar.org" {
		t.Fatalf("HORIZON_URL = %v, want it read back", id.HorizonURL)
	}
	if id.Version == nil || *id.Version != "1.0" {
		t.Fatalf("VERSION = %v, want it read back", id.Version)
	}
	if len(id.Accounts) != 2 || id.Accounts[0] != validAccount || id.Accounts[1] != otherAccount {
		t.Fatalf("ACCOUNTS = %v, want both listed accounts", id.Accounts)
	}
	if id.MalformedAccounts != 0 {
		t.Fatalf("MalformedAccounts = %d, want 0", id.MalformedAccounts)
	}
	if !id.Recognized() {
		t.Fatal("Recognized() = false, want true for a document with identity keys")
	}
}

func TestParseIgnoresUnrelatedKeys(t *testing.T) {
	t.Parallel()
	// A real stellar.toml carries far more than the fields Gauge reads. Unknown
	// keys must not make it unreachable: that reading would fail almost every
	// issuer on the network. The identity key comes first because everything
	// after a [table] header belongs to that table, not to the top level.
	body := "NETWORK_PASSPHRASE = \"" + testnetPass + "\"\nORG = \"Acme\"\nHOME_DEPOSIT_URL = \"https://acme.example\"\n" +
		"[Currencies.USDC]\ncode = \"USDC\"\nissuer = \"" + validAccount + "\"\n"
	id, step, detail := Parse([]byte(body))
	if step != StepOK {
		t.Fatalf("Parse step = %v (%q), want ok", step, detail)
	}
	if id.NetworkPassphrase == nil || *id.NetworkPassphrase != testnetPass {
		t.Fatalf("NETWORK_PASSPHRASE = %v, want the one recognized key kept", id.NetworkPassphrase)
	}
	if id.Accounts != nil {
		t.Fatalf("ACCOUNTS = %v, want nil when the key is absent", id.Accounts)
	}
}

func TestParseNeedsAtLeastOneRecognizedKey(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"empty document":     "",
		"comment only":       "# nothing here\n",
		"unrelated keys":     "ORG = \"Acme\"\nTHUMBNAIL = \"logo.png\"\n",
		"html page not toml": "<!doctype html><html><body><h1>Account</h1></body></html>",
		"currencies only":    "[Currencies.X]\ncode = \"X\"\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, step, detail := Parse([]byte(body))
			if step == StepOK {
				t.Fatalf("Parse(%q) = ok, want a failure", body)
			}
			if step != StepParse && step != StepNoKeys {
				t.Fatalf("Parse(%q) step = %v, want parse or no-recognized-keys", body, step)
			}
			if detail == "" {
				t.Fatalf("Parse(%q) detail is empty, want the failure named", body)
			}
		})
	}
}

func TestParseKeepsValidAccountsAndCountsTheRest(t *testing.T) {
	t.Parallel()
	body := "ACCOUNTS = [\"" + validAccount + "\", \"" + muxedKey + "\", \"" + badChecksum + "\", \"not-a-key\", \"\"]\n" +
		"NETWORK_PASSPHRASE = \"" + testnetPass + "\"\n"
	id, step, _ := Parse([]byte(body))
	if step != StepOK {
		t.Fatalf("Parse step = %v, want ok", step)
	}
	if len(id.Accounts) != 1 || id.Accounts[0] != validAccount {
		t.Fatalf("ACCOUNTS = %v, want only the valid G address kept", id.Accounts)
	}
	if id.MalformedAccounts != 4 {
		t.Fatalf("MalformedAccounts = %d, want 4 counted", id.MalformedAccounts)
	}
}

func TestParseRejectsAnIdentityKeyOfTheWrongType(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"accounts is a table":    "ACCOUNTS = { one = \"" + validAccount + "\" }\n",
		"accounts is a string":   "ACCOUNTS = \"" + validAccount + "\"\n",
		"passphrase is a number": "NETWORK_PASSPHRASE = 7\n",
		"truncated string":       "NETWORK_PASSPHRASE = \"unterminated\n",
		"duplicate key":          "VERSION = \"1\"\nVERSION = \"2\"\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, step, detail := Parse([]byte(body))
			if step != StepParse {
				t.Fatalf("Parse step = %v (%q), want parse", step, detail)
			}
		})
	}
	// The parser's own message can quote the hostile body, so it must never
	// reach the detail string.
	_, step, detail := Parse([]byte("NETWORK_PASSPHRASE = \"unterminated\n"))
	if step != StepParse || strings.Contains(detail, "unterminated") {
		t.Fatalf("detail = %q, want a generic parse failure that does not repeat the body", detail)
	}
}

func TestParseSanitizesTheValuesItKeeps(t *testing.T) {
	t.Parallel()
	// A TOML basic string cannot hold a raw control byte, so the hostile
	// characters reach the parser as escape sequences it decodes itself.
	hostile := "VERSION = \"1.0\\u001b[31m\\u202e  injected\"\nNETWORK_PASSPHRASE = \"" + testnetPass + "\"\n"
	id, step, _ := Parse([]byte(hostile))
	if step != StepOK {
		t.Fatalf("Parse step = %v, want ok", step)
	}
	if id.Version == nil {
		t.Fatal("Version = nil, want the key kept")
	}
	if strings.ContainsRune(*id.Version, 0x1b) || strings.ContainsRune(*id.Version, 0x202e) {
		t.Fatalf("Version = %q, want control and bidi characters stripped", *id.Version)
	}
	long := "SIGNING_KEY = \"" + strings.Repeat("a", 900) + "\"\nNETWORK_PASSPHRASE = \"" + testnetPass + "\"\n"
	id, step, _ = Parse([]byte(long))
	if step != StepOK || id.SigningKey == nil {
		t.Fatalf("Parse step = %v, want ok with the key kept", step)
	}
	if len([]rune(*id.SigningKey)) > 260 {
		t.Fatalf("SignKey length = %d, want it truncated to the field cap", len([]rune(*id.SigningKey)))
	}
}

func TestStepNamesAreStableLabels(t *testing.T) {
	t.Parallel()
	for _, s := range []Step{StepOK, StepHostname, StepResolve, StepGuard, StepTransport, StepRedirect, StepStatus, StepSize, StepParse, StepNoKeys} {
		if s.String() == "" {
			t.Fatalf("Step(%d).String() is empty", s)
		}
	}
}

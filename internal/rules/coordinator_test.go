package rules

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
)

func strPtr(s string) *string { return &s }

func TestIdMultisigCoordinatorUnexpected(t *testing.T) {
	t.Parallel()

	satoshiB64 := "djEubXVsdGlzaWcuc2F0b3NoaXBheS5pbw=="
	b64 := func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	}
	baseSnap := signable(model.AccountSnapshot{
		ID:   testAccount,
		Data: map[string]string{"config.multisig.coordinator": satoshiB64},
	})

	t.Run("missing and mismatch skip rule", func(t *testing.T) {
		t.Parallel()
		for _, flag := range []string{"missing", "mismatch"} {
			ctx := Context{Account: policy.Account{ID: testAccount}, Snapshot: baseSnap}
			if flag == "missing" {
				ctx.Missing = true
			} else {
				ctx.Mismatch = true
			}
			if got := idMultisigCoordinatorUnexpected(ctx); len(got) != 0 {
				t.Fatalf("%s raised %d findings, want 0", flag, len(got))
			}
		}
	})

	t.Run("no coordinator data entry stays silent", func(t *testing.T) {
		t.Parallel()
		snap := signable(model.AccountSnapshot{ID: testAccount, Data: map[string]string{"config.memo_required": "MQ=="}})
		for _, decl := range []*string{nil, strPtr(""), strPtr("v1.multisig.satoshipay.io")} {
			ctx := Context{Account: policy.Account{ID: testAccount, Coordinator: decl}, Snapshot: snap}
			if got := idMultisigCoordinatorUnexpected(ctx); len(got) != 0 {
				t.Fatalf("no entry with %v raised %d findings, want 0", decl, len(got))
			}
		}
	})

	t.Run("undeclared policy warns and names key and domain", func(t *testing.T) {
		t.Parallel()
		ctx := Context{Account: policy.Account{ID: testAccount, Role: model.RoleOther}, Snapshot: baseSnap}
		got := idMultisigCoordinatorUnexpected(ctx)
		if len(got) != 1 {
			t.Fatalf("got %d findings, want 1: %+v", len(got), got)
		}
		f := got[0]
		if f.RuleID != "id-multisig-coordinator-unexpected" || f.Severity != model.SevWarn {
			t.Fatalf("unexpected finding header: %+v", f)
		}
		if !strings.Contains(f.Evidence, "config.multisig.coordinator") || !strings.Contains(f.Evidence, "v1.multisig.satoshipay.io") {
			t.Fatalf("evidence missing key or domain: %q", f.Evidence)
		}
	})

	t.Run("alias key matches and warns when undeclared", func(t *testing.T) {
		t.Parallel()
		snap := signable(model.AccountSnapshot{
			ID:   testAccount,
			Data: map[string]string{"lobstr.multisig.coordinator": b64("vault.lobstr.co")},
		})
		ctx := Context{Account: policy.Account{ID: testAccount}, Snapshot: snap}
		got := idMultisigCoordinatorUnexpected(ctx)
		if len(got) != 1 || !strings.Contains(got[0].Evidence, "lobstr.multisig.coordinator") || !strings.Contains(got[0].Evidence, "vault.lobstr.co") {
			t.Fatalf("got %+v, want alias key and domain", got)
		}
	})

	t.Run("declared empty string silences warning", func(t *testing.T) {
		t.Parallel()
		ctx := Context{Account: policy.Account{ID: testAccount, Coordinator: strPtr("")}, Snapshot: baseSnap}
		if got := idMultisigCoordinatorUnexpected(ctx); len(got) != 0 {
			t.Fatalf("empty declaration raised %+v, want 0", got)
		}
	})

	t.Run("matching normalized url and case is silent", func(t *testing.T) {
		t.Parallel()
		snap := signable(model.AccountSnapshot{
			ID:   testAccount,
			Data: map[string]string{"config.multisig.coordinator": b64("https://V1.Multisig.SatoshiPay.IO/signers")},
		})
		ctx := Context{
			Account:  policy.Account{ID: testAccount, Coordinator: strPtr("v1.multisig.satoshipay.io.")},
			Snapshot: snap,
		}
		if got := idMultisigCoordinatorUnexpected(ctx); len(got) != 0 {
			t.Fatalf("matching normalized coordinator raised %+v, want 0", got)
		}
	})

	t.Run("mismatch warns naming both normalized domains", func(t *testing.T) {
		t.Parallel()
		snap := signable(model.AccountSnapshot{
			ID:   testAccount,
			Data: map[string]string{"config.multisig.coordinator": b64("https://Other.Coordinator.Example/api")},
		})
		ctx := Context{
			Account:  policy.Account{ID: testAccount, Coordinator: strPtr("V1.Multisig.SatoshiPay.IO")},
			Snapshot: snap,
		}
		got := idMultisigCoordinatorUnexpected(ctx)
		if len(got) != 1 || got[0].Severity != model.SevWarn {
			t.Fatalf("got %+v, want 1 warn", got)
		}
		ev := got[0].Evidence
		if !strings.Contains(ev, "other.coordinator.example") || !strings.Contains(ev, "v1.multisig.satoshipay.io") {
			t.Fatalf("mismatch evidence missing normalized domains: %q", ev)
		}
	})

	t.Run("unreadable hostile value warns without echoing raw bytes", func(t *testing.T) {
		t.Parallel()
		rawHostile := "\x1b[31m\u202ehostile-token.evil"
		hostileB64 := b64(rawHostile)
		snap := signable(model.AccountSnapshot{
			ID:   testAccount,
			Data: map[string]string{"config.multisig.coordinator": hostileB64},
		})
		ctx := Context{
			Account:  policy.Account{ID: testAccount, Coordinator: strPtr("v1.multisig.satoshipay.io")},
			Snapshot: snap,
		}
		got := idMultisigCoordinatorUnexpected(ctx)
		if len(got) != 1 || got[0].Severity != model.SevWarn {
			t.Fatalf("got %+v, want 1 warn", got)
		}
		ev := got[0].Evidence
		if !strings.Contains(ev, "config.multisig.coordinator") {
			t.Fatalf("evidence should name data key: %q", ev)
		}
		if strings.Contains(ev, hostileB64) || strings.Contains(ev, "hostile-token") || strings.ContainsRune(ev, 0x1b) {
			t.Fatalf("evidence leaked raw hostile bytes/payload: %q", ev)
		}
	})

	t.Run("all roles stay WARN per D-17", func(t *testing.T) {
		t.Parallel()
		for _, role := range []model.Role{model.RoleIssuer, model.RoleTreasury, model.RoleDistribution, model.RoleMerchant, model.RoleOther} {
			if sev := policy.SeverityFor("id-multisig-coordinator-unexpected", string(role), model.SevWarn); sev != model.SevWarn {
				t.Fatalf("role %s SeverityFor = %s, want warn", role, sev)
			}
		}
	})
}

func TestCoordinatorFactsAndHostileValue(t *testing.T) {
	t.Parallel()

	t.Run("valid entry populates domain and valid flag", func(t *testing.T) {
		t.Parallel()
		snap := signable(model.AccountSnapshot{
			ID:   testAccount,
			Data: map[string]string{"config.multisig.coordinator": "djEubXVsdGlzaWcuc2F0b3NoaXBheS5pbw=="},
		})
		facts := Facts(Context{Account: policy.Account{ID: testAccount, Coordinator: strPtr("v1.multisig.satoshipay.io")}, Snapshot: snap})
		if facts.Coordinator == nil || !facts.Coordinator.Valid || facts.Coordinator.DataKey != "config.multisig.coordinator" || facts.Coordinator.Domain != "v1.multisig.satoshipay.io" {
			t.Fatalf("unexpected CoordinatorFact: %+v", facts.Coordinator)
		}
		if facts.Declared == nil || facts.Declared.Coordinator == nil || *facts.Declared.Coordinator != "v1.multisig.satoshipay.io" {
			t.Fatalf("unexpected DeclaredFacts.Coordinator: %+v", facts.Declared)
		}
	})

	t.Run("unreadable hostile value marks Valid false without leaking raw text", func(t *testing.T) {
		t.Parallel()
		hostile := base64.StdEncoding.EncodeToString([]byte("\x1b[31m\u202eevil.invalid"))
		snap := signable(model.AccountSnapshot{
			ID:   testAccount,
			Data: map[string]string{"custom_multisig_coordinator": hostile},
		})
		facts := Facts(Context{Account: policy.Account{ID: testAccount}, Snapshot: snap})
		if facts.Coordinator == nil || facts.Coordinator.Valid || facts.Coordinator.DataKey != "custom_multisig_coordinator" || facts.Coordinator.Domain != "" {
			t.Fatalf("unexpected invalid CoordinatorFact: %+v", facts.Coordinator)
		}
	})
}

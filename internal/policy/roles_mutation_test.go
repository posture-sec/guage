package policy

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// roleFloors returns a fresh map per call, and this is the test that keeps it that
// way. Issue #144 replaced a package-level severity map with the function because
// a caller could reach the catalog through SeverityFor and rewrite it for every
// later run in the same process. The roles and rules come from the catalog helpers
// rather than a second hand-written list, so a new row is covered the day it is
// added.
func TestRoleFloorsAreMutationIsolated(t *testing.T) {
	t.Parallel()
	for _, role := range catalogRoles() {
		floors := roleFloors(role)
		before := make(map[string]model.Severity, len(floors))
		for rule, floor := range floors {
			before[rule] = floor
			// A caller that mutates its own copy must not reach the catalog.
			floors[rule] = model.SevHigh
		}
		floors["sig-forged"] = model.SevHigh

		again := roleFloors(role)
		if len(again) != len(before) {
			t.Errorf("role %s: the next call returned %d rows, want %d", role, len(again), len(before))
		}
		for rule, expected := range before {
			if got := again[rule]; got != expected {
				t.Errorf("role %s, rule %s: the next call returned %s after a caller mutated a row, want %s",
					role, rule, got, expected)
			}
		}
		if _, forged := again["sig-forged"]; forged {
			t.Errorf("role %s: a key added by a caller came back from roleFloors", role)
		}
	}
}

// A second pass reads the same catalog as the first. That is what makes the
// severity a report prints independent of how many findings were judged before it,
// which is the property the mutation test above is protecting.
func TestRoleFloorsAgreeAcrossRepeatedPasses(t *testing.T) {
	t.Parallel()
	for _, role := range catalogRoles() {
		for _, rule := range catalogRules() {
			first := SeverityFor(role, rule, model.SevInfo)
			floors := roleFloors(role)
			for key := range floors {
				floors[key] = model.SevInfo
			}
			if got := SeverityFor(role, rule, model.SevInfo); got != first {
				t.Errorf("role %s, rule %s: a second pass read %s after a caller mutated an earlier row, want %s",
					role, rule, got, first)
			}
		}
	}
}

// An unknown role has no row at all. An empty map is what SeverityFor reads as
// "raise nothing", and it must stay empty however a caller treats it.
func TestRoleFloorsForUnknownRoleAreEmptyAndIsolated(t *testing.T) {
	t.Parallel()
	for _, role := range []model.Role{"", "custodian"} {
		floors := roleFloors(role)
		if len(floors) != 0 {
			t.Errorf("role %q: got %d rows, want none", role, len(floors))
		}
		floors["flag-drift"] = model.SevHigh
		if got := roleFloors(role); len(got) != 0 {
			t.Errorf("role %q: a caller's write made the next call return %d rows", role, len(got))
		}
	}
}

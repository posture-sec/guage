package model_test

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// jsonKeys marshals v and returns the sorted keys of the object it produced.
func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %T: %v", v, err)
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// The clawback bit is one concept, so a report spells it one way. The snapshot's Flags
// and the declaration's DeclaredFlags both render in one account object, and a consumer
// should not need a special case for whichever block it is reading.
func TestDeclaredFlagKeys(t *testing.T) {
	t.Parallel()
	want := []string{"auth_immutable", "auth_required", "auth_revocable", "clawback_enabled"}

	yes := true
	for _, tt := range []struct {
		name  string
		flags model.DeclaredFlags
	}{
		{"nothing declared", model.DeclaredFlags{}},
		{"everything declared", model.DeclaredFlags{AuthRequired: &yes, AuthRevocable: &yes, AuthImmutable: &yes, ClawbackEnabled: &yes}},
		{"only clawback declared", model.DeclaredFlags{ClawbackEnabled: &yes}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// An omitted flag is null, not absent, so the key set does not depend on the values.
			got := jsonKeys(t, tt.flags)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("declared flag keys = %v, want %v", got, want)
			}
			for _, key := range got {
				if key == "clawback" {
					t.Fatal(`the key "clawback" is back; the declared block must say clawback_enabled`)
				}
			}
		})
	}

	t.Run("snapshot and declaration agree", func(t *testing.T) {
		t.Parallel()
		observed := jsonKeys(t, model.Flags{})
		declared := jsonKeys(t, model.DeclaredFlags{})
		if !reflect.DeepEqual(observed, declared) {
			t.Fatalf("Flags keys %v differ from DeclaredFlags keys %v", observed, declared)
		}
	})
}

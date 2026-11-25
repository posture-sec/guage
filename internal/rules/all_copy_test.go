package rules

import (
	"reflect"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// documentedRuleIDs is the shipped evaluation order. It is repeated here rather
// than read from All() so a reordered registry fails instead of defining its own
// answer. TestAllListsEveryRuleInOrder holds a second copy for the same reason.
func documentedRuleIDs() []string {
	return []string{
		"acct-missing",
		"acct-network-mismatch",
		"sig-no-signing-weight",
		"sig-threshold-unreachable",
		"sig-threshold-order",
		"sig-single-key-high",
		"sig-hashx",
		"sig-unexpected-signer",
		"sig-missing-expected",
		"sig-weight-drift",
		"thr-drift",
		"flag-drift",
		"flag-undeclared-clawback",
		"id-no-home-domain",
		"id-toml-unreachable",
		"id-toml-account-unlisted",
		"id-sep29-memo-required",
	}
}

func checkRuleIDs(t *testing.T, when string, got []Rule) {
	t.Helper()
	want := documentedRuleIDs()
	if len(got) != len(want) {
		t.Fatalf("%s: All() has %d rules, want %d", when, len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("%s: rule %d is %q, want %q", when, i, got[i].ID, id)
		}
	}
}

// TestAllReturnsIndependentCopy pins the promise All() makes in its comment: a
// caller gets a slice it cannot use to edit the registry. The registry is both
// the evaluation order and the ID list a fingerprint hashes, so a write through
// the returned slice would change every later run in the process.
//
// TestAllReturnsACopy already covers element assignment. The cases here are the
// ones that survive an element copy: two entries swapped, the slice truncated,
// and an append that writes into whatever capacity the returned slice carries.
// A hybrid that copied the slice header but shared the array would pass the
// older test and fail these.
func TestAllReturnsIndependentCopy(t *testing.T) {
	t.Parallel()

	checkRuleIDs(t, "before any edit", All())

	t.Run("reordering", func(t *testing.T) {
		t.Parallel()
		first := All()
		first[0], first[1] = first[1], first[0]
		checkRuleIDs(t, "after swapping two entries", All())
	})

	t.Run("truncation", func(t *testing.T) {
		t.Parallel()
		if short := All()[:2]; len(short) == 0 {
			t.Fatal("truncated slice is empty, so the case proves nothing")
		}
		checkRuleIDs(t, "after truncating the returned slice", All())
	})

	t.Run("append through spare capacity", func(t *testing.T) {
		t.Parallel()
		extended := append(All(), Rule{
			ID:    "synthetic-rule",
			Check: func(Context) []model.Finding { return nil },
		})
		extended[len(extended)-1].ID = "written-through-append"
		checkRuleIDs(t, "after appending a synthetic rule and writing to it", All())
	})

	t.Run("check identity is stable", func(t *testing.T) {
		t.Parallel()
		first, second := All(), All()
		for i := range first {
			got := reflect.ValueOf(second[i].Check).Pointer()
			want := reflect.ValueOf(first[i].Check).Pointer()
			if got != want {
				t.Errorf("rule %q has a different Check function on the second call: two different copies of the registry are in play", first[i].ID)
			}
		}
	})
}

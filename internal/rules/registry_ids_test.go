package rules

import (
	"regexp"
	"strings"
	"testing"
)

// ruleIDFamilies is the closed prefix set CONTRIBUTING.md fixes for rule IDs.
// Building the pattern from this table keeps a new family to one line here.
var ruleIDFamilies = []string{"acct", "sig", "thr", "flag", "id", "src"}

// TestRegistryIDs pins the shape of every shipped rule ID, not just the list
// TestAllListsEveryRuleInOrder already covers. A malformed ID would key
// fingerprints and [[accept]] entries that no policy can name.
func TestRegistryIDs(t *testing.T) {
	t.Parallel()

	const wantCount = 17
	pattern := regexp.MustCompile(`^(` + strings.Join(ruleIDFamilies, "|") + `)-[a-z0-9]+(-[a-z0-9]+)*$`)

	all := All()
	if len(all) != wantCount {
		t.Fatalf("registry has %d rules, want %d", len(all), wantCount)
	}

	seen := make(map[string]int, len(all))
	for i, rule := range all {
		if first, ok := seen[rule.ID]; ok {
			t.Errorf("rule %d repeats ID %q, first seen at rule %d", i, rule.ID, first)
			continue
		}
		seen[rule.ID] = i
		if !pattern.MatchString(rule.ID) {
			t.Errorf("rule %d ID %q does not match %s", i, rule.ID, pattern)
		}
	}
}

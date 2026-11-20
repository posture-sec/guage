package policy

import (
	"errors"
	"strings"
	"testing"
)

// TestUnknownKeysAreAllReported pins that one load names every key the schema
// does not define, so an operator with three misspellings fixes them in one
// edit cycle instead of three. The keys are written out of order and one sits
// under [[account]], so the test fails if only the first undecoded key is
// rendered, if the list is left in file order, or if a nested key loses its
// table prefix.
func TestUnknownKeysAreAllReported(t *testing.T) {
	t.Parallel()
	// Each unknown key carries a distinctive value, so the test can prove the
	// detail names paths and never echoes what the file assigned to them.
	const (
		valueOne   = "value-never-echoed-one"
		valueTwo   = "value-never-echoed-two"
		valueThree = "value-never-echoed-three"
	)
	cases := []struct {
		name       string
		body       string
		wantDetail string
		values     []string
	}{
		{
			name: "two top-level keys and one under account",
			body: "version = 1\n" +
				"network = \"testnet\"\n" +
				"zendpoints = [\"" + valueOne + "\"]\n" +
				"expectedflags = \"" + valueTwo + "\"\n" +
				"[[account]]\n" +
				"id = \"" + accountOne + "\"\n" +
				"owners = [\"" + valueThree + "\"]\n",
			wantDetail: "account.owners, expectedflags, zendpoints",
			values:     []string{valueOne, valueTwo, valueThree},
		},
		{
			name:       "a single unknown key keeps the old detail shape",
			body:       "version = 1\nnetwork = \"testnet\"\naccounts = [\"" + valueOne + "\"]\n",
			wantDetail: "accounts",
			values:     []string{valueOne},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(strings.NewReader(tc.body))
			if !errors.Is(err, ErrUnknownKey) {
				t.Fatalf("Load error = %v, want ErrUnknownKey", err)
			}
			var perr *Error
			if !errors.As(err, &perr) {
				t.Fatalf("Load error %T is not a *policy.Error", err)
			}
			if perr.Kind != ErrUnknownKey.Kind || perr.Message != ErrUnknownKey.Message {
				t.Errorf("kind and message = (%q, %q), want (%q, %q)", perr.Kind, perr.Message, ErrUnknownKey.Kind, ErrUnknownKey.Message)
			}
			if perr.Detail != tc.wantDetail {
				t.Errorf("Detail = %q, want %q", perr.Detail, tc.wantDetail)
			}
			for _, value := range tc.values {
				if strings.Contains(err.Error(), value) {
					t.Errorf("error %q echoes a value read from the file", err.Error())
				}
			}
		})
	}
}

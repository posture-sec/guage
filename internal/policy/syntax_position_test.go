package policy

import (
	"errors"
	"strings"
	"testing"
)

func TestLoadSyntaxErrorKeepsParsePosition(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name   string
		input  string
		detail string
		absent string
	}{
		{
			name:   "malformed header",
			input:  "version = 1\nnetwork = \"testnet\"\n[account\n",
			detail: "line 4, column 9",
			absent: "last key",
		},
		{
			name:   "malformed value with a last key",
			input:  "version = 1\nnetwork = \"testnet\"\nname = \"unterminated\n",
			detail: "line 3, column 21, last key \"name\"",
			absent: "unterminated",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Load(strings.NewReader(tt.input))
			if !errors.Is(err, ErrSyntax) {
				t.Fatalf("error = %v, want ErrSyntax", err)
			}
			var policyErr *Error
			if !errors.As(err, &policyErr) {
				t.Fatalf("error %v is not a policy.Error", err)
			}
			if policyErr.Detail == "" {
				t.Fatal("syntax error carries no parse position")
			}
			if !strings.Contains(policyErr.Detail, tt.detail) {
				t.Fatalf("detail = %q, want it to contain %q", policyErr.Detail, tt.detail)
			}
			if strings.Contains(strings.ToLower(policyErr.Detail), tt.absent) {
				t.Fatalf("detail = %q, which contains %q", policyErr.Detail, tt.absent)
			}
		})
	}
}

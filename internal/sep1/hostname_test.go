package sep1

import (
	"errors"
	"strings"
	"testing"
)

func TestHostnameRefusalClasses(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		in      string
		wantErr error
	}{
		{
			name:    "single label",
			in:      "localhost",
			wantErr: errHostLabel,
		},
		{
			name:    "empty label",
			in:      "example..com",
			wantErr: errHostLabel,
		},
		{
			name:    "64-character label",
			in:      strings.Repeat("a", 64) + ".example.com",
			wantErr: errHostLabel,
		},
		{
			name:    "trailing hyphen",
			in:      "example-.com",
			wantErr: errHostLabel,
		},
		{
			name:    "leading hyphen",
			in:      "-example.com",
			wantErr: errHostLabel,
		},
		{
			name:    "port",
			in:      "example.com:8443",
			wantErr: errHostChars,
		},
		{
			name:    "userinfo",
			in:      "user:pass@example.com",
			wantErr: errHostChars,
		},
		{
			name:    "path",
			in:      "example.com/stellar.toml",
			wantErr: errHostChars,
		},
		{
			name:    "ipv4 literal",
			in:      "192.0.2.1",
			wantErr: errHostIP,
		},
		{
			name:    "bracketed ipv6 literal",
			in:      "[2001:db8::1]",
			wantErr: errHostChars,
		},
		{
			name:    "254-character name",
			in:      longHostname(254),
			wantErr: errHostTooLong,
		},
		{
			name:    "empty name",
			in:      "",
			wantErr: errHostEmpty,
		},
		{
			name:    "non ascii",
			in:      "exämple.com",
			wantErr: errHostNotASCII,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidateHostname(tc.in)
			if err == nil {
				t.Fatalf("ValidateHostname(%q) = %q, want err", tc.in, got)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("ValidateHostname(%q) err = %v, want %v", tc.in, err, tc.wantErr)
			}

			errMsg := err.Error()

			// The error must never echo the full input at any length
			if tc.in != "" && strings.Contains(errMsg, tc.in) {
				t.Fatalf("ValidateHostname(%q) echoed input in error %q", tc.in, errMsg)
			}

			// The error must never echo any label from the input
			for _, label := range strings.Split(tc.in, ".") {
				cleanLabel := strings.Trim(label, "[]:/@ \t\n")
				if len(cleanLabel) > 1 && strings.Contains(errMsg, cleanLabel) {
					t.Fatalf("ValidateHostname(%q) echoed label %q in error %q", tc.in, cleanLabel, errMsg)
				}
			}
		})
	}
}

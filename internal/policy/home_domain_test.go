package policy

import (
	"errors"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/sep1"
)

func TestValidHomeDomain(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		domain string
		want   bool
	}{
		{"valid example.com", "example.com", true},
		{"valid a-b.example.co.uk", "a-b.example.co.uk", true},
		{"valid multi label", "sub.domain.example.com", true},
		{"valid with digits", "example123.com", true},
		{"valid single char labels", "a.b", true},
		{"empty", "", false},
		{"trailing dot", "example.com.", false},
		{"over 253 chars", strings.Repeat("a.", 126) + "com", false},
		{"asterisk label", "*.example.com", false},
		{"underscore label", "exa_mple.com", false},
		{"localhost single label", "localhost", false},
		{"double dot", "example..com", false},
		{"leading hyphen", "-bad.example.com", false},
		{"trailing hyphen", "bad-.example.com", false},
		{"label too long", strings.Repeat("a", 64) + ".com", false},
		{"space", "ex ample.com", false},
		{"tab", "ex\tample.com", false},
		{"slash", "ex/ample.com", false},
		{"at", "ex@ample.com", false},
		{"colon", "ex:ample.com", false},
		{"question", "ex?ample.com", false},
		{"hash", "ex#ample.com", false},
		{"bracket open", "ex[ample.com", false},
		{"bracket close", "ex]ample.com", false},
		{"IP literal v4", "192.0.2.1", false},
		{"IP literal v6", "2001:db8::1", false},
		{"non-ASCII", "examplé.com", false},
		{"uppercase", "EXAMPLE.COM", true},
		{"mixed case", "Example.Com", true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := validHomeDomain(tt.domain)
			if got != tt.want {
				t.Fatalf("validHomeDomain(%q) = %v, want %v", tt.domain, got, tt.want)
			}
			_, err := Load(strings.NewReader("version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ\"\nhome_domain = \"" + tt.domain + "\""))
			if tt.want {
				if err != nil {
					t.Fatalf("Load(%q) = %v, want success", tt.domain, err)
				}
			} else {
				if !errors.Is(err, ErrHomeDomain) && !errors.Is(err, ErrSyntax) {
					t.Fatalf("Load(%q) error = %v, want %v or %v", tt.domain, err, ErrHomeDomain, ErrSyntax)
				}
			}
		})
	}
}

func TestValidHomeDomainAgreesWithSEP1(t *testing.T) {
	t.Parallel()
	corpus := []string{
		"example.com",
		"a-b.example.co.uk",
		"sub.domain.example.com",
		"example123.com",
		"a.b",
		"EXAMPLE.COM",
		"Example.Com",
		"",
		"example.com.",
		strings.Repeat("a.", 126) + "com",
		"*.example.com",
		"exa_mple.com",
		"localhost",
		"example..com",
		"-bad.example.com",
		"bad-.example.com",
		strings.Repeat("a", 64) + ".com",
		"ex ample.com",
		"ex\tample.com",
		"ex/ample.com",
		"ex@ample.com",
		"ex:ample.com",
		"ex?ample.com",
		"ex#ample.com",
		"ex[ample.com",
		"ex]ample.com",
		"192.0.2.1",
		"2001:db8::1",
		"examplé.com",
	}
	for _, domain := range corpus {
		domain := domain
		t.Run(domain, func(t *testing.T) {
			t.Parallel()
			policyResult := validHomeDomain(domain)
			_, sep1Err := sep1.ValidateHostname(domain)
			sep1Result := sep1Err == nil
			if policyResult != sep1Result {
				t.Fatalf("validHomeDomain(%q) = %v, sep1.ValidateHostname = %v, want agreement", domain, policyResult, sep1Result)
			}
		})
	}
}

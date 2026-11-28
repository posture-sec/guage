package sanitize

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   string
		wantOK bool
	}{
		{name: "standard domain", in: "v1.multisig.satoshipay.io", want: "v1.multisig.satoshipay.io", wantOK: true},
		{name: "uppercase and trailing dot", in: "V1.Multisig.SatoshiPay.IO.", want: "v1.multisig.satoshipay.io", wantOK: true},
		{name: "hyphenated labels", in: "coord-1.sub-2.example.org", want: "coord-1.sub-2.example.org", wantOK: true},
		{name: "empty rejected", in: "", wantOK: false},
		{name: "single label rejected", in: "localhost", wantOK: false},
		{name: "double trailing dot rejected", in: "example.com..", wantOK: false},
		{name: "empty label rejected", in: "example..com", wantOK: false},
		{name: "leading hyphen rejected", in: "-bad.example.com", wantOK: false},
		{name: "trailing hyphen rejected", in: "bad-.example.com", wantOK: false},
		{name: "ipv4 rejected", in: "127.0.0.1", wantOK: false},
		{name: "ipv6 rejected", in: "::1", wantOK: false},
		{name: "scheme rejected in bare normalizer", in: "https://example.com", wantOK: false},
		{name: "port rejected", in: "example.com:8443", wantOK: false},
		{name: "whitespace rejected", in: " example.com", wantOK: false},
		{name: "non-ascii rejected", in: "ex\u00e4mple.com", wantOK: false},
		{name: "underscore rejected", in: "bad_host.example.com", wantOK: false},
		{name: "label over 63 chars rejected", in: strings.Repeat("a", 64) + ".com", wantOK: false},
		{name: "max length 253 accepted", in: strings.Repeat("a", 61) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 63), want: strings.Repeat("a", 61) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 63), wantOK: true},
		{name: "domain over 253 rejected", in: strings.Repeat("a", 62) + "." + strings.Repeat("b", 63) + "." + strings.Repeat("c", 63) + "." + strings.Repeat("d", 63), wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := NormalizeDomain(tc.in)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("NormalizeDomain(%q) = (%q, %v), want (%q, %v)", tc.in, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestDecodeB64Domain(t *testing.T) {
	b64 := func(s string) string {
		return base64.StdEncoding.EncodeToString([]byte(s))
	}

	tests := []struct {
		name   string
		raw    string
		want   string
		wantOK bool
	}{
		{
			name:   "live corpus satoshipay entry",
			raw:    "djEubXVsdGlzaWcuc2F0b3NoaXBheS5pbw==",
			want:   "v1.multisig.satoshipay.io",
			wantOK: true,
		},
		{
			name:   "https url with path unwrapped",
			raw:    b64("https://V1.Multisig.SatoshiPay.IO/api/v1"),
			want:   "v1.multisig.satoshipay.io",
			wantOK: true,
		},
		{
			name:   "http url with trailing slash unwrapped",
			raw:    b64("http://coord.example.com/"),
			want:   "coord.example.com",
			wantOK: true,
		},
		{
			name:   "trimmed surrounding newline",
			raw:    b64("  coord.example.com.\n"),
			want:   "coord.example.com",
			wantOK: true,
		},
		{name: "empty raw rejected", raw: "", wantOK: false},
		{name: "raw over 128 rejected", raw: strings.Repeat("A", 132), wantOK: false},
		{name: "raw containing newline rejected", raw: "djE=\n", wantOK: false},
		{name: "invalid base64 rejected", raw: "not!base64", wantOK: false},
		{name: "empty decoded rejected", raw: b64("   "), wantOK: false},
		{name: "invalid utf8 rejected", raw: base64.StdEncoding.EncodeToString([]byte{0xff, 0xfe}), wantOK: false},
		{name: "control char inside decoded rejected", raw: b64("coord\x1b[31m.example.com"), wantOK: false},
		{name: "bidi override inside decoded rejected", raw: b64("coord\u202e.example.com"), wantOK: false},
		{name: "userinfo @ rejected", raw: b64("https://expected.example@evil.example"), wantOK: false},
		{name: "query string rejected", raw: b64("https://example.com/?next=evil"), wantOK: false},
		{name: "fragment rejected", raw: b64("https://example.com/#frag"), wantOK: false},
		{name: "backslash rejected", raw: b64(`https://example.com\evil.com`), wantOK: false},
		{name: "path without scheme rejected", raw: b64("example.com/path"), wantOK: false},
		{name: "non-http scheme rejected", raw: b64("ftp://example.com"), wantOK: false},
		{name: "port in url rejected", raw: b64("https://example.com:8443/v1"), wantOK: false},
		{name: "ip literal in url rejected", raw: b64("https://169.254.169.254/latest"), wantOK: false},
		{name: "single label rejected", raw: b64("https://localhost/"), wantOK: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := DecodeB64Domain(tc.raw)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("DecodeB64Domain(%q) = (%q, %v), want (%q, %v)", tc.raw, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

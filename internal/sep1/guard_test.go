package sep1

import (
	"net"
	"strings"
	"testing"
)

func TestValidateHostnameAccepts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "example.com", "example.com"},
		{"subdomain", "a.b.c.example.com", "a.b.c.example.com"},
		{"case folded", "Example.COM", "example.com"},
		{"hyphen inside label", "a-b.example.com", "a-b.example.com"},
		{"digits", "1.example.com", "1.example.com"},
		{"longest label", strings.Repeat("a", 63) + ".example.com", strings.Repeat("a", 63) + ".example.com"},
		{"punycode stays ascii", "xn--80ak6aa92e.com", "xn--80ak6aa92e.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidateHostname(tc.in)
			if err != nil {
				t.Fatalf("ValidateHostname(%q) = %v, want accepted", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("ValidateHostname(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestValidateHostnameRejects(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"space only", " "},
		{"single label", "localhost"},
		{"single label example", "test"},
		{"ipv4 literal", "192.0.2.1"},
		{"ipv6 literal", "2001:db8::1"},
		{"ipv6 bracketed", "[2001:db8::1]"},
		{"port", "example.com:8443"},
		{"userinfo", "user:pass@example.com"},
		{"at sign", "@example.com"},
		{"path", "example.com/stellar.toml"},
		{"query", "example.com?x=1"},
		{"fragment", "example.com#x"},
		{"scheme prefix", "https://example.com"},
		{"inner space", "exa mple.com"},
		{"tab", "exa\tmple.com"},
		{"newline", "example.com\n"},
		{"empty label", "example..com"},
		{"leading dot", ".example.com"},
		{"trailing dot", "example.com."},
		{"label too long", strings.Repeat("a", 64) + ".example.com"},
		{"total too long", longHostname(254)},
		{"hyphen lead label", "-example.com"},
		{"hyphen tail label", "example-.com"},
		{"underscore", "_dmarc.example.com"},
		{"percent", "ex%00ample.com"},
		{"non ascii", "exämple.com"},
		{"zero width", "exam\u200bple.com"},
		{"bidi override", "example\u202e.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ValidateHostname(tc.in)
			if err == nil {
				t.Fatalf("ValidateHostname(%q) = %q, want rejected", tc.in, got)
			}
			if strings.Contains(err.Error(), tc.in) && len(tc.in) > 12 {
				t.Fatalf("ValidateHostname(%q) echoed the value in the error: %v", tc.in, err)
			}
		})
	}
}

func TestHomeDomainTrimsTheSpaceOnlyCase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw    string
		domain string
		wanted bool
		reason string
	}{
		{"example.com", "example.com", true, "plain"},
		{" example.com ", "example.com", true, "padded"},
		{"\tEXAMPLE.COM\n", "example.com", true, "padded and folded"},
		{"", "", false, "empty"},
		{" ", "", false, "single space"},
		{"   ", "", false, "several spaces"},
		{"192.0.2.1", "", false, "ip literal"},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			t.Parallel()
			got, ok := HomeDomain(tc.raw)
			if ok != tc.wanted || got != tc.domain {
				t.Fatalf("HomeDomain(%q) = (%q, %t), want (%q, %t)", tc.raw, got, ok, tc.domain, tc.wanted)
			}
		})
	}
}

func TestClassifyIP(t *testing.T) {
	t.Parallel()
	cases := []struct {
		ip   string
		want Class
		dial bool
	}{
		// A public answer is the only class Gauge will connect to.
		{"93.184.216.34", ClassPublic, true},
		{"203.0.113.7", ClassPublic, true},
		{"8.8.8.8", ClassPublic, true},
		{"2606:4700:4700::1111", ClassPublic, true},
		// Loopback, in both families and smuggled inside an IPv6 mapping.
		{"127.0.0.1", ClassLoopback, false},
		{"127.255.255.255", ClassLoopback, false},
		{"::1", ClassLoopback, false},
		{"::ffff:127.0.0.1", ClassLoopback, false},
		// RFC1918 private space, in both families and via NAT64 and 6to4.
		{"10.0.0.1", ClassPrivate, false},
		{"172.16.0.1", ClassPrivate, false},
		{"172.31.255.255", ClassPrivate, false},
		{"192.168.1.1", ClassPrivate, false},
		{"::ffff:192.168.1.1", ClassPrivate, false},
		{"64:ff9b::a00:1", ClassPrivate, false},
		{"2002:0a00:0001::1", ClassPrivate, false},
		// Link-local, including the cloud metadata address.
		{"169.254.169.254", ClassLinkLocal, false},
		{"169.254.0.1", ClassLinkLocal, false},
		{"fe80::1", ClassLinkLocal, false},
		{"febf::1", ClassLinkLocal, false},
		// IPv6 unique-local.
		{"fc00::1", ClassUniqueLocal, false},
		{"fd12:3456::1", ClassUniqueLocal, false},
		// Carrier-grade NAT.
		{"100.64.0.1", ClassCarrierNAT, false},
		{"100.127.255.255", ClassCarrierNAT, false},
		// Addresses that are never a destination.
		{"0.0.0.0", ClassUnspecified, false},
		{"0.1.2.3", ClassUnspecified, false},
		{"::", ClassUnspecified, false},
		{"224.0.0.1", ClassMulticast, false},
		{"ff02::1", ClassMulticast, false},
		{"255.255.255.255", ClassReserved, false},
		{"240.0.0.1", ClassReserved, false},
		{"192.0.0.1", ClassReserved, false},
		{"198.18.0.1", ClassReserved, false},
		{"100::1", ClassReserved, false},
	}
	for _, tc := range cases {
		t.Run(tc.ip, func(t *testing.T) {
			t.Parallel()
			got := ClassifyIP(net.ParseIP(tc.ip))
			if got != tc.want {
				t.Fatalf("ClassifyIP(%s) = %v, want %v", tc.ip, got, tc.want)
			}
			if got.Dialable() != tc.dial {
				t.Fatalf("ClassifyIP(%s).Dialable() = %t, want %t", tc.ip, got.Dialable(), tc.dial)
			}
		})
	}
}

func TestClassifyIPRejectsAnUnparsableAddress(t *testing.T) {
	t.Parallel()
	for _, ip := range []net.IP{nil, {}, {1, 2}, {1, 2, 3, 4, 5}} {
		if got := ClassifyIP(ip); got.Dialable() {
			t.Fatalf("ClassifyIP(%v) = %v, want a class that is not dialable", ip, got)
		}
	}
}

func longHostname(total int) string {
	var labels []string
	for n := 0; n < total; n += 8 {
		labels = append(labels, "aaaaaaa")
	}
	host := strings.Join(labels, ".")
	for len(host) < total {
		host += "a"
	}
	return host
}

package sep1

import (
	"errors"
	"net"
	"strings"
)

// Class names the address range a resolved host answer belongs to. Gauge refuses
// everything that is not ClassPublic, because the home domain it fetches comes
// off a hostile chain: an account can point its own record at a host that
// resolves to a metadata service or a private subnet.
type Class uint8

const (
	// ClassPublic is a globally routable unicast address: the only dialable one.
	ClassPublic Class = iota
	ClassLoopback
	ClassPrivate
	ClassLinkLocal
	ClassUniqueLocal
	ClassCarrierNAT
	ClassUnspecified
	ClassMulticast
	ClassReserved
)

func (c Class) String() string {
	switch c {
	case ClassPublic:
		return "public"
	case ClassLoopback:
		return "loopback"
	case ClassPrivate:
		return "private"
	case ClassLinkLocal:
		return "link-local"
	case ClassUniqueLocal:
		return "unique-local"
	case ClassCarrierNAT:
		return "carrier-grade NAT"
	case ClassUnspecified:
		return "unspecified"
	case ClassMulticast:
		return "multicast"
	default:
		return "reserved"
	}
}

// Dialable reports whether Gauge may open a socket to an address of this class.
func (c Class) Dialable() bool { return c == ClassPublic }

const maxHostnameLen = 253

// These errors name why a host was refused and never carry the value itself: one
// of the strings that reaches them is chain-derived and hostile.
var (
	errHostEmpty    = errors.New("sep1: the home domain is empty")
	errHostTooLong  = errors.New("sep1: the home domain is longer than 253 characters")
	errHostNotASCII = errors.New("sep1: the home domain is not ASCII, and no IDNA mapping is available in the standard library")
	errHostChars    = errors.New("sep1: the home domain is a bare host name, not a URL with a scheme, port, path or credentials")
	errHostIP       = errors.New("sep1: the home domain is an IP address literal")
	errHostLabel    = errors.New("sep1: the home domain has an empty label, an over-long label, a leading or trailing hyphen, or fewer than two labels")
)

// ValidateHostname applies the strict host name check from ARCHITECTURE.md 6.5:
// ASCII letters, digits, hyphens and dots only, two or more labels of 1 to 63
// characters, no port, no userinfo, no path and no IP literal. The result is
// case-folded.
//
// It is deliberately stricter than policy.validHomeDomain, which accepts
// non-ASCII: those hosts are refused here rather than fetched, because the
// standard library has no IDNA mapping and golang.org/x/net/idna would be a new
// dependency. The two checks are kept apart on purpose; the config side decides
// what a maintainer may write, this side decides what Gauge may dial.
func ValidateHostname(host string) (string, error) {
	if host == "" {
		return "", errHostEmpty
	}
	if len(host) > maxHostnameLen {
		return "", errHostTooLong
	}
	for _, r := range host {
		if r > 0x7f {
			return "", errHostNotASCII
		}
	}
	if strings.IndexFunc(host, func(r rune) bool { return r <= 0x20 || r == 0x7f }) >= 0 {
		return "", errHostChars
	}
	if net.ParseIP(host) != nil {
		return "", errHostIP
	}
	for _, r := range host {
		if !validHostRune(r) {
			return "", errHostChars
		}
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return "", errHostLabel
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", errHostLabel
		}
	}
	return strings.ToLower(host), nil
}

func validHostRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == '-' || r == '.':
		return true
	default:
		return false
	}
}

// HomeDomain turns the home_domain field of an account record into a host to
// fetch, or reports that there is nothing to fetch. Surrounding whitespace is
// trimmed first: a real captured account carries a single space in this field,
// which is an absent domain rather than a hostile one.
func HomeDomain(raw string) (string, bool) {
	host, err := ValidateHostname(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	return host, true
}

// ClassifyIP names the range an address belongs to. The table is written out
// rather than derived from ip.IsGlobalUnicast, which accepts 100.64.0.0/10 and
// RFC4193 unique-local and so would let a carrier-grade NAT or an internal IPv6
// address through. An address that cannot be classified fails closed as
// ClassReserved.
func ClassifyIP(ip net.IP) Class {
	if len(ip) != 4 && len(ip) != 16 {
		return ClassReserved
	}
	if v4 := ip.To4(); v4 != nil {
		return classifyV4(v4)
	}
	// 64:ff9b::/96 is the well-known NAT64 prefix: its last four bytes are an
	// IPv4 address, which must be classified as the IPv4 it really reaches.
	if len(ip) == 16 && ip[0] == 0x00 && ip[1] == 0x64 && ip[2] == 0xff && ip[3] == 0x9b && allZero(ip[4:12]) {
		return classifyV4(ip[12:16])
	}
	// 2002::/16 is 6to4, which embeds an IPv4 address in bytes two through five.
	if len(ip) == 16 && ip[0] == 0x20 && ip[1] == 0x02 {
		return classifyV4(ip[2:6])
	}
	return classifyV6(ip)
}

func classifyV4(ip net.IP) Class {
	switch {
	case ip[0] == 0:
		return ClassUnspecified
	case ip[0] == 127:
		return ClassLoopback
	case ip[0] == 10, ip[0] == 192 && ip[1] == 168, ip[0] == 172 && ip[1] >= 16 && ip[1] <= 31:
		return ClassPrivate
	case ip[0] == 169 && ip[1] == 254:
		return ClassLinkLocal
	case ip[0] == 100 && ip[1] >= 64 && ip[1] <= 127:
		return ClassCarrierNAT
	case ip[0] >= 224 && ip[0] < 240:
		return ClassMulticast
	case ip[0] >= 240:
		return ClassReserved
	case ip[0] == 192 && ip[1] == 0 && ip[2] == 0:
		return ClassReserved
	case ip[0] == 198 && (ip[1] == 18 || ip[1] == 19):
		return ClassReserved
	default:
		return ClassPublic
	}
}

func classifyV6(ip net.IP) Class {
	switch {
	case allZero(ip):
		return ClassUnspecified
	case allZero(ip[0:15]) && ip[15] == 1:
		return ClassLoopback
	case ip[0]&0xfe == 0xfc:
		return ClassUniqueLocal
	case ip[0] == 0xfe && ip[1]&0xc0 == 0x80:
		return ClassLinkLocal
	case ip[0] == 0xff:
		return ClassMulticast
	case ip[0] == 0x01 && ip[1] == 0x00 && allZero(ip[2:8]):
		return ClassReserved
	default:
		return ClassPublic
	}
}

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

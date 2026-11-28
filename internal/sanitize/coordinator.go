package sanitize

import (
	"encoding/base64"
	"net"
	"strings"
	"unicode/utf8"
)

const (
	maxCoordinatorB64Len     = 128
	maxCoordinatorDecodedLen = 64
	maxCoordinatorDomainLen  = 253
	maxCoordinatorLabelLen   = 63
)

// NormalizeDomain validates and normalizes raw as an ASCII bare hostname
// (at least two labels of 1..63 [a-zA-Z0-9-] runes, not an IP literal).
// A single trailing dot is stripped and the hostname is lowercased.
func NormalizeDomain(raw string) (string, bool) {
	if raw == "" || len(raw) > maxCoordinatorDomainLen {
		return "", false
	}
	host := strings.TrimSuffix(raw, ".")
	if host == "" || len(host) > maxCoordinatorDomainLen {
		return "", false
	}
	if net.ParseIP(host) != nil {
		return "", false
	}
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return "", false
	}
	for _, label := range labels {
		if !validHostLabel(label) {
			return "", false
		}
	}
	return strings.ToLower(host), true
}

// DecodeB64Domain decodes a Horizon base64 data entry value into a validated,
// normalized hostname. Leading http:// or https:// schemes (with an optional
// trailing /path) are unwrapped to the host portion; any control, whitespace,
// userinfo (@), query (?), fragment (#), backslash (\), unprefixed slash, port,
// IP literal, or non-ASCII rune is rejected.
func DecodeB64Domain(raw string) (string, bool) {
	if raw == "" || len(raw) > maxCoordinatorB64Len {
		return "", false
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", false
	}
	if len(decoded) == 0 || len(decoded) > maxCoordinatorDecodedLen || !utf8.Valid(decoded) {
		return "", false
	}
	s := strings.TrimSpace(string(decoded))
	if s == "" {
		return "", false
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b <= 0x20 || b >= 0x7f {
			return "", false
		}
		switch b {
		case '@', '?', '#', '\\':
			return "", false
		}
	}
	lower := strings.ToLower(s)
	hasScheme := false
	switch {
	case strings.HasPrefix(lower, "https://"):
		s = s[len("https://"):]
		hasScheme = true
	case strings.HasPrefix(lower, "http://"):
		s = s[len("http://"):]
		hasScheme = true
	}
	if hasScheme {
		if idx := strings.IndexByte(s, '/'); idx >= 0 {
			s = s[:idx]
		}
	} else if strings.ContainsRune(s, '/') {
		return "", false
	}
	return NormalizeDomain(s)
}

func validHostLabel(label string) bool {
	if len(label) == 0 || len(label) > maxCoordinatorLabelLen {
		return false
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for i := 0; i < len(label); i++ {
		b := label[i]
		isAlphaNum := (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
		if !isAlphaNum && b != '-' {
			return false
		}
	}
	return true
}

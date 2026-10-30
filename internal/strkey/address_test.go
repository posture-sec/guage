package strkey

import (
	"errors"
	"strings"
	"testing"
)

// The vectors below are the valid and invalid strkeys published in SEP-23,
// copied verbatim. They were run against this package on 2026-10-30 for
// task 2.2 and are the contract Classify is held to.
var sep23Valid = []struct {
	s    string
	kind Kind
}{
	{"GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ", AccountID},
	{"MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUQ", MuxedAccount},
	{"MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLK", MuxedAccount},
	{"PA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAQACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB6IBZGM", SignedPayload},
	{"PA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAOQCAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUAAAAFGBU", SignedPayload},
	{"CA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUWDA", Contract},
	{"LA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUPJN", LiquidityPool},
	{"BAAD6DBUX6J22DMZOHIEZTEQ64CVCHEDRKWZONFEUL5Q26QD7R76RGR4TU", ClaimableBalance},
	{"WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAAAWWC", MuxedContract},
	{"WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAACWJY", MuxedContract},
}

var sep23Invalid = []string{
	"GAAAAAAAACGC6",
	"MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUR",
	"GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZA",
	"GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUACUSI",
	"G47QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVP2I",
	"MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAJLKA",
	"MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVAAAAAAAAAAAAAAV75I",
	"M47QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUQ",
	"MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUK===",
	"MA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAACJUO",
	"PA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAQACAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DUPB6IAAAAAAAAPM",
	"PA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAOQCAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4Z2PQ",
	"PA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAOQCAQDAQCQMBYIBEFAWDANBYHRAEISCMKBKFQXDAMRUGY4DXFH6",
	"BAAD6DBUX6J22DMZOHIEZTEQ64CVCHEDRKWZONFEUL5Q26QD7R76RGR4TV",
	"BAAT6DBUX6J22DMZOHIEZTEQ64CVCHEDRKWZONFEUL5Q26QD7R76RGXACA",
	"WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAAAWWD",
	"WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAAAWWCA",
	"WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAAAAIOUI",
	"W47QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAADXHW",
	"WA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJUAAAAAAAAAAAAAWWA",
}

func TestClassifyValidVectors(t *testing.T) {
	t.Parallel()
	for _, tt := range sep23Valid {
		t.Run(tt.s, func(t *testing.T) {
			t.Parallel()
			kind, err := Classify(tt.s)
			if err != nil {
				t.Fatalf("Classify rejected a valid vector: %v", err)
			}
			if kind != tt.kind {
				t.Fatalf("kind = %q, want %q", kind, tt.kind)
			}
		})
	}
}

func TestClassifyRejectsInvalidVectors(t *testing.T) {
	t.Parallel()
	for _, s := range sep23Invalid {
		t.Run(s, func(t *testing.T) {
			t.Parallel()
			if _, err := Classify(s); err == nil {
				t.Fatal("Classify accepted an invalid vector")
			}
		})
	}
}

func TestClassifyRejectsNonCanonicalCase(t *testing.T) {
	t.Parallel()
	if _, err := Classify(strings.ToLower(sep23Valid[0].s)); !errors.Is(err, ErrCanonical) {
		t.Fatalf("error = %v, want %v", err, ErrCanonical)
	}
}

func TestClassifyShapeAndChecksum(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name    string
		version byte
		body    []byte
		wantErr error
	}{
		{"unknown version", 0, make([]byte, 32), ErrVersion},
		{"short account payload", 6, make([]byte, 31), ErrPayload},
		{"long account payload", 6, make([]byte, 33), ErrPayload},
		{"muxed without id", 12, make([]byte, 32), ErrPayload},
		{"claimable balance type 1", 1, append([]byte{1}, make([]byte, 32)...), ErrPayload},
		{"signed payload zero length", 15, signedPayloadBody(0), ErrPayload},
		{"signed payload overlong prefix", 15, signedPayloadBody(33), ErrPayload},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := encodeStrkey(tt.version, tt.body)
			if _, err := Classify(s); !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}

	t.Run("nonzero algorithm specifier", func(t *testing.T) {
		t.Parallel()
		raw := append([]byte{6<<3 | 1}, make([]byte, 32)...)
		if _, err := Classify(encodeRaw(raw)); !errors.Is(err, ErrSpecifier) {
			t.Fatalf("error = %v, want %v", err, ErrSpecifier)
		}
	})

	t.Run("corrupted checksum", func(t *testing.T) {
		t.Parallel()
		s := sep23Valid[0].s
		corrupted := s[:len(s)-1] + "A"
		if corrupted == s {
			corrupted = s[:len(s)-1] + "B"
		}
		if _, err := Classify(corrupted); !errors.Is(err, ErrChecksum) {
			t.Fatalf("error = %v, want %v", err, ErrChecksum)
		}
	})

	t.Run("nonzero trailing bits", func(t *testing.T) {
		t.Parallel()
		// SEP-23's own vector for an M address whose final symbol sets bits
		// that carry no data. The checksum still matches, so only the
		// re-encoding rule catches it.
		if _, err := Classify(sep23Invalid[1]); !errors.Is(err, ErrCanonical) {
			t.Fatalf("error = %v, want %v", err, ErrCanonical)
		}
	})
}

func TestClassifyAcceptsSynthesizedTypes(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		version byte
		kind    Kind
	}{
		{18, Seed},
		{19, PreAuthTx},
		{23, HashX},
	} {
		t.Run(string(tt.kind), func(t *testing.T) {
			t.Parallel()
			kind, err := Classify(encodeStrkey(tt.version, make([]byte, 32)))
			if err != nil {
				t.Fatalf("synthesized vector rejected: %v", err)
			}
			if kind != tt.kind {
				t.Fatalf("kind = %q, want %q", kind, tt.kind)
			}
		})
	}
}

func TestIsAccountID(t *testing.T) {
	t.Parallel()
	if !IsAccountID(sep23Valid[0].s) {
		t.Fatal("valid account address rejected")
	}
	if IsAccountID(strings.ToLower(sep23Valid[0].s)) {
		t.Fatal("lowercase address accepted as an account ID, which Horizon would reject")
	}
	for _, v := range sep23Valid[1:] {
		if IsAccountID(v.s) {
			t.Fatalf("%q accepted as an account ID", string(v.kind))
		}
	}
	if IsAccountID(sep23Invalid[1]) {
		t.Fatal("invalid strkey accepted as an account ID")
	}
}

func TestIsSeed(t *testing.T) {
	t.Parallel()
	seed := encodeStrkey(18, make([]byte, 32))
	if !IsSeed(seed) {
		t.Fatal("synthesized seed not detected")
	}
	if IsSeed(sep23Valid[0].s) {
		t.Fatal("account address reported as a seed")
	}
	if IsSeed(sep23Valid[8].s) {
		t.Fatal("muxed contract reported as a seed")
	}
	if IsSeed("S") || IsSeed("") {
		t.Fatal("non-strkey reported as a seed")
	}
}

func TestErrorsNeverEchoTheValue(t *testing.T) {
	t.Parallel()
	seed := encodeStrkey(18, make([]byte, 32))
	corrupted := seed[:len(seed)-1] + "A"
	if corrupted == seed {
		corrupted = seed[:len(seed)-1] + "B"
	}
	_, err := Classify(corrupted)
	if err == nil {
		t.Fatal("corrupted seed accepted")
	}
	if strings.Contains(err.Error(), corrupted) {
		t.Fatal("error message echoed the input")
	}
}

// signedPayloadBody builds a P-type payload with an explicit length prefix.
func signedPayloadBody(prefix int) []byte {
	body := make([]byte, 36)
	body[32] = byte(prefix >> 24)
	body[33] = byte(prefix >> 16)
	body[34] = byte(prefix >> 8)
	body[35] = byte(prefix)
	return append(body, make([]byte, 32)...)
}

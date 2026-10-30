package strkey

import (
	"encoding/base32"
	"errors"
	"strings"
)

// Kind is the SEP-23 type of a strkey, written as its letter prefix.
type Kind string

const (
	AccountID        Kind = "G"
	MuxedAccount     Kind = "M"
	Seed             Kind = "S"
	PreAuthTx        Kind = "T"
	HashX            Kind = "X"
	SignedPayload    Kind = "P"
	Contract         Kind = "C"
	LiquidityPool    Kind = "L"
	ClaimableBalance Kind = "B"
	MuxedContract    Kind = "W"
)

// The errors below never contain the input, because one of the inputs this
// package inspects may be a secret seed that must not reach a log or a terminal.
var (
	ErrEmpty     = errors.New("strkey: empty input")
	ErrPadding   = errors.New("strkey: base32 padding is not allowed")
	ErrLength    = errors.New("strkey: encoded length is not a valid base32 length")
	ErrAlphabet  = errors.New("strkey: characters outside the base32 alphabet")
	ErrCanonical = errors.New("strkey: encoding is not in SEP-23 canonical form")
	ErrChecksum  = errors.New("strkey: checksum mismatch")
	ErrSpecifier = errors.New("strkey: algorithm specifier is not zero")
	ErrVersion   = errors.New("strkey: unknown version byte")
	ErrPayload   = errors.New("strkey: payload shape does not match its version byte")
)

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

// Classify validates s as a SEP-23 strkey and reports its type.
//
// Validity is the rule the specification states: decode, check the checksum and
// the payload shape, then re-encode and require the exact same string. The
// round trip is what rejects nonzero unused bits in the final symbol, which
// Go's base32 decoder would otherwise discard silently.
func Classify(s string) (Kind, error) {
	if s == "" {
		return "", ErrEmpty
	}
	if strings.ContainsRune(s, '=') {
		return "", ErrPadding
	}
	if n := len(s) % 8; n == 1 || n == 3 || n == 6 {
		return "", ErrLength
	}
	if s != strings.ToUpper(s) {
		return "", ErrCanonical
	}

	raw, err := base32NoPad().DecodeString(s)
	if err != nil {
		return "", ErrAlphabet
	}
	if len(raw) < 4 {
		return "", ErrPayload
	}

	sum := checksum16(raw[:len(raw)-2])
	if got := uint16(raw[len(raw)-2]) | uint16(raw[len(raw)-1])<<8; got != sum {
		return "", ErrChecksum
	}
	if raw[0]&0x07 != 0 {
		return "", ErrSpecifier
	}

	kind, err := kindFor(raw[0] >> 3)
	if err != nil {
		return "", err
	}
	if err := checkShape(kind, raw[1:len(raw)-2]); err != nil {
		return "", err
	}
	if encodeRaw(raw) != s {
		return "", ErrCanonical
	}
	return kind, nil
}

// IsAccountID reports whether s is a valid account address. Only a canonical
// G strkey passes, because that is the only form Horizon resolves.
func IsAccountID(s string) bool {
	kind, err := Classify(s)
	return err == nil && kind == AccountID
}

func kindFor(version byte) (Kind, error) {
	switch version {
	case 1:
		return ClaimableBalance, nil
	case 2:
		return Contract, nil
	case 6:
		return AccountID, nil
	case 11:
		return LiquidityPool, nil
	case 12:
		return MuxedAccount, nil
	case 15:
		return SignedPayload, nil
	case 18:
		return Seed, nil
	case 19:
		return PreAuthTx, nil
	case 22:
		return MuxedContract, nil
	case 23:
		return HashX, nil
	default:
		return "", ErrVersion
	}
}

func checkShape(kind Kind, body []byte) error {
	switch kind {
	case ClaimableBalance:
		if len(body) != 33 || body[0] != 0 {
			return ErrPayload
		}
	case MuxedAccount, MuxedContract:
		if len(body) != 40 {
			return ErrPayload
		}
	case SignedPayload:
		// 32 key bytes, a big-endian length, the payload, then 0 to 3 zero
		// bytes, the whole thing padded to a multiple of 4.
		if len(body) < 40 || (len(body)-36)%4 != 0 {
			return ErrPayload
		}
		n := int(uint32(body[32])<<24 | uint32(body[33])<<16 | uint32(body[34])<<8 | uint32(body[35]))
		if n < 1 || n > len(body)-36 {
			return ErrPayload
		}
		if pad := len(body) - 36 - n; pad > 3 {
			return ErrPayload
		}
		for _, b := range body[36+n:] {
			if b != 0 {
				return ErrPayload
			}
		}
	case AccountID, Contract, LiquidityPool, Seed, PreAuthTx, HashX:
		if len(body) != 32 {
			return ErrPayload
		}
	default:
		return ErrVersion
	}
	return nil
}

func encodeStrkey(version byte, body []byte) string {
	raw := make([]byte, 1+len(body)+2)
	raw[0] = version << 3
	copy(raw[1:], body)
	return encodeRaw(raw)
}

func encodeRaw(raw []byte) string {
	buf := make([]byte, len(raw))
	copy(buf, raw)
	sum := checksum16(buf[:len(buf)-2])
	buf[len(buf)-2] = byte(sum)
	buf[len(buf)-1] = byte(sum >> 8)
	return base32NoPad().EncodeToString(buf)
}

func base32NoPad() *base32.Encoding {
	return base32.NewEncoding(alphabet).WithPadding(base32.NoPadding)
}

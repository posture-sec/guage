package policy

import (
	"io"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/posture-sec/guage/internal/strkey"
)

// MaxConfigBytes bounds how much of a policy file is read. A policy is a small
// declaration; anything larger is a mistake or an attack, not a policy.
const MaxConfigBytes = 1 << 20

// Load reads and strictly validates a gauge.toml. Unknown keys, duplicate
// accounts and a secret seed anywhere in the file are errors.
func Load(r io.Reader) (*Config, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxConfigBytes+1))
	if err != nil {
		return nil, withDetail(ErrRead, "read")
	}
	if len(data) > MaxConfigBytes {
		return nil, ErrTooLarge
	}
	if containsSeed(data) {
		return nil, ErrSecretSeed
	}

	var cfg Config
	md, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return nil, ErrSyntax
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, withDetail(ErrUnknownKey, undecoded[0].String())
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// LoadFile opens path and calls Load. The file is never logged.
func LoadFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, withDetail(ErrRead, "open")
	}
	defer f.Close()
	return Load(f)
}

// containsSeed scans the raw text for a token that is a valid S strkey, so a
// seed is rejected even in a comment or an unknown key. The token is never
// returned and the caller never sees it.
func containsSeed(data []byte) bool {
	for i := 0; i < len(data); {
		if !base32Byte(data[i]) {
			i++
			continue
		}
		j := i
		for j < len(data) && base32Byte(data[j]) {
			j++
		}
		tok := string(data[i:j])
		if len(tok) == 56 && (tok[0] == 'S' || tok[0] == 's') && strkey.IsSeed(strings.ToUpper(tok)) {
			return true
		}
		i = j
	}
	return false
}

func base32Byte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '2' && c <= '7'
}

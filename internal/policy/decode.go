package policy

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
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
		if detail := parseDetail(err); detail != "" {
			return nil, withDetail(ErrSyntax, detail)
		}
		return nil, ErrSyntax
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		return nil, withDetail(ErrUnknownKey, unknownKeys(undecoded))
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// unknownKeys renders every undecoded key path, sorted and joined, so one load
// names all of an operator's misspellings rather than the first. Key.String
// yields a dotted path and never the value assigned to it, which keeps the
// detail within the Error contract.
func unknownKeys(undecoded []toml.Key) string {
	paths := make([]string, len(undecoded))
	for i, key := range undecoded {
		paths[i] = key.String()
	}
	sort.Strings(paths)
	return strings.Join(paths, ", ")
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

// parseDetail keeps the decoder's position without keeping its message, which
// can quote a line from the file. BurntSushi/toml v1.6.0 returns ParseError as
// a value, so both the value and pointer shapes are checked.
func parseDetail(err error) string {
	var value toml.ParseError
	if !errors.As(err, &value) {
		var pointer *toml.ParseError
		if !errors.As(err, &pointer) || pointer == nil {
			return ""
		}
		value = *pointer
	}
	detail := fmt.Sprintf("line %d, column %d", value.Position.Line, value.Position.Col)
	if value.LastKey != "" {
		detail += ", last key " + strconv.Quote(value.LastKey)
	}
	return detail
}

// containsSeed scans the raw text for a 56-character base32 token beginning
// with S or s, the shape of a secret seed. It rejects the token regardless of
// whether the checksum validates, because a single mistyped character turns a
// real seed into a corrupted copy that must not be echoed. The token is never
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
		if len(tok) == 56 && (tok[0] == 'S' || tok[0] == 's') {
			return true
		}
		i = j
	}
	return false
}

func base32Byte(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '2' && c <= '7'
}

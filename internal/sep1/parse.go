package sep1

import (
	"github.com/BurntSushi/toml"

	"github.com/posture-sec/guage/internal/sanitize"
	"github.com/posture-sec/guage/internal/strkey"
)

// Step names the stage of the identity check that produced a Result. A finding
// carries it so an operator can tell a dead domain from a misconfigured one.
type Step uint8

const (
	// StepOK means the document was fetched under the guard and parsed.
	StepOK Step = iota
	StepHostname
	StepResolve
	StepGuard
	StepTransport
	StepRedirect
	StepStatus
	StepSize
	StepParse
	// StepNoKeys means the body parsed as TOML but named none of the identity
	// fields, which is a misconfigured stellar.toml rather than a dead host.
	StepNoKeys
)

func (s Step) String() string {
	switch s {
	case StepOK:
		return "ok"
	case StepHostname:
		return "hostname"
	case StepResolve:
		return "resolve"
	case StepGuard:
		return "guard"
	case StepTransport:
		return "transport"
	case StepRedirect:
		return "redirect"
	case StepStatus:
		return "status"
	case StepSize:
		return "size"
	case StepParse:
		return "parse"
	case StepNoKeys:
		return "no-recognized-keys"
	default:
		return "unknown"
	}
}

// Identity holds the fields of a stellar.toml that Gauge reads. SEP-1 v2.7.0
// names no ACCOUNT_ID field, so there is none here. A nil pointer means the key
// is absent, which is not the same as an empty value.
type Identity struct {
	NetworkPassphrase *string
	SigningKey        *string
	HorizonURL        *string
	Version           *string
	// Accounts lists only the entries that classify as a G address. Rejected
	// entries are counted in MalformedAccounts and never carried.
	Accounts          []string
	MalformedAccounts int
}

// Recognized reports whether the document named at least one identity field.
func (i Identity) Recognized() bool {
	return i.NetworkPassphrase != nil || i.SigningKey != nil || i.HorizonURL != nil ||
		i.Version != nil || i.Accounts != nil
}

// doc is the shape of the keys Gauge reads. Everything else in a stellar.toml is
// ignored on purpose: real files carry ORG, THUMBNAIL, [Currencies.X] tables and
// dozens of keys with no bearing on an account posture audit, and refusing them
// would mark nearly every issuer on the network unreachable. Strictness here
// means strict syntax plus at least one recognized key, not an unknown-key check
// the way internal/policy reads a config file.
type doc struct {
	NetworkPassphrase *string `toml:"NETWORK_PASSPHRASE"`
	SigningKey        *string `toml:"SIGNING_KEY"`
	HorizonURL        *string `toml:"HORIZON_URL"`
	Version           *string `toml:"VERSION"`
	Accounts          []any   `toml:"ACCOUNTS"`
}

// Parse decodes a stellar.toml body and returns the identity fields, the step
// that decided the outcome, and a short sanitized reason. The body itself never
// leaves this function: the TOML library's error text can quote the line that
// failed, and that line is hostile content.
func Parse(body []byte) (Identity, Step, string) {
	var d doc
	if err := toml.Unmarshal(body, &d); err != nil {
		return Identity{}, StepParse, "the document is not a configuration file this parser accepts"
	}
	id := Identity{
		NetworkPassphrase: clean(d.NetworkPassphrase),
		SigningKey:        clean(d.SigningKey),
		HorizonURL:        clean(d.HorizonURL),
		Version:           clean(d.Version),
	}
	for _, entry := range d.Accounts {
		value, ok := entry.(string)
		if !ok {
			id.MalformedAccounts++
			continue
		}
		if strkey.IsAccountID(value) {
			id.Accounts = append(id.Accounts, value)
			continue
		}
		id.MalformedAccounts++
	}
	if !id.Recognized() {
		return id, StepNoKeys, "the document names no identity field"
	}
	return id, StepOK, ""
}

func clean(s *string) *string {
	if s == nil {
		return nil
	}
	c := sanitize.Text(*s)
	return &c
}

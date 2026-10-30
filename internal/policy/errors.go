package policy

// ErrorKind names a category of policy rejection. Kinds are stable because the
// CLI maps them to exit 2; Error.Detail carries a key path, never a value,
// because a value may be a secret seed.
type ErrorKind string

const (
	kindRead            ErrorKind = "read"
	kindTooLarge        ErrorKind = "too-large"
	kindSyntax          ErrorKind = "syntax"
	kindUnknownKey      ErrorKind = "unknown-key"
	kindVersion         ErrorKind = "version"
	kindNetwork         ErrorKind = "network"
	kindEndpoint        ErrorKind = "endpoint"
	kindNotAccount      ErrorKind = "not-account"
	kindRole            ErrorKind = "role"
	kindSignerKey       ErrorKind = "signer-key"
	kindWeight          ErrorKind = "weight"
	kindThreshold       ErrorKind = "threshold"
	kindDuplicateSigner ErrorKind = "duplicate-signer"
	kindDuplicateAcct   ErrorKind = "duplicate-account"
	kindHomeDomain      ErrorKind = "home-domain"
	kindAccept          ErrorKind = "accept"
	kindAcceptRule      ErrorKind = "accept-rule"
	kindDuplicateAccept ErrorKind = "duplicate-accept"
	kindSecretSeed      ErrorKind = "secret-seed"
)

// Error is a policy rejection. Detail names a field path or an unknown key and
// never contains a value read from the file.
type Error struct {
	Kind    ErrorKind
	Message string
	Detail  string
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return "policy: " + e.Message
	}
	return "policy: " + e.Message + " (" + e.Detail + ")"
}

func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)
	return ok && t.Kind == e.Kind
}

var (
	ErrRead             = &Error{Kind: kindRead, Message: "the policy file could not be read"}
	ErrTooLarge         = &Error{Kind: kindTooLarge, Message: "the policy file is larger than 1 MiB"}
	ErrSyntax           = &Error{Kind: kindSyntax, Message: "the policy file is not valid TOML"}
	ErrUnknownKey       = &Error{Kind: kindUnknownKey, Message: "the policy contains a key this version does not define"}
	ErrVersion          = &Error{Kind: kindVersion, Message: "version must be 1"}
	ErrNetwork          = &Error{Kind: kindNetwork, Message: "network must be mainnet, testnet or a full network passphrase"}
	ErrEndpoint         = &Error{Kind: kindEndpoint, Message: "each endpoint must be an https URL with a host and no userinfo, query or fragment"}
	ErrNotAccount       = &Error{Kind: kindNotAccount, Message: "account IDs must be canonical G addresses; multiplexed and contract addresses are not accepted"}
	ErrRole             = &Error{Kind: kindRole, Message: "role must be issuer, treasury, distribution, merchant or other"}
	ErrSignerKey        = &Error{Kind: kindSignerKey, Message: "expected signer keys must be G, T, X or P strkeys"}
	ErrWeight           = &Error{Kind: kindWeight, Message: "signer weights must be between 0 and 255"}
	ErrThreshold        = &Error{Kind: kindThreshold, Message: "thresholds must be between 0 and 255"}
	ErrDuplicateSigner  = &Error{Kind: kindDuplicateSigner, Message: "an expected signer is declared twice for one account"}
	ErrDuplicateAccount = &Error{Kind: kindDuplicateAcct, Message: "an account is declared twice"}
	ErrHomeDomain       = &Error{Kind: kindHomeDomain, Message: "home_domain must be a bare host name, not a URL or an IP address"}
	ErrAccept           = &Error{Kind: kindAccept, Message: "each accept entry needs a rule, a G account and a reason, and an optional YYYY-MM-DD expiry"}
	ErrAcceptRule       = &Error{Kind: kindAcceptRule, Message: "the accept rule must be a concrete rule ID that may be accepted"}
	ErrDuplicateAccept  = &Error{Kind: kindDuplicateAccept, Message: "the same rule is accepted twice for one account"}
	ErrSecretSeed       = &Error{Kind: kindSecretSeed, Message: "a secret seed is present in the policy; remove it and rotate the key"}
)

func withDetail(base *Error, detail string) error {
	return &Error{Kind: base.Kind, Message: base.Message, Detail: detail}
}

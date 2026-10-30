package source

import "errors"

var (
	// ErrNotFound reports an account the endpoint does not serve.
	ErrNotFound = errors.New("source: account not found")
	// ErrRequest reports a request Gauge refuses to make.
	ErrRequest = errors.New("source: invalid request")
	// ErrEndpoint reports an endpoint URL Gauge refuses to use.
	ErrEndpoint = errors.New("source: invalid endpoint")
	// ErrOptions reports nonsensical client options.
	ErrOptions = errors.New("source: invalid options")
	// ErrStatus reports an HTTP status outside the handled set.
	ErrStatus = errors.New("source: unexpected HTTP status")
	// ErrTooLarge reports a response over the configured size cap.
	ErrTooLarge = errors.New("source: response exceeds the size cap")
	// ErrDecode reports a response whose fields do not match the expected shape.
	ErrDecode = errors.New("source: malformed response")
	// ErrPassphraseMismatch reports an endpoint on a different network.
	ErrPassphraseMismatch = errors.New("source: network passphrase mismatch")
)

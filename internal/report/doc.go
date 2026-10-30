// Package report renders a model.Report as text or JSON with deterministic
// ordering and a limits footer, and maps findings to an exit code. Every
// chain-derived string is sanitized before it reaches the writer.
package report

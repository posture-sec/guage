// Package policy loads gauge.toml strictly and validates declared expectations,
// roles and accepted risks. Rejections never echo a value, because a value may
// be a secret seed.
package policy

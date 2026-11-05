// Package threshold answers what a set of signers can authorize. The rules are
// the protocol's, verified against stellar-core in ARCHITECTURE.md 6.3: a tier is
// reachable when the largest weights an envelope can present sum to the threshold,
// and a pre-authorized transaction signer is not standing control. Pure, no I/O.
package threshold

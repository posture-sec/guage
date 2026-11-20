package model_test

import (
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

// fingerprintCase pins the digest of one finding. The expected hex was computed outside
// this module, from the rule in internal/model/finding.go:
//
//	python3 -c 'import hashlib,sys; r,a,e = sys.argv[1:]; print(hashlib.sha256((r + "\x00" + a + "\x00" + " ".join(e.split())).encode()).hexdigest())' RULE ACCOUNT EVIDENCE
//
// That is the sha256 of the rule, the account and the evidence with every whitespace run
// collapsed to one space, each joined by a NUL byte. A test that only compared two values
// from the Go function could not notice a changed separator, a reordered field or a
// different normalizer, because both sides would move together.
//
// variantOf names the row a case differs from only by whitespace. Such a case must have
// the same digest as that row, which is what normalization promises. Every other case
// must have a digest of its own.
type fingerprintCase struct {
	name, rule, account, evidence, want, variantOf string
}

func TestFingerprintGolden(t *testing.T) {
	t.Parallel()
	cases := []fingerprintCase{
		{"baseline", "sig-single-key-high", "account-reference", "evidence", "3720b0cb79ea13674c63e23bffdccaac2203b4d6f25e0390825f58b871a70f21", ""},
		{"account address and a drift message", "thr-drift", "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7", "low declared 2, chain shows 1; medium declared 2, chain shows 2", "6515e787266ea0d304a546f75a6f920d9eba95504b1e0a13490acf30d8b59013", ""},
		{"single spaces", "sig-threshold-order", "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7", "low 1 medium 2 high 3", "8b64ab0c689cc1cfb89d9905be53a548886d8a2588c4c164bc8af268af3a7893", ""},
		{"tabs and newlines between words", "sig-threshold-order", "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7", "low 1\tmedium 2\nhigh 3", "8b64ab0c689cc1cfb89d9905be53a548886d8a2588c4c164bc8af268af3a7893", "single spaces"},
		{"runs of inner spaces", "sig-threshold-order", "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7", "low  1   medium 2 high  3", "8b64ab0c689cc1cfb89d9905be53a548886d8a2588c4c164bc8af268af3a7893", "single spaces"},
		{"whitespace around the evidence", "sig-threshold-order", "GAAZI4TCR3TY5OJHCTJC2A4QSY6CJWJH5IAJTGKIN2ER7LBNVKOCCWN7", "\n\t low 1 medium 2 high 3 \t\n", "8b64ab0c689cc1cfb89d9905be53a548886d8a2588c4c164bc8af268af3a7893", "single spaces"},
		{"empty evidence", "sig-single-key-high", "account-reference", "", "5c5fe3a0e994d6424a1e4e03f2e26f8113e3979c21c4359202d3a210ac690110", ""},
		{"NUL boundary, rule ab and account c", "ab", "c", "evidence", "f411e1b9c3dfd86b2cb0cdd5d909dbc12c238cba76b9b60fb889565a1f290ecb", ""},
		{"NUL boundary, rule a and account bc", "a", "bc", "evidence", "d782e677a72bec9a369977fcb3ac7136312281c004f3750b65b61a64247c9603", ""},
		{"whitespace inside the account is kept", "r", "a b", "e", "031ba0d3acf602adfc5f66c39ab06ba0c2fcd6af0ff46d5366afae51f25d6e48", ""},
		{"the same text split differently", "r", "a", "b e", "66dda14397504cf671895e4366bb796f67cbb66c9a739c40e62ab724fa30503a", ""},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := model.NewFinding(tt.rule, model.SevHigh, tt.account, "message", tt.evidence).Fingerprint
			if got != tt.want {
				t.Fatalf("fingerprint(%q, %q, %q) = %s, want %s", tt.rule, tt.account, tt.evidence, got, tt.want)
			}
		})
	}

	t.Run("only whitespace variants share a digest", func(t *testing.T) {
		t.Parallel()
		byName := map[string]fingerprintCase{}
		owner := map[string]string{}
		for _, tt := range cases {
			byName[tt.name] = tt
		}
		for _, tt := range cases {
			if tt.variantOf != "" {
				base, ok := byName[tt.variantOf]
				if !ok {
					t.Fatalf("%q is a variant of %q, which is not a case", tt.name, tt.variantOf)
				}
				if tt.want != base.want {
					t.Fatalf("%q should share the digest of %q, got %s and %s", tt.name, base.name, tt.want, base.want)
				}
				continue
			}
			if other, dup := owner[tt.want]; dup {
				t.Fatalf("%q and %q share the digest %s, so their inputs are not distinct", other, tt.name, tt.want)
			}
			owner[tt.want] = tt.name
		}
	})
}

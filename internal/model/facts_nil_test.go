package model_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func factsIntPtr(v int) *int { return &v }

func factsBoolPtr(v bool) *bool { return &v }

func factsStringPtr(v string) *string { return &v }

func innerFactsWithNilPointers() model.AccountFacts {
	return model.AccountFacts{
		Reachability: &model.Reachability{},
		Declared:     &model.DeclaredFacts{},
		Identity:     &model.IdentityFacts{},
	}
}

func factsWithZeroPointers() model.AccountFacts {
	return model.AccountFacts{
		Reachability: &model.Reachability{
			Low: factsIntPtr(0), Medium: factsIntPtr(0), High: factsIntPtr(0),
		},
		Declared: &model.DeclaredFacts{
			Role:               "other",
			ExpectedSigners:    factsIntPtr(0),
			ExpectedThresholds: &model.DeclaredThresholds{Low: factsIntPtr(0), Medium: factsIntPtr(0), High: factsIntPtr(0)},
			ExpectedFlags: &model.DeclaredFlags{
				AuthRequired: factsBoolPtr(false), AuthRevocable: factsBoolPtr(false),
				AuthImmutable: factsBoolPtr(false), ClawbackEnabled: factsBoolPtr(false),
			},
			HomeDomain:   factsStringPtr(""),
			ExpectLocked: factsBoolPtr(false),
		},
		Identity: &model.IdentityFacts{
			NetworkPassphrase: factsStringPtr(""),
			NetworkMatches:    factsBoolPtr(false),
			SigningKey:        factsStringPtr(""),
			HorizonURL:        factsStringPtr(""),
			Version:           factsStringPtr(""),
			Accounts:          []string{},
		},
	}
}

func TestAccountFactsNullVersusZero(t *testing.T) {
	t.Parallel()

	t.Run("nil blocks stay null", func(t *testing.T) {
		t.Parallel()
		in := model.AccountFacts{}
		want := `{"reachability":null,"declared":null,"identity":null}`
		data, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("JSON = %s, want %s", data, want)
		}
		var got model.AccountFacts
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.Reachability != nil || got.Declared != nil || got.Identity != nil {
			t.Fatal("nil blocks did not round trip as nil")
		}
	})

	t.Run("nil inner pointers stay null", func(t *testing.T) {
		t.Parallel()
		in := innerFactsWithNilPointers()
		want := `{"reachability":{"low":null,"medium":null,"high":null},"declared":{"role":"","expected_signers":null,"expected_thresholds":null,"expected_flags":null,"home_domain":null,"expect_locked":null},"identity":{"home_domain":"","reachable":false,"step":"","status":0,"hops":0,"bytes":0,"cors_present":false,"network_passphrase":null,"network_matches":null,"signing_key":null,"horizon_url":null,"version":null,"accounts":null,"malformed_accounts":0}}`
		data, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("JSON = %s, want %s", data, want)
		}
		var got model.AccountFacts
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, in) {
			t.Fatal("nil inner pointers did not round trip as nil")
		}
		if got.Reachability.Low != nil || got.Reachability.Medium != nil || got.Reachability.High != nil {
			t.Fatal("a nil reachability tier became a value")
		}
		if got.Declared.ExpectedSigners != nil || got.Declared.ExpectedThresholds != nil ||
			got.Declared.ExpectedFlags != nil || got.Declared.HomeDomain != nil || got.Declared.ExpectLocked != nil {
			t.Fatal("a nil declared pointer became a value")
		}
		if got.Identity.NetworkPassphrase != nil || got.Identity.NetworkMatches != nil ||
			got.Identity.SigningKey != nil || got.Identity.HorizonURL != nil || got.Identity.Version != nil {
			t.Fatal("a nil identity pointer became a value")
		}
	})

	t.Run("zero and false are values", func(t *testing.T) {
		t.Parallel()
		in := factsWithZeroPointers()
		want := `{"reachability":{"low":0,"medium":0,"high":0},"declared":{"role":"other","expected_signers":0,"expected_thresholds":{"low":0,"medium":0,"high":0},"expected_flags":{"auth_required":false,"auth_revocable":false,"auth_immutable":false,"clawback_enabled":false},"home_domain":"","expect_locked":false},"identity":{"home_domain":"","reachable":false,"step":"","status":0,"hops":0,"bytes":0,"cors_present":false,"network_passphrase":"","network_matches":false,"signing_key":"","horizon_url":"","version":"","accounts":[],"malformed_accounts":0}}`
		data, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Fatalf("JSON = %s, want %s", data, want)
		}
		var got model.AccountFacts
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, in) {
			t.Fatal("zero and false did not round trip as set pointers")
		}
		if got.Reachability.Low == nil || *got.Reachability.Low != 0 {
			t.Fatal("a zero tier did not round trip as a non-nil pointer to zero")
		}
		if got.Identity.NetworkMatches == nil || *got.Identity.NetworkMatches {
			t.Fatal("a false network match did not round trip as a non-nil pointer to false")
		}
		if got.Declared.ExpectLocked == nil || *got.Declared.ExpectLocked {
			t.Fatal("a false expect_locked did not round trip as a non-nil pointer to false")
		}
	})
}

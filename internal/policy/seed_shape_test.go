package policy

import (
	"errors"
	"strings"
	"testing"
)

func TestContainsSeedRejectsValidSeed(t *testing.T) {
	t.Parallel()
	seed := testSeed()
	if !containsSeed([]byte(seed)) {
		t.Fatal("containsSeed missed a valid seed")
	}
	_, err := Load(strings.NewReader("version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + seed + "\""))
	if !errors.Is(err, ErrSecretSeed) {
		t.Fatalf("Load error = %v, want %v", err, ErrSecretSeed)
	}
	if err != nil && strings.Contains(err.Error(), seed) {
		t.Fatal("error message echoed the valid seed")
	}
}

func TestContainsSeedRejectsNearMiss(t *testing.T) {
	t.Parallel()
	seed := testSeed()
	if len(seed) != 56 {
		t.Fatalf("testSeed length = %d, want 56", len(seed))
	}
	nearMiss := seed[:27] + "X" + seed[28:]
	if nearMiss == seed {
		t.Fatal("nearMiss did not change the seed")
	}
	if !containsSeed([]byte(nearMiss)) {
		t.Fatal("containsSeed missed a near-miss seed shape")
	}
	_, err := Load(strings.NewReader("version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + nearMiss + "\""))
	if !errors.Is(err, ErrSecretSeed) {
		t.Fatalf("Load error = %v, want %v", err, ErrSecretSeed)
	}
	if err != nil && (strings.Contains(err.Error(), nearMiss) || strings.Contains(err.Error(), seed)) {
		t.Fatal("error message echoed the near-miss or the valid seed")
	}
}

func TestContainsSeedRejectsLowercaseNearMiss(t *testing.T) {
	t.Parallel()
	seed := testSeed()
	nearMiss := strings.ToLower(seed[:27] + "x" + seed[28:])
	if !containsSeed([]byte(nearMiss)) {
		t.Fatal("containsSeed missed a lowercase near-miss seed shape")
	}
	_, err := Load(strings.NewReader("version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + nearMiss + "\""))
	if !errors.Is(err, ErrSecretSeed) {
		t.Fatalf("Load error = %v, want %v", err, ErrSecretSeed)
	}
	if err != nil && strings.Contains(err.Error(), nearMiss) {
		t.Fatal("error message echoed the near-miss")
	}
}

func TestContainsSeedAcceptsNonSeedBase32(t *testing.T) {
	t.Parallel()
	account := "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ"
	if len(account) == 56 && (account[0] == 'S' || account[0] == 's') {
		t.Fatal("test account unexpectedly starts with S")
	}
	if containsSeed([]byte(account)) {
		t.Fatal("containsSeed rejected a valid account ID")
	}
	_, err := Load(strings.NewReader("version = 1\nnetwork = \"testnet\"\n[[account]]\nid = \"" + account + "\""))
	if err != nil {
		t.Fatalf("Load rejected valid account: %v", err)
	}
}

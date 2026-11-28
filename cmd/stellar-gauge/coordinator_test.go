package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestCoordinatorCLICheckAndAudit(t *testing.T) {
	t.Parallel()

	const coordID = "GAUA7XL5K54CC2DDGP77FJ2YBHRJLT36CPZDXWPM6MP7MANOGG77PNJU"
	rootBody := readFixture(t, fixtureRoot)
	coordBody := readFixture(t, "../../testdata/horizon/17-coordinator-entry.json")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/hal+json; charset=utf-8")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write(rootBody)
		case "/accounts/" + coordID:
			_, _ = w.Write(coordBody)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"title":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)

	t.Run("check fails on warn and prints coordinator fact", func(t *testing.T) {
		t.Parallel()
		code, stdout, stderr := execCLIWith(t, offlineDeps(nil),
			"check", "--network", testNetwork, "--endpoint", srv.URL, "--fail-on", "warn", coordID,
		)
		if code != int(model.ExitFindings) {
			t.Fatalf("exit = %d, stderr = %s, stdout = %s", code, stderr, stdout)
		}
		for _, want := range []string{
			"  coordinator: v1.multisig.satoshipay.io (data entry config.multisig.coordinator)",
			"[warn] id-multisig-coordinator-unexpected",
			"v1.multisig.satoshipay.io",
		} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("stdout missing %q:\n%s", want, stdout)
			}
		}
	})

	t.Run("audit matching coordinator exits zero", func(t *testing.T) {
		t.Parallel()
		cfgPath := filepath.Join(t.TempDir(), "gauge.toml")
		cfg := "version = 1\nnetwork = \"" + testNetwork + "\"\nendpoints = [\"" + srv.URL + "\"]\n\n[[account]]\nid = \"" + coordID + "\"\nrole = \"other\"\ncoordinator = \"v1.multisig.satoshipay.io\"\n"
		if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr := execCLIWith(t, offlineDeps(nil), "audit", "--config", cfgPath, "--fail-on", "warn")
		if code != int(model.ExitPass) {
			t.Fatalf("exit = %d, stderr = %s, stdout = %s", code, stderr, stdout)
		}
		if !strings.Contains(stdout, `coordinator: "v1.multisig.satoshipay.io"`) {
			t.Fatalf("audit output missing declared coordinator:\n%s", stdout)
		}
		if strings.Contains(stdout, "id-multisig-coordinator-unexpected") {
			t.Fatalf("matched audit unexpectedly raised coordinator warning:\n%s", stdout)
		}
	})

	t.Run("audit mismatching coordinator exits one", func(t *testing.T) {
		t.Parallel()
		cfgPath := filepath.Join(t.TempDir(), "gauge.toml")
		cfg := "version = 1\nnetwork = \"" + testNetwork + "\"\nendpoints = [\"" + srv.URL + "\"]\n\n[[account]]\nid = \"" + coordID + "\"\nrole = \"treasury\"\ncoordinator = \"expected.coordinator.example\"\n"
		if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
		code, stdout, _ := execCLIWith(t, offlineDeps(nil), "audit", "--config", cfgPath, "--fail-on", "warn")
		if code != int(model.ExitFindings) {
			t.Fatalf("exit = %d, want %d; stdout = %s", code, model.ExitFindings, stdout)
		}
		if !strings.Contains(stdout, "expected.coordinator.example") || !strings.Contains(stdout, "v1.multisig.satoshipay.io") {
			t.Fatalf("mismatch evidence missing normalized domains:\n%s", stdout)
		}
	})
}

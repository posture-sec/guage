package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
)

func TestCLISnapshotAndDiff(t *testing.T) {
	srv := startEndpoint(t, endpoint{})
	defer srv.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "gauge.toml")
	baselinePath := filepath.Join(dir, "baseline.json")

	cfg := fmt.Sprintf(`version = 1
network = "testnet"
endpoints = ["%s"]

[[account]]
id = "%s"
name = "treasury"
role = "treasury"
`, srv.URL, accountOne)

	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}

	// 1. Snapshot with missing --out flag -> ExitUsage (2)
	var stdout, stderr bytes.Buffer
	code := runCLI([]string{"snapshot", "--config", configPath}, &stdout, &stderr, deps{})
	if code != int(model.ExitUsage) {
		t.Errorf("expected ExitUsage (2) when --out is omitted, got %d", code)
	}

	// 2. Snapshot successful -> ExitPass (0) and file created
	stdout.Reset()
	stderr.Reset()
	code = runCLI([]string{"snapshot", "--config", configPath, "--out", baselinePath}, &stdout, &stderr, deps{})
	if code != int(model.ExitPass) {
		t.Fatalf("snapshot failed with exit %d: %s", code, stderr.String())
	}

	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("baseline file was not written: %v", err)
	}
	var baseline model.Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatalf("parsing written baseline JSON failed: %v", err)
	}
	if baseline.Banner != model.BaselineBanner {
		t.Errorf("expected review banner in baseline, got: %s", baseline.Banner)
	}
	if len(baseline.Accounts) != 1 || baseline.Accounts[0].ID != accountOne {
		t.Fatalf("expected account %s in baseline, got: %+v", accountOne, baseline.Accounts)
	}

	// 3. Diff with missing --baseline flag -> ExitUsage (2)
	stdout.Reset()
	stderr.Reset()
	code = runCLI([]string{"diff", "--config", configPath}, &stdout, &stderr, deps{})
	if code != int(model.ExitUsage) {
		t.Errorf("expected ExitUsage (2) when --baseline is omitted, got %d", code)
	}

	// 4. Diff against identical baseline -> ExitPass (0)
	stdout.Reset()
	stderr.Reset()
	code = runCLI([]string{"diff", "--config", configPath, "--baseline", baselinePath}, &stdout, &stderr, deps{})
	if code != int(model.ExitPass) {
		t.Fatalf("diff failed on identical baseline with exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "all accounts match baseline") {
		t.Errorf("expected 'all accounts match baseline' in output, got: %s", stdout.String())
	}

	// 5. Diff with modified baseline (e.g. low threshold 99) -> ExitFindings (1)
	baseline.Accounts[0].Thresholds.Low = 99
	modData, _ := json.MarshalIndent(baseline, "", "  ")
	modBaselinePath := filepath.Join(dir, "modified_baseline.json")
	if err := os.WriteFile(modBaselinePath, modData, 0644); err != nil {
		t.Fatalf("writing modified baseline: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = runCLI([]string{"diff", "--config", configPath, "--baseline", modBaselinePath}, &stdout, &stderr, deps{})
	if code != int(model.ExitFindings) {
		t.Errorf("expected ExitFindings (1) when baseline differs, got %d", code)
	}
	if !strings.Contains(stdout.String(), "drift detected from baseline") {
		t.Errorf("expected drift detected in output, got: %s", stdout.String())
	}

	// 6. Diff with --format json and --format markdown
	stdout.Reset()
	stderr.Reset()
	code = runCLI([]string{"diff", "--config", configPath, "--baseline", modBaselinePath, "--format", "json"}, &stdout, &stderr, deps{})
	if code != int(model.ExitFindings) {
		t.Errorf("expected ExitFindings (1) for JSON diff, got %d", code)
	}
	var jsonDoc map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &jsonDoc); err != nil {
		t.Errorf("invalid JSON from diff --format json: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	code = runCLI([]string{"diff", "--config", configPath, "--baseline", modBaselinePath, "--format", "markdown"}, &stdout, &stderr, deps{})
	if code != int(model.ExitFindings) {
		t.Errorf("expected ExitFindings (1) for Markdown diff, got %d", code)
	}
	if !strings.Contains(stdout.String(), "### Baseline Comparison Report") {
		t.Errorf("expected markdown header in output, got: %s", stdout.String())
	}
}

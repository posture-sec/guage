package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/report"
)

func sampleDiffReport(hasDiff bool) model.DiffReport {
	d := model.DiffReport{
		Banner:         model.BaselineBanner,
		ToolVersion:    "v0.1.0",
		Network:        "Test SDF Network ; September 2015",
		BaselineLedger: 1000,
		CurrentLedger:  1050,
		HasDiff:        hasDiff,
		ExitCode:       model.ExitPass,
	}
	if hasDiff {
		d.ExitCode = model.ExitFindings
		d.Accounts = []model.AccountDiff{
			{
				AccountID: "GA7QYNF7SOWQ3GLR2BGMZEHXAVIRZA4KVWLTJJFC7MGXUA74P7UJVSGZ",
				Thresholds: []model.ThresholdDiff{
					{Tier: "medium", Old: 1, New: 2},
				},
				Signers: []model.SignerDiff{
					{Key: "GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3", Type: model.SignerEd25519, Weight: 1, Kind: "added"},
				},
				Flags: []model.FlagDiff{
					{Flag: "auth_required", Old: false, New: true},
				},
				HomeDomain: &model.DomainDiff{
					Old: "old.org",
					New: "new.org",
				},
			},
		}
	}
	return d
}

func TestDiffText(t *testing.T) {
	// 1. Clean report
	var bufClean bytes.Buffer
	if err := report.DiffText(&bufClean, sampleDiffReport(false)); err != nil {
		t.Fatalf("DiffText clean failed: %v", err)
	}
	cleanStr := bufClean.String()
	if !strings.Contains(cleanStr, model.BaselineBanner) {
		t.Errorf("clean report missing banner")
	}
	if !strings.Contains(cleanStr, "all accounts match baseline") {
		t.Errorf("clean report missing match confirmation")
	}
	if !strings.Contains(cleanStr, report.Limits) {
		t.Errorf("clean report missing Limits footer")
	}

	// 2. Report with drift
	var bufDrift bytes.Buffer
	if err := report.DiffText(&bufDrift, sampleDiffReport(true)); err != nil {
		t.Fatalf("DiffText drift failed: %v", err)
	}
	driftStr := bufDrift.String()
	if !strings.Contains(driftStr, model.BaselineBanner) {
		t.Errorf("drift report missing banner")
	}
	if !strings.Contains(driftStr, "drift detected from baseline") {
		t.Errorf("drift report missing drift header")
	}
	if !strings.Contains(driftStr, "medium: changed from 1 to 2") {
		t.Errorf("drift report missing threshold change")
	}
	if !strings.Contains(driftStr, "+ added: GBK4NFJ7SVMUNPBIYBC4WNMCTKI6Z66MYYWZQSSLQNOSUSTMKVUKE7M3") {
		t.Errorf("drift report missing signer added")
	}
	if !strings.Contains(driftStr, "auth_required: changed from false to true") {
		t.Errorf("drift report missing flag change")
	}
	if !strings.Contains(driftStr, "home domain: changed from old.org to new.org") {
		t.Errorf("drift report missing home domain change")
	}
}

func TestDiffJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := report.DiffJSON(&buf, sampleDiffReport(true)); err != nil {
		t.Fatalf("DiffJSON failed: %v", err)
	}
	var doc struct {
		model.DiffReport
		Limits string `json:"limits"`
	}
	if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshaling DiffJSON output failed: %v", err)
	}
	if doc.Banner != model.BaselineBanner {
		t.Errorf("JSON output missing banner")
	}
	if !doc.HasDiff {
		t.Errorf("JSON output expected HasDiff true")
	}
	if doc.Limits != report.Limits {
		t.Errorf("JSON output missing limits")
	}
}

func TestDiffMarkdown(t *testing.T) {
	var buf bytes.Buffer
	if err := report.DiffMarkdown(&buf, sampleDiffReport(true)); err != nil {
		t.Fatalf("DiffMarkdown failed: %v", err)
	}
	md := buf.String()
	if !strings.Contains(md, "### Baseline Comparison Report") {
		t.Errorf("Markdown output missing header")
	}
	if !strings.Contains(md, model.BaselineBanner) {
		t.Errorf("Markdown output missing banner blockquote")
	}
	if !strings.Contains(md, "**+ Added:**") {
		t.Errorf("Markdown output missing added signer")
	}
}

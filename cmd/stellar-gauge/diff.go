package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/report"
	"github.com/posture-sec/guage/internal/run"
)

func diffCommand(args []string, stdout, stderr io.Writer, d deps) int {
	fs := newFlagSet("diff", stderr,
		"usage: stellar-gauge diff [--config gauge.toml] --baseline baseline.json [--format text|json|markdown]")
	configPath := fs.String("config", defaultConfig, "policy file to load")
	baselinePath := fs.String("baseline", "", "path to the saved baseline JSON file")
	format := fs.String("format", "text", "output format: text, json or markdown")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return int(model.ExitPass)
		}
		return int(model.ExitUsage)
	}
	if fs.NArg() != 0 {
		return usageError(stderr, "diff takes no positional arguments; declare accounts in the policy file")
	}
	if *baselinePath == "" {
		return usageError(stderr, "--baseline is required; specify the path of the saved baseline file")
	}
	if !validFormat(*format) {
		return usageError(stderr, "--format must be text, json or markdown")
	}

	data, err := os.ReadFile(*baselinePath)
	if err != nil {
		fmt.Fprintf(stderr, "stellar-gauge: reading baseline file: %v\n", err)
		return int(model.ExitUsage)
	}

	var baseline model.Baseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		fmt.Fprintf(stderr, "stellar-gauge: parsing baseline file: %v\n", err)
		return int(model.ExitUsage)
	}

	cfg, err := policy.LoadFile(*configPath)
	if err != nil {
		fmt.Fprintf(stderr, "stellar-gauge: %v\n", err)
		if errors.Is(err, policy.ErrRead) {
			fmt.Fprintf(stderr, "stellar-gauge: no accounts were named, so a policy file is required; "+
				"copy %s to %s and edit it, or audit accounts on the command line with \"check\"\n",
				exampleConfig, defaultConfig)
		}
		return int(model.ExitUsage)
	}

	diffReport, err := run.Diff(context.Background(), cfg, baseline, run.DiffOptions{
		ToolVersion: version,
		NewSource:   d.newSource,
	})
	if err != nil {
		return reportError(stderr, err)
	}

	switch *format {
	case "json":
		err = report.DiffJSON(stdout, diffReport)
	case "markdown":
		err = report.DiffMarkdown(stdout, diffReport)
	default:
		err = report.DiffText(stdout, diffReport)
	}
	if err != nil {
		fmt.Fprintf(stderr, "stellar-gauge: writing diff report: %v\n", err)
		return int(model.ExitSource)
	}
	return int(diffReport.ExitCode)
}

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
	"github.com/posture-sec/guage/internal/run"
)

func snapshotCommand(args []string, stdout, stderr io.Writer, d deps) int {
	fs := newFlagSet("snapshot", stderr,
		"usage: stellar-gauge snapshot [--config gauge.toml] --out baseline.json")
	configPath := fs.String("config", defaultConfig, "policy file to load")
	outPath := fs.String("out", "", "output file path for the baseline JSON")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return int(model.ExitPass)
		}
		return int(model.ExitUsage)
	}
	if fs.NArg() != 0 {
		return usageError(stderr, "snapshot takes no positional arguments; declare accounts in the policy file")
	}
	if *outPath == "" {
		return usageError(stderr, "--out is required; specify the path to write the baseline file")
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

	baseline, err := run.Snapshot(context.Background(), cfg, run.SnapshotOptions{
		ToolVersion: version,
		NewSource:   d.newSource,
	})
	if err != nil {
		return reportError(stderr, err)
	}

	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "stellar-gauge: serializing baseline: %v\n", err)
		return int(model.ExitSource)
	}

	if err := os.WriteFile(*outPath, append(data, '\n'), 0644); err != nil {
		fmt.Fprintf(stderr, "stellar-gauge: writing baseline file: %v\n", err)
		return int(model.ExitSource)
	}

	fmt.Fprintf(stdout, "stellar-gauge: recorded baseline for %d account(s) at ledger %d to %s\n",
		len(baseline.Accounts), baseline.Ledger, *outPath)
	return int(model.ExitPass)
}

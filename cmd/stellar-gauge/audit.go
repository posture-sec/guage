package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/policy"
	"github.com/posture-sec/guage/internal/run"
)

func auditCommand(args []string, stdout, stderr io.Writer, d deps) int {
	fs := newFlagSet("audit", stderr,
		"usage: stellar-gauge audit [--config gauge.toml] [--format text|json] [--fail-on high|warn]")
	configPath := fs.String("config", defaultConfig, "policy file to load")
	format := fs.String("format", "text", "output format: text or json")
	failOn := fs.String("fail-on", "high", "fail at this severity: high or warn")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return int(model.ExitPass)
		}
		return int(model.ExitUsage)
	}
	if fs.NArg() != 0 {
		return usageError(stderr, "audit takes no positional arguments; declare accounts in the policy file")
	}
	if *format == "markdown" {
		return usageError(stderr, "markdown output is not implemented yet")
	}
	if !validFormat(*format) {
		return usageError(stderr, "--format must be text or json")
	}
	severity, ok := parseFailOn(*failOn)
	if !ok {
		return usageError(stderr, "--fail-on must be high or warn")
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
	result, err := run.Audit(context.Background(), cfg, run.AuditOptions{
		ToolVersion: version,
		FailOn:      severity,
		NewSource:   d.newSource,
		NewIdentity: d.newIdentity,
	})
	if err != nil {
		return reportError(stderr, err)
	}
	return render(stdout, stderr, result, *format)
}

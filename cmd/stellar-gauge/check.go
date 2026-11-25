package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/run"
	"github.com/posture-sec/guage/internal/strkey"
)

func checkCommand(args []string, stdout, stderr io.Writer, d deps) int {
	fs := newFlagSet("check", stderr,
		"usage: stellar-gauge check --network NETWORK --endpoint URL [--endpoint URL ...] [--format text|json|markdown] [--fail-on high|warn] [--concurrency N] G...")
	network := fs.String("network", "", "mainnet, testnet, or a full network passphrase")
	endpoints := &stringList{}
	fs.Var(endpoints, "endpoint", "Horizon endpoint; repeat it, up to five times, to compare endpoints")
	format := fs.String("format", "text", "output format: text, json or markdown")
	failOn := fs.String("fail-on", "high", "fail at this severity: high or warn")
	concurrency := fs.String("concurrency", "8", "maximum concurrent account and identity reads (1-32)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return int(model.ExitPass)
		}
		return int(model.ExitUsage)
	}
	if *network == "" {
		return usageError(stderr, "--network is required and has no default: mainnet, testnet or a full passphrase")
	}
	// The count above one is the orchestrator's to bound, so a sixth endpoint and
	// a repeated one are refused in one place for check and audit alike.
	if endpoints.Len() == 0 {
		return usageError(stderr, "--endpoint is required and has no default: "+
			namedEndpoints+"Repeat it to compare endpoints.")
	}
	ids := fs.Args()
	if len(ids) == 0 {
		return usageError(stderr, "at least one G account address is required")
	}
	severity, ok := parseFailOn(*failOn)
	if !ok {
		return usageError(stderr, "--fail-on must be high or warn")
	}
	if !validFormat(*format) {
		return usageError(stderr, formatRejection())
	}
	concurrencyLimit, ok := parseConcurrency(*concurrency)
	if !ok {
		return usageError(stderr, "--concurrency must be between 1 and 32")
	}
	if err := validateAccountArgs(ids); err != nil {
		fmt.Fprintf(stderr, "stellar-gauge: %v\n", err)
		return int(model.ExitUsage)
	}

	ctx := d.ctx
	if ctx == nil {
		ctx = context.Background()
	}

	result, err := run.Check(ctx, run.CheckRequest{
		ToolVersion: version,
		Network:     *network,
		Endpoints:   *endpoints,
		Accounts:    ids,
		FailOn:      severity,
		Concurrency: concurrencyLimit,
		NewSource:   d.newSource,
		NewIdentity: d.newIdentity,
	})
	if err != nil {
		return reportError(stderr, err)
	}
	return render(stdout, stderr, result, *format)
}

// validateAccountArgs rejects everything but a G address without ever echoing
// the supplied value, which may be a secret seed.
func validateAccountArgs(ids []string) error {
	for _, id := range ids {
		kind, err := strkey.Classify(id)
		if err == nil {
			switch kind {
			case strkey.AccountID:
				continue
			case strkey.MuxedAccount, strkey.MuxedContract:
				return errors.New("muxed account addresses are not supported; pass the underlying G address")
			case strkey.Contract:
				return errors.New("contract addresses are not supported; pass a G account address")
			case strkey.Seed:
				return errors.New("a secret seed was supplied; remove it from your shell history and rotate that key")
			}
		}
		return errors.New("a supplied value is not a valid Stellar account address; expected a G address")
	}
	return nil
}

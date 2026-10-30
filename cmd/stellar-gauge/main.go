package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/posture-sec/guage/internal/model"
	"github.com/posture-sec/guage/internal/report"
	"github.com/posture-sec/guage/internal/run"
)

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func runCLI(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return int(model.ExitUsage)
	}
	switch args[0] {
	case "check":
		return checkCommand(args[1:], stdout, stderr)
	case "audit":
		return auditCommand(args[1:], stdout, stderr)
	case "version":
		fmt.Fprintf(stdout, "stellar-gauge %s\n", version)
		return int(model.ExitPass)
	case "help", "-h", "--help":
		usage(stdout)
		return int(model.ExitPass)
	default:
		fmt.Fprintf(stderr, "stellar-gauge: unknown command %q\n", args[0])
		usage(stderr)
		return int(model.ExitUsage)
	}
}

func usage(w io.Writer) {
	fmt.Fprint(w, `usage: stellar-gauge <command> [options]

commands:
  check     audit accounts named on the command line, with no policy file
  audit     audit the accounts declared in a policy file
  version   print the version

run "stellar-gauge check -h" or "stellar-gauge audit -h" for the options.
`)
}

// parseFailOn maps the CLI value to the severity at which a finding fails the
// run. The default is high, so a warning alone does not fail a scheduled job.
func parseFailOn(value string) (model.Severity, bool) {
	switch value {
	case "high":
		return model.SevHigh, true
	case "warn":
		return model.SevWarn, true
	default:
		return model.SevInfo, false
	}
}

func validFormat(value string) bool {
	return value == "text" || value == "json"
}

// reportError prints an orchestrator error and maps it to an exit code. Usage
// problems exit 2; everything else is a source or internal failure, exit 3.
func reportError(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "stellar-gauge: %v\n", err)
	if errors.Is(err, run.ErrUsage) {
		return int(model.ExitUsage)
	}
	return int(model.ExitSource)
}

// stringList collects a repeatable string flag.
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func (s *stringList) Len() int { return len(*s) }

func newFlagSet(name string, stderr io.Writer, usageLine string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, usageLine)
		fs.PrintDefaults()
	}
	return fs
}

func usageError(stderr io.Writer, message string) int {
	fmt.Fprintf(stderr, "stellar-gauge: %s\n", message)
	return int(model.ExitUsage)
}

func render(stdout, stderr io.Writer, result model.Report, format string) int {
	var err error
	if format == "json" {
		err = report.JSON(stdout, result)
	} else {
		err = report.Text(stdout, result)
	}
	if err != nil {
		fmt.Fprintf(stderr, "stellar-gauge: writing the report: %v\n", err)
		return int(model.ExitSource)
	}
	return int(result.ExitCode)
}

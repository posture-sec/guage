package main

import (
	"strings"
	"testing"
)

func TestFormatRejectionIsShared(t *testing.T) {
	for _, command := range []string{"check", "audit"} {
		for _, format := range []string{"yaml", ""} {
			t.Run(command+"/"+format, func(t *testing.T) {
				args := []string{command, "--format", format}
				if command == "check" {
					args = append(args, "--network", testNetwork, "--endpoint", "https://horizon.example", accountOne)
				}
				code, _, stderr := execCLI(t, args...)
				want := "stellar-gauge: --format must be text, json or markdown\n"
				if code != 2 || !strings.Contains(stderr, want) {
					t.Fatalf("exit/stderr = %d/%q, want exit 2 and %q", code, stderr, want)
				}
				if strings.Count(stderr, want) != 1 {
					t.Fatalf("format rejection was not one exact shared line: %q", stderr)
				}
			})
		}
	}
}

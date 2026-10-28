package main

import (
	"fmt"
	"os"

	"github.com/posture-sec/guage/internal/model"
)

func main() {
	fmt.Fprintln(os.Stderr, "stellar-gauge: scaffold only; no account posture checks were performed.")
	os.Exit(int(model.ExitSource))
}

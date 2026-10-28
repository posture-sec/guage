package model

type ExitCode int

const (
	ExitPass     ExitCode = 0
	ExitFindings ExitCode = 1
	ExitUsage    ExitCode = 2
	ExitSource   ExitCode = 3
)

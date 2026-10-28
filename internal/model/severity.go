package model

type Severity int

const (
	SevInfo Severity = iota
	SevWarn
	SevHigh
)

package main

func formatRejection(value string) string {
	if value == "markdown" {
		return "--format markdown output is not implemented yet"
	}
	return "--format must be text, json or markdown"
}

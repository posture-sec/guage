package main

// formatRejection is the one line both commands print for a --format value they
// do not accept. Markdown had a separate "not implemented yet" message until
// task 5.3 shipped the renderer, so the text names all three accepted values and
// no longer implies that one of them is missing.
func formatRejection() string {
	return "--format must be text, json or markdown"
}

package strkey

// IsSeed reports whether s is a valid ed25519 secret seed. It never returns,
// prints or stores the value: a caller that gets true must reject the input
// with exit 2 and a rotate-the-key message.
func IsSeed(s string) bool {
	kind, err := Classify(s)
	return err == nil && kind == Seed
}

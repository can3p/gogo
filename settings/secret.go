package settings

import "log/slog"

const redacted = "[redacted]"

// Secret is a string setting that doesn't print: fmt, slog and JSON show it
// redacted. Reveal returns the value.
type Secret string

// Reveal returns the secret's value, for the one place that needs it.
func (s Secret) Reveal() string {
	return string(s)
}

// String redacts a set secret. An empty one prints as empty, so a dump of the
// settings still shows which secrets are missing.
func (s Secret) String() string {
	if s == "" {
		return ""
	}

	return redacted
}

// GoString redacts the secret for %#v.
func (s Secret) GoString() string {
	return `"` + s.String() + `"`
}

// LogValue redacts the secret for slog.
func (s Secret) LogValue() slog.Value {
	return slog.StringValue(s.String())
}

// MarshalText redacts the secret for encoding/json and other text encoders.
func (s Secret) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

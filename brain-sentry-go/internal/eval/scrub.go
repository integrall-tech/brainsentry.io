package eval

import "regexp"

// ScrubQuery removes obvious PII / secrets from a captured query before it
// lands on disk. Conservative — when in doubt redact. The goal is "the
// operator can post the eval bundle on a public PR" not "the operator can
// share with their lawyer."
//
// Patterns mirror service/pii.go but are *redactions* not maskings: we keep
// shape (e.g. `[email]`) so the query still parses and replays.
var scrubPatterns = []struct {
	pattern *regexp.Regexp
	repl    string
}{
	{regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`), "[email]"},
	{regexp.MustCompile(`(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`), "[phone]"},
	{regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`), "[ssn]"},
	{regexp.MustCompile(`\b(?:\d{4}[-\s]?){3}\d{4}\b`), "[card]"},
	{regexp.MustCompile(`(?i)(?:api[_-]?key|apikey|secret_key|bearer)\s*[=:]\s*["']?[a-zA-Z0-9_\-]{16,}["']?`), "[apikey]"},
	{regexp.MustCompile(`eyJ[a-zA-Z0-9_-]+\.eyJ[a-zA-Z0-9_-]+\.[a-zA-Z0-9_-]+`), "[jwt]"},
	{regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`), "[ip]"},
	{regexp.MustCompile(`-----BEGIN[^-]+PRIVATE KEY-----[\s\S]*?-----END[^-]+PRIVATE KEY-----`), "[private_key]"},
}

// ScrubQuery returns a redacted copy of q.
func ScrubQuery(q string) string {
	out := q
	for _, p := range scrubPatterns {
		out = p.pattern.ReplaceAllString(out, p.repl)
	}
	return out
}

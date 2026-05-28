package crossmodal

import (
	"encoding/json"
	"strings"
)

// RepairJSON tries to coerce a model's reply into a parseable JSON object.
// LLMs love to:
//   - wrap their JSON in ```json fences
//   - add a trailing comma
//   - prepend a friendly "Sure, here is the json:"
//
// We strip the obvious noise and try again. On success returns the bytes
// that successfully parsed; on failure returns the (best-effort) bytes and
// the error so the caller can attribute the failure cleanly.
func RepairJSON(raw string) ([]byte, error) {
	candidates := []string{
		raw,
		stripFence(raw),
		extractFirstObject(raw),
		removeTrailingCommas(stripFence(raw)),
		removeTrailingCommas(extractFirstObject(raw)),
	}
	var lastErr error
	for _, c := range candidates {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		var probe any
		if err := json.Unmarshal([]byte(c), &probe); err == nil {
			return []byte(c), nil
		} else {
			lastErr = err
		}
	}
	return []byte(raw), lastErr
}

// stripFence removes surrounding ```json ... ``` (or just ``` ... ```)
// fences if present. Conservative — only the *first* code block.
func stripFence(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return s
	}
	t = strings.TrimPrefix(t, "```json")
	t = strings.TrimPrefix(t, "```")
	if i := strings.LastIndex(t, "```"); i >= 0 {
		t = t[:i]
	}
	return t
}

// extractFirstObject grabs the substring from the first '{' to the matching
// closing '}'. Naive bracket counter; ignores braces inside strings.
func extractFirstObject(s string) string {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:] // unbalanced — let json.Unmarshal report it
}

// removeTrailingCommas turns ",}" into "}" and ",]" into "]". Cheap loop;
// safe to call on input that doesn't have the issue.
func removeTrailingCommas(s string) string {
	out := strings.ReplaceAll(s, ",}", "}")
	out = strings.ReplaceAll(out, ", }", "}")
	out = strings.ReplaceAll(out, ",]", "]")
	out = strings.ReplaceAll(out, ", ]", "]")
	return out
}

// ParseJudgement parses the prompt-shaped reply that scorers are asked to
// produce: {"scores":[{"dim":"correctness","value":9,"comment":"..."}, ...]}
//
// Models are flaky about wrapping the structure — RepairJSON is run first.
// The model name and OK flag are filled by the caller after parsing.
func ParseJudgement(model, raw string) Judgement {
	repaired, err := RepairJSON(raw)
	if err != nil {
		return Judgement{Model: model, OK: false, Detail: "unparseable JSON: " + truncate(raw, 200)}
	}
	var body struct {
		Scores []Score `json:"scores"`
	}
	if err := json.Unmarshal(repaired, &body); err != nil {
		return Judgement{Model: model, OK: false, Detail: "JSON shape mismatch: " + err.Error()}
	}
	if len(body.Scores) == 0 {
		return Judgement{Model: model, OK: false, Detail: "scorer returned no scores"}
	}
	return Judgement{Model: model, OK: true, Scores: body.Scores}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

package crossmodal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Scorer is the interface a vendor adapter implements. The adapter is
// responsible for talking to its provider, formatting the prompt, and
// returning the raw model reply (which RepairJSON + ParseJudgement will
// then coerce). Returning ("", err) is fine — the runner wraps err into a
// not-OK Judgement with the err in Detail.
type Scorer interface {
	Name() string
	Score(ctx context.Context, task, output string) (string, error)
}

// Run fans out task+output to every scorer (in parallel) and returns the
// aggregated Result. Per-call timeout is enforced individually so a slow
// vendor cannot stall the gate.
func Run(ctx context.Context, scorers []Scorer, task, output string, perCallTimeout time.Duration) Result {
	if perCallTimeout <= 0 {
		perCallTimeout = 30 * time.Second
	}
	judgements := make([]Judgement, len(scorers))
	var wg sync.WaitGroup
	for i, s := range scorers {
		wg.Add(1)
		go func(i int, s Scorer) {
			defer wg.Done()
			cctx, cancel := context.WithTimeout(ctx, perCallTimeout)
			defer cancel()
			raw, err := s.Score(cctx, task, output)
			if err != nil {
				judgements[i] = Judgement{Model: s.Name(), OK: false, Detail: err.Error()}
				return
			}
			j := ParseJudgement(s.Name(), raw)
			judgements[i] = j
		}(i, s)
	}
	wg.Wait()
	return Aggregate(judgements)
}

// Receipt is the persistable artifact a CI system commits / archives. The
// file name encodes a hash of (task,output) so a re-run on the same input
// is idempotent and the human reviewer can spot intentional re-scores.
type Receipt struct {
	Slug      string `json:"slug"`
	Sha256    string `json:"sha256"`
	Task      string `json:"task"`
	Output    string `json:"output"`
	Result    Result `json:"result"`
}

// SaveReceipt writes the receipt as JSON under dir. Returns the full path.
// dir is created if missing. The file name is `<slug>-<sha8>.json` to
// match gbrain's convention.
func SaveReceipt(dir, slug, task, output string, result Result) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(task + "\x00" + output))
	hexHash := hex.EncodeToString(hash[:])
	short := hexHash[:8]
	rcpt := Receipt{
		Slug:   slug,
		Sha256: hexHash,
		Task:   task,
		Output: output,
		Result: result,
	}
	name := fmt.Sprintf("%s-%s.json", safeSlug(slug), short)
	full := filepath.Join(dir, name)
	f, err := os.Create(full)
	if err != nil {
		return "", err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rcpt); err != nil {
		return "", err
	}
	return full, nil
}

// LoadReceipt reads a JSON receipt back. Useful when CI wants to compare
// the latest run with a committed historical one.
func LoadReceipt(r io.Reader) (Receipt, error) {
	var rcpt Receipt
	if err := json.NewDecoder(r).Decode(&rcpt); err != nil {
		return Receipt{}, err
	}
	return rcpt, nil
}

// safeSlug strips characters that would be awkward in a filename. We don't
// try to be exhaustive — operators choose slugs.
func safeSlug(s string) string {
	if s == "" {
		return "untitled"
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			out = append(out, c)
		case c == '-' || c == '_':
			out = append(out, c)
		default:
			out = append(out, '-')
		}
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return string(out)
}

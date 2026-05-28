package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/integraltech/brainsentry/internal/eval"
	"github.com/spf13/cobra"
)

// EvalOptions controls the `brainsentry eval` family.
type EvalOptions struct {
	ServerURL  string
	Threshold  float64
	JSON       bool
	OutputPath string // for export; "" => stdout
	InputPath  string // for replay
	K          int    // override per-candidate k when running on a fresh build
	Writer     io.Writer
	HTTP       *http.Client
}

func newEvalCmd(_ *App) *cobra.Command {
	opts := &EvalOptions{Writer: os.Stdout, HTTP: &http.Client{Timeout: 60 * time.Second}}

	cmd := &cobra.Command{
		Use:   "eval",
		Short: "Capture / export / replay retrieval queries to gate regressions",
		Long: `eval is the honest way to gate retrieval changes (spreading activation,
query expansion, source boosting) against real traffic.

Workflow:
  1. Set BRAINSENTRY_EVAL_CAPTURE=1 in the server env so search calls land
     in the in-memory candidate buffer.
  2. brainsentry eval export > baseline.ndjson  (committable artifact)
  3. After a code change, brainsentry eval replay baseline.ndjson
     reports mean_jaccard@k and exits 1 if below --threshold.

Schema is versioned (schema_version 1); replay refuses unknown versions
so a future-baseline can never silently produce nonsense scores.`,
	}

	exportCmd := &cobra.Command{
		Use:   "export",
		Short: "Export the captured candidate buffer as NDJSON",
		RunE: func(c *cobra.Command, _ []string) error {
			body, err := getJSON(c.Context(), opts.ServerURL+"/v1/eval/candidates.ndjson", opts.HTTP)
			if err != nil {
				return err
			}
			out := opts.Writer
			if opts.OutputPath != "" {
				f, err := os.Create(opts.OutputPath)
				if err != nil {
					return err
				}
				defer f.Close()
				out = f
			}
			_, err = out.Write(body)
			return err
		},
	}

	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show capture status (count + whether the env flag is on)",
		RunE: func(c *cobra.Command, _ []string) error {
			body, err := getJSON(c.Context(), opts.ServerURL+"/v1/eval/candidates/stats", opts.HTTP)
			if err != nil {
				return err
			}
			fmt.Fprintln(opts.Writer, strings.TrimSpace(string(body)))
			return nil
		},
	}

	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Empty the in-memory candidate buffer (after a successful export)",
		RunE: func(c *cobra.Command, _ []string) error {
			req, err := http.NewRequestWithContext(c.Context(), http.MethodPost, opts.ServerURL+"/v1/eval/candidates/reset", nil)
			if err != nil {
				return err
			}
			resp, err := opts.HTTP.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("HTTP %d", resp.StatusCode)
			}
			fmt.Fprintln(opts.Writer, "candidate buffer reset")
			return nil
		},
	}

	replayCmd := &cobra.Command{
		Use:   "replay [baseline.ndjson]",
		Short: "Replay a baseline against the current build and report jaccard@k",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			f, err := os.Open(args[0])
			if err != nil {
				return err
			}
			defer f.Close()
			cands, err := eval.LoadCandidates(f)
			if err != nil {
				return fmt.Errorf("load %s: %w", args[0], err)
			}
			ctx, cancel := context.WithTimeout(c.Context(), 5*time.Minute)
			defer cancel()
			fn := buildReplayQueryFn(opts.ServerURL, opts.HTTP)
			sum := eval.Run(ctx, cands, fn)

			if opts.JSON {
				_ = json.NewEncoder(opts.Writer).Encode(sum)
			} else {
				fmt.Fprint(opts.Writer, sum.FormatText())
			}
			if !sum.Pass(opts.Threshold) {
				os.Exit(1)
			}
			return nil
		},
	}

	for _, c := range []*cobra.Command{exportCmd, statsCmd, resetCmd, replayCmd} {
		c.Flags().StringVar(&opts.ServerURL, "server", "http://localhost:8080", "brainsentry server base URL")
	}
	exportCmd.Flags().StringVar(&opts.OutputPath, "out", "", "write to file instead of stdout")
	replayCmd.Flags().Float64Var(&opts.Threshold, "threshold", 0.85, "minimum mean jaccard required to PASS")
	replayCmd.Flags().BoolVar(&opts.JSON, "json", false, "emit JSON summary instead of text")

	cmd.AddCommand(exportCmd, statsCmd, resetCmd, replayCmd, newCrossModalCmd(nil))
	return cmd
}

// buildReplayQueryFn returns the closure Run() uses to query the live
// server. We call POST /v1/memories/search with the same query+k a baseline
// would have produced, then extract the result IDs.
func buildReplayQueryFn(baseURL string, cli *http.Client) eval.QueryFn {
	return func(ctx context.Context, query string, k int) ([]string, time.Duration, error) {
		body := []byte(`{"query":` + jsonStringLit(query) + `,"limit":` + itoa(k) + `}`)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/memories/search", strings.NewReader(string(body)))
		if err != nil {
			return nil, 0, err
		}
		req.Header.Set("Content-Type", "application/json")
		t0 := time.Now()
		resp, err := cli.Do(req)
		if err != nil {
			return nil, 0, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, 0, fmt.Errorf("HTTP %d", resp.StatusCode)
		}
		var sr struct {
			Results []struct {
				ID string `json:"id"`
			} `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
			return nil, 0, err
		}
		ids := make([]string, 0, len(sr.Results))
		for _, r := range sr.Results {
			ids = append(ids, r.ID)
		}
		return ids, time.Since(t0), nil
	}
}

// jsonStringLit escapes a string for JSON literal embedding without pulling
// in an encoder allocation. Mirrors the helper in internal/models/probe.go;
// duplicated here to avoid an import cycle.
func jsonStringLit(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

func itoa(n int) string { return fmt.Sprintf("%d", n) }

package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/integraltech/brainsentry/internal/eval/crossmodal"
	"github.com/spf13/cobra"
)

// CrossModalOptions controls `brainsentry eval cross-modal`.
type CrossModalOptions struct {
	TaskFile    string
	OutputFile  string
	Slug        string
	ReceiptsDir string
	JSON        bool
	Writer      io.Writer

	// Scorer factory — overridable in tests so we don't have to hit live
	// providers. Production resolves this from runtime config (TODO once
	// AnthropicProvider/GeminiProvider land in service/).
	BuildScorers func() ([]crossmodal.Scorer, error)
}

func newCrossModalCmd(_ *App) *cobra.Command {
	opts := &CrossModalOptions{Writer: os.Stdout}
	cmd := &cobra.Command{
		Use:   "cross-modal",
		Short: "Score an OUTPUT against a TASK using 3 vendor models in parallel",
		Long: `cross-modal is a quality gate for prompt outputs (compression results,
extraction summaries, generated code). Three independent models score
the same OUTPUT against the same TASK across five dimensions
(correctness, completeness, faithfulness, format, safety).

Pass:         when (>=2 models returned valid JSON) AND
              (every dim mean >= 7) AND (every dim min >= 5)
Fail:         when threshold check above fails despite 2+ valid voters
Inconclusive: when fewer than 2 voters returned valid JSON

Receipts are written to ~/.brainsentry/eval-receipts/<slug>-<sha8>.json
so CI can diff scores between runs and operators can attach a
human-readable artifact to PRs.`,
		RunE: func(c *cobra.Command, _ []string) error {
			task, err := readTextArg(opts.TaskFile)
			if err != nil {
				return fmt.Errorf("read --task: %w", err)
			}
			output, err := readTextArg(opts.OutputFile)
			if err != nil {
				return fmt.Errorf("read --output: %w", err)
			}
			if opts.BuildScorers == nil {
				return fmt.Errorf("no scorers wired (cross-modal requires Anthropic/OpenAI/Gemini providers — coming with the multi-provider PR)")
			}
			scorers, err := opts.BuildScorers()
			if err != nil {
				return err
			}
			ctx, cancel := context.WithTimeout(c.Context(), 90*time.Second)
			defer cancel()
			res := crossmodal.Run(ctx, scorers, task, output, 30*time.Second)

			if opts.JSON {
				_ = json.NewEncoder(opts.Writer).Encode(res)
			} else {
				renderCrossModal(opts.Writer, res)
			}

			if opts.ReceiptsDir == "" {
				home, _ := os.UserHomeDir()
				opts.ReceiptsDir = filepath.Join(home, ".brainsentry", "eval-receipts")
			}
			if path, err := crossmodal.SaveReceipt(opts.ReceiptsDir, opts.Slug, task, output, res); err == nil {
				fmt.Fprintf(opts.Writer, "\nreceipt: %s\n", path)
			} else {
				fmt.Fprintf(opts.Writer, "\nreceipt save failed: %v\n", err)
			}

			if res.Verdict == crossmodal.VerdictFail {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.TaskFile, "task", "", "task description (string or @path/to/file)")
	cmd.Flags().StringVar(&opts.OutputFile, "output", "", "output to evaluate (string or @path/to/file)")
	cmd.Flags().StringVar(&opts.Slug, "slug", "untitled", "short identifier used in the receipt filename")
	cmd.Flags().StringVar(&opts.ReceiptsDir, "receipts-dir", "", "where to write the receipt JSON (default ~/.brainsentry/eval-receipts)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "emit JSON result instead of human-readable text")
	_ = cmd.MarkFlagRequired("task")
	_ = cmd.MarkFlagRequired("output")
	return cmd
}

// readTextArg accepts either a literal string or "@path/to/file" pointing
// to a file whose contents should be read. Mirrors curl's @-prefix
// convention so muscle memory transfers.
func readTextArg(arg string) (string, error) {
	if len(arg) > 0 && arg[0] == '@' {
		b, err := os.ReadFile(arg[1:])
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return arg, nil
}

func renderCrossModal(w io.Writer, r crossmodal.Result) {
	fmt.Fprintf(w, "brainsentry cross-modal — %s\n", r.Verdict)
	fmt.Fprintf(w, "  reason: %s\n", r.Reason)
	fmt.Fprintf(w, "  voters: %d/%d returned valid JSON\n", r.OKCount, r.Total)
	fmt.Fprintln(w, "  dimensions:")
	for _, d := range r.Dimensions {
		fmt.Fprintf(w, "    %-14s mean=%.2f  min=%d  max=%d  (n=%d)\n", d.Dim, d.Mean, d.Min, d.Max, d.Count)
	}
	fmt.Fprintln(w, "  judges:")
	for _, j := range r.Judgements {
		mark := "OK"
		if !j.OK {
			mark = "FAIL"
		}
		fmt.Fprintf(w, "    [%s] %s", mark, j.Model)
		if j.Detail != "" {
			fmt.Fprintf(w, "  — %s", j.Detail)
		}
		fmt.Fprintln(w)
	}
}

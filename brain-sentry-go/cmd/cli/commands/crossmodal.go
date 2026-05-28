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
	"github.com/integraltech/brainsentry/internal/eval/crossmodal/wire"
	"github.com/integraltech/brainsentry/internal/service"
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

	// Model selection per vendor. Empty disables that vendor; at least 2
	// vendors must be configured or the gate refuses to run (Aggregate
	// classifies <2 OK voters as Inconclusive).
	AnthropicModel string
	GeminiModel    string
	OpenRouterModel string

	// Scorer factory — overridable in tests so we don't have to hit live
	// providers. Production resolves vendors from --anthropic-model /
	// --gemini-model / --openrouter-model flags + env-var API keys.
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
				opts.BuildScorers = defaultBuildScorers(opts)
			}
			scorers, err := opts.BuildScorers()
			if err != nil {
				return err
			}
			if len(scorers) < 2 {
				return fmt.Errorf("cross-modal requires at least 2 configured vendors (got %d) — set ANTHROPIC_API_KEY, GEMINI_API_KEY, OPENROUTER_API_KEY and pass --anthropic-model / --gemini-model / --openrouter-model", len(scorers))
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
	cmd.Flags().StringVar(&opts.AnthropicModel, "anthropic-model", "", "Anthropic model id (uses ANTHROPIC_API_KEY); empty disables")
	cmd.Flags().StringVar(&opts.GeminiModel, "gemini-model", "", "Gemini model id (uses GEMINI_API_KEY); empty disables")
	cmd.Flags().StringVar(&opts.OpenRouterModel, "openrouter-model", "", "OpenRouter model id (uses OPENROUTER_API_KEY); empty disables")
	_ = cmd.MarkFlagRequired("task")
	_ = cmd.MarkFlagRequired("output")
	return cmd
}

// defaultBuildScorers wires Anthropic / Gemini / OpenRouter providers from
// env-var API keys + --*-model flags. A vendor is included only when BOTH
// the model flag is non-empty AND its API key env is set; otherwise it
// quietly drops out of the slate. Aggregate enforces the 2-voter floor.
func defaultBuildScorers(opts *CrossModalOptions) func() ([]crossmodal.Scorer, error) {
	return func() ([]crossmodal.Scorer, error) {
		providers := make([]service.LLMProvider, 0, 3)
		displayNames := make([]string, 0, 3)
		if k := os.Getenv("ANTHROPIC_API_KEY"); k != "" && opts.AnthropicModel != "" {
			cfg := service.DefaultAnthropicConfig(k)
			cfg.Model = opts.AnthropicModel
			providers = append(providers, service.NewAnthropicProvider(cfg))
			displayNames = append(displayNames, "anthropic/"+opts.AnthropicModel)
		}
		if k := os.Getenv("GEMINI_API_KEY"); k != "" && opts.GeminiModel != "" {
			cfg := service.DefaultGeminiConfig(k)
			cfg.Model = opts.GeminiModel
			providers = append(providers, service.NewGeminiProvider(cfg))
			displayNames = append(displayNames, "google/"+opts.GeminiModel)
		}
		if k := os.Getenv("OPENROUTER_API_KEY"); k != "" && opts.OpenRouterModel != "" {
			or := service.NewOpenRouterService(k, "", opts.OpenRouterModel, 0.7, 4096, 60*time.Second, 0)
			providers = append(providers, service.NewOpenRouterProvider(or))
			displayNames = append(displayNames, "openrouter/"+opts.OpenRouterModel)
		}
		return wire.Scorers(providers, displayNames), nil
	}
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

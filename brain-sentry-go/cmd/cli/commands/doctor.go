package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/integraltech/brainsentry/internal/diagnostics"
	"github.com/spf13/cobra"
)

// DoctorOptions controls how the doctor sub-command behaves.
type DoctorOptions struct {
	JSON      bool
	Fast      bool
	ServerURL string // hits a running brainsentry server's /v1/diagnostics
	Writer    io.Writer
	HTTP      *http.Client
}

func newDoctorCmd(_ *App) *cobra.Command {
	opts := &DoctorOptions{Writer: os.Stdout, HTTP: &http.Client{Timeout: 15 * time.Second}}
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run health & self-check probes against the local environment or a running server",
		Long: `doctor runs a battery of subsystem probes (Postgres, FalkorDB, Redis,
LLM provider, schema version, embedding coverage, ...) and reports the
aggregate health.

Two modes:
  - default: hits the configured --server URL (operates on a running brainsentry).
  - --offline (future): runs the probes in-process from the CLI host.

Exit code 0 if all checks pass (or warn-only); 1 if any check fails.
The JSON shape is stable for CI gating.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			rep, err := fetchRemoteReport(ctx, opts.ServerURL, opts.HTTP)
			if err != nil {
				return fmt.Errorf("fetch diagnostics: %w", err)
			}
			renderReport(opts.Writer, rep, opts.JSON)
			if rep.ExitCode() != 0 {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "emit JSON instead of human-readable text")
	cmd.Flags().BoolVar(&opts.Fast, "fast", false, "skip slower checks (placeholder for future use)")
	cmd.Flags().StringVar(&opts.ServerURL, "server", "http://localhost:8080", "brainsentry server base URL")
	return cmd
}

func fetchRemoteReport(ctx context.Context, baseURL string, cli *http.Client) (diagnostics.Report, error) {
	url := baseURL + "/v1/diagnostics"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return diagnostics.Report{}, err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return diagnostics.Report{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return diagnostics.Report{}, fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}
	var rep diagnostics.Report
	if err := json.NewDecoder(resp.Body).Decode(&rep); err != nil {
		return diagnostics.Report{}, err
	}
	return rep, nil
}

func renderReport(w io.Writer, rep diagnostics.Report, asJSON bool) {
	if asJSON {
		_ = json.NewEncoder(w).Encode(rep)
		return
	}
	fmt.Fprint(w, rep.FormatText())
}

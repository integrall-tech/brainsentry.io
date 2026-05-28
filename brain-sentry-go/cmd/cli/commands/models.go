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

	"github.com/integraltech/brainsentry/internal/models"
	"github.com/spf13/cobra"
)

// ModelsOptions controls the `brainsentry models` family.
type ModelsOptions struct {
	JSON      bool
	ServerURL string
	Writer    io.Writer
	HTTP      *http.Client
}

func newModelsCmd(_ *App) *cobra.Command {
	opts := &ModelsOptions{Writer: os.Stdout, HTTP: &http.Client{Timeout: 30 * time.Second}}
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Inspect tier-routing config and probe model availability",
		Long: `models shows which model each tier resolves to and (with the
'doctor' subcommand) burns a single token against each model to verify the
provider, key and routing actually work end-to-end.

A failing probe is classified into one of:
  model_not_found | auth | rate_limit | network | timeout | invalid_request | unknown

Doctor exits 1 if any tier fails. List exits 1 if any tier resolves to empty.`,
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "Show the resolved model for every tier",
		RunE: func(c *cobra.Command, _ []string) error {
			snap, err := fetchSnapshot(c.Context(), opts.ServerURL, opts.HTTP)
			if err != nil {
				return err
			}
			renderSnapshot(opts.Writer, snap, opts.JSON)
			for _, r := range snap {
				if r.Model == "" {
					os.Exit(1)
				}
			}
			return nil
		},
	}
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Probe each tier's model with a 1-token request",
		RunE: func(c *cobra.Command, _ []string) error {
			ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
			defer cancel()
			rep, err := fetchModelsDoctor(ctx, opts.ServerURL, opts.HTTP)
			if err != nil {
				return err
			}
			renderModelsDoctor(opts.Writer, rep, opts.JSON)
			if !rep.OK {
				os.Exit(1)
			}
			return nil
		},
	}

	for _, c := range []*cobra.Command{listCmd, doctorCmd} {
		c.Flags().BoolVar(&opts.JSON, "json", false, "emit JSON instead of text")
		c.Flags().StringVar(&opts.ServerURL, "server", "http://localhost:8080", "brainsentry server base URL")
	}
	cmd.AddCommand(listCmd, doctorCmd)
	return cmd
}

func fetchSnapshot(ctx context.Context, baseURL string, cli *http.Client) ([]models.ResolveResult, error) {
	body, err := getJSON(ctx, baseURL+"/v1/models", cli)
	if err != nil {
		return nil, err
	}
	var out struct {
		Snapshot []models.ResolveResult `json:"snapshot"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out.Snapshot, nil
}

func fetchModelsDoctor(ctx context.Context, baseURL string, cli *http.Client) (models.DoctorReport, error) {
	body, err := getJSON(ctx, baseURL+"/v1/models/doctor", cli)
	if err != nil {
		return models.DoctorReport{}, err
	}
	var rep models.DoctorReport
	if err := json.Unmarshal(body, &rep); err != nil {
		return rep, err
	}
	return rep, nil
}

func getJSON(ctx context.Context, url string, cli *http.Client) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	const maxBody = 1 << 20
	buf := make([]byte, 0, 8<<10)
	tmp := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if len(buf) > maxBody {
			return nil, fmt.Errorf("response too large from %s", url)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}

func renderSnapshot(w io.Writer, snap []models.ResolveResult, asJSON bool) {
	if asJSON {
		_ = json.NewEncoder(w).Encode(map[string]any{"snapshot": snap})
		return
	}
	fmt.Fprintln(w, "TIER         MODEL                                 SOURCE")
	for _, r := range snap {
		model := r.Model
		if model == "" {
			model = "(unresolved)"
		}
		fmt.Fprintf(w, "%-12s %-37s %s\n", r.Tier, model, r.Source)
	}
}

func renderModelsDoctor(w io.Writer, rep models.DoctorReport, asJSON bool) {
	if asJSON {
		_ = json.NewEncoder(w).Encode(rep)
		return
	}
	status := "ok"
	if !rep.OK {
		status = "fail"
	}
	fmt.Fprintf(w, "brainsentry models doctor — %s (%dms)\n\n", status, rep.Duration.Milliseconds())
	for _, r := range rep.Results {
		mark := "PASS"
		if !r.OK {
			mark = "FAIL"
		}
		fmt.Fprintf(w, "[%s] %-12s %s (%dms)\n", mark, r.Tier, r.Model, r.Duration.Milliseconds())
		if r.Failure != "" {
			fmt.Fprintf(w, "       failure: %s\n", r.Failure)
		}
		if r.Detail != "" {
			fmt.Fprintf(w, "       detail:  %s\n", strings.TrimSpace(r.Detail))
		}
		if r.Hint != "" {
			fmt.Fprintf(w, "       hint:    %s\n", r.Hint)
		}
	}
}

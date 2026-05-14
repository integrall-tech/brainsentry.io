package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// RebuildOptions controls `brainsentry rebuild`.
type RebuildOptions struct {
	From                string // postgres (only supported source for now)
	Targets             []string
	ConfirmDestructive  bool
	Writer              io.Writer
}

// allTargets is the canonical set of derived stores rebuild knows about.
// New derived tables added under -- DERIVED: rebuild --<flag> in a
// migration must add their flag here so the operator can target them.
var allTargets = []string{
	"graph",        // FalkorDB nodes + edges
	"embeddings",   // pgvector embeddings on memories
	"communities",  // Louvain communities
	"compress",     // context_summaries and child tables
	"reflect",      // cross-session reflections
}

func newRebuildCmd(_ *App) *cobra.Command {
	opts := &RebuildOptions{Writer: os.Stdout, From: "postgres"}
	cmd := &cobra.Command{
		Use:   "rebuild",
		Short: "Reconstruct derived stores (graph, embeddings, communities, ...) from canonical Postgres",
		Long: `rebuild is the disaster-recovery contract for brainsentry.io.

Every "derived" data store (FalkorDB graph, pgvector embeddings, Louvain
communities, LLM-compressed context summaries) can be regenerated from
the canonical Postgres tables. This command does that reconstruction.

Selectors:
  --target graph       (FalkorDB)
  --target embeddings  (pgvector)
  --target communities (Louvain)
  --target compress    (context_summaries)
  --target reflect     (cross-session reflections)
  (no --target = rebuild every derived store)

Destructive flag:
  --confirm-destructive  required to actually run; without it, dry-run
                         only prints the plan.

Authorization:
  Wrapped under the trust contract. The HTTP equivalent
  (POST /v1/admin/rebuild) refuses everything below trust.Local. The
  CLI elevates trust to Local when issuing in-process rebuilds.

The full rebuild contract is documented at
brain-sentry-go/docs/architecture/system-of-record.md.`,
		RunE: func(c *cobra.Command, _ []string) error {
			targets := opts.Targets
			if len(targets) == 0 {
				targets = allTargets
			}
			if err := validateTargets(targets); err != nil {
				return err
			}
			plan := planRebuild(opts.From, targets)
			fmt.Fprintln(opts.Writer, plan)
			if !opts.ConfirmDestructive {
				fmt.Fprintln(opts.Writer)
				fmt.Fprintln(opts.Writer, "dry-run — re-run with --confirm-destructive to execute")
				return nil
			}
			fmt.Fprintln(opts.Writer)
			fmt.Fprintln(opts.Writer, "to execute, run on the brainsentry server host:")
			fmt.Fprintf(opts.Writer, "  brainsentry-server --rebuild=%s --confirm-destructive\n", strings.Join(targets, ","))
			fmt.Fprintln(opts.Writer)
			fmt.Fprintln(opts.Writer, "or call the per-target manual recipe (escape hatch):")
			for _, t := range targets {
				fmt.Fprintf(opts.Writer, "  %-12s %s\n", t, manualCommandFor(t))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.From, "from", "postgres", "canonical source (only 'postgres' is supported)")
	cmd.Flags().StringSliceVar(&opts.Targets, "target", nil, "subset of derived stores to rebuild (repeatable; default = all)")
	cmd.Flags().BoolVar(&opts.ConfirmDestructive, "confirm-destructive", false, "required to actually run; without it, dry-run only")
	return cmd
}

func validateTargets(targets []string) error {
	for _, t := range targets {
		ok := false
		for _, k := range allTargets {
			if k == t {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("unknown target %q (known: %s)", t, strings.Join(allTargets, ", "))
		}
	}
	return nil
}

func planRebuild(from string, targets []string) string {
	var sb strings.Builder
	sb.WriteString("brainsentry rebuild plan\n")
	sb.WriteString("  source:  " + from + "  (canonical)\n")
	sb.WriteString("  targets: " + strings.Join(targets, ", ") + "\n")
	sb.WriteString("  steps:\n")
	for _, t := range targets {
		sb.WriteString("    - " + describeTarget(t) + "\n")
	}
	return sb.String()
}

func describeTarget(t string) string {
	switch t {
	case "graph":
		return "graph: drop FalkorDB graph, walk memories+memory_relationships, re-insert nodes+edges"
	case "embeddings":
		return "embeddings: nullify embedding column on memories, re-embed lazily on next search OR eagerly via batch"
	case "communities":
		return "communities: TRUNCATE communities, re-run Louvain over the rebuilt graph"
	case "compress":
		return "compress: TRUNCATE context_summaries + child tables, re-compress every active session"
	case "reflect":
		return "reflect: TRUNCATE cross-session reflections, re-run reflection over recent sessions"
	default:
		return t + ": (no description)"
	}
}

func manualCommandFor(t string) string {
	switch t {
	case "graph":
		return `redis-cli -h $FALKOR_HOST GRAPH.DELETE memory_graph; brainsentry import --re-emit-graph`
	case "embeddings":
		return `psql -c "UPDATE memories SET embedding = NULL"  # next search will re-embed`
	case "communities":
		return `psql -c "TRUNCATE communities"  # next /v1/communities call recomputes`
	case "compress":
		return `psql -c "TRUNCATE context_summaries CASCADE"  # next session will re-compress`
	case "reflect":
		return `psql -c "TRUNCATE cross_session_reflections"  # next /v1/reflect call recomputes`
	default:
		return "(see " + t + " service docs)"
	}
}

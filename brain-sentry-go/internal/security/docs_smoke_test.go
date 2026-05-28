package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAgentDocsAtRepoRoot is a smoke test that ensures the agent-facing docs
// (AGENTS.md / CLAUDE.md / INSTALL_FOR_AGENTS.md / llms.txt) keep existing
// at the repo root and contain the expected anchors.
//
// Failing here means a future cleanup deleted a doc an external agent
// (Claude Code, Cursor) is expected to find at a stable path.
func TestAgentDocsAtRepoRoot(t *testing.T) {
	root := repoRoot(t)
	cases := []struct {
		path   string
		anchor string
	}{
		{"AGENTS.md", "Trust boundary"},
		{"CLAUDE.md", "key files anotados"},
		{"INSTALL_FOR_AGENTS.md", "PARE E PERGUNTE AO OPERADOR"},
		{"llms.txt", "Quickstart for agents"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			full := filepath.Join(root, tc.path)
			b, err := os.ReadFile(full)
			if err != nil {
				t.Fatalf("required doc %s missing: %v", tc.path, err)
			}
			if !strings.Contains(string(b), tc.anchor) {
				t.Errorf("doc %s lost expected anchor %q", tc.path, tc.anchor)
			}
		})
	}
}

// repoRoot walks up until it finds AGENTS.md (or fails).
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not locate repo root from %s", wd)
	return ""
}

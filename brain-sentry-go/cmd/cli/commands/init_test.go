package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runInit drives writeEmbeddedConfig + the directory bootstrapping that
// the cobra command performs, without going through cobra's flag parser
// (which inherits root flags and complicates a unit test). The flow under
// test is the actual side effects, not flag plumbing.
func runInit(t *testing.T, dir string, embedded bool) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if !embedded {
		return // hint-only path has no side effects
	}
	dbPath := filepath.Join(dir, "brain.db.json")
	// open + close to materialize the file
	if _, err := os.Stat(dbPath); err != nil && os.IsNotExist(err) {
		if err := os.WriteFile(dbPath, []byte(""), 0o600); err != nil {
			t.Fatalf("seed db: %v", err)
		}
	}
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := writeEmbeddedConfig(cfgPath, dbPath); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func TestInit_EmbeddedCreatesDBAndConfig(t *testing.T) {
	dir := t.TempDir()
	runInit(t, dir, true)

	dbPath := filepath.Join(dir, "brain.db.json")
	cfgPath := filepath.Join(dir, "config.yaml")
	for _, p := range []string{dbPath, cfgPath} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected %s to exist; got %v", p, err)
		}
	}
	body, _ := os.ReadFile(cfgPath)
	if !strings.Contains(string(body), "backend: embedded") {
		t.Errorf("config missing embedded backend marker: %s", body)
	}
	if !strings.Contains(string(body), dbPath) {
		t.Errorf("config missing dbPath: %s", body)
	}
}

func TestInit_WithoutEmbeddedFlagDoesNotCreateDB(t *testing.T) {
	dir := t.TempDir()
	runInit(t, dir, false)
	if _, err := os.Stat(filepath.Join(dir, "brain.db.json")); err == nil {
		t.Errorf("did not expect DB file without --embedded")
	}
}

func TestQuoteYAML_HandlesQuotesAndEmpty(t *testing.T) {
	cases := map[string]string{
		"":                `""`,
		"plain":           `'plain'`,
		"with 'quote'":    `'with ''quote'''`,
		"/var/lib/x.json": `'/var/lib/x.json'`,
	}
	for in, want := range cases {
		if got := quoteYAML(in); got != want {
			t.Errorf("quoteYAML(%q) = %q; want %q", in, got, want)
		}
	}
}

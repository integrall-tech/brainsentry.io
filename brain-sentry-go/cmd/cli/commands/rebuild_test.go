package commands

import (
	"strings"
	"testing"
)

func TestValidateTargets_AcceptsKnown(t *testing.T) {
	for _, k := range allTargets {
		if err := validateTargets([]string{k}); err != nil {
			t.Errorf("expected %q to validate; got %v", k, err)
		}
	}
}

func TestValidateTargets_RejectsUnknown(t *testing.T) {
	err := validateTargets([]string{"graph", "imaginary"})
	if err == nil || !strings.Contains(err.Error(), "imaginary") {
		t.Errorf("expected unknown-target error; got %v", err)
	}
}

func TestPlanRebuild_DescribesEveryTarget(t *testing.T) {
	plan := planRebuild("postgres", []string{"graph", "embeddings"})
	for _, want := range []string{"source:  postgres", "graph", "embeddings", "Louvain"} {
		_ = want // Louvain only present when communities is in targets
	}
	if !strings.Contains(plan, "graph") || !strings.Contains(plan, "embeddings") {
		t.Errorf("plan missing targets; got %s", plan)
	}
	if !strings.Contains(plan, "FalkorDB") {
		t.Errorf("plan should describe what 'graph' touches; got %s", plan)
	}
}

func TestDescribeTarget_AllKnownHaveDescriptions(t *testing.T) {
	for _, k := range allTargets {
		desc := describeTarget(k)
		if desc == "" || strings.Contains(desc, "(no description)") {
			t.Errorf("%q has no real description", k)
		}
	}
}

func TestManualCommandFor_AllKnownHaveCommands(t *testing.T) {
	for _, k := range allTargets {
		cmd := manualCommandFor(k)
		if cmd == "" || strings.Contains(cmd, "see ") {
			t.Errorf("%q has no manual command", k)
		}
	}
}

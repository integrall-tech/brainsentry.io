package models

import "testing"

func TestFromYAML_FallsBackToLegacyAIModel(t *testing.T) {
	cfg := FromYAML("", nil, "anthropic/claude-fallback")
	if cfg.Default != "anthropic/claude-fallback" {
		t.Errorf("expected legacy AI.Model used as Default; got %q", cfg.Default)
	}
}

func TestFromYAML_KeepsExplicitDefault(t *testing.T) {
	cfg := FromYAML("custom-default", nil, "anthropic/claude-fallback")
	if cfg.Default != "custom-default" {
		t.Errorf("expected explicit default; got %q", cfg.Default)
	}
}

func TestFromYAML_DropsUnknownTiers(t *testing.T) {
	cfg := FromYAML("d", map[string]string{
		"utility":   "ut",
		"reasoning": "re",
		"bogus":     "x",
	}, "")
	if cfg.Tier[TierUtility] != "ut" {
		t.Errorf("utility lost")
	}
	if cfg.Tier[TierReasoning] != "re" {
		t.Errorf("reasoning lost")
	}
	if _, ok := cfg.Tier[Tier("bogus")]; ok {
		t.Errorf("bogus tier should be dropped")
	}
}

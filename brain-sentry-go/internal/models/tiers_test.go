package models

import (
	"strings"
	"testing"
)

func TestResolve_RespectsResolutionOrder(t *testing.T) {
	cfg := Config{
		Default: "default-model",
		Tier: map[Tier]string{
			TierUtility:   "tier-util",
			TierReasoning: "tier-reason",
		},
	}

	cases := []struct {
		name       string
		tier       Tier
		opts       []Option
		envKey     string
		envVal     string
		wantModel  string
		wantSource string
	}{
		{name: "cli flag wins", tier: TierUtility,
			opts: []Option{WithCLIFlag("from-cli"), WithOverride("from-override"), WithFallback("fb")},
			wantModel: "from-cli", wantSource: "cli"},
		{name: "override beats config", tier: TierUtility,
			opts: []Option{WithOverride("from-override")},
			wantModel: "from-override", wantSource: "override"},
		{name: "tier config beats default config", tier: TierUtility,
			wantModel: "tier-util", wantSource: "config-tier"},
		{name: "default config when tier empty", tier: TierDeep,
			wantModel: "default-model", wantSource: "config-default"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := Resolve(cfg, tc.tier, tc.opts...)
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			if r.Model != tc.wantModel {
				t.Errorf("expected model %q; got %q", tc.wantModel, r.Model)
			}
			if r.Source != tc.wantSource {
				t.Errorf("expected source %q; got %q", tc.wantSource, r.Source)
			}
		})
	}
}

func TestResolve_FallsThroughToEnv(t *testing.T) {
	t.Setenv("BRAINSENTRY_MODEL_REASONING", "env-reasoning")
	r, err := Resolve(Config{}, TierReasoning)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if r.Model != "env-reasoning" || r.Source != "env" {
		t.Errorf("expected env tier override; got %+v", r)
	}
}

func TestResolve_FallsThroughToDefaultEnv(t *testing.T) {
	t.Setenv("BRAINSENTRY_MODEL_DEFAULT", "env-default")
	r, err := Resolve(Config{}, TierUtility)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if r.Model != "env-default" || r.Source != "env" {
		t.Errorf("expected env default; got %+v", r)
	}
}

func TestResolve_TierDefaultsLastResort(t *testing.T) {
	r, err := Resolve(Config{}, TierUtility)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if r.Source != "tier-default" {
		t.Errorf("expected tier-default source; got %s", r.Source)
	}
	if r.Model == "" {
		t.Errorf("expected non-empty model")
	}
}

func TestResolve_CallerFallback(t *testing.T) {
	saved := TierDefaults
	t.Cleanup(func() { TierDefaults = saved })
	TierDefaults = map[Tier]string{}
	r, err := Resolve(Config{}, TierUtility, WithFallback("emergency"))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if r.Model != "emergency" || r.Source != "caller-fallback" {
		t.Errorf("expected caller fallback; got %+v", r)
	}
}

func TestResolve_NoModelAvailableErrors(t *testing.T) {
	saved := TierDefaults
	t.Cleanup(func() { TierDefaults = saved })
	TierDefaults = map[Tier]string{}
	_, err := Resolve(Config{}, TierUtility)
	if err == nil {
		t.Errorf("expected error when nothing resolves")
	}
}

func TestResolve_RejectsUnknownTier(t *testing.T) {
	_, err := Resolve(Config{Default: "x"}, Tier("imaginary"))
	if err == nil || !strings.Contains(err.Error(), "unknown tier") {
		t.Errorf("expected unknown-tier error; got %v", err)
	}
}

func TestSnapshot_CoversAllTiers(t *testing.T) {
	snap := Snapshot(Config{Default: "d"})
	if len(snap) != len(AllTiers()) {
		t.Errorf("expected %d entries; got %d", len(AllTiers()), len(snap))
	}
}

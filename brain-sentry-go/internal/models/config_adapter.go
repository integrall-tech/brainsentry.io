package models

// FromYAML converts the operator-facing YAML config (where keys are plain
// strings to keep yaml ergonomic) into the typed Config the resolver
// consumes. Unknown tier keys are silently dropped — Validate-on-resolve
// catches them when something actually asks for them.
//
// AI.Model is treated as a fallback for Default when Default is empty;
// this keeps existing config.yaml files (no `models:` section) working.
func FromYAML(rawDefault string, tier map[string]string, legacyAIModel string) Config {
	cfg := Config{Default: rawDefault, Tier: map[Tier]string{}}
	if cfg.Default == "" {
		cfg.Default = legacyAIModel
	}
	for k, v := range tier {
		t := Tier(k)
		if t.Validate() == nil {
			cfg.Tier[t] = v
		}
	}
	return cfg
}

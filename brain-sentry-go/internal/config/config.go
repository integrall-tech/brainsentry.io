package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Database     DatabaseConfig     `yaml:"database"`
	Redis        RedisConfig        `yaml:"redis"`
	FalkorDB     FalkorDBConfig     `yaml:"falkordb"`
	Security     SecurityConfig     `yaml:"security"`
	Tenant       TenantConfig       `yaml:"tenant"`
	AI           AIConfig           `yaml:"ai"`
	Anthropic    AnthropicProviderConfig `yaml:"anthropic"`
	Gemini       GeminiProviderConfig    `yaml:"gemini"`
	Models       ModelsConfig       `yaml:"models"`
	Store        StoreConfig        `yaml:"store"`
	Embedding    EmbeddingConfig    `yaml:"embedding"`
	Interception InterceptionConfig `yaml:"interception"`
	Memory       MemoryConfig       `yaml:"memory"`
	Logging      LoggingConfig      `yaml:"logging"`
}

type ServerConfig struct {
	Port            int           `yaml:"port"`
	ContextPath     string        `yaml:"context_path"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
	// Environment selects runtime mode: "development" (default) or
	// "production". Production enables hard validation (Validate) and
	// stricter defaults (e.g. sslmode=require).
	Environment string `yaml:"environment"`
}

type DatabaseConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	Name           string `yaml:"name"`
	User           string `yaml:"user"`
	Password       string `yaml:"password"`
	SSLMode        string `yaml:"sslmode"`
	MaxConnections int    `yaml:"max_connections"`
	MinConnections int    `yaml:"min_connections"`
}

func (d DatabaseConfig) DSN() string {
	sslMode := d.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, sslMode)
}

type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

func (r RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", r.Host, r.Port)
}

type FalkorDBConfig struct {
	Host      string `yaml:"host"`
	Port      int    `yaml:"port"`
	Password  string `yaml:"password"`
	GraphName string `yaml:"graph_name"`
}

func (f FalkorDBConfig) Addr() string {
	return fmt.Sprintf("%s:%d", f.Host, f.Port)
}

type SecurityConfig struct {
	JWTSecret     string     `yaml:"jwt_secret"`
	JWTExpiration time.Duration `yaml:"jwt_expiration"`
	BcryptCost    int        `yaml:"bcrypt_cost"`
	CORS          CORSConfig `yaml:"cors"`
	// DemoAuthEnabled exposes POST /v1/auth/demo, which logs anyone in as
	// the shared demo user. Dev/demo convenience only — Validate refuses
	// it in production.
	DemoAuthEnabled bool `yaml:"demo_auth_enabled"`
}

type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins"`
	AllowedMethods []string `yaml:"allowed_methods"`
}

type TenantConfig struct {
	DefaultID string `yaml:"default_id"`
}

type AIConfig struct {
	Provider    string        `yaml:"provider"`
	Model       string        `yaml:"model"`
	APIKey      string        `yaml:"api_key"`
	BaseURL     string        `yaml:"base_url"`
	Temperature float64       `yaml:"temperature"`
	MaxTokens   int           `yaml:"max_tokens"`
	Timeout     time.Duration `yaml:"timeout"`
	MaxRetries  int           `yaml:"max_retries"`
}

// AnthropicProviderConfig holds optional native Anthropic credentials. When
// APIKey is non-empty the server adds AnthropicProvider to the LLM fallback
// chain and registers an Anthropic scorer for the cross-modal eval gate.
type AnthropicProviderConfig struct {
	APIKey      string        `yaml:"api_key"`
	BaseURL     string        `yaml:"base_url"`
	Model       string        `yaml:"model"`
	MaxTokens   int           `yaml:"max_tokens"`
	Temperature float64       `yaml:"temperature"`
	Timeout     time.Duration `yaml:"timeout"`
}

// GeminiProviderConfig holds optional native Google Gemini credentials. Same
// opt-in semantics as Anthropic.
type GeminiProviderConfig struct {
	APIKey      string        `yaml:"api_key"`
	BaseURL     string        `yaml:"base_url"`
	Model       string        `yaml:"model"`
	MaxTokens   int           `yaml:"max_tokens"`
	Temperature float64       `yaml:"temperature"`
	Timeout     time.Duration `yaml:"timeout"`
}

// ModelsConfig is the operator-facing tier routing config. Optional —
// resolution falls back to AI.Model and built-in tier defaults when unset.
// See internal/models/tiers.go for the full resolution chain.
type ModelsConfig struct {
	Default string            `yaml:"default"`
	Tier    map[string]string `yaml:"tier"`
}

// StoreConfig selects the backend behind the small store.MemoryStore
// surface (mounted at /v1/store/memories). The full production memory
// pipeline always uses Postgres; this knob exists so the zero-config
// embedded mode (`brainsentry init --embedded`) can serve a working API
// without Postgres at all.
//
// backend = "" or "postgres" → store.PostgresStore wraps the existing repo
// backend = "embedded"        → store.EmbeddedStore at Embedded.Path
type StoreConfig struct {
	Backend  string             `yaml:"backend"`
	Embedded EmbeddedStoreConfig `yaml:"embedded"`
}

// EmbeddedStoreConfig points the embedded backend at a JSON file. Created
// on first write; safe to point at a non-existent path.
type EmbeddedStoreConfig struct {
	Path string `yaml:"path"`
}

type EmbeddingConfig struct {
	Model      string `yaml:"model"`
	Dimensions int    `yaml:"dimensions"`
}

type InterceptionConfig struct {
	QuickCheckEnabled  bool    `yaml:"quick_check_enabled"`
	DeepAnalysisEnabled bool   `yaml:"deep_analysis_enabled"`
	RelevanceThreshold float64 `yaml:"relevance_threshold"`
}

type MemoryConfig struct {
	AutoCapture      bool `yaml:"auto_capture"`
	AutoImportance   bool `yaml:"auto_importance"`
	ObsolescenceDays int  `yaml:"obsolescence_days"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// defaultJWTSecret is the placeholder shipped in config.yaml. Validate
// refuses to boot a production server still using it.
const defaultJWTSecret = "your-256-bit-secret-key-change-in-production-min-32-chars"

// Load reads configuration from a YAML file and applies environment variable overrides.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	// Environment variable overrides
	applyEnvOverrides(cfg)

	// Stricter defaults in production: encrypted Postgres connection unless
	// the operator explicitly opted out via config/env.
	if cfg.IsProduction() && cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "require"
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// IsProduction reports whether the server runs in production mode.
func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.Server.Environment, "production")
}

// Validate enforces invariants that must hold before the server is allowed
// to boot. All production checks fail closed: a misconfigured production
// server must not come up at all.
func (c *Config) Validate() error {
	if !c.IsProduction() {
		return nil
	}
	if c.Security.JWTSecret == "" || c.Security.JWTSecret == defaultJWTSecret {
		return fmt.Errorf("config: production requires a real security.jwt_secret (set JWT_SECRET); refusing to boot with the default/empty secret")
	}
	if len(c.Security.JWTSecret) < 32 {
		return fmt.Errorf("config: production security.jwt_secret must be at least 32 characters")
	}
	if c.Security.DemoAuthEnabled {
		return fmt.Errorf("config: security.demo_auth_enabled must be false in production (shared demo user with a known password)")
	}
	return nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Server.Port)
	}
	if v := os.Getenv("BRAINSENTRY_ENV"); v != "" {
		cfg.Server.Environment = v
	}
	if v := os.Getenv("DB_SSLMODE"); v != "" {
		cfg.Database.SSLMode = v
	}
	if v := os.Getenv("DEMO_AUTH_ENABLED"); v != "" {
		cfg.Security.DemoAuthEnabled = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		// Parse DATABASE_URL if provided
		cfg.Database.Host = v // simplified; full parsing in production
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Database.Port)
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Database.Name = v
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.Redis.Port)
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("FALKORDB_HOST"); v != "" {
		cfg.FalkorDB.Host = v
	}
	if v := os.Getenv("FALKORDB_PORT"); v != "" {
		fmt.Sscanf(v, "%d", &cfg.FalkorDB.Port)
	}
	if v := os.Getenv("FALKORDB_PASSWORD"); v != "" {
		cfg.FalkorDB.Password = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Security.JWTSecret = v
	}
	if v := os.Getenv("BRAINSENTRY_AI_AGENTIC_MODEL_API_KEY"); v != "" {
		cfg.AI.APIKey = v
	}
	if v := os.Getenv("AI_MODEL"); v != "" {
		cfg.AI.Model = v
	}
	if v := os.Getenv("CORS_ORIGINS"); v != "" {
		cfg.Security.CORS.AllowedOrigins = strings.Split(v, ",")
	}
}

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Load – file handling
// ---------------------------------------------------------------------------

func TestLoad_NonExistentFile(t *testing.T) {
	_, err := Load("/tmp/does-not-exist-brainsentry-config.yaml")
	if err == nil {
		t.Fatal("expected error when loading non-existent file, got nil")
	}
	if !strings.Contains(err.Error(), "reading config file") {
		t.Errorf("expected error to mention 'reading config file', got: %v", err)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	f := writeTempFile(t, "invalid: yaml: [unclosed")
	defer os.Remove(f)

	_, err := Load(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
	if !strings.Contains(err.Error(), "parsing config file") {
		t.Errorf("expected error to mention 'parsing config file', got: %v", err)
	}
}

func TestLoad_EmptyFile(t *testing.T) {
	f := writeTempFile(t, "")
	defer os.Remove(f)

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error loading empty YAML: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for empty YAML")
	}
}

// ---------------------------------------------------------------------------
// Load – valid YAML (mirrors config.yaml in project root)
// ---------------------------------------------------------------------------

const sampleYAML = `
server:
  port: 9090
  context_path: /api/v1
  shutdown_timeout: 15s

database:
  host: db.example.com
  port: 5432
  name: testdb
  user: testuser
  password: testpass
  max_connections: 10
  min_connections: 2

redis:
  host: redis.example.com
  port: 6380
  password: redispass
  db: 1

falkordb:
  host: falkor.example.com
  port: 6381
  password: falkorpass
  graph_name: mygraph

security:
  jwt_secret: "super-secret-key-for-testing-only"
  jwt_expiration: 12h
  bcrypt_cost: 10
  cors:
    allowed_origins:
      - "http://localhost:3000"
    allowed_methods:
      - GET
      - POST

tenant:
  default_id: "00000000-0000-0000-0000-000000000001"

ai:
  provider: openrouter
  model: gpt-4
  api_key: sk-test
  base_url: "https://openrouter.ai/api/v1"
  temperature: 0.7
  max_tokens: 1024
  timeout: 30s
  max_retries: 2

embedding:
  model: all-MiniLM-L6-v2
  dimensions: 384

interception:
  quick_check_enabled: true
  deep_analysis_enabled: false
  relevance_threshold: 0.5

memory:
  auto_capture: true
  auto_importance: false
  obsolescence_days: 30

logging:
  level: debug
  format: text
`

func TestLoad_ValidYAML(t *testing.T) {
	f := writeTempFile(t, sampleYAML)
	defer os.Remove(f)

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Server
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port: want 9090, got %d", cfg.Server.Port)
	}
	if cfg.Server.ContextPath != "/api/v1" {
		t.Errorf("Server.ContextPath: want '/api/v1', got %q", cfg.Server.ContextPath)
	}
	if cfg.Server.ShutdownTimeout != 15*time.Second {
		t.Errorf("Server.ShutdownTimeout: want 15s, got %v", cfg.Server.ShutdownTimeout)
	}

	// Database
	if cfg.Database.Host != "db.example.com" {
		t.Errorf("Database.Host: want 'db.example.com', got %q", cfg.Database.Host)
	}
	if cfg.Database.Port != 5432 {
		t.Errorf("Database.Port: want 5432, got %d", cfg.Database.Port)
	}
	if cfg.Database.Name != "testdb" {
		t.Errorf("Database.Name: want 'testdb', got %q", cfg.Database.Name)
	}
	if cfg.Database.MaxConnections != 10 {
		t.Errorf("Database.MaxConnections: want 10, got %d", cfg.Database.MaxConnections)
	}

	// Redis
	if cfg.Redis.Host != "redis.example.com" {
		t.Errorf("Redis.Host: want 'redis.example.com', got %q", cfg.Redis.Host)
	}
	if cfg.Redis.Port != 6380 {
		t.Errorf("Redis.Port: want 6380, got %d", cfg.Redis.Port)
	}
	if cfg.Redis.DB != 1 {
		t.Errorf("Redis.DB: want 1, got %d", cfg.Redis.DB)
	}

	// FalkorDB
	if cfg.FalkorDB.GraphName != "mygraph" {
		t.Errorf("FalkorDB.GraphName: want 'mygraph', got %q", cfg.FalkorDB.GraphName)
	}

	// Security
	if cfg.Security.JWTSecret != "super-secret-key-for-testing-only" {
		t.Errorf("Security.JWTSecret mismatch")
	}
	if cfg.Security.JWTExpiration != 12*time.Hour {
		t.Errorf("Security.JWTExpiration: want 12h, got %v", cfg.Security.JWTExpiration)
	}
	if cfg.Security.BcryptCost != 10 {
		t.Errorf("Security.BcryptCost: want 10, got %d", cfg.Security.BcryptCost)
	}
	if len(cfg.Security.CORS.AllowedOrigins) != 1 || cfg.Security.CORS.AllowedOrigins[0] != "http://localhost:3000" {
		t.Errorf("CORS.AllowedOrigins mismatch: %v", cfg.Security.CORS.AllowedOrigins)
	}

	// AI
	if cfg.AI.Provider != "openrouter" {
		t.Errorf("AI.Provider: want 'openrouter', got %q", cfg.AI.Provider)
	}
	if cfg.AI.Temperature != 0.7 {
		t.Errorf("AI.Temperature: want 0.7, got %f", cfg.AI.Temperature)
	}
	if cfg.AI.Timeout != 30*time.Second {
		t.Errorf("AI.Timeout: want 30s, got %v", cfg.AI.Timeout)
	}

	// Interception
	if !cfg.Interception.QuickCheckEnabled {
		t.Error("Interception.QuickCheckEnabled: want true")
	}
	if cfg.Interception.DeepAnalysisEnabled {
		t.Error("Interception.DeepAnalysisEnabled: want false")
	}
	if cfg.Interception.RelevanceThreshold != 0.5 {
		t.Errorf("Interception.RelevanceThreshold: want 0.5, got %f", cfg.Interception.RelevanceThreshold)
	}

	// Memory
	if !cfg.Memory.AutoCapture {
		t.Error("Memory.AutoCapture: want true")
	}
	if cfg.Memory.AutoImportance {
		t.Error("Memory.AutoImportance: want false")
	}
	if cfg.Memory.ObsolescenceDays != 30 {
		t.Errorf("Memory.ObsolescenceDays: want 30, got %d", cfg.Memory.ObsolescenceDays)
	}

	// Logging
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level: want 'debug', got %q", cfg.Logging.Level)
	}
}

// ---------------------------------------------------------------------------
// Default values (zero-value YAML)
// ---------------------------------------------------------------------------

func TestLoad_DefaultValues(t *testing.T) {
	f := writeTempFile(t, "{}")
	defer os.Remove(f)

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All numeric fields should be zero by default.
	if cfg.Server.Port != 0 {
		t.Errorf("Server.Port default: want 0, got %d", cfg.Server.Port)
	}
	if cfg.Database.MaxConnections != 0 {
		t.Errorf("Database.MaxConnections default: want 0, got %d", cfg.Database.MaxConnections)
	}
	if cfg.Security.BcryptCost != 0 {
		t.Errorf("Security.BcryptCost default: want 0, got %d", cfg.Security.BcryptCost)
	}
	if cfg.AI.Temperature != 0 {
		t.Errorf("AI.Temperature default: want 0, got %f", cfg.AI.Temperature)
	}
}

// ---------------------------------------------------------------------------
// Environment variable overrides
// ---------------------------------------------------------------------------

func TestLoad_EnvOverrides_PORT(t *testing.T) {
	f := writeTempFile(t, "server:\n  port: 8080\n")
	defer os.Remove(f)

	t.Setenv("PORT", "9999")

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("expected Server.Port override to 9999, got %d", cfg.Server.Port)
	}
}

func TestLoad_EnvOverrides_DBHost(t *testing.T) {
	f := writeTempFile(t, "database:\n  host: original-host\n")
	defer os.Remove(f)

	t.Setenv("DB_HOST", "overridden-host")

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database.Host != "overridden-host" {
		t.Errorf("expected Database.Host 'overridden-host', got %q", cfg.Database.Host)
	}
}

func TestLoad_EnvOverrides_RedisHost(t *testing.T) {
	f := writeTempFile(t, "redis:\n  host: localhost\n  port: 6379\n")
	defer os.Remove(f)

	t.Setenv("REDIS_HOST", "redis-override")

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Redis.Host != "redis-override" {
		t.Errorf("expected Redis.Host 'redis-override', got %q", cfg.Redis.Host)
	}
}

func TestLoad_EnvOverrides_JWTSecret(t *testing.T) {
	f := writeTempFile(t, "security:\n  jwt_secret: original\n")
	defer os.Remove(f)

	t.Setenv("JWT_SECRET", "env-secret")

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Security.JWTSecret != "env-secret" {
		t.Errorf("expected JWTSecret 'env-secret', got %q", cfg.Security.JWTSecret)
	}
}

func TestLoad_EnvOverrides_CORSOrigins(t *testing.T) {
	f := writeTempFile(t, "security:\n  cors:\n    allowed_origins:\n      - http://localhost:3000\n")
	defer os.Remove(f)

	t.Setenv("CORS_ORIGINS", "https://app.example.com,https://admin.example.com")

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Security.CORS.AllowedOrigins) != 2 {
		t.Fatalf("expected 2 allowed origins, got %d: %v", len(cfg.Security.CORS.AllowedOrigins), cfg.Security.CORS.AllowedOrigins)
	}
	if cfg.Security.CORS.AllowedOrigins[0] != "https://app.example.com" {
		t.Errorf("unexpected first origin: %q", cfg.Security.CORS.AllowedOrigins[0])
	}
}

func TestLoad_EnvOverrides_AIAPIKey(t *testing.T) {
	f := writeTempFile(t, "ai:\n  api_key: original-key\n")
	defer os.Remove(f)

	t.Setenv("BRAINSENTRY_AI_AGENTIC_MODEL_API_KEY", "env-api-key")

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AI.APIKey != "env-api-key" {
		t.Errorf("expected AI.APIKey 'env-api-key', got %q", cfg.AI.APIKey)
	}
}

// ---------------------------------------------------------------------------
// DatabaseConfig.DSN()
// ---------------------------------------------------------------------------

func TestDatabaseConfig_DSN(t *testing.T) {
	tests := []struct {
		name string
		cfg  DatabaseConfig
		want string
	}{
		{
			name: "all fields populated",
			cfg:  DatabaseConfig{Host: "localhost", Port: 5432, Name: "mydb", User: "admin", Password: "secret"},
			want: "postgres://admin:secret@localhost:5432/mydb?sslmode=disable",
		},
		{
			name: "empty password",
			cfg:  DatabaseConfig{Host: "db.example.com", Port: 5433, Name: "app", User: "user", Password: ""},
			want: "postgres://user:@db.example.com:5433/app?sslmode=disable",
		},
		{
			name: "zero port",
			cfg:  DatabaseConfig{Host: "h", Port: 0, Name: "n", User: "u", Password: "p"},
			want: "postgres://u:p@h:0/n?sslmode=disable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cfg.DSN()
			if got != tc.want {
				t.Errorf("DSN() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RedisConfig.Addr()
// ---------------------------------------------------------------------------

func TestRedisConfig_Addr(t *testing.T) {
	tests := []struct {
		name string
		cfg  RedisConfig
		want string
	}{
		{"standard", RedisConfig{Host: "localhost", Port: 6379}, "localhost:6379"},
		{"custom port", RedisConfig{Host: "redis.example.com", Port: 6380}, "redis.example.com:6380"},
		{"empty host", RedisConfig{Host: "", Port: 6379}, ":6379"},
		{"zero port", RedisConfig{Host: "localhost", Port: 0}, "localhost:0"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cfg.Addr()
			if got != tc.want {
				t.Errorf("Addr() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FalkorDBConfig.Addr()
// ---------------------------------------------------------------------------

func TestFalkorDBConfig_Addr(t *testing.T) {
	tests := []struct {
		name string
		cfg  FalkorDBConfig
		want string
	}{
		{"standard", FalkorDBConfig{Host: "localhost", Port: 6379}, "localhost:6379"},
		{"custom port", FalkorDBConfig{Host: "falkor.example.com", Port: 6381}, "falkor.example.com:6381"},
		{"empty host", FalkorDBConfig{Host: "", Port: 6379}, ":6379"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.cfg.Addr()
			if got != tc.want {
				t.Errorf("Addr() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Load real project config.yaml (integration-style, no external services)
// ---------------------------------------------------------------------------

func TestLoad_RealProjectConfig(t *testing.T) {
	// Walk up from this file's location to find the project root config.yaml.
	// The file is at <root>/config.yaml; this test file is at
	// <root>/internal/config/config_test.go so two levels up.
	// Go test runs with cwd set to the package directory.
	projectRoot := filepath.Join("..", "..")
	configPath := filepath.Join(projectRoot, "config.yaml")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Skip("config.yaml not found – skipping real project config test")
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load(config.yaml) failed: %v", err)
	}

	if cfg.Server.Port == 0 {
		t.Error("expected non-zero Server.Port from config.yaml")
	}
	if cfg.Database.Host == "" {
		t.Error("expected non-empty Database.Host from config.yaml")
	}
	if cfg.Redis.Host == "" {
		t.Error("expected non-empty Redis.Host from config.yaml")
	}
}

// ---------------------------------------------------------------------------
// Production validation (fail-closed boot)
// ---------------------------------------------------------------------------

const strongSecret = "0123456789abcdef0123456789abcdef0123456789abcdef"

func TestLoad_ProductionRefusesDefaultJWTSecret(t *testing.T) {
	f := writeTempFile(t, "server:\n  environment: production\nsecurity:\n  jwt_secret: \""+defaultJWTSecret+"\"\n")
	defer os.Remove(f)

	if _, err := Load(f); err == nil {
		t.Fatal("expected production boot to refuse the default JWT secret")
	}
}

func TestLoad_ProductionRefusesEmptyJWTSecret(t *testing.T) {
	f := writeTempFile(t, "server:\n  environment: production\n")
	defer os.Remove(f)

	if _, err := Load(f); err == nil {
		t.Fatal("expected production boot to refuse an empty JWT secret")
	}
}

func TestLoad_ProductionRefusesShortJWTSecret(t *testing.T) {
	f := writeTempFile(t, "server:\n  environment: production\nsecurity:\n  jwt_secret: short\n")
	defer os.Remove(f)

	if _, err := Load(f); err == nil {
		t.Fatal("expected production boot to refuse a <32 char JWT secret")
	}
}

func TestLoad_ProductionRefusesDemoAuth(t *testing.T) {
	f := writeTempFile(t, "server:\n  environment: production\nsecurity:\n  jwt_secret: \""+strongSecret+"\"\n  demo_auth_enabled: true\n")
	defer os.Remove(f)

	if _, err := Load(f); err == nil {
		t.Fatal("expected production boot to refuse demo_auth_enabled")
	}
}

func TestLoad_ProductionDefaultsSSLModeToRequire(t *testing.T) {
	f := writeTempFile(t, "server:\n  environment: production\nsecurity:\n  jwt_secret: \""+strongSecret+"\"\n")
	defer os.Remove(f)

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Database.SSLMode != "require" {
		t.Errorf("expected production sslmode default 'require', got %q", cfg.Database.SSLMode)
	}
	if !strings.Contains(cfg.Database.DSN(), "sslmode=require") {
		t.Errorf("expected DSN to carry sslmode=require, got %q", cfg.Database.DSN())
	}
}

func TestLoad_DevelopmentAllowsDevDefaults(t *testing.T) {
	f := writeTempFile(t, "security:\n  jwt_secret: \""+defaultJWTSecret+"\"\n  demo_auth_enabled: true\n")
	defer os.Remove(f)

	if _, err := Load(f); err != nil {
		t.Fatalf("development mode must tolerate dev defaults, got: %v", err)
	}
}

func TestLoad_EnvOverrides_EnvironmentAndSSLMode(t *testing.T) {
	f := writeTempFile(t, "security:\n  jwt_secret: \""+strongSecret+"\"\n")
	defer os.Remove(f)

	t.Setenv("BRAINSENTRY_ENV", "production")
	t.Setenv("DB_SSLMODE", "verify-full")

	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.IsProduction() {
		t.Error("expected BRAINSENTRY_ENV=production to enable production mode")
	}
	if cfg.Database.SSLMode != "verify-full" {
		t.Errorf("expected DB_SSLMODE override 'verify-full', got %q", cfg.Database.SSLMode)
	}
}

func TestDatabaseConfig_DSN_CustomSSLMode(t *testing.T) {
	cfg := DatabaseConfig{Host: "h", Port: 5432, Name: "n", User: "u", Password: "p", SSLMode: "require"}
	want := "postgres://u:p@h:5432/n?sslmode=require"
	if got := cfg.DSN(); got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// writeTempFile creates a temporary file with the given content and returns
// its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "brainsentry-config-test-*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()
	return f.Name()
}

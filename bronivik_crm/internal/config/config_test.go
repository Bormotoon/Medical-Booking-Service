package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRejectsPostgresConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `
telegram:
  bot_token: "crm-token"
database:
  path: "data/test.db"
  postgres:
    host: "localhost"
    user: "crm"
    dbname: "crm"
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected postgres config to be rejected")
	}
}

func TestLoadSampleConfigFromEnv(t *testing.T) {
	t.Setenv("CRM_BOT_TOKEN", "crm-token")
	t.Setenv("CRM_DB_PATH", filepath.Join(t.TempDir(), "crm.db"))
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("CRM_REDIS_DB", "1")
	t.Setenv("JR_API_BASE_URL", "http://localhost:18080")
	t.Setenv("CRM_API_KEY", "crm-key")
	t.Setenv("CRM_API_EXTRA", "crm-extra")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("CRM_HEALTH_PORT", "18090")

	cfg, err := Load(filepath.Join("..", "..", "configs", "config.yaml"))
	if err != nil {
		t.Fatalf("failed to load sample config: %v", err)
	}

	if cfg.Database.Path == "" {
		t.Fatalf("expected database path to be populated from env")
	}
	if cfg.Redis.Address != "127.0.0.1:6379" || cfg.Redis.DB != 1 {
		t.Fatalf("expected redis settings from env, got %s/%d", cfg.Redis.Address, cfg.Redis.DB)
	}
	if cfg.API.BaseURL != "http://localhost:18080" {
		t.Fatalf("expected API base URL from env, got %q", cfg.API.BaseURL)
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected log level from env, got %q", cfg.Logging.Level)
	}
	if cfg.Monitoring.HealthCheckPort != 18090 {
		t.Fatalf("expected health port from env, got %d", cfg.Monitoring.HealthCheckPort)
	}
}

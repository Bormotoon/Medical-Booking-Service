package config

import (
	"os"
	"path/filepath"
	"testing"

	"bronivik/internal/models"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
telegram:
  bot_token: "test_token"
database:
  path: "test.db"
items:
  - id: 1
    name: "Item 1"
    total_quantity: 1
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Telegram.BotToken != "test_token" {
		t.Errorf("expected bot_token test_token, got %s", cfg.Telegram.BotToken)
	}

	if len(cfg.Items) != 1 || cfg.Items[0].ID != 1 {
		t.Errorf("expected 1 item with ID 1")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: Config{
				Telegram: TelegramConfig{BotToken: "token"},
				Database: DatabaseConfig{Path: "path"},
				Items:    []models.Item{{ID: 1, Name: "Item 1"}},
			},
			wantErr: false,
		},
		{
			name: "missing token",
			cfg: Config{
				Telegram: TelegramConfig{BotToken: ""},
				Database: DatabaseConfig{Path: "path"},
			},
			wantErr: true,
		},
		{
			name: "duplicate item id",
			cfg: Config{
				Telegram: TelegramConfig{BotToken: "token"},
				Database: DatabaseConfig{Path: "path"},
				Items: []models.Item{
					{ID: 1, Name: "Item 1"},
					{ID: 1, Name: "Item 2"},
				},
			},
			wantErr: true,
		},
		{
			name: "postgres unsupported",
			cfg: Config{
				Telegram: TelegramConfig{BotToken: "token"},
				Database: DatabaseConfig{
					Path: "path",
					Postgres: PostgresConfig{
						Host:   "localhost",
						User:   "user",
						DBName: "db",
					},
				},
				Items: []models.Item{{ID: 1, Name: "Item 1"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.applyDefaults()

	expectedReminder := "09:00"
	if cfg.Bot.ReminderTime != expectedReminder {
		t.Errorf("expected default reminder time %s, got %s", expectedReminder, cfg.Bot.ReminderTime)
	}
	if cfg.Bot.PaginationSize != models.DefaultPaginationSize {
		t.Errorf("expected default pagination size %d, got %d", models.DefaultPaginationSize, cfg.Bot.PaginationSize)
	}
	if cfg.API.GRPC.Port != 8081 {
		t.Errorf("expected default gRPC port 8081, got %d", cfg.API.GRPC.Port)
	}
	if cfg.Bot.RateLimitMessages != models.RateLimitMessages {
		t.Errorf("expected default rate limit messages %d, got %d", models.RateLimitMessages, cfg.Bot.RateLimitMessages)
	}
	if !cfg.API.Auth.Enabled {
		t.Errorf("expected auth to be enabled by default")
	}
}

func TestLoadConfig_PreservesExplicitAuthDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	yamlContent := `
telegram:
  bot_token: "test_token"
database:
  path: "test.db"
api:
  auth:
    enabled: false
items:
  - id: 1
    name: "Item 1"
    total_quantity: 1
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.API.Auth.Enabled {
		t.Fatalf("expected auth.enabled=false to be preserved")
	}
}

func TestLoadConfig_UsesDotEnvAsFallbackWithoutMutatingEnv(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	yamlContent := `
telegram:
  bot_token: "${LOCAL_BOT_TOKEN}"
database:
  path: "${LOCAL_DB_PATH:-test.db}"
items:
  - id: 1
    name: "Item 1"
    total_quantity: 1
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	if err := os.WriteFile(".env", []byte("LOCAL_BOT_TOKEN=dotenv_token\nLOCAL_DB_PATH=dotenv.db\n"), 0o644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Telegram.BotToken != "dotenv_token" {
		t.Fatalf("expected bot token from .env fallback, got %q", cfg.Telegram.BotToken)
	}
	if cfg.Database.Path != "dotenv.db" {
		t.Fatalf("expected database path from .env fallback, got %q", cfg.Database.Path)
	}
	if _, ok := os.LookupEnv("LOCAL_BOT_TOKEN"); ok {
		t.Fatalf("expected Load not to mutate process env with .env values")
	}
}

func TestLoadConfig_PrefersProcessEnvOverDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	configPath := filepath.Join(tmpDir, "config.yaml")
	yamlContent := `
telegram:
  bot_token: "${LOCAL_BOT_TOKEN}"
database:
  path: "${LOCAL_DB_PATH:-test.db}"
google:
  credentials_file: "${LOCAL_GOOGLE_CREDENTIALS:-}"
items:
  - id: 1
    name: "Item 1"
    total_quantity: 1
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	if err := os.WriteFile(".env", []byte(
		"LOCAL_BOT_TOKEN=dotenv_token\nLOCAL_DB_PATH=dotenv.db\nLOCAL_GOOGLE_CREDENTIALS=/tmp/dotenv.json\n",
	), 0o644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	t.Setenv("LOCAL_BOT_TOKEN", "env_token")
	t.Setenv("LOCAL_DB_PATH", "env.db")
	t.Setenv("LOCAL_GOOGLE_CREDENTIALS", "")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if cfg.Telegram.BotToken != "env_token" {
		t.Fatalf("expected bot token from process env, got %q", cfg.Telegram.BotToken)
	}
	if cfg.Database.Path != "env.db" {
		t.Fatalf("expected database path from process env, got %q", cfg.Database.Path)
	}
	if cfg.Google.GoogleCredentialsFile != "" {
		t.Fatalf("expected empty env to win over .env fallback, got %q", cfg.Google.GoogleCredentialsFile)
	}
}

func TestLoadSampleConfigFromEnv(t *testing.T) {
	t.Setenv("BOT_TOKEN", "test_token")
	t.Setenv("JR_DB_PATH", filepath.Join(t.TempDir(), "jr.db"))
	t.Setenv("REDIS_ADDR", "127.0.0.1:6379")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("JR_API_HTTP_PORT", "18080")
	t.Setenv("JR_API_GRPC_PORT", "18081")
	t.Setenv("JR_API_AUTH_ENABLED", "false")
	t.Setenv("GOOGLE_CREDENTIALS_FILE", "/tmp/google.json")
	t.Setenv("USERS_SPREADSHEET_ID", "users-sheet")
	t.Setenv("BOOKINGS_SPREADSHEET_ID", "bookings-sheet")

	cfg, err := Load(filepath.Join("..", "..", "configs", "config.yaml"))
	if err != nil {
		t.Fatalf("failed to load sample config: %v", err)
	}

	if cfg.Database.Path == "" {
		t.Fatalf("expected database path to be populated from env")
	}
	if cfg.Logging.Level != "debug" {
		t.Fatalf("expected log level from env, got %q", cfg.Logging.Level)
	}
	if cfg.API.HTTP.Port != 18080 || cfg.API.GRPC.Port != 18081 {
		t.Fatalf("expected API ports from env, got http=%d grpc=%d", cfg.API.HTTP.Port, cfg.API.GRPC.Port)
	}
	if cfg.API.Auth.Enabled {
		t.Fatalf("expected auth to be disabled from env-backed config")
	}
	if cfg.Google.GoogleCredentialsFile != "/tmp/google.json" {
		t.Fatalf("expected google credentials file from env, got %q", cfg.Google.GoogleCredentialsFile)
	}
}

func TestValidateItems(t *testing.T) {
	tests := []struct {
		name    string
		items   []models.Item
		wantErr bool
	}{
		{
			name: "Valid items",
			items: []models.Item{
				{ID: 1, Name: "Item 1"},
				{ID: 2, Name: "Item 2"},
			},
			wantErr: false,
		},
		{
			name: "Duplicate ID",
			items: []models.Item{
				{ID: 1, Name: "Item 1"},
				{ID: 1, Name: "Item 2"},
			},
			wantErr: true,
		},
		{
			name: "ID 0",
			items: []models.Item{
				{ID: 0, Name: "Item 1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateItems(tt.items)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateItems() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

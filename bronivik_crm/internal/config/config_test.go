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

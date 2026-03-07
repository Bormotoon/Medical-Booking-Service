package main

import (
	"os"
	"path/filepath"
	"testing"

	"bronivik/internal/config"

	"github.com/rs/zerolog"
)

func TestParseCommandDefaultsToBotMode(t *testing.T) {
	cmd, err := parseCommand(nil)
	if err != nil {
		t.Fatalf("parseCommand: %v", err)
	}
	if cmd.mode != modeBot {
		t.Fatalf("mode: got %s want %s", cmd.mode, modeBot)
	}
}

func TestParseCommandWorkerReminders(t *testing.T) {
	cmd, err := parseCommand([]string{"worker", "--job=reminders"})
	if err != nil {
		t.Fatalf("parseCommand: %v", err)
	}
	if cmd.mode != modeWorker {
		t.Fatalf("mode: got %s want %s", cmd.mode, modeWorker)
	}
	if cmd.workerJob != workerJobReminders {
		t.Fatalf("worker job: got %s want %s", cmd.workerJob, workerJobReminders)
	}
}

func TestParseCommandRejectsInvalidInput(t *testing.T) {
	tests := [][]string{
		{"worker"},
		{"worker", "--job=unknown"},
		{"api"},
	}

	for _, args := range tests {
		if _, err := parseCommand(args); err == nil {
			t.Fatalf("expected error for args %v", args)
		}
	}
}

func TestPrepareDirectoriesSkipsExportsWhenDisabled(t *testing.T) {
	baseDir := t.TempDir()
	logger := zerolog.Nop()
	cfg := &config.Config{
		Database: config.DatabaseConfig{Path: filepath.Join(baseDir, "db", "bookings.db")},
		Exports:  config.ExportConfig{Path: filepath.Join(baseDir, "exports")},
	}

	if err := prepareDirectories(cfg, &logger, false); err != nil {
		t.Fatalf("prepareDirectories: %v", err)
	}

	if _, err := os.Stat(filepath.Join(baseDir, "db")); err != nil {
		t.Fatalf("expected db dir to exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "exports")); !os.IsNotExist(err) {
		t.Fatalf("expected exports dir to stay absent, got err=%v", err)
	}
}

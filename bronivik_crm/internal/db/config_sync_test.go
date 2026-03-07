package db

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"bronivik/bronivik_crm/internal/config"
)

func TestSyncCabinetsFromConfig_RollsBackOnSetDayOffError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "crm.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `DROP TABLE crm_cabinet_schedule_overrides`); err != nil {
		t.Fatalf("drop overrides table: %v", err)
	}

	cfg := &config.CabinetsConfig{
		Cabinets: []config.CabinetConfig{
			{ID: 1, Name: "Cab1", IsActive: true},
		},
		Defaults: config.DefaultsConfig{
			Schedule: &config.CabinetScheduleConfig{
				StartTime:           "09:00",
				EndTime:             "18:00",
				SlotDurationMinutes: 60,
			},
		},
		Holidays: []config.HolidayConfig{
			{Date: "2030-01-01", Name: "Holiday"},
		},
	}

	err = db.SyncCabinetsFromConfig(ctx, cfg)
	if err == nil {
		t.Fatal("expected sync error when SetDayOff fails")
	}
	if !strings.Contains(err.Error(), "sync holiday") {
		t.Fatalf("expected holiday sync context, got %v", err)
	}

	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM cabinets`).Scan(&count); err != nil {
		t.Fatalf("count cabinets: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected sync rollback, got %d cabinets", count)
	}
}

func TestGetOrCreateUserByTelegramID_ReturnsUpdateError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "crm.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	user, err := db.GetOrCreateUserByTelegramID(ctx, 123, "u", "First", "Last", "")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if _, err := db.ExecContext(ctx, `
		CREATE TRIGGER fail_user_update
		BEFORE UPDATE ON crm_users
		BEGIN
			SELECT RAISE(FAIL, 'profile update boom');
		END;
	`); err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	_, err = db.GetOrCreateUserByTelegramID(ctx, 123, "new-user", "Updated", "Name", "")
	if err == nil {
		t.Fatal("expected update error")
	}
	if !strings.Contains(err.Error(), "update user") {
		t.Fatalf("expected update context in error, got %v", err)
	}

	var username string
	if err := db.QueryRowContext(ctx, `SELECT username FROM users WHERE id = ?`, user.ID).Scan(&username); err != nil {
		t.Fatalf("load user: %v", err)
	}
	if username != "u" {
		t.Fatalf("expected rollback to preserve original username, got %q", username)
	}
}

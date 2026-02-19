package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"bronivik/bronivik_crm/internal/model"
)

func TestGetAvailableSlots_RespectsBookings(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "crm.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	user, err := db.GetOrCreateUserByTelegramID(ctx, 123, "u", "First", "Last", "")
	if err != nil {
		t.Fatalf("GetOrCreateUserByTelegramID: %v", err)
	}

	cab := &model.Cabinet{Name: "Cab1", Description: ""}
	if err = db.CreateCabinet(ctx, cab); err != nil {
		t.Fatalf("CreateCabinet: %v", err)
	}

	date := time.Date(2026, 1, 5, 0, 0, 0, 0, time.Local)
	dow := int(date.Weekday())
	if dow == 0 {
		dow = 7
	}
	if err = db.CreateSchedule(ctx, &model.CabinetSchedule{
		CabinetID: cab.ID, DayOfWeek: dow, StartTime: "09:00",
		EndTime: "12:00", SlotDuration: 60,
	}); err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	bk := &model.HourlyBooking{
		UserID:    user.ID,
		CabinetID: cab.ID,
		StartTime: time.Date(2026, 1, 5, 10, 0, 0, 0, time.Local),
		EndTime:   time.Date(2026, 1, 5, 11, 0, 0, 0, time.Local),
		Status:    "pending",
	}
	if err = db.CreateHourlyBooking(ctx, bk); err != nil {
		t.Fatalf("CreateHourlyBooking: %v", err)
	}

	slots, err := db.GetAvailableSlots(ctx, cab.ID, date)
	if err != nil {
		t.Fatalf("GetAvailableSlots: %v", err)
	}
	if len(slots) != 3 {
		t.Fatalf("expected 3 slots, got %d", len(slots))
	}
	// 09-10 free, 10-11 busy, 11-12 free
	if slots[1].Available {
		t.Fatalf("expected middle slot to be unavailable")
	}
}

func TestCreateHourlyBookingWithChecks_BusySlot(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "crm.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	user, err := db.GetOrCreateUserByTelegramID(ctx, 123, "u", "First", "Last", "")
	if err != nil {
		t.Fatalf("GetOrCreateUserByTelegramID: %v", err)
	}

	cab := &model.Cabinet{Name: "Cab1", Description: ""}
	if err = db.CreateCabinet(ctx, cab); err != nil {
		t.Fatalf("CreateCabinet: %v", err)
	}

	date := time.Date(2026, 1, 5, 0, 0, 0, 0, time.Local)
	dow := int(date.Weekday())
	if dow == 0 {
		dow = 7
	}
	err = db.CreateSchedule(ctx, &model.CabinetSchedule{
		CabinetID: cab.ID, DayOfWeek: dow, StartTime: "09:00",
		EndTime: "12:00", SlotDuration: 60,
	})
	if err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	busy := &model.HourlyBooking{
		UserID:    user.ID,
		CabinetID: cab.ID,
		StartTime: time.Date(2026, 1, 5, 10, 0, 0, 0, time.Local),
		EndTime:   time.Date(2026, 1, 5, 11, 0, 0, 0, time.Local),
		Status:    "pending",
	}
	if err = db.CreateHourlyBooking(ctx, busy); err != nil {
		t.Fatalf("CreateHourlyBooking: %v", err)
	}

	attempt := &model.HourlyBooking{
		UserID:    user.ID,
		CabinetID: cab.ID,
		StartTime: time.Date(2026, 1, 5, 10, 0, 0, 0, time.Local),
		EndTime:   time.Date(2026, 1, 5, 11, 0, 0, 0, time.Local),
		Status:    "pending",
	}
	if err := db.CreateHourlyBookingWithChecks(ctx, attempt, nil); err != ErrSlotNotAvailable {
		t.Fatalf("expected ErrSlotNotAvailable, got %v", err)
	}
}

func TestLegacyTablesMigrationToNamespacedTables(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "crm_legacy.db")

	legacyDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}

	legacySchema := []string{
		`CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			telegram_id INTEGER UNIQUE NOT NULL,
			username TEXT,
			first_name TEXT,
			last_name TEXT,
			phone TEXT,
			is_manager BOOLEAN NOT NULL DEFAULT 0,
			is_blacklisted BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE cabinets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			description TEXT,
			is_active BOOLEAN DEFAULT 1,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE hourly_bookings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			cabinet_id INTEGER NOT NULL,
			item_id INTEGER,
			item_name TEXT,
			client_name TEXT,
			client_phone TEXT,
			start_time DATETIME NOT NULL,
			end_time DATETIME NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			comment TEXT,
			manager_comment TEXT,
			reminder_sent BOOLEAN NOT NULL DEFAULT 0,
			external_device_booking_id INTEGER,
			created_at DATETIME,
			updated_at DATETIME
		)`,
	}

	for _, stmt := range legacySchema {
		if _, err = legacyDB.Exec(stmt); err != nil {
			_ = legacyDB.Close()
			t.Fatalf("create legacy schema: %v", err)
		}
	}

	now := time.Now()
	if _, err = legacyDB.Exec(`INSERT INTO users (id, telegram_id, username, first_name, last_name, phone, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, 1, 100500, "legacy_user", "Legacy", "User", "+70000000000", now, now); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("insert legacy user: %v", err)
	}
	if _, err = legacyDB.Exec(`INSERT INTO cabinets (id, name, description, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`, 10, "LegacyCab", "legacy", 1, now, now); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("insert legacy cabinet: %v", err)
	}
	if _, err = legacyDB.Exec(`INSERT INTO hourly_bookings (id, user_id, cabinet_id, item_name, client_name, client_phone, start_time, end_time, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, 100, 1, 10, "Аппарат", "Клиент", "+71111111111", now, now.Add(time.Hour), "pending", now, now); err != nil {
		_ = legacyDB.Close()
		t.Fatalf("insert legacy booking: %v", err)
	}

	if err = legacyDB.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	crmDB, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB with migration: %v", err)
	}

	assertCount := func(table string, expected int) {
		t.Helper()
		var count int
		if scanErr := crmDB.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); scanErr != nil {
			t.Fatalf("count rows in %s: %v", table, scanErr)
		}
		if count != expected {
			t.Fatalf("unexpected rows in %s: got %d want %d", table, count, expected)
		}
	}

	assertCount("crm_users", 1)
	assertCount("crm_cabinets", 1)
	assertCount("crm_hourly_bookings", 1)
	assertCount("users", 1)

	if err = crmDB.Close(); err != nil {
		t.Fatalf("close first migrated db: %v", err)
	}

	reopened, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("reopen db after migration: %v", err)
	}
	defer reopened.Close()

	var count int
	if err = reopened.DB.QueryRow("SELECT COUNT(*) FROM crm_users").Scan(&count); err != nil {
		t.Fatalf("count rows in crm_users after reopen: %v", err)
	}
	if count != 1 {
		t.Fatalf("migration is not idempotent, crm_users count = %d", count)
	}
}

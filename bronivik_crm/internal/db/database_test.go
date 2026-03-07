package db

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"bronivik/bronivik_crm/internal/model"
)

func setupHourlyBookingTestDB(t *testing.T, startTime, endTime string, slotDuration int) (*DB, *model.User, *model.Cabinet, time.Time) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "crm.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

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
		CabinetID:    cab.ID,
		DayOfWeek:    dow,
		StartTime:    startTime,
		EndTime:      endTime,
		SlotDuration: slotDuration,
	}); err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	return db, user, cab, date
}

func at(date time.Time, hour, minute int) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, date.Location())
}

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

func TestUpdateHourlyBookingStatus_ReturnsNotFoundWhenMissing(t *testing.T) {
	db, _, _, _ := setupHourlyBookingTestDB(t, "09:00", "12:00", 60)

	err := db.UpdateHourlyBookingStatus(context.Background(), 999999, "approved", "")
	if err != ErrBookingNotFound {
		t.Fatalf("expected ErrBookingNotFound, got %v", err)
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

func TestCreateHourlyBookingWithChecks_AllowsSixtyMinuteRange(t *testing.T) {
	db, user, cab, date := setupHourlyBookingTestDB(t, "09:00", "12:00", 30)

	booking := &model.HourlyBooking{
		UserID:    user.ID,
		CabinetID: cab.ID,
		StartTime: at(date, 9, 0),
		EndTime:   at(date, 10, 0),
		Status:    "pending",
	}
	if err := db.CreateHourlyBookingWithChecks(context.Background(), booking, nil); err != nil {
		t.Fatalf("CreateHourlyBookingWithChecks: %v", err)
	}

	stored, err := db.GetHourlyBooking(context.Background(), booking.ID)
	if err != nil {
		t.Fatalf("GetHourlyBooking: %v", err)
	}
	if got := stored.EndTime.Sub(stored.StartTime); got != time.Hour {
		t.Fatalf("duration: got %v want %v", got, time.Hour)
	}
}

func TestCreateHourlyBookingWithChecks_AllowsNinetyMinuteRange(t *testing.T) {
	db, user, cab, date := setupHourlyBookingTestDB(t, "09:00", "12:00", 30)

	booking := &model.HourlyBooking{
		UserID:    user.ID,
		CabinetID: cab.ID,
		StartTime: at(date, 9, 0),
		EndTime:   at(date, 10, 30),
		Status:    "pending",
	}
	if err := db.CreateHourlyBookingWithChecks(context.Background(), booking, nil); err != nil {
		t.Fatalf("CreateHourlyBookingWithChecks: %v", err)
	}

	stored, err := db.GetHourlyBooking(context.Background(), booking.ID)
	if err != nil {
		t.Fatalf("GetHourlyBooking: %v", err)
	}
	if got := stored.EndTime.Sub(stored.StartTime); got != 90*time.Minute {
		t.Fatalf("duration: got %v want %v", got, 90*time.Minute)
	}
}

func TestCreateHourlyBookingWithChecks_RejectsPartiallyOccupiedRange(t *testing.T) {
	db, user, cab, date := setupHourlyBookingTestDB(t, "09:00", "12:00", 30)

	busy := &model.HourlyBooking{
		UserID:    user.ID,
		CabinetID: cab.ID,
		StartTime: at(date, 9, 30),
		EndTime:   at(date, 10, 0),
		Status:    "pending",
	}
	if err := db.CreateHourlyBooking(context.Background(), busy); err != nil {
		t.Fatalf("CreateHourlyBooking: %v", err)
	}

	attempt := &model.HourlyBooking{
		UserID:    user.ID,
		CabinetID: cab.ID,
		StartTime: at(date, 9, 0),
		EndTime:   at(date, 10, 30),
		Status:    "pending",
	}
	if err := db.CreateHourlyBookingWithChecks(context.Background(), attempt, nil); err != ErrSlotNotAvailable {
		t.Fatalf("expected ErrSlotNotAvailable, got %v", err)
	}
}

func TestGetAvailableSlots_SkipsLunchBreak(t *testing.T) {
	db, _, cab, date := setupHourlyBookingTestDB(t, "09:00", "15:00", 30)

	ctx := context.Background()
	day := int(date.Weekday())
	if day == 0 {
		day = 7
	}
	if err := db.UpdateScheduleLunch(ctx, cab.ID, day, "12:00", "13:00"); err != nil {
		t.Fatalf("UpdateScheduleLunch: %v", err)
	}

	slots, err := db.GetAvailableSlots(ctx, cab.ID, date)
	if err != nil {
		t.Fatalf("GetAvailableSlots: %v", err)
	}

	for _, slot := range slots {
		if slot.StartTime >= "12:00" && slot.StartTime < "13:00" {
			t.Fatalf("lunch slot should not be returned: %+v", slot)
		}
	}

	ok, err := db.CheckSlotAvailability(ctx, cab.ID, date, at(date, 12, 0), at(date, 12, 30))
	if err != nil {
		t.Fatalf("CheckSlotAvailability: %v", err)
	}
	if ok {
		t.Fatal("expected lunch slot to be unavailable")
	}
}

func TestGetAvailableSlots_UsesOverrideAndDayOff(t *testing.T) {
	db, _, cab, _ := setupHourlyBookingTestDB(t, "09:00", "15:00", 30)

	ctx := context.Background()
	overrideDate := time.Date(2030, 4, 8, 0, 0, 0, 0, time.Local)
	if err := db.CreateScheduleOverride(ctx, &model.CabinetScheduleOverride{
		CabinetID:  cab.ID,
		Date:       overrideDate,
		StartTime:  "11:00",
		EndTime:    "13:00",
		LunchStart: "12:00",
		LunchEnd:   "12:30",
	}); err != nil {
		t.Fatalf("CreateScheduleOverride: %v", err)
	}

	slots, err := db.GetAvailableSlots(ctx, cab.ID, overrideDate)
	if err != nil {
		t.Fatalf("GetAvailableSlots: %v", err)
	}
	if len(slots) != 3 {
		t.Fatalf("expected 3 override slots, got %d", len(slots))
	}
	if slots[0].StartTime != "11:00" || slots[0].EndTime != "11:30" {
		t.Fatalf("unexpected first override slot: %+v", slots[0])
	}
	if slots[1].StartTime != "11:30" || slots[1].EndTime != "12:00" {
		t.Fatalf("unexpected second override slot: %+v", slots[1])
	}
	if slots[2].StartTime != "12:30" || slots[2].EndTime != "13:00" {
		t.Fatalf("unexpected slot after lunch override: %+v", slots[2])
	}

	dayOffDate := overrideDate.AddDate(0, 0, 1)
	if err := db.SetDayOff(ctx, cab.ID, dayOffDate, "holiday"); err != nil {
		t.Fatalf("SetDayOff: %v", err)
	}

	dayOffSlots, err := db.GetAvailableSlots(ctx, cab.ID, dayOffDate)
	if err != nil {
		t.Fatalf("GetAvailableSlots day off: %v", err)
	}
	if len(dayOffSlots) != 0 {
		t.Fatalf("expected no slots on day off, got %d", len(dayOffSlots))
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

func TestNewDBWithDriverRejectsUnsupportedDriver(t *testing.T) {
	_, err := NewDBWithDriver("postgres", "ignored", "postgres://example")
	if err == nil {
		t.Fatal("expected postgres driver to be rejected")
	}
}

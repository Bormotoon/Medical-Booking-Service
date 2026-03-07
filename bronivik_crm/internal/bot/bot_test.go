package bot

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"bronivik/bronivik_crm/internal/db"
	"bronivik/bronivik_crm/internal/model"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeAndValidatePhone(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		ok       bool
	}{
		{"+7 999 123-45-67", "+79991234567", true},
		{"89991234567", "89991234567", true},
		{"9991234567", "9991234567", true},
		{"123", "", false},
		{"", "", false},
		{"+1234567890123456", "", false}, // too long
	}

	for _, tt := range tests {
		res, ok := normalizeAndValidatePhone(tt.input)
		assert.Equal(t, tt.ok, ok, "input: %s", tt.input)
		assert.Equal(t, tt.expected, res, "input: %s", tt.input)
	}
}

func TestFilterDigits(t *testing.T) {
	assert.Equal(t, "123456", filterDigits("123-456 abc"))
	assert.Equal(t, "", filterDigits("abc"))
}

func TestParseTimeLabel(t *testing.T) {
	date := time.Date(2024, 12, 25, 0, 0, 0, 0, time.Local)
	label := "10:00-11:30"

	start, end, err := parseTimeLabel(date, label)
	assert.NoError(t, err)
	assert.Equal(t, 10, start.Hour())
	assert.Equal(t, 0, start.Minute())
	assert.Equal(t, 11, end.Hour())
	assert.Equal(t, 30, end.Minute())
	assert.Equal(t, date.Year(), start.Year())
}

func TestValidateBookingTime(t *testing.T) {
	b := &Bot{
		rules: &BookingRules{
			MinAdvance: 1 * time.Hour,
			MaxAdvance: 24 * time.Hour,
		},
	}

	now := time.Now()

	t.Run("TooSoon", func(t *testing.T) {
		err := b.validateBookingTime(now.Add(30 * time.Minute))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Слишком близко")
	})

	t.Run("TooFar", func(t *testing.T) {
		err := b.validateBookingTime(now.Add(48 * time.Hour))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Слишком далеко")
	})

	t.Run("OK", func(t *testing.T) {
		err := b.validateBookingTime(now.Add(2 * time.Hour))
		assert.NoError(t, err)
	})
}

func TestAvailableDurationOptionsStopAtBusyRange(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "crm.db")
	crmDB, err := db.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer crmDB.Close()

	ctx := context.Background()
	user, err := crmDB.GetOrCreateUserByTelegramID(ctx, 123, "u", "First", "Last", "")
	if err != nil {
		t.Fatalf("GetOrCreateUserByTelegramID: %v", err)
	}

	cab := &model.Cabinet{Name: "Cab1", Description: ""}
	if err = crmDB.CreateCabinet(ctx, cab); err != nil {
		t.Fatalf("CreateCabinet: %v", err)
	}

	date := time.Date(2026, 1, 5, 0, 0, 0, 0, time.Local)
	dow := int(date.Weekday())
	if dow == 0 {
		dow = 7
	}
	if err = crmDB.CreateSchedule(ctx, &model.CabinetSchedule{
		CabinetID:    cab.ID,
		DayOfWeek:    dow,
		StartTime:    "09:00",
		EndTime:      "12:00",
		SlotDuration: 30,
	}); err != nil {
		t.Fatalf("CreateSchedule: %v", err)
	}

	busy := &model.HourlyBooking{
		UserID:    user.ID,
		CabinetID: cab.ID,
		StartTime: time.Date(2026, 1, 5, 10, 0, 0, 0, time.Local),
		EndTime:   time.Date(2026, 1, 5, 10, 30, 0, 0, time.Local),
		Status:    "pending",
	}
	if err = crmDB.CreateHourlyBooking(ctx, busy); err != nil {
		t.Fatalf("CreateHourlyBooking: %v", err)
	}

	b := &Bot{db: crmDB}
	options, err := b.availableDurationOptions(ctx, cab.ID, date, time.Date(2026, 1, 5, 9, 0, 0, 0, time.Local))
	if err != nil {
		t.Fatalf("availableDurationOptions: %v", err)
	}

	assert.Equal(t, []int{30, 60}, options)
}

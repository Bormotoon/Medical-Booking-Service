package google

import (
	"bronivik/internal/models"
	"testing"
	"time"
)

func TestFilterActiveBookings(t *testing.T) {
	s := &SheetsService{}

	bookings := []models.Booking{
		{ID: 1, Status: "pending"},
		{ID: 2, Status: "confirmed"},
		{ID: 3, Status: "cancelled"},
		{ID: 4, Status: "completed"},
	}

	active := s.filterActiveBookings(bookings)

	if len(active) != 3 {
		t.Errorf("Expected 3 active bookings, got %d", len(active))
	}

	for _, b := range active {
		if b.Status == "cancelled" {
			t.Errorf("Cancelled booking found in active list")
		}
	}
}

func TestBookingRowValues(t *testing.T) {
	date := time.Date(2024, 12, 25, 0, 0, 0, 0, time.UTC)
	createdAt := time.Date(2024, 12, 20, 10, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2024, 12, 21, 11, 0, 0, 0, time.UTC)

	booking := &models.Booking{
		ID:        123,
		UserID:    456,
		ItemID:    789,
		Date:      date,
		Status:    "confirmed",
		UserName:  "Test User",
		Phone:     "79991234567",
		ItemName:  "Test Item",
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	values := bookingRowValues(booking)

	expected := []interface{}{
		int64(123),
		int64(456),
		int64(789),
		"2024-12-25",
		"confirmed",
		"Test User",
		"79991234567",
		"Test Item",
		"2024-12-20 10:00:00",
		"2024-12-21 11:00:00",
	}

	if len(values) != len(expected) {
		t.Fatalf("Expected %d values, got %d", len(expected), len(values))
	}

	for i, v := range values {
		if v != expected[i] {
			t.Errorf("At index %d: expected %v, got %v", i, expected[i], v)
		}
	}
}

func TestCacheOperations(t *testing.T) {
	s := &SheetsService{
		rowCache: make(map[int64]int),
	}

	s.setCachedRow(100, 5)
	row, ok := s.getCachedRow(100)
	if !ok || row != 5 {
		t.Errorf("Expected row 5, got %d (ok=%v)", row, ok)
	}

	s.deleteCacheRow(100)
	_, ok = s.getCachedRow(100)
	if ok {
		t.Errorf("Expected row to be deleted from cache")
	}

	s.setCachedRow(200, 10)
	s.ClearCache()
	_, ok = s.getCachedRow(200)
	if ok {
		t.Errorf("Expected cache to be cleared")
	}
}
func TestPrepareDateHeaders(t *testing.T) {
	s := &SheetsService{}
	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)

	headers, cols := s.prepareDateHeaders(startDate, endDate)
	if cols != 3 {
		t.Errorf("Expected 3 columns, got %d", cols)
	}
	if len(headers) != 4 {
		t.Errorf("Expected 4 headers, got %d", len(headers))
	}
	if headers[1] != "01.01" || headers[2] != "02.01" || headers[3] != "03.01" {
		t.Errorf("Unexpected headers: %v", headers)
	}
}

func TestFormatScheduleCell(t *testing.T) {
	s := &SheetsService{}
	item := models.Item{Name: "Camera", TotalQuantity: 2}

	t.Run("Empty", func(t *testing.T) {
		val, color := s.formatScheduleCell(item, nil)
		if val == "" || color == nil {
			t.Error("Expected non-empty value and color")
		}
	})

	t.Run("Booked", func(t *testing.T) {
		bookings := []models.Booking{
			{ID: 1, UserName: "User 1", Phone: "111", Status: models.StatusConfirmed},
		}
		val, _ := s.formatScheduleCell(item, bookings)
		if val == "" {
			t.Error("Expected non-empty value")
		}
	})
}

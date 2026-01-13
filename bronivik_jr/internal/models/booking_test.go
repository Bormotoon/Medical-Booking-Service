package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Helper function to create a date
func day(year int, month time.Month, d int) time.Time {
	return time.Date(year, month, d, 0, 0, 0, 0, time.UTC)
}

// Helper function to create a pointer to time
func dayPtr(year int, month time.Month, d int) *time.Time {
	t := day(year, month, d)
	return &t
}

func TestBooking_GetEffectiveEndTime(t *testing.T) {
	tests := []struct {
		name     string
		booking  Booking
		expected time.Time
	}{
		{
			name: "nil end_time returns date",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: nil,
			},
			expected: day(2026, 1, 15),
		},
		{
			name: "non-nil end_time returns end_time",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			expected: day(2026, 1, 20),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.booking.GetEffectiveEndTime()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBooking_IsRangeBooking(t *testing.T) {
	tests := []struct {
		name     string
		booking  Booking
		expected bool
	}{
		{
			name: "nil end_time is not range",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: nil,
			},
			expected: false,
		},
		{
			name: "end_time equals date is not range",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 15),
			},
			expected: false,
		},
		{
			name: "end_time after date is range",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.booking.IsRangeBooking()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBooking_OverlapsWith(t *testing.T) {
	tests := []struct {
		name     string
		existing Booking
		request  Booking
		overlap  bool
	}{
		{
			name: "no overlap - request before existing",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			request: Booking{
				Date:    day(2026, 1, 10),
				EndTime: dayPtr(2026, 1, 14),
			},
			overlap: false,
		},
		{
			name: "no overlap - request after existing",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			request: Booking{
				Date:    day(2026, 1, 21),
				EndTime: dayPtr(2026, 1, 25),
			},
			overlap: false,
		},
		{
			name: "overlap - request starts before, ends during",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			request: Booking{
				Date:    day(2026, 1, 13),
				EndTime: dayPtr(2026, 1, 16),
			},
			overlap: true,
		},
		{
			name: "overlap - request starts during, ends after",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			request: Booking{
				Date:    day(2026, 1, 19),
				EndTime: dayPtr(2026, 1, 25),
			},
			overlap: true,
		},
		{
			name: "overlap - request contained within existing",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			request: Booking{
				Date:    day(2026, 1, 16),
				EndTime: dayPtr(2026, 1, 18),
			},
			overlap: true,
		},
		{
			name: "overlap - request contains existing",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			request: Booking{
				Date:    day(2026, 1, 10),
				EndTime: dayPtr(2026, 1, 25),
			},
			overlap: true,
		},
		{
			name: "edge case - adjacent dates (no overlap with inclusive boundaries)",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			request: Booking{
				Date:    day(2026, 1, 20),
				EndTime: dayPtr(2026, 1, 25),
			},
			overlap: true, // 20th is included in both - overlap
		},
		{
			name: "edge case - day after existing ends",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			request: Booking{
				Date:    day(2026, 1, 21),
				EndTime: dayPtr(2026, 1, 25),
			},
			overlap: false,
		},
		{
			name: "null end_time treated as start_time - same day",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: nil,
			},
			request: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 15),
			},
			overlap: true,
		},
		{
			name: "null end_time - different days no overlap",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: nil,
			},
			request: Booking{
				Date:    day(2026, 1, 16),
				EndTime: dayPtr(2026, 1, 20),
			},
			overlap: false,
		},
		{
			name: "both null end_times - same day",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: nil,
			},
			request: Booking{
				Date:    day(2026, 1, 15),
				EndTime: nil,
			},
			overlap: true,
		},
		{
			name: "both null end_times - different days",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: nil,
			},
			request: Booking{
				Date:    day(2026, 1, 16),
				EndTime: nil,
			},
			overlap: false,
		},
		{
			name: "exact same range",
			existing: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			request: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			overlap: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.existing.OverlapsWith(&tt.request)
			assert.Equal(t, tt.overlap, result, "OverlapsWith returned unexpected result")

			// Also test reverse direction - overlap should be symmetric
			reverseResult := tt.request.OverlapsWith(&tt.existing)
			assert.Equal(t, tt.overlap, reverseResult, "OverlapsWith should be symmetric")
		})
	}
}

func TestBooking_ContainsDate(t *testing.T) {
	tests := []struct {
		name     string
		booking  Booking
		date     time.Time
		contains bool
	}{
		{
			name: "single day booking - exact match",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: nil,
			},
			date:     day(2026, 1, 15),
			contains: true,
		},
		{
			name: "single day booking - different day",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: nil,
			},
			date:     day(2026, 1, 16),
			contains: false,
		},
		{
			name: "range booking - date at start",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			date:     day(2026, 1, 15),
			contains: true,
		},
		{
			name: "range booking - date at end",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			date:     day(2026, 1, 20),
			contains: true,
		},
		{
			name: "range booking - date in middle",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			date:     day(2026, 1, 17),
			contains: true,
		},
		{
			name: "range booking - date before",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			date:     day(2026, 1, 14),
			contains: false,
		},
		{
			name: "range booking - date after",
			booking: Booking{
				Date:    day(2026, 1, 15),
				EndTime: dayPtr(2026, 1, 20),
			},
			date:     day(2026, 1, 21),
			contains: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.booking.ContainsDate(tt.date)
			assert.Equal(t, tt.contains, result)
		})
	}
}

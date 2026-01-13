package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func datetime(year int, month time.Month, day, hour, min int) time.Time {
	return time.Date(year, month, day, hour, min, 0, 0, time.UTC)
}

func TestHourlyBooking_Duration(t *testing.T) {
	b := HourlyBooking{
		StartTime: datetime(2026, 1, 15, 10, 0),
		EndTime:   datetime(2026, 1, 15, 12, 30),
	}
	assert.Equal(t, 2*time.Hour+30*time.Minute, b.Duration())
}

func TestHourlyBooking_SlotCount(t *testing.T) {
	b := HourlyBooking{
		StartTime: datetime(2026, 1, 15, 10, 0),
		EndTime:   datetime(2026, 1, 15, 12, 0),
	}
	assert.Equal(t, 4, b.SlotCount())
}

func TestHourlyBooking_IsRangeBooking(t *testing.T) {
	sameDay := HourlyBooking{
		StartTime: datetime(2026, 1, 15, 10, 0),
		EndTime:   datetime(2026, 1, 15, 14, 0),
	}
	assert.False(t, sameDay.IsRangeBooking())

	multiDay := HourlyBooking{
		StartTime: datetime(2026, 1, 15, 10, 0),
		EndTime:   datetime(2026, 1, 16, 10, 0),
	}
	assert.True(t, multiDay.IsRangeBooking())
}

func TestHourlyBooking_OverlapsWith(t *testing.T) {
	existing := HourlyBooking{
		StartTime: datetime(2026, 1, 15, 10, 0),
		EndTime:   datetime(2026, 1, 15, 14, 0),
	}

	// No overlap - before
	before := HourlyBooking{
		StartTime: datetime(2026, 1, 15, 8, 0),
		EndTime:   datetime(2026, 1, 15, 10, 0),
	}
	assert.False(t, existing.OverlapsWith(&before))

	// No overlap - after
	after := HourlyBooking{
		StartTime: datetime(2026, 1, 15, 14, 0),
		EndTime:   datetime(2026, 1, 15, 16, 0),
	}
	assert.False(t, existing.OverlapsWith(&after))

	// Overlap - starts during
	during := HourlyBooking{
		StartTime: datetime(2026, 1, 15, 12, 0),
		EndTime:   datetime(2026, 1, 15, 16, 0),
	}
	assert.True(t, existing.OverlapsWith(&during))

	// Overlap - contained
	contained := HourlyBooking{
		StartTime: datetime(2026, 1, 15, 11, 0),
		EndTime:   datetime(2026, 1, 15, 13, 0),
	}
	assert.True(t, existing.OverlapsWith(&contained))
}

func TestHourlyBooking_ContainsTime(t *testing.T) {
	b := HourlyBooking{
		StartTime: datetime(2026, 1, 15, 10, 0),
		EndTime:   datetime(2026, 1, 15, 14, 0),
	}

	assert.True(t, b.ContainsTime(datetime(2026, 1, 15, 10, 0)))
	assert.True(t, b.ContainsTime(datetime(2026, 1, 15, 12, 0)))
	assert.False(t, b.ContainsTime(datetime(2026, 1, 15, 14, 0)))
	assert.False(t, b.ContainsTime(datetime(2026, 1, 15, 9, 0)))
}

func TestHourlyBooking_ContainsDate(t *testing.T) {
	multiDay := HourlyBooking{
		StartTime: datetime(2026, 1, 15, 10, 0),
		EndTime:   datetime(2026, 1, 17, 14, 0),
	}

	assert.True(t, multiDay.ContainsDate(datetime(2026, 1, 15, 0, 0)))
	assert.True(t, multiDay.ContainsDate(datetime(2026, 1, 16, 0, 0)))
	assert.True(t, multiDay.ContainsDate(datetime(2026, 1, 17, 0, 0)))
	assert.False(t, multiDay.ContainsDate(datetime(2026, 1, 14, 0, 0)))
	assert.False(t, multiDay.ContainsDate(datetime(2026, 1, 18, 0, 0)))
}

package bot

import (
	"testing"
	"time"

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
		rules: BookingRules{
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

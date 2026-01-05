package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserState_Helpers(t *testing.T) {
	state := &UserState{
		TempData: map[string]interface{}{
			"int":    int64(123),
			"float":  123.45,
			"string": "hello",
			"time":   "2025-01-01T10:00:00Z",
			"dates":  []interface{}{"2025-01-01T10:00:00Z", "2025-01-02T10:00:00Z"},
		},
	}

	t.Run("GetInt64", func(t *testing.T) {
		assert.Equal(t, int64(123), state.GetInt64("int"))
		assert.Equal(t, int64(123), state.GetInt64("float"))
		assert.Equal(t, int64(0), state.GetInt64("string"))
		assert.Equal(t, int64(0), state.GetInt64("missing"))
	})

	t.Run("GetString", func(t *testing.T) {
		assert.Equal(t, "hello", state.GetString("string"))
		assert.Equal(t, "", state.GetString("int"))
		assert.Equal(t, "", state.GetString("missing"))
	})

	t.Run("GetTime", func(t *testing.T) {
		tm := state.GetTime("time")
		assert.False(t, tm.IsZero())
		assert.Equal(t, 2025, tm.Year())

		assert.True(t, state.GetTime("missing").IsZero())
	})

	t.Run("GetDates", func(t *testing.T) {
		dates := state.GetDates("dates")
		assert.Len(t, dates, 2)
		if len(dates) == 2 {
			assert.Equal(t, 2025, dates[0].Year())
		}

		assert.Nil(t, state.GetDates("missing"))
	})
}

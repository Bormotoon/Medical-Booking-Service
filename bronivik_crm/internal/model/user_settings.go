package model

import "time"

// UserSettings stores user preferences like reminder settings.
type UserSettings struct {
	ID                  int64     `json:"id"`
	UserID              int64     `json:"user_id"`
	RemindersEnabled    bool      `json:"reminders_enabled"`
	ReminderHoursBefore int       `json:"reminder_hours_before"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

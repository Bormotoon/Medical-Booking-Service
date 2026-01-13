package reminders

import (
	"context"
	"time"
)

// Booking represents a booking that may need a reminder.
type Booking interface {
	GetID() int64
	GetUserID() int64
	GetStartTime() time.Time
	IsReminderSent() bool
}

// BookingStore provides access to bookings for the reminder service.
type BookingStore interface {
	// GetUpcomingBookings returns bookings starting within the given duration
	// that haven't had reminders sent yet.
	GetUpcomingBookings(ctx context.Context, within time.Duration) ([]Booking, error)

	// MarkReminderSent marks a booking as having had its reminder sent.
	MarkReminderSent(ctx context.Context, bookingID int64) error
}

// UserSettingsStore provides access to user reminder settings.
type UserSettingsStore interface {
	// GetUserSettings returns reminder settings for a user.
	// If no settings exist, returns default settings (reminders enabled, 24h before).
	GetUserSettings(ctx context.Context, userID int64) (*UserSettings, error)
}

// UserSettings holds user preferences for reminders.
type UserSettings struct {
	UserID              int64
	RemindersEnabled    bool
	ReminderHoursBefore int
}

// DefaultUserSettings returns default settings for a user.
func DefaultUserSettings(userID int64) *UserSettings {
	return &UserSettings{
		UserID:              userID,
		RemindersEnabled:    true,
		ReminderHoursBefore: 24,
	}
}

// Notifier sends reminder notifications to users.
type Notifier interface {
	// SendReminder sends a reminder notification to the user about their booking.
	SendReminder(ctx context.Context, userID int64, booking Booking) error
}

// Logger interface for logging.
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

package reminders

import (
	"context"
	"time"
)

// ReminderType defines the type of reminder.
type ReminderType string

const (
	ReminderType24HBefore    ReminderType = "24h_before"
	ReminderTypeDayOfBooking ReminderType = "day_of_booking"
	ReminderTypeCustom       ReminderType = "custom"
)

// ReminderStatus defines the status of a reminder.
type ReminderStatus string

const (
	ReminderStatusPending    ReminderStatus = "pending"
	ReminderStatusScheduled  ReminderStatus = "scheduled"
	ReminderStatusProcessing ReminderStatus = "processing"
	ReminderStatusSent       ReminderStatus = "sent"
	ReminderStatusFailed     ReminderStatus = "failed"
	ReminderStatusCancelled  ReminderStatus = "cancelled"
)

// Reminder represents a scheduled reminder.
type Reminder struct {
	ID           int64
	UserID       int64
	BookingID    int64
	ReminderType ReminderType
	ScheduledAt  time.Time
	SentAt       *time.Time
	Status       ReminderStatus
	Enabled      bool
	RetryCount   int
	LastError    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ReminderFilter defines criteria for querying reminders.
type ReminderFilter struct {
	Enabled           *bool
	Status            []ReminderStatus
	ScheduledAtBefore *time.Time
	SentBefore        *time.Time
	UserID            *int64
	BookingID         *int64
}

// ReminderRepository provides access to reminders storage.
type ReminderRepository interface {
	// CreateReminder creates a new reminder.
	CreateReminder(ctx context.Context, r *Reminder) error

	// UpdateReminder updates an existing reminder.
	UpdateReminder(ctx context.Context, r *Reminder) error

	// FindReminders returns reminders matching the filter.
	FindReminders(ctx context.Context, filter ReminderFilter) ([]Reminder, error)

	// TryAcquireReminder atomically acquires a reminder for processing.
	// Returns true if acquired, false if already being processed.
	TryAcquireReminder(ctx context.Context, id int64) (bool, error)

	// ReleaseReminder releases a reminder after processing.
	ReleaseReminder(ctx context.Context, id int64) error

	// DeleteReminders deletes reminders matching the filter.
	DeleteReminders(ctx context.Context, filter ReminderFilter) (int64, error)

	// GetReminderByKey returns a reminder by unique key (user_id, booking_id, reminder_type).
	GetReminderByKey(ctx context.Context, userID, bookingID int64, reminderType ReminderType) (*Reminder, error)

	// CountPendingReminders returns the count of pending reminders.
	CountPendingReminders(ctx context.Context) (int64, error)
}

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

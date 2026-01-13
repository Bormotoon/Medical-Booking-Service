package models

import "time"

// Booking represents a device booking record.
type Booking struct {
	ID                int64      `json:"id"`
	UserID            int64      `json:"user_id"`
	UserName          string     `json:"user_name"`
	UserNickname      string     `json:"user_nickname"`
	Phone             string     `json:"phone"`
	ItemID            int64      `json:"item_id"`
	ItemName          string     `json:"item_name"`
	Date              time.Time  `json:"date"`               // start_time (kept as "date" for compatibility)
	EndTime           *time.Time `json:"end_time,omitempty"` // nullable: NULL means single-slot (end_time = Date)
	Status            string     `json:"status"`             // pending, confirmed, canceled, changed, completed
	Comment           string     `json:"comment"`
	ReminderSent      bool       `json:"reminder_sent"`
	ExternalBookingID string     `json:"external_booking_id,omitempty"` // ID from CRM bot
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	Version           int64      `json:"version"`
}

// GetEffectiveEndTime returns the effective end time for the booking.
// If EndTime is nil, it returns Date (single-slot booking).
func (b *Booking) GetEffectiveEndTime() time.Time {
	if b.EndTime != nil {
		return *b.EndTime
	}
	return b.Date
}

// IsRangeBooking returns true if this is a range booking (multiple days).
func (b *Booking) IsRangeBooking() bool {
	return b.EndTime != nil && !b.EndTime.Equal(b.Date)
}

// OverlapsWith checks if this booking overlaps with another booking.
// Uses half-open interval [start, end) semantics - end boundary is exclusive.
func (b *Booking) OverlapsWith(other *Booking) bool {
	// This booking's range
	thisStart := b.Date
	thisEnd := b.GetEffectiveEndTime()

	// Other booking's range
	otherStart := other.Date
	otherEnd := other.GetEffectiveEndTime()

	// Two intervals [A, B) and [C, D) overlap if A < D && C < B
	// For inclusive end boundaries: A <= D && C <= B
	// We use inclusive boundaries for date-based bookings
	return !thisEnd.Before(otherStart) && !otherEnd.Before(thisStart)
}

// ContainsDate checks if the booking covers a specific date.
func (b *Booking) ContainsDate(date time.Time) bool {
	start := b.Date
	end := b.GetEffectiveEndTime()

	// Normalize to date only (ignore time component)
	dateOnly := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	startOnly := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
	endOnly := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, end.Location())

	return !dateOnly.Before(startOnly) && !dateOnly.After(endOnly)
}

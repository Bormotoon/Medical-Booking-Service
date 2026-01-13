package model

import "time"

// HourlyBooking represents a cabinet hourly booking record.
type HourlyBooking struct {
	ID                      int64     `json:"id"`
	UserID                  int64     `json:"user_id"`
	CabinetID               int64     `json:"cabinet_id"`
	ItemID                  int64     `json:"item_id,omitempty"`
	ItemName                string    `json:"item_name"`
	ClientName              string    `json:"client_name"`
	ClientPhone             string    `json:"client_phone"`
	CabinetName             string    `json:"cabinet_name,omitempty"`
	StartTime               time.Time `json:"start_time"`
	EndTime                 time.Time `json:"end_time"`
	Status                  string    `json:"status"`
	Comment                 string    `json:"comment"`
	ManagerComment          string    `json:"manager_comment,omitempty"`
	ReminderSent            bool      `json:"reminder_sent"`
	ExternalDeviceBookingID int64     `json:"external_device_booking_id,omitempty"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// Duration returns the booking duration.
func (b *HourlyBooking) Duration() time.Duration {
	return b.EndTime.Sub(b.StartTime)
}

// SlotCount returns the number of 30-minute slots in this booking.
func (b *HourlyBooking) SlotCount() int {
	return int(b.Duration().Minutes() / 30)
}

// IsRangeBooking returns true if this booking spans multiple days.
func (b *HourlyBooking) IsRangeBooking() bool {
	startDate := time.Date(b.StartTime.Year(), b.StartTime.Month(), b.StartTime.Day(), 0, 0, 0, 0, b.StartTime.Location())
	endDate := time.Date(b.EndTime.Year(), b.EndTime.Month(), b.EndTime.Day(), 0, 0, 0, 0, b.EndTime.Location())
	return !startDate.Equal(endDate)
}

// OverlapsWith checks if this booking overlaps with another booking.
// Uses time-based overlap check for hourly bookings.
func (b *HourlyBooking) OverlapsWith(other *HourlyBooking) bool {
	// Two intervals [A, B) and [C, D) overlap if A < D && C < B
	return b.StartTime.Before(other.EndTime) && other.StartTime.Before(b.EndTime)
}

// ContainsTime checks if the booking covers a specific time.
func (b *HourlyBooking) ContainsTime(t time.Time) bool {
	return !t.Before(b.StartTime) && t.Before(b.EndTime)
}

// ContainsDate checks if the booking covers a specific date.
func (b *HourlyBooking) ContainsDate(date time.Time) bool {
	// Normalize to date only
	dateOnly := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	startDate := time.Date(b.StartTime.Year(), b.StartTime.Month(), b.StartTime.Day(), 0, 0, 0, 0, b.StartTime.Location())
	endDate := time.Date(b.EndTime.Year(), b.EndTime.Month(), b.EndTime.Day(), 0, 0, 0, 0, b.EndTime.Location())

	return !dateOnly.Before(startDate) && !dateOnly.After(endDate)
}

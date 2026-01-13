package model

import "time"

type CabinetSchedule struct {
	ID           int64     `json:"id"`
	CabinetID    int64     `json:"cabinet_id"`
	DayOfWeek    int       `json:"day_of_week"`   // 1=Mon, 7=Sun
	StartTime    string    `json:"start_time"`    // "10:00"
	EndTime      string    `json:"end_time"`      // "22:00"
	LunchStart   string    `json:"lunch_start"`   // "13:00" (optional)
	LunchEnd     string    `json:"lunch_end"`     // "14:00" (optional)
	SlotDuration int       `json:"slot_duration"` // minutes (default 30)
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CabinetScheduleOverride struct {
	ID         int64     `json:"id"`
	CabinetID  int64     `json:"cabinet_id"`
	Date       time.Time `json:"date"`
	IsClosed   bool      `json:"is_closed"` // fully closed
	StartTime  string    `json:"start_time"`
	EndTime    string    `json:"end_time"`
	LunchStart string    `json:"lunch_start"`
	LunchEnd   string    `json:"lunch_end"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

package model

import "time"

type CabinetSchedule struct {
	ID           int64     `json:"id"`
	CabinetID    int64     `json:"cabinet_id"`
	DayOfWeek    int       `json:"day_of_week"`   // 0-6 (Sunday-Saturday)
	StartTime    string    `json:"start_time"`    // "09:00"
	EndTime      string    `json:"end_time"`      // "18:00"
	SlotDuration int       `json:"slot_duration"` // minutes
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CabinetScheduleOverride struct {
	ID        int64     `json:"id"`
	CabinetID int64     `json:"cabinet_id"`
	Date      time.Time `json:"date"`
	IsClosed  bool      `json:"is_closed"` // fully closed
	StartTime string    `json:"start_time"`
	EndTime   string    `json:"end_time"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

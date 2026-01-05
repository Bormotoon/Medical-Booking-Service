package models

import "time"

type HourlyBooking struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	CabinetID int64     `json:"cabinet_id"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Status    string    `json:"status"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

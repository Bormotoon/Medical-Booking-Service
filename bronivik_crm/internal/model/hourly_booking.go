package model

import "time"

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

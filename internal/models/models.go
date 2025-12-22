package models

import "time"

type UserState struct {
	UserID      int64
	CurrentStep string
	TempData    map[string]interface{}
}

type Availability struct {
	Date      time.Time `json:"date"`
	ItemID    int64     `json:"item_id"`
	Booked    int64     `json:"booked"`
	Available int64     `json:"available"`
}

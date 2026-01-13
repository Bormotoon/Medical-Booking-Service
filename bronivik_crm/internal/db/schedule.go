package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"bronivik/bronivik_crm/internal/model"
)

// DefaultScheduleConfig provides default values for schedule.
var DefaultScheduleConfig = struct {
	StartTime    string
	EndTime      string
	SlotDuration int
}{
	StartTime:    "10:00",
	EndTime:      "22:00",
	SlotDuration: 30,
}

// EnsureDefaultSchedules creates default schedules for all active cabinets.
func (db *DB) EnsureDefaultSchedules(ctx context.Context) error {
	cabinets, err := db.ListActiveCabinets(ctx)
	if err != nil {
		return fmt.Errorf("list cabinets: %w", err)
	}

	for _, cab := range cabinets {
		for dayOfWeek := 1; dayOfWeek <= 7; dayOfWeek++ {
			exists, err := db.scheduleExists(ctx, cab.ID, dayOfWeek)
			if err != nil {
				return fmt.Errorf("check schedule: %w", err)
			}
			if exists {
				continue
			}

			sched := &model.CabinetSchedule{
				CabinetID:    cab.ID,
				DayOfWeek:    dayOfWeek,
				StartTime:    DefaultScheduleConfig.StartTime,
				EndTime:      DefaultScheduleConfig.EndTime,
				SlotDuration: DefaultScheduleConfig.SlotDuration,
			}
			if err := db.CreateSchedule(ctx, sched); err != nil {
				return fmt.Errorf("create schedule for cabinet %d day %d: %w", cab.ID, dayOfWeek, err)
			}
		}
	}
	return nil
}

func (db *DB) scheduleExists(ctx context.Context, cabinetID int64, dayOfWeek int) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM cabinet_schedules WHERE cabinet_id = ? AND day_of_week = ?",
		cabinetID, dayOfWeek,
	).Scan(&count)
	return count > 0, err
}

// GetScheduleForDate returns effective schedule for a specific date.
// First checks overrides, then falls back to regular weekly schedule.
func (db *DB) GetScheduleForDate(ctx context.Context, cabinetID int64, date time.Time) (*model.CabinetSchedule, *model.CabinetScheduleOverride, error) {
	// Check for override first
	override, err := db.GetScheduleOverride(ctx, cabinetID, date)
	if err != nil && err != sql.ErrNoRows {
		return nil, nil, err
	}

	// Get regular weekly schedule
	dayOfWeek := int(date.Weekday())
	if dayOfWeek == 0 {
		dayOfWeek = 7 // Sunday = 7
	}

	sched, err := db.GetScheduleByDay(ctx, cabinetID, dayOfWeek)
	if err != nil {
		return nil, override, err
	}

	return sched, override, nil
}

// GetScheduleByDay returns schedule for a specific day of week.
func (db *DB) GetScheduleByDay(ctx context.Context, cabinetID int64, dayOfWeek int) (*model.CabinetSchedule, error) {
	var s model.CabinetSchedule
	var lunchStart, lunchEnd sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT id, cabinet_id, day_of_week, start_time, end_time, 
		       lunch_start, lunch_end, slot_duration, is_active, created_at, updated_at
		FROM cabinet_schedules 
		WHERE cabinet_id = ? AND day_of_week = ? AND is_active = 1
		LIMIT 1`,
		cabinetID, dayOfWeek,
	).Scan(
		&s.ID, &s.CabinetID, &s.DayOfWeek, &s.StartTime, &s.EndTime,
		&lunchStart, &lunchEnd, &s.SlotDuration, &s.IsActive, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lunchStart.Valid {
		s.LunchStart = lunchStart.String
	}
	if lunchEnd.Valid {
		s.LunchEnd = lunchEnd.String
	}
	return &s, nil
}

// GetScheduleOverride returns override for a specific date.
func (db *DB) GetScheduleOverride(ctx context.Context, cabinetID int64, date time.Time) (*model.CabinetScheduleOverride, error) {
	var o model.CabinetScheduleOverride
	var startTime, endTime, lunchStart, lunchEnd, reason sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT id, cabinet_id, date, is_closed, start_time, end_time, 
		       lunch_start, lunch_end, reason, created_at, updated_at
		FROM cabinet_schedule_overrides 
		WHERE cabinet_id = ? AND date(date) = date(?)
		LIMIT 1`,
		cabinetID, date,
	).Scan(
		&o.ID, &o.CabinetID, &o.Date, &o.IsClosed, &startTime, &endTime,
		&lunchStart, &lunchEnd, &reason, &o.CreatedAt, &o.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if startTime.Valid {
		o.StartTime = startTime.String
	}
	if endTime.Valid {
		o.EndTime = endTime.String
	}
	if lunchStart.Valid {
		o.LunchStart = lunchStart.String
	}
	if lunchEnd.Valid {
		o.LunchEnd = lunchEnd.String
	}
	if reason.Valid {
		o.Reason = reason.String
	}
	return &o, nil
}

// CreateScheduleOverride creates or updates an override for a specific date.
func (db *DB) CreateScheduleOverride(ctx context.Context, o *model.CabinetScheduleOverride) error {
	if o == nil {
		return fmt.Errorf("override is nil")
	}

	now := time.Now()
	_, err := db.ExecContext(ctx, `
		INSERT INTO cabinet_schedule_overrides (
			cabinet_id, date, is_closed, start_time, end_time, 
			lunch_start, lunch_end, reason, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(cabinet_id, date) DO UPDATE SET
			is_closed = excluded.is_closed,
			start_time = excluded.start_time,
			end_time = excluded.end_time,
			lunch_start = excluded.lunch_start,
			lunch_end = excluded.lunch_end,
			reason = excluded.reason,
			updated_at = excluded.updated_at`,
		o.CabinetID, o.Date, o.IsClosed, o.StartTime, o.EndTime,
		o.LunchStart, o.LunchEnd, o.Reason, now, now,
	)
	return err
}

// DeleteScheduleOverride removes an override for a specific date.
func (db *DB) DeleteScheduleOverride(ctx context.Context, cabinetID int64, date time.Time) error {
	_, err := db.ExecContext(ctx,
		"DELETE FROM cabinet_schedule_overrides WHERE cabinet_id = ? AND date(date) = date(?)",
		cabinetID, date,
	)
	return err
}

// SetDayOff marks a specific date as closed.
func (db *DB) SetDayOff(ctx context.Context, cabinetID int64, date time.Time, reason string) error {
	override := &model.CabinetScheduleOverride{
		CabinetID: cabinetID,
		Date:      date,
		IsClosed:  true,
		Reason:    reason,
	}
	return db.CreateScheduleOverride(ctx, override)
}

// SetSpecialHours sets special working hours for a specific date.
func (db *DB) SetSpecialHours(ctx context.Context, cabinetID int64, date time.Time, startTime, endTime string) error {
	override := &model.CabinetScheduleOverride{
		CabinetID: cabinetID,
		Date:      date,
		IsClosed:  false,
		StartTime: startTime,
		EndTime:   endTime,
	}
	return db.CreateScheduleOverride(ctx, override)
}

// UpdateScheduleHours updates working hours for a specific day of week.
func (db *DB) UpdateScheduleHours(ctx context.Context, cabinetID int64, dayOfWeek int, startTime, endTime string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE cabinet_schedules 
		SET start_time = ?, end_time = ?, updated_at = ?
		WHERE cabinet_id = ? AND day_of_week = ?`,
		startTime, endTime, time.Now(), cabinetID, dayOfWeek,
	)
	return err
}

// UpdateScheduleLunch updates lunch break for a specific day of week.
func (db *DB) UpdateScheduleLunch(ctx context.Context, cabinetID int64, dayOfWeek int, lunchStart, lunchEnd string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE cabinet_schedules 
		SET lunch_start = ?, lunch_end = ?, updated_at = ?
		WHERE cabinet_id = ? AND day_of_week = ?`,
		lunchStart, lunchEnd, time.Now(), cabinetID, dayOfWeek,
	)
	return err
}

// ListScheduleOverrides returns all overrides for a cabinet within date range.
func (db *DB) ListScheduleOverrides(ctx context.Context, cabinetID int64, from, to time.Time) ([]model.CabinetScheduleOverride, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, cabinet_id, date, is_closed, start_time, end_time, 
		       lunch_start, lunch_end, reason, created_at, updated_at
		FROM cabinet_schedule_overrides 
		WHERE cabinet_id = ? AND date >= ? AND date <= ?
		ORDER BY date`,
		cabinetID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var overrides []model.CabinetScheduleOverride
	for rows.Next() {
		var o model.CabinetScheduleOverride
		var startTime, endTime, lunchStart, lunchEnd, reason sql.NullString
		if err := rows.Scan(
			&o.ID, &o.CabinetID, &o.Date, &o.IsClosed, &startTime, &endTime,
			&lunchStart, &lunchEnd, &reason, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if startTime.Valid {
			o.StartTime = startTime.String
		}
		if endTime.Valid {
			o.EndTime = endTime.String
		}
		if lunchStart.Valid {
			o.LunchStart = lunchStart.String
		}
		if lunchEnd.Valid {
			o.LunchEnd = lunchEnd.String
		}
		if reason.Valid {
			o.Reason = reason.String
		}
		overrides = append(overrides, o)
	}
	return overrides, rows.Err()
}

// HasActiveBookingsOnDate checks if there are any non-canceled bookings for a date.
func (db *DB) HasActiveBookingsOnDate(ctx context.Context, cabinetID int64, date time.Time) (bool, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM hourly_bookings
		WHERE cabinet_id = ? 
		AND start_time >= ? AND start_time < ?
		AND status NOT IN ('canceled', 'rejected')`,
		cabinetID, startOfDay, endOfDay,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetActiveBookingsOnDate returns all active bookings for a specific date.
func (db *DB) GetActiveBookingsOnDate(ctx context.Context, cabinetID int64, date time.Time) ([]model.HourlyBooking, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, cabinet_id, item_name, client_name, client_phone,
		       start_time, end_time, status, comment, created_at, updated_at
		FROM hourly_bookings
		WHERE cabinet_id = ?
		AND start_time >= ? AND start_time < ?
		AND status NOT IN ('canceled', 'rejected')
		ORDER BY start_time`,
		cabinetID, startOfDay, endOfDay,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []model.HourlyBooking
	for rows.Next() {
		b, err := scanHourly(rows)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, *b)
	}
	return bookings, rows.Err()
}

// IsSlotBooked checks if a slot is already booked.
func (db *DB) IsSlotBooked(ctx context.Context, cabinetID int64, start, end time.Time) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM hourly_bookings
		WHERE cabinet_id = ?
		AND start_time < ? AND end_time > ?
		AND status NOT IN ('canceled', 'rejected')`,
		cabinetID, end, start,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

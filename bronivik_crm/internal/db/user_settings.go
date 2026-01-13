package db

import (
	"context"
	"database/sql"
	"time"

	"bronivik/bronivik_crm/internal/model"
)

// GetUserSettings returns user settings by user ID.
// If no settings exist, returns default settings.
func (db *DB) GetUserSettings(ctx context.Context, userID int64) (*model.UserSettings, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, user_id, reminders_enabled, reminder_hours_before, 
		       created_at, updated_at
		FROM user_settings
		WHERE user_id = ?`, userID)

	var s model.UserSettings
	err := row.Scan(&s.ID, &s.UserID, &s.RemindersEnabled, &s.ReminderHoursBefore,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return default settings
			return &model.UserSettings{
				UserID:              userID,
				RemindersEnabled:    true,
				ReminderHoursBefore: 24,
			}, nil
		}
		return nil, err
	}
	return &s, nil
}

// UpsertUserSettings creates or updates user settings.
func (db *DB) UpsertUserSettings(ctx context.Context, userID int64, remindersEnabled bool, hoursBefore int) error {
	now := time.Now()

	_, err := db.ExecContext(ctx, `
		INSERT INTO user_settings (user_id, reminders_enabled, reminder_hours_before, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			reminders_enabled = excluded.reminders_enabled,
			reminder_hours_before = excluded.reminder_hours_before,
			updated_at = excluded.updated_at`,
		userID, remindersEnabled, hoursBefore, now, now)
	return err
}

// ToggleReminders toggles reminder setting for a user and returns new state.
func (db *DB) ToggleReminders(ctx context.Context, userID int64) (bool, error) {
	settings, err := db.GetUserSettings(ctx, userID)
	if err != nil {
		return false, err
	}

	newState := !settings.RemindersEnabled
	err = db.UpsertUserSettings(ctx, userID, newState, settings.ReminderHoursBefore)
	if err != nil {
		return false, err
	}

	return newState, nil
}

// GetUpcomingBookingsForReminders returns bookings that need reminders sent.
func (db *DB) GetUpcomingBookingsForReminders(ctx context.Context, within time.Duration) ([]model.HourlyBooking, error) {
	now := time.Now()
	until := now.Add(within)

	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, cabinet_id, item_id, item_name, client_name, client_phone,
		       start_time, end_time, status, comment, manager_comment, reminder_sent,
		       external_device_booking_id, created_at, updated_at
		FROM hourly_bookings
		WHERE start_time BETWEEN ? AND ?
		  AND status IN ('pending', 'approved', 'confirmed')
		  AND reminder_sent = 0
		ORDER BY start_time ASC`, now, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []model.HourlyBooking
	for rows.Next() {
		var b model.HourlyBooking
		var itemID, extID sql.NullInt64
		var itemName, mgrComment sql.NullString
		err := rows.Scan(&b.ID, &b.UserID, &b.CabinetID, &itemID, &itemName,
			&b.ClientName, &b.ClientPhone, &b.StartTime, &b.EndTime, &b.Status,
			&b.Comment, &mgrComment, &b.ReminderSent, &extID, &b.CreatedAt, &b.UpdatedAt)
		if err != nil {
			return nil, err
		}
		if itemID.Valid {
			b.ItemID = itemID.Int64
		}
		if itemName.Valid {
			b.ItemName = itemName.String
		}
		if mgrComment.Valid {
			b.ManagerComment = mgrComment.String
		}
		if extID.Valid {
			b.ExternalDeviceBookingID = extID.Int64
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

// MarkBookingReminderSent marks a booking as having had its reminder sent.
func (db *DB) MarkBookingReminderSent(ctx context.Context, bookingID int64) error {
	_, err := db.ExecContext(ctx, `
		UPDATE hourly_bookings SET reminder_sent = 1, updated_at = ?
		WHERE id = ?`, time.Now(), bookingID)
	return err
}

// DeleteOldBookings deletes bookings older than the given duration.
// Returns the number of deleted rows.
func (db *DB) DeleteOldBookings(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := db.ExecContext(ctx, `
		DELETE FROM hourly_bookings WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

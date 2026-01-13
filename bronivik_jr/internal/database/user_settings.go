package database

import (
	"context"
	"database/sql"
	"time"

	"bronivik/internal/models"
)

// GetUserSettings returns user settings by telegram ID.
// If no settings exist, returns default settings.
func (db *DB) GetUserSettings(ctx context.Context, telegramID int64) (*models.UserSettings, error) {
	row := db.QueryRowContext(ctx, `
		SELECT us.id, us.user_id, us.reminders_enabled, us.reminder_hours_before, 
		       us.created_at, us.updated_at
		FROM user_settings us
		JOIN users u ON u.telegram_id = us.user_id
		WHERE u.telegram_id = ?`, telegramID)

	var s models.UserSettings
	err := row.Scan(&s.ID, &s.UserID, &s.RemindersEnabled, &s.ReminderHoursBefore,
		&s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			// Return default settings
			return &models.UserSettings{
				UserID:              telegramID,
				RemindersEnabled:    true,
				ReminderHoursBefore: 24,
			}, nil
		}
		return nil, err
	}
	return &s, nil
}

// UpsertUserSettings creates or updates user settings.
func (db *DB) UpsertUserSettings(ctx context.Context, telegramID int64, remindersEnabled bool, hoursBefore int) error {
	now := time.Now()

	// First ensure user exists and get their telegram_id for FK
	_, err := db.ExecContext(ctx, `
		INSERT INTO user_settings (user_id, reminders_enabled, reminder_hours_before, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
			reminders_enabled = excluded.reminders_enabled,
			reminder_hours_before = excluded.reminder_hours_before,
			updated_at = excluded.updated_at`,
		telegramID, remindersEnabled, hoursBefore, now, now)
	return err
}

// ToggleReminders toggles reminder setting for a user and returns new state.
func (db *DB) ToggleReminders(ctx context.Context, telegramID int64) (bool, error) {
	settings, err := db.GetUserSettings(ctx, telegramID)
	if err != nil {
		return false, err
	}

	newState := !settings.RemindersEnabled
	err = db.UpsertUserSettings(ctx, telegramID, newState, settings.ReminderHoursBefore)
	if err != nil {
		return false, err
	}

	return newState, nil
}

// GetUpcomingBookingsForReminders returns bookings that need reminders sent.
func (db *DB) GetUpcomingBookingsForReminders(ctx context.Context, within time.Duration) ([]models.Booking, error) {
	now := time.Now()
	until := now.Add(within)

	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, user_name, user_nickname, phone, item_id, item_name,
		       date, status, comment, reminder_sent, external_booking_id,
		       created_at, updated_at, version
		FROM bookings
		WHERE date BETWEEN ? AND ?
		  AND status IN ('pending', 'confirmed')
		  AND reminder_sent = 0
		ORDER BY date ASC`, now, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		var b models.Booking
		var extID sql.NullString
		err := rows.Scan(&b.ID, &b.UserID, &b.UserName, &b.UserNickname, &b.Phone,
			&b.ItemID, &b.ItemName, &b.Date, &b.Status, &b.Comment, &b.ReminderSent,
			&extID, &b.CreatedAt, &b.UpdatedAt, &b.Version)
		if err != nil {
			return nil, err
		}
		if extID.Valid {
			b.ExternalBookingID = extID.String
		}
		bookings = append(bookings, b)
	}
	return bookings, rows.Err()
}

// MarkBookingReminderSent marks a booking as having had its reminder sent.
func (db *DB) MarkBookingReminderSent(ctx context.Context, bookingID int64) error {
	_, err := db.ExecContext(ctx, `
		UPDATE bookings SET reminder_sent = 1, updated_at = ?
		WHERE id = ?`, time.Now(), bookingID)
	return err
}

// DeleteOldBookings deletes bookings older than the given duration.
// Returns the number of deleted rows.
func (db *DB) DeleteOldBookings(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := db.ExecContext(ctx, `
		DELETE FROM bookings WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

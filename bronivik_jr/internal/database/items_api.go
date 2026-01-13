package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"bronivik/internal/models"
)

// CreateExternalBooking creates a booking from bronivik_crm API.
func (db *DB) CreateExternalBooking(
	ctx context.Context,
	itemID int64,
	itemName string,
	date time.Time,
	externalBookingID string,
	clientName string,
	clientPhone string,
) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Check if external booking ID already exists
	var existingID int64
	err = tx.QueryRowContext(ctx,
		"SELECT id FROM bookings WHERE external_booking_id = ?",
		externalBookingID,
	).Scan(&existingID)
	if err == nil {
		return existingID, nil // Already exists, return existing ID
	}
	if err != sql.ErrNoRows {
		return 0, fmt.Errorf("check existing: %w", err)
	}

	// Check availability
	var bookedCount int64
	err = tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM bookings 
		WHERE item_id = ? AND date(date) = date(?) AND status = 'approved'`,
		itemID, date,
	).Scan(&bookedCount)
	if err != nil {
		return 0, fmt.Errorf("check availability: %w", err)
	}

	// Get item quantity
	var totalQty int64
	err = tx.QueryRowContext(ctx,
		"SELECT total_quantity FROM items WHERE id = ?",
		itemID,
	).Scan(&totalQty)
	if err != nil {
		return 0, fmt.Errorf("get quantity: %w", err)
	}

	if bookedCount >= totalQty {
		return 0, ErrNotAvailable
	}

	// Create booking
	now := time.Now()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO bookings (
			user_id, user_name, user_nickname, phone, item_id, item_name,
			date, status, external_booking_id, created_at, updated_at, version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		0, // API booking has no telegram user
		clientName,
		"",
		clientPhone,
		itemID,
		itemName,
		date,
		"approved", // Auto-approve external bookings
		externalBookingID,
		now,
		now,
		1,
	)
	if err != nil {
		return 0, fmt.Errorf("insert booking: %w", err)
	}

	bookingID, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	return bookingID, nil
}

// CancelExternalBooking cancels a booking by external ID.
func (db *DB) CancelExternalBooking(ctx context.Context, externalBookingID string) error {
	result, err := db.ExecContext(ctx, `
		UPDATE bookings 
		SET status = 'canceled', updated_at = ?
		WHERE external_booking_id = ? AND status != 'canceled'`,
		time.Now(), externalBookingID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("booking not found or already canceled")
	}

	return nil
}

// GetExternalBooking returns booking by external ID.
func (db *DB) GetExternalBooking(ctx context.Context, externalBookingID string) (*models.Booking, error) {
	var b models.Booking
	err := db.QueryRowContext(ctx, `
		SELECT id, user_id, user_name, user_nickname, phone, item_id, item_name,
		       date, status, comment, reminder_sent, external_booking_id,
		       created_at, updated_at, version
		FROM bookings WHERE external_booking_id = ?`,
		externalBookingID,
	).Scan(
		&b.ID, &b.UserID, &b.UserName, &b.UserNickname, &b.Phone,
		&b.ItemID, &b.ItemName, &b.Date, &b.Status, &b.Comment,
		&b.ReminderSent, &b.ExternalBookingID, &b.CreatedAt, &b.UpdatedAt, &b.Version,
	)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// SetItemPermanentReserved marks/unmarks an item as permanently reserved.
func (db *DB) SetItemPermanentReserved(ctx context.Context, itemID int64, reserved bool) error {
	result, err := db.ExecContext(ctx, `
		UPDATE items SET permanent_reserved = ?, updated_at = ? WHERE id = ?`,
		reserved, time.Now(), itemID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("item not found")
	}

	// Refresh cache
	_ = db.LoadItems(ctx)
	return nil
}

// ListPermanentReservedItems returns items marked as permanently reserved.
func (db *DB) ListPermanentReservedItems(ctx context.Context) ([]models.Item, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, description, total_quantity, sort_order,
		       is_active, permanent_reserved, created_at, updated_at
		FROM items WHERE permanent_reserved = 1 AND is_active = 1
		ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Item
	for rows.Next() {
		var i models.Item
		if err := rows.Scan(
			&i.ID, &i.Name, &i.Description, &i.TotalQuantity, &i.SortOrder,
			&i.IsActive, &i.PermanentReserved, &i.CreatedAt, &i.UpdatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, rows.Err()
}

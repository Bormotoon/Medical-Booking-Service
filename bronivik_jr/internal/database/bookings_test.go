package database

import (
	"context"
	"os"
	"testing"
	"time"

	"bronivik/internal/models"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *DB {
	logger := zerolog.New(os.Stdout)
	db, err := NewDB(":memory:", &logger)
	require.NoError(t, err)
	return db
}

func TestGetAvailabilityForPeriod(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Create test item
	item := &models.Item{
		Name:          "Test Item",
		TotalQuantity: 2,
		IsActive:      true,
	}
	err := db.CreateItem(ctx, item)
	require.NoError(t, err)

	// Get the created item to have its ID
	items, err := db.GetActiveItems(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	itemID := items[0].ID

	startDate := time.Now().Truncate(24 * time.Hour)

	// Create some bookings
	// Date 0: 2 bookings (Full)
	// Date 1: 1 booking (1 available)
	// Date 2: 0 bookings (2 available)

	bookings := []models.Booking{
		{
			ItemID: itemID, ItemName: item.Name, Date: startDate,
			UserID: 1, UserName: "User 1", Phone: "123", Status: models.StatusConfirmed,
		},
		{
			ItemID: itemID, ItemName: item.Name, Date: startDate,
			UserID: 2, UserName: "User 2", Phone: "456", Status: models.StatusConfirmed,
		},
		{
			ItemID: itemID, ItemName: item.Name, Date: startDate.AddDate(0, 0, 1),
			UserID: 3, UserName: "User 3", Phone: "789", Status: models.StatusConfirmed,
		},
	}

	for _, b := range bookings {
		err = db.CreateBooking(ctx, &b)
		require.NoError(t, err)
	}

	// Test GetAvailabilityForPeriod
	availability, err := db.GetAvailabilityForPeriod(ctx, itemID, startDate, 3)
	require.NoError(t, err)
	require.Len(t, availability, 3)

	// Date 0
	assert.Equal(t, startDate, availability[0].Date)
	assert.Equal(t, int64(0), availability[0].Available)
	assert.Equal(t, int64(2), availability[0].Booked)

	// Date 1
	assert.Equal(t, startDate.AddDate(0, 0, 1), availability[1].Date)
	assert.Equal(t, int64(1), availability[1].Available)
	assert.Equal(t, int64(1), availability[1].Booked)

	// Date 2
	assert.Equal(t, startDate.AddDate(0, 0, 2), availability[2].Date)
	assert.Equal(t, int64(2), availability[2].Available)
	assert.Equal(t, int64(0), availability[2].Booked)
}

func TestCheckAvailability(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	item := &models.Item{
		Name:          "Test Item",
		TotalQuantity: 1,
		IsActive:      true,
	}
	err := db.CreateItem(ctx, item)
	require.NoError(t, err)

	items, _ := db.GetActiveItems(ctx)
	itemID := items[0].ID

	date := time.Now().Truncate(24 * time.Hour)

	// Initially available
	available, err := db.CheckAvailability(ctx, itemID, date)
	require.NoError(t, err)
	assert.True(t, available)

	// Create booking
	booking := &models.Booking{
		ItemID: itemID, ItemName: item.Name, Date: date, UserID: 1, UserName: "User 1", Phone: "123", Status: models.StatusConfirmed,
	}
	err = db.CreateBooking(ctx, booking)
	require.NoError(t, err)

	// Now unavailable
	available, err = db.CheckAvailability(ctx, itemID, date)
	require.NoError(t, err)
	assert.False(t, available)
}

func TestGetBookedCountCountsTimestampDates(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	item := &models.Item{
		Name:          "Timestamped Item",
		TotalQuantity: 1,
		IsActive:      true,
	}
	err := db.CreateItem(ctx, item)
	require.NoError(t, err)

	items, err := db.GetActiveItems(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	itemID := items[0].ID

	date := time.Date(2026, 2, 21, 0, 0, 0, 0, time.UTC)
	timestampDate := date.Add(13 * time.Hour)
	now := time.Now()

	_, err = db.ExecContext(ctx, `
		INSERT INTO bookings (
			user_id, user_name, user_nickname, phone, item_id, item_name,
			date, status, external_booking_id, created_at, updated_at, version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		0,
		"External Client",
		"",
		"+70000000000",
		itemID,
		item.Name,
		timestampDate,
		"approved",
		"legacy-ts-1",
		now,
		now,
		1,
	)
	require.NoError(t, err)

	bookedCount, err := db.GetBookedCount(ctx, itemID, date)
	require.NoError(t, err)
	assert.Equal(t, 1, bookedCount)

	available, err := db.CheckAvailability(ctx, itemID, date)
	require.NoError(t, err)
	assert.False(t, available)
}

func TestCreateExternalBookingBlocksPendingBooking(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	item := &models.Item{Name: "Shared Item", TotalQuantity: 1, IsActive: true}
	require.NoError(t, db.CreateItem(ctx, item))

	date := time.Date(2026, 3, 21, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.CreateBooking(ctx, &models.Booking{
		ItemID:   item.ID,
		ItemName: item.Name,
		Date:     date,
		UserID:   1,
		UserName: "Pending User",
		Phone:    "+70000000001",
		Status:   models.StatusPending,
	}))

	bookingID, err := db.CreateExternalBooking(ctx, item.ID, item.Name, date, "ext-pending", "External", "+79990000001")
	assert.ErrorIs(t, err, ErrNotAvailable)
	assert.Zero(t, bookingID)
}

func TestCreateExternalBookingBlocksConfirmedBooking(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	item := &models.Item{Name: "Shared Item", TotalQuantity: 1, IsActive: true}
	require.NoError(t, db.CreateItem(ctx, item))

	date := time.Date(2026, 3, 22, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.CreateBooking(ctx, &models.Booking{
		ItemID:   item.ID,
		ItemName: item.Name,
		Date:     date,
		UserID:   2,
		UserName: "Confirmed User",
		Phone:    "+70000000002",
		Status:   models.StatusConfirmed,
	}))

	bookingID, err := db.CreateExternalBooking(ctx, item.ID, item.Name, date, "ext-confirmed", "External", "+79990000002")
	assert.ErrorIs(t, err, ErrNotAvailable)
	assert.Zero(t, bookingID)
}

func TestCreateExternalBookingAllowsBookingAfterCancellation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	item := &models.Item{Name: "Shared Item", TotalQuantity: 1, IsActive: true}
	require.NoError(t, db.CreateItem(ctx, item))

	date := time.Date(2026, 3, 23, 0, 0, 0, 0, time.UTC)
	internalBooking := &models.Booking{
		ItemID:   item.ID,
		ItemName: item.Name,
		Date:     date,
		UserID:   3,
		UserName: "Canceled User",
		Phone:    "+70000000003",
		Status:   models.StatusPending,
	}
	require.NoError(t, db.CreateBooking(ctx, internalBooking))
	require.NoError(t, db.UpdateBookingStatus(ctx, internalBooking.ID, models.StatusCanceled))

	bookingID, err := db.CreateExternalBooking(ctx, item.ID, item.Name, date, "ext-canceled", "External", "+79990000003")
	require.NoError(t, err)
	assert.NotZero(t, bookingID)

	stored, err := db.GetExternalBooking(ctx, "ext-canceled")
	require.NoError(t, err)
	assert.Equal(t, models.StatusConfirmed, stored.Status)

	bookedCount, err := db.GetBookedCount(ctx, item.ID, date)
	require.NoError(t, err)
	assert.Equal(t, 1, bookedCount)
}

func TestOptimisticLocking(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Create booking
	booking := &models.Booking{
		ItemID: 1, ItemName: "Test", Date: time.Now(), UserID: 1, UserName: "User 1", Phone: "123", Status: models.StatusPending,
	}
	err := db.CreateBooking(ctx, booking)
	require.NoError(t, err)
	assert.Equal(t, int64(1), booking.Version)

	// Successful update
	err = db.UpdateBookingStatusWithVersion(ctx, booking.ID, booking.Version, models.StatusConfirmed)
	require.NoError(t, err)

	// Failed update with old version
	err = db.UpdateBookingStatusWithVersion(ctx, booking.ID, booking.Version, models.StatusCanceled)
	assert.ErrorIs(t, err, ErrConcurrentModification)

	// Successful update with new version
	updated, _ := db.GetBooking(ctx, booking.ID)
	assert.Equal(t, int64(2), updated.Version)
	err = db.UpdateBookingStatusWithVersion(ctx, updated.ID, updated.Version, models.StatusCanceled)
	require.NoError(t, err)
}

func TestGetDailyBookings(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	date := time.Date(2025, 5, 22, 0, 0, 0, 0, time.UTC)

	err := db.CreateBooking(ctx, &models.Booking{
		ItemID: 1, ItemName: "Item 1", Date: date, UserID: 1, UserName: "User 1", Phone: "123", Status: models.StatusConfirmed,
	})
	require.NoError(t, err)

	daily, err := db.GetDailyBookings(ctx, date, date)
	require.NoError(t, err)
	assert.NotEmpty(t, daily)
	assert.Len(t, daily["2025-05-22"], 1)
}

func TestGetUserBookings(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	err := db.CreateBooking(ctx, &models.Booking{
		ItemID: 1, ItemName: "Item 1", Date: time.Now(), UserID: 123, UserName: "User 1", Phone: "123", Status: models.StatusConfirmed,
	})
	require.NoError(t, err)

	bookings, err := db.GetUserBookings(ctx, 123)
	require.NoError(t, err)
	assert.Len(t, bookings, 1)
}

func TestUpdateBookingItem(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	booking := &models.Booking{
		ItemID: 1, ItemName: "Item 1", Date: time.Now(), UserID: 1, UserName: "User 1", Phone: "123", Status: models.StatusPending,
	}
	err := db.CreateBooking(ctx, booking)
	require.NoError(t, err)

	err = db.UpdateBookingItemAndStatusWithVersion(ctx, booking.ID, booking.Version, 2, "Item 2", models.StatusChanged)
	require.NoError(t, err)

	updated, _ := db.GetBooking(ctx, booking.ID)
	assert.Equal(t, int64(2), updated.ItemID)
	assert.Equal(t, "Item 2", updated.ItemName)
	assert.Equal(t, models.StatusChanged, updated.Status)
}

func TestBookingUpdateExtras(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	booking := &models.Booking{
		ItemID: 1, ItemName: "Item 1", Date: time.Now(), UserID: 1, UserName: "User 1", Phone: "123", Status: models.StatusPending,
	}
	err := db.CreateBooking(ctx, booking)
	require.NoError(t, err)

	t.Run("UpdateComment", func(t *testing.T) {
		err := db.UpdateBookingComment(ctx, booking.ID, "New Comment")
		require.NoError(t, err)
		updated, _ := db.GetBooking(ctx, booking.ID)
		assert.Equal(t, "New Comment", updated.Comment)
	})

	t.Run("UpdateStatus", func(t *testing.T) {
		err := db.UpdateBookingStatus(ctx, booking.ID, models.StatusConfirmed)
		require.NoError(t, err)
		updated, _ := db.GetBooking(ctx, booking.ID)
		assert.Equal(t, models.StatusConfirmed, updated.Status)
	})

	t.Run("UpdateItem", func(t *testing.T) {
		err := db.UpdateBookingItem(ctx, booking.ID, 2, "Item 2")
		require.NoError(t, err)
		updated, _ := db.GetBooking(ctx, booking.ID)
		assert.Equal(t, int64(2), updated.ItemID)
		assert.Equal(t, "Item 2", updated.ItemName)
	})

	t.Run("UpdateItemWithVersion", func(t *testing.T) {
		// Version should have increased from previous updates
		// (2 updates = +0 since not using WithVersion versions of update in subtests above until now)
		// Wait, UpdateBookingComment and UpdateBookingStatus do NOT increment version in the current implementation!
		// Only WithVersion versions increment it.
		// Let's check GetBooking to see current version.
		current, _ := db.GetBooking(ctx, booking.ID)
		v := current.Version

		err := db.UpdateBookingItemWithVersion(ctx, booking.ID, v, 3, "Item 3")
		require.NoError(t, err)

		err = db.UpdateBookingItemWithVersion(ctx, booking.ID, v, 4, "Item 4")
		assert.ErrorIs(t, err, ErrConcurrentModification)
	})

	t.Run("UpdateDateAndStatusWithVersion", func(t *testing.T) {
		current, _ := db.GetBooking(ctx, booking.ID)
		v := current.Version
		newDate := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)

		err := db.UpdateBookingDateAndStatusWithVersion(ctx, booking.ID, v, newDate, models.StatusChanged)
		require.NoError(t, err)

		updated, _ := db.GetBooking(ctx, booking.ID)
		assert.Equal(t, newDate.Format("2006-01-02"), updated.Date.Format("2006-01-02"))
		assert.Equal(t, models.StatusChanged, updated.Status)

		err = db.UpdateBookingDateAndStatusWithVersion(ctx, booking.ID, v, newDate.AddDate(0, 0, 1), models.StatusChanged)
		assert.ErrorIs(t, err, ErrConcurrentModification)
	})
}

func TestGetBookingWithAvailability(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	item1 := &models.Item{Name: "Item 1", TotalQuantity: 1, IsActive: true}
	err := db.CreateItem(ctx, item1)
	require.NoError(t, err)

	date := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	booking := &models.Booking{
		ItemID:   item1.ID,
		ItemName: item1.Name,
		Date:     date,
		UserID:   1,
		UserName: "U1",
		Phone:    "1",
		Status:   models.StatusConfirmed,
	}
	err = db.CreateBooking(ctx, booking)
	require.NoError(t, err)

	// Check another item at same date
	item2 := &models.Item{Name: "Item 2", TotalQuantity: 1, IsActive: true}
	err = db.CreateItem(ctx, item2)
	require.NoError(t, err)

	b, available, err := db.GetBookingWithAvailability(ctx, booking.ID, item2.ID)
	require.NoError(t, err)
	assert.Equal(t, booking.ID, b.ID)
	assert.True(t, available)

	// Check same item (should be unavailable since it's fully booked by itself)
	_, available, err = db.GetBookingWithAvailability(ctx, booking.ID, item1.ID)
	require.NoError(t, err)
	assert.False(t, available)
}

func TestCreateBookingWithLock_Failure(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()
	item := &models.Item{Name: "Item 1", TotalQuantity: 1, IsActive: true}
	err := db.CreateItem(ctx, item)
	require.NoError(t, err)

	date := time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)
	b1 := &models.Booking{
		ItemID: item.ID, ItemName: "Item 1", Date: date,
		UserID: 1, UserName: "U1", Phone: "1", Status: models.StatusConfirmed,
	}
	err = db.CreateBookingWithLock(ctx, b1)
	require.NoError(t, err)

	b2 := &models.Booking{
		ItemID: item.ID, ItemName: "Item 1", Date: date,
		UserID: 2, UserName: "U2", Phone: "2", Status: models.StatusConfirmed,
	}
	err = db.CreateBookingWithLock(ctx, b2)
	assert.ErrorIs(t, err, ErrNotAvailable)
}

package database

import (
	"context"
	"testing"

	"bronivik/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestItemCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	item := &models.Item{
		Name:          "Item 1",
		Description:   "Desc 1",
		TotalQuantity: 5,
		SortOrder:     10,
		IsActive:      true,
	}

	// Create
	err := db.CreateItem(ctx, item)
	require.NoError(t, err)

	// Get by Name
	found, err := db.GetItemByName(ctx, "Item 1")
	require.NoError(t, err)
	assert.Equal(t, item.Description, found.Description)
	assert.Equal(t, item.TotalQuantity, found.TotalQuantity)

	// Update
	found.TotalQuantity = 10
	err = db.UpdateItem(ctx, found)
	require.NoError(t, err)

	updated, _ := db.GetItemByName(ctx, "Item 1")
	assert.Equal(t, int64(10), updated.TotalQuantity)

	// Deactivate
	err = db.DeactivateItem(ctx, updated.ID)
	require.NoError(t, err)

	activeItems, err := db.GetActiveItems(ctx)
	require.NoError(t, err)
	assert.Len(t, activeItems, 0)
}

func TestItemReordering(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	db.CreateItem(ctx, &models.Item{Name: "A", SortOrder: 1, IsActive: true, TotalQuantity: 1})
	db.CreateItem(ctx, &models.Item{Name: "B", SortOrder: 2, IsActive: true, TotalQuantity: 1})

	items, _ := db.GetActiveItems(ctx)
	require.Len(t, items, 2)
	assert.Equal(t, "A", items[0].Name)

	// Reorder B to 0 (should come before A)
	err := db.ReorderItem(ctx, items[1].ID, 0)
	require.NoError(t, err)

	items, _ = db.GetActiveItems(ctx)
	assert.Equal(t, "B", items[0].Name)
}

func TestItemSync(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	configItems := []models.Item{
		{Name: "Item A", Description: "Desc A", TotalQuantity: 5, IsActive: true},
		{Name: "Item B", Description: "Desc B", TotalQuantity: 3, IsActive: true},
	}

	// First sync - should create items
	err := db.SyncItems(ctx, configItems)
	require.NoError(t, err)

	items, err := db.GetActiveItems(ctx)
	require.NoError(t, err)
	assert.Len(t, items, 2)

	// Second sync - should not create duplicates
	err = db.SyncItems(ctx, configItems)
	require.NoError(t, err)

	items, _ = db.GetActiveItems(ctx)
	assert.Len(t, items, 2)
}

func TestItemCache(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	item := &models.Item{Name: "Cached Item", TotalQuantity: 1, IsActive: true}
	db.CreateItem(ctx, item)

	// Get by ID (should populate cache)
	found, err := db.GetItemByID(ctx, item.ID)
	require.NoError(t, err)
	assert.Equal(t, item.Name, found.Name)

	// Manually modify DB without updating cache
	_, err = db.ExecContext(ctx, "UPDATE items SET total_quantity = 100 WHERE id = ?", item.ID)
	require.NoError(t, err)

	// Get by ID again (should return cached value, not 100)
	found, _ = db.GetItemByID(ctx, item.ID)
	assert.Equal(t, int64(1), found.TotalQuantity)

	// Force reload
	err = db.LoadItems(ctx)
	require.NoError(t, err)

	// Now should have updated value
	found, _ = db.GetItemByID(ctx, item.ID)
	assert.Equal(t, int64(100), found.TotalQuantity)
}

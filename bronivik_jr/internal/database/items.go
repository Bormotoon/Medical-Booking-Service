package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"bronivik/internal/models"
)

func (db *DB) LoadItems(ctx context.Context) error {
	query := `SELECT id, name, description, total_quantity, sort_order, 
              is_active, permanent_reserved, cabinet_id, created_at, updated_at FROM items`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to load items: %w", err)
	}
	defer rows.Close()

	db.mu.Lock()
	defer db.mu.Unlock()
	db.itemsCache = make(map[int64]models.Item)

	for rows.Next() {
		var item models.Item
		var cabinet sql.NullInt64
		if err := rows.Scan(
			&item.ID, &item.Name, &item.Description, &item.TotalQuantity,
			&item.SortOrder, &item.IsActive, &item.PermanentReserved, &cabinet, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return fmt.Errorf("failed to scan item: %w", err)
		}
		if cabinet.Valid {
			item.CabinetID = &cabinet.Int64
		}
		db.itemsCache[item.ID] = item
	}
	db.cacheTime = time.Now()
	return nil
}

func (db *DB) SyncItems(ctx context.Context, configItems []models.Item) error {
	for i := range configItems {
		cfgItem := &configItems[i]
		// Check if item exists by name
		var existingID int64
		err := db.QueryRowContext(ctx, "SELECT id FROM items WHERE name = ?", cfgItem.Name).Scan(&existingID)
		if err == sql.ErrNoRows {
			// Create new item — config items are active by default
			item := *cfgItem
			item.IsActive = true
			if err = db.CreateItem(ctx, &item); err != nil {
				return fmt.Errorf("failed to sync item %s: %w", cfgItem.Name, err)
			}
		} else if err != nil {
			return fmt.Errorf("failed to check item %s: %w", cfgItem.Name, err)
		}
		// If exists, we could update it, but for now let's just ensure it exists
	}

	// Reload everything into cache
	return db.LoadItems(ctx)
}

func (db *DB) SetItems(items []*models.Item) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.itemsCache = make(map[int64]models.Item)
	for i := range items {
		item := items[i]
		db.itemsCache[item.ID] = *item
	}
	db.cacheTime = time.Now()
}

func (db *DB) checkCacheTTL(ctx context.Context) {
	db.mu.RLock()
	expired := time.Since(db.cacheTime) > time.Duration(models.ItemsCacheTTL)*time.Second
	db.mu.RUnlock()

	if expired {
		_ = db.LoadItems(ctx)
	}
}

func (db *DB) itemByNameFromCache(name string) (*models.Item, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	for _, item := range db.itemsCache {
		if item.Name == name {
			return &item, true
		}
	}
	return nil, false
}

func (db *DB) CreateItem(ctx context.Context, item *models.Item) error {
	if !item.IsActive {
		item.IsActive = true
	}

	query := `INSERT INTO items (name, description, total_quantity, sort_order, 
              is_active, permanent_reserved, cabinet_id, created_at, updated_at)
	              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	id, err := db.insertAndReturnID(ctx, query,
		item.Name,
		item.Description,
		item.TotalQuantity,
		item.SortOrder,
		item.IsActive,
		item.PermanentReserved,
		item.CabinetID,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to create item: %w", err)
	}
	item.ID = id
	item.CreatedAt = now
	item.UpdatedAt = now

	// Update cache
	db.mu.Lock()
	db.itemsCache[id] = *item
	db.mu.Unlock()

	return nil
}

func (db *DB) GetItemByID(ctx context.Context, id int64) (*models.Item, error) {
	db.checkCacheTTL(ctx)
	db.mu.RLock()
	item, ok := db.itemsCache[id]
	db.mu.RUnlock()
	if ok {
		return &item, nil
	}

	var dbItem models.Item
	var cabinet sql.NullInt64
	query := `SELECT id, name, description, total_quantity, sort_order, 
              is_active, permanent_reserved, cabinet_id, created_at, updated_at 
              FROM items WHERE id = ?`
	err := db.QueryRowContext(ctx, query, id).Scan(
		&dbItem.ID, &dbItem.Name, &dbItem.Description, &dbItem.TotalQuantity,
		&dbItem.SortOrder, &dbItem.IsActive, &dbItem.PermanentReserved, &cabinet,
		&dbItem.CreatedAt, &dbItem.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get item by id: %w", err)
	}
	if cabinet.Valid {
		dbItem.CabinetID = &cabinet.Int64
	}

	// Update cache
	db.mu.Lock()
	db.itemsCache[dbItem.ID] = dbItem
	db.mu.Unlock()

	return &dbItem, nil
}

func (db *DB) GetItemByName(ctx context.Context, name string) (*models.Item, error) {
	db.checkCacheTTL(ctx)
	// Try cache first
	if item, ok := db.itemByNameFromCache(name); ok {
		return item, nil
	}

	var item models.Item
	var cabinet sql.NullInt64
	query := `SELECT id, name, description, total_quantity, sort_order, 
              is_active, permanent_reserved, cabinet_id, created_at, updated_at 
              FROM items WHERE name = ?`
	err := db.QueryRowContext(ctx, query, name).Scan(
		&item.ID, &item.Name, &item.Description, &item.TotalQuantity,
		&item.SortOrder, &item.IsActive, &item.PermanentReserved, &cabinet,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get item by name: %w", err)
	}
	if cabinet.Valid {
		item.CabinetID = &cabinet.Int64
	}

	// Update cache
	db.mu.Lock()
	db.itemsCache[item.ID] = item
	db.mu.Unlock()

	return &item, nil
}

func (db *DB) GetItemAvailabilityByName(ctx context.Context, itemName string, date time.Time) (*models.AvailabilityInfo, error) {
	item, err := db.GetActiveItemByName(ctx, itemName)
	if err != nil {
		return nil, err
	}

	if item.PermanentReserved {
		return &models.AvailabilityInfo{
			ItemName:    item.Name,
			Date:        date,
			Available:   false,
			BookedCount: 0,
			Total:       item.TotalQuantity,
		}, nil
	}

	bookedCount, err := db.GetBookedCount(ctx, item.ID, date)
	if err != nil {
		return nil, err
	}

	return &models.AvailabilityInfo{
		ItemName:    item.Name,
		Date:        date,
		Available:   bookedCount < int(item.TotalQuantity),
		BookedCount: int64(bookedCount),
		Total:       item.TotalQuantity,
	}, nil
}

func (db *DB) GetActiveItems(ctx context.Context) ([]*models.Item, error) {
	items, err := db.listItemsByQuery(ctx, `
		SELECT id, name, description, total_quantity, sort_order,
		       is_active, permanent_reserved, cabinet_id, created_at, updated_at
		FROM items
		WHERE is_active = 1 AND permanent_reserved = 0
		ORDER BY sort_order, id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get active items: %w", err)
	}
	return items, nil
}

func (db *DB) GetCurrentItems(ctx context.Context) ([]*models.Item, error) {
	items, err := db.listItemsByQuery(ctx, `
		SELECT id, name, description, total_quantity, sort_order,
		       is_active, permanent_reserved, cabinet_id, created_at, updated_at
		FROM items
		ORDER BY sort_order, id
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get current items: %w", err)
	}
	return items, nil
}

func (db *DB) UpdateItem(ctx context.Context, item *models.Item) error {
	query := `UPDATE items SET name = ?, description = ?, total_quantity = ?, 
              sort_order = ?, is_active = ?, permanent_reserved = ?, cabinet_id = ?, 
              updated_at = ? WHERE id = ?`
	now := time.Now()
	_, err := db.ExecContext(
		ctx, query, item.Name, item.Description, item.TotalQuantity,
		item.SortOrder, item.IsActive, item.PermanentReserved, item.CabinetID, now, item.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update item: %w", err)
	}
	item.UpdatedAt = now
	db.mu.Lock()
	db.itemsCache[item.ID] = *item
	db.mu.Unlock()
	return nil
}

func (db *DB) DeactivateItem(ctx context.Context, id int64) error {
	query := `UPDATE items SET is_active = 0, updated_at = ? WHERE id = ?`
	now := time.Now()
	_, err := db.ExecContext(ctx, query, now, id)
	if err != nil {
		return err
	}

	db.mu.Lock()
	if item, ok := db.itemsCache[id]; ok {
		item.IsActive = false
		item.UpdatedAt = now
		db.itemsCache[id] = item
	}
	db.mu.Unlock()
	return nil
}

func (db *DB) ReorderItem(ctx context.Context, id, newOrder int64) error {
	query := `UPDATE items SET sort_order = ?, updated_at = ? WHERE id = ?`
	now := time.Now()
	_, err := db.ExecContext(ctx, query, newOrder, now, id)
	if err != nil {
		return err
	}

	db.mu.Lock()
	if item, ok := db.itemsCache[id]; ok {
		item.SortOrder = newOrder
		item.UpdatedAt = now
		db.itemsCache[id] = item
	}
	db.mu.Unlock()
	return nil
}

func (db *DB) GetItems() []*models.Item {
	db.mu.RLock()
	defer db.mu.RUnlock()
	items := make([]*models.Item, 0, len(db.itemsCache))
	for _, item := range db.itemsCache {
		it := item
		items = append(items, &it)
	}
	return items
}

func (db *DB) GetActiveItemByName(ctx context.Context, name string) (*models.Item, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return nil, fmt.Errorf("failed to get active item by name: %w", sql.ErrNoRows)
	}

	item, err := db.queryItemRow(
		ctx,
		`SELECT id, name, description, total_quantity, sort_order,
		        is_active, permanent_reserved, cabinet_id, created_at, updated_at
		   FROM items
		  WHERE is_active = 1
		    AND lower(trim(name)) = ?
		  LIMIT 1`,
		normalized,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get active item by name: %w", err)
	}
	return item, nil
}

func (db *DB) listItemsByQuery(ctx context.Context, query string, args ...any) ([]*models.Item, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]*models.Item, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (db *DB) queryItemRow(ctx context.Context, query string, args ...any) (*models.Item, error) {
	return scanItem(db.QueryRowContext(ctx, query, args...))
}

func scanItem(scanner interface{ Scan(...any) error }) (*models.Item, error) {
	var item models.Item
	var cabinet sql.NullInt64
	if err := scanner.Scan(
		&item.ID, &item.Name, &item.Description, &item.TotalQuantity,
		&item.SortOrder, &item.IsActive, &item.PermanentReserved, &cabinet,
		&item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	if cabinet.Valid {
		item.CabinetID = &cabinet.Int64
	}
	return &item, nil
}

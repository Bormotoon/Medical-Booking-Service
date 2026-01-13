package db

import (
    "context"
    "fmt"
    "time"

    "bronivik/bronivik_crm/internal/config"
)

// SyncCabinetsFromConfig applies cabinets.yaml to the database and schedules.
// It upserts cabinets, aligns weekly schedules, and marks missing cabinets inactive.
func (db *DB) SyncCabinetsFromConfig(ctx context.Context, cfg *config.CabinetsConfig) error {
    if cfg == nil {
        return fmt.Errorf("cabinet config is nil")
    }

    now := time.Now()
    seen := make(map[int64]struct{})

    for _, cab := range cfg.Cabinets {
        isActive := 0
        if cab.IsActive {
            isActive = 1
        }

        // Preserve created_at if the cabinet already exists.
        _, err := db.ExecContext(ctx, `
            INSERT INTO cabinets (id, name, description, is_active, created_at, updated_at)
            VALUES (?, ?, ?, ?, COALESCE((SELECT created_at FROM cabinets WHERE id = ?), ?), ?)
            ON CONFLICT(id) DO UPDATE SET
                name = excluded.name,
                description = excluded.description,
                is_active = excluded.is_active,
                updated_at = excluded.updated_at`,
            cab.ID, cab.Name, cab.Description, isActive, cab.ID, now, now,
        )
        if err != nil {
            return fmt.Errorf("sync cabinet %d: %w", cab.ID, err)
        }
        seen[int64(cab.ID)] = struct{}{}

        if err := db.applyScheduleFromConfig(ctx, int64(cab.ID), cab.DefaultSchedule, cfg.Defaults.Schedule); err != nil {
            return fmt.Errorf("sync cabinet %d schedule: %w", cab.ID, err)
        }
    }

    // Deactivate cabinets that disappeared from config.
    rows, err := db.QueryContext(ctx, `SELECT id FROM cabinets`)
    if err != nil {
        return err
    }
    defer rows.Close()

    for rows.Next() {
        var id int64
        if err := rows.Scan(&id); err != nil {
            return err
        }
        if _, ok := seen[id]; ok {
            continue
        }
        if _, err := db.ExecContext(ctx, `UPDATE cabinets SET is_active = 0, updated_at = ? WHERE id = ?`, now, id); err != nil {
            return fmt.Errorf("deactivate cabinet %d: %w", id, err)
        }
    }
    if err := rows.Err(); err != nil {
        return err
    }

    // Best-effort creation of day-off overrides for configured holidays.
    for _, h := range cfg.Holidays {
        dt, err := time.Parse("2006-01-02", h.Date)
        if err != nil {
            return fmt.Errorf("parse holiday %s: %w", h.Date, err)
        }
        for id := range seen {
            _ = db.SetDayOff(ctx, id, dt, h.Name)
        }
    }

    return nil
}

func (db *DB) applyScheduleFromConfig(
    ctx context.Context,
    cabinetID int64,
    cabinetSchedule *config.CabinetScheduleConfig,
    defaultSchedule *config.CabinetScheduleConfig,
) error {
    effective := cabinetSchedule
    if effective == nil {
        effective = defaultSchedule
    }

    start := DefaultScheduleConfig.StartTime
    end := DefaultScheduleConfig.EndTime
    slot := DefaultScheduleConfig.SlotDuration
    var lunchStart, lunchEnd *string

    if effective != nil {
        if effective.StartTime != "" {
            start = effective.StartTime
        }
        if effective.EndTime != "" {
            end = effective.EndTime
        }
        if effective.SlotDurationMinutes > 0 {
            slot = effective.SlotDurationMinutes
        }
        if effective.LunchStart != "" {
            lunchStart = &effective.LunchStart
        }
        if effective.LunchEnd != "" {
            lunchEnd = &effective.LunchEnd
        }
    }

    if start == "" {
        start = DefaultScheduleConfig.StartTime
    }
    if end == "" {
        end = DefaultScheduleConfig.EndTime
    }
    if slot == 0 {
        slot = DefaultScheduleConfig.SlotDuration
    }

    // Keep defaults aligned for code paths that rely on DefaultScheduleConfig.
    DefaultScheduleConfig.StartTime = start
    DefaultScheduleConfig.EndTime = end
    DefaultScheduleConfig.SlotDuration = slot

    for day := 1; day <= 7; day++ {
        if err := db.upsertScheduleRow(ctx, cabinetID, day, start, end, lunchStart, lunchEnd, slot); err != nil {
            return err
        }
    }
    return nil
}

func (db *DB) upsertScheduleRow(
    ctx context.Context,
    cabinetID int64,
    day int,
    start, end string,
    lunchStart, lunchEnd *string,
    slot int,
) error {
    now := time.Now()

    res, err := db.ExecContext(ctx, `
        UPDATE cabinet_schedules
        SET start_time = ?, end_time = ?, lunch_start = ?, lunch_end = ?, slot_duration = ?, is_active = 1, updated_at = ?
        WHERE cabinet_id = ? AND day_of_week = ?`,
        start, end, lunchStart, lunchEnd, slot, now, cabinetID, day,
    )
    if err != nil {
        return err
    }

    rows, _ := res.RowsAffected()
    if rows == 0 {
        _, err = db.ExecContext(ctx, `
            INSERT INTO cabinet_schedules (
                cabinet_id, day_of_week, start_time, end_time, lunch_start, lunch_end, slot_duration, is_active, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, 1, ?, ?)`,
            cabinetID, day, start, end, lunchStart, lunchEnd, slot, now, now,
        )
        if err != nil {
            return err
        }
    }

    return nil
}

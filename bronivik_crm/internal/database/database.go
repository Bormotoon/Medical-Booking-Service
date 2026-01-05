package database

import (
    "database/sql"
    "fmt"
    "strings"
    "time"
)

// DB wraps sql.DB for the CRM bot.
type DB struct {
    *sql.DB
}

// NewDB opens database at path and runs migrations.
func NewDB(path string) (*DB, error) {
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, fmt.Errorf("open db: %w", err)
    }
    if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
        return nil, fmt.Errorf("enable foreign keys: %w", err)
    }
    if err := createTables(db); err != nil {
        return nil, err
    }
    return &DB{db}, nil
}

func createTables(db *sql.DB) error {
    queries := []string{
        // Users (simplified; extend as needed)
        `CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            telegram_id INTEGER UNIQUE NOT NULL,
            username TEXT,
            first_name TEXT,
            last_name TEXT,
            phone TEXT,
            is_manager BOOLEAN NOT NULL DEFAULT 0,
            is_blacklisted BOOLEAN NOT NULL DEFAULT 0,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )`,

        // Cabinets
        `CREATE TABLE IF NOT EXISTS cabinets (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT UNIQUE NOT NULL,
            description TEXT,
            is_active BOOLEAN DEFAULT 1,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )`,

        // Cabinet schedules
        `CREATE TABLE IF NOT EXISTS cabinet_schedules (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            cabinet_id INTEGER NOT NULL,
            day_of_week INTEGER NOT NULL,
            start_time TEXT NOT NULL,
            end_time TEXT NOT NULL,
            slot_duration INTEGER DEFAULT 60,
            is_active BOOLEAN DEFAULT 1,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (cabinet_id) REFERENCES cabinets(id)
        )`,

        // Schedule overrides
        `CREATE TABLE IF NOT EXISTS cabinet_schedule_overrides (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            cabinet_id INTEGER NOT NULL,
            date DATETIME NOT NULL,
            is_closed BOOLEAN DEFAULT 0,
            start_time TEXT,
            end_time TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (cabinet_id) REFERENCES cabinets(id)
        )`,

        // Hourly bookings
        `CREATE TABLE IF NOT EXISTS hourly_bookings (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER NOT NULL,
            cabinet_id INTEGER NOT NULL,
            start_time DATETIME NOT NULL,
            end_time DATETIME NOT NULL,
            status TEXT NOT NULL DEFAULT 'pending',
            comment TEXT,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (cabinet_id) REFERENCES cabinets(id),
            FOREIGN KEY (user_id) REFERENCES users(id)
        )`,

        // Indexes
        `CREATE INDEX IF NOT EXISTS idx_cabinets_active ON cabinets(is_active)`,
        `CREATE INDEX IF NOT EXISTS idx_schedules_cabinet ON cabinet_schedules(cabinet_id, day_of_week)`,
        `CREATE INDEX IF NOT EXISTS idx_overrides_cabinet_date ON cabinet_schedule_overrides(cabinet_id, date)`,
        `CREATE INDEX IF NOT EXISTS idx_hourly_bookings_times ON hourly_bookings(cabinet_id, start_time, end_time)`,
        `CREATE INDEX IF NOT EXISTS idx_hourly_bookings_status ON hourly_bookings(status)`,
    }

    for _, q := range queries {
        if _, err := db.Exec(q); err != nil {
            return fmt.Errorf("exec migration %s: %w", trimSQL(q), err)
        }
    }
    return nil
}

func trimSQL(s string) string {
    s = strings.TrimSpace(s)
    if len(s) > 60 {
        return s[:60] + "..."
    }
    return s
}

// TouchUpdated sets updated_at for a row.
func TouchUpdated(db *sql.DB, table string, id int64) error {
    _, err := db.Exec(fmt.Sprintf("UPDATE %s SET updated_at = ? WHERE id = ?", table), time.Now(), id)
    return err
}

-- Rollback: Remove end_time field
-- WARNING: This will lose range booking data

-- Drop indexes first
DROP INDEX IF EXISTS idx_bookings_item_time;
DROP INDEX IF EXISTS idx_bookings_time_range;

-- SQLite doesn't support DROP COLUMN directly
-- We need to recreate the table without end_time

-- Step 1: Create temporary table
CREATE TABLE bookings_backup AS SELECT 
    id, user_id, user_name, user_nickname, phone, 
    item_id, item_name, date, status, comment, 
    reminder_sent, external_booking_id, 
    created_at, updated_at, version 
FROM bookings;

-- Step 2: Drop original table
DROP TABLE bookings;

-- Step 3: Recreate without end_time
CREATE TABLE bookings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    user_name TEXT NOT NULL,
    user_nickname TEXT,
    phone TEXT NOT NULL,
    item_id INTEGER NOT NULL,
    item_name TEXT NOT NULL,
    date DATETIME NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    comment TEXT,
    reminder_sent BOOLEAN NOT NULL DEFAULT 0,
    external_booking_id TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    version INTEGER NOT NULL DEFAULT 1,
    FOREIGN KEY (item_id) REFERENCES items(id),
    FOREIGN KEY (user_id) REFERENCES users(telegram_id)
);

-- Step 4: Restore data
INSERT INTO bookings SELECT * FROM bookings_backup;

-- Step 5: Clean up
DROP TABLE bookings_backup;

-- Step 6: Recreate original indexes
CREATE INDEX idx_bookings_date ON bookings(date);
CREATE INDEX idx_bookings_status ON bookings(status);
CREATE INDEX idx_bookings_item_id ON bookings(item_id);
CREATE INDEX idx_bookings_user_id ON bookings(user_id);
CREATE INDEX idx_bookings_item_date_status ON bookings(item_id, date, status);
CREATE INDEX idx_bookings_reminder ON bookings(reminder_sent, date);
CREATE INDEX idx_bookings_external ON bookings(external_booking_id);

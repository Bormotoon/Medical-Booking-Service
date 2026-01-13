-- Migration: Create reminders table
-- Date: 2026-01-13
-- Description: Extended reminder system with deduplication and retry support

CREATE TABLE IF NOT EXISTS reminders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    booking_id INTEGER NOT NULL,
    reminder_type TEXT NOT NULL,              -- '24h_before', 'day_of_booking', 'custom'
    scheduled_at DATETIME NOT NULL,           -- planned send time
    sent_at DATETIME,                         -- actual send time (NULL if not sent)
    status TEXT NOT NULL DEFAULT 'pending',   -- pending, scheduled, processing, sent, failed, cancelled
    enabled BOOLEAN NOT NULL DEFAULT 1,
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    
    -- Unique constraint for deduplication
    UNIQUE(user_id, booking_id, reminder_type)
);

-- Index for selecting pending reminders
CREATE INDEX IF NOT EXISTS idx_reminders_pending 
ON reminders(scheduled_at, status, enabled);

-- Index for user lookup
CREATE INDEX IF NOT EXISTS idx_reminders_user 
ON reminders(user_id);

-- Index for booking lookup
CREATE INDEX IF NOT EXISTS idx_reminders_booking 
ON reminders(booking_id);

-- Index for cleanup queries
CREATE INDEX IF NOT EXISTS idx_reminders_cleanup 
ON reminders(status, sent_at);

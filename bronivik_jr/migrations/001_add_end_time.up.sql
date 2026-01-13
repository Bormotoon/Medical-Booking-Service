-- Migration: Add end_time field for range bookings
-- Date: 2026-01-13
-- Description: Adds support for multi-day bookings ("permanent reservations")

-- Add nullable end_time column
ALTER TABLE bookings ADD COLUMN end_time DATETIME NULL;

-- Create indexes for range queries
CREATE INDEX IF NOT EXISTS idx_bookings_time_range ON bookings(date, end_time);
CREATE INDEX IF NOT EXISTS idx_bookings_item_time ON bookings(item_id, date, end_time);

-- Note: NULL end_time means single-day booking (end_time = date)
-- Backfill is not needed - NULL is the correct default for existing records

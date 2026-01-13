-- Rollback: Drop reminders table
-- WARNING: This will delete all reminder data

DROP INDEX IF EXISTS idx_reminders_cleanup;
DROP INDEX IF EXISTS idx_reminders_booking;
DROP INDEX IF EXISTS idx_reminders_user;
DROP INDEX IF EXISTS idx_reminders_pending;

DROP TABLE IF EXISTS reminders;

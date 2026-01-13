package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bronivik/internal/models"

	_ "github.com/mattn/go-sqlite3" // sqlite3 driver
	"github.com/rs/zerolog"
)

// DB represents the database connection and its cache.
type DB struct {
	*sql.DB
	itemsCache map[int64]models.Item
	cacheTime  time.Time
	mu         sync.RWMutex
	logger     *zerolog.Logger
}

var (
	ErrConcurrentModification = errors.New("concurrent modification")
	ErrNotAvailable           = errors.New("not available")
	ErrPastDate               = errors.New("cannot book in the past")
	ErrDateTooFar             = errors.New("date is too far in the future")
)

// NewDB initializes a new database connection and creates tables if they don't exist.
func NewDB(path string, logger *zerolog.Logger) (*DB, error) {
	// Создаем директорию для БД, если её нет
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Добавляем параметры для SQLite: WAL mode, busy timeout
	dsn := path + "?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Настройка пула соединений
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(time.Hour)

	// Проверяем соединение
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	instance := &DB{
		DB:         db,
		itemsCache: make(map[int64]models.Item),
		logger:     logger,
	}

	// Создаем таблицы
	if err := instance.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	// Load items into cache
	if err := instance.LoadItems(context.Background()); err != nil {
		logger.Error().Err(err).Msg("Failed to load items into cache")
		// We don't return error here to allow the app to start even if items are missing
	}

	logger.Info().Str("path", path).Msg("Database initialized")
	return instance, nil
}

func (db *DB) createTables() error {
	queries := []string{
		// Таблица предметов (аппаратов)
		`CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			description TEXT,
			total_quantity INTEGER NOT NULL DEFAULT 1,
			sort_order INTEGER NOT NULL DEFAULT 0,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			permanent_reserved BOOLEAN NOT NULL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Таблица пользователей
		`CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            telegram_id INTEGER UNIQUE NOT NULL,
            username TEXT,
            first_name TEXT NOT NULL,
            last_name TEXT,
            phone TEXT,
            is_manager BOOLEAN NOT NULL DEFAULT 0,
            is_blacklisted BOOLEAN NOT NULL DEFAULT 0,
            language_code TEXT,
            last_activity DATETIME NOT NULL,
            created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
            updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
        )`,
		// Таблица настроек пользователя (напоминания)
		`CREATE TABLE IF NOT EXISTS user_settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL UNIQUE,
			reminders_enabled BOOLEAN NOT NULL DEFAULT 1,
			reminder_hours_before INTEGER NOT NULL DEFAULT 24,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(telegram_id) ON DELETE CASCADE
		)`,
		// Таблица бронирований
		`CREATE TABLE IF NOT EXISTS bookings (
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
			FOREIGN KEY(item_id) REFERENCES items(id),
			FOREIGN KEY(user_id) REFERENCES users(telegram_id)
		)`,

		// Индексы для пользователей
		`CREATE INDEX IF NOT EXISTS idx_users_telegram_id ON users(telegram_id)`,
		`CREATE INDEX IF NOT EXISTS idx_users_is_manager ON users(is_manager)`,
		`CREATE INDEX IF NOT EXISTS idx_users_is_blacklisted ON users(is_blacklisted)`,

		// Индексы для items
		`CREATE INDEX IF NOT EXISTS idx_items_sort ON items(sort_order, id)`,

		// Уникальный индекс для предотвращения двойного бронирования (если количество = 1)
		// Примечание: это работает только если TotalQuantity всегда 1.
		// Если TotalQuantity > 1, логика должна быть сложнее (в коде через транзакции).
		// Но для базовой защиты добавим индекс по (item_id, date, status)
		`CREATE INDEX IF NOT EXISTS idx_bookings_item_date_status ON bookings(item_id, date, status)`,

		// Очередь синхронизации в Sheets
		`CREATE TABLE IF NOT EXISTS sync_queue (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_type TEXT NOT NULL,
			booking_id INTEGER NOT NULL,
			payload TEXT,
			status TEXT DEFAULT 'pending',
			retry_count INTEGER DEFAULT 0,
			last_error TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			processed_at DATETIME,
			next_retry_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_queue_status ON sync_queue(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sync_queue_next_retry ON sync_queue(next_retry_at)`,

		// Существующие индексы для бронирований
		`CREATE INDEX IF NOT EXISTS idx_bookings_date ON bookings(date)`,
		`CREATE INDEX IF NOT EXISTS idx_bookings_status ON bookings(status)`,
		`CREATE INDEX IF NOT EXISTS idx_bookings_item_id ON bookings(item_id)`,
		`CREATE INDEX IF NOT EXISTS idx_bookings_user_id ON bookings(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_bookings_reminder ON bookings(reminder_sent, date)`,
		`CREATE INDEX IF NOT EXISTS idx_bookings_external ON bookings(external_booking_id)`,

		// Индексы для items
		`CREATE INDEX IF NOT EXISTS idx_items_active ON items(is_active)`,
		`CREATE INDEX IF NOT EXISTS idx_items_permanent ON items(permanent_reserved)`,

		// Индексы для user_settings
		`CREATE INDEX IF NOT EXISTS idx_user_settings_user_id ON user_settings(user_id)`,

		// Таблица заблокированных пользователей
		`CREATE TABLE IF NOT EXISTS blocked_users (
			user_id INTEGER PRIMARY KEY,
			blocked_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			reason TEXT,
			blocked_by INTEGER NOT NULL,
			FOREIGN KEY (blocked_by) REFERENCES users(telegram_id)
		)`,

		// Таблица менеджеров
		`CREATE TABLE IF NOT EXISTS managers (
			user_id INTEGER PRIMARY KEY,
			chat_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			added_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			added_by INTEGER NOT NULL DEFAULT 0
		)`,

		// Индексы для access control
		`CREATE INDEX IF NOT EXISTS idx_blocked_users_blocked_at ON blocked_users(blocked_at)`,
		`CREATE INDEX IF NOT EXISTS idx_managers_chat_id ON managers(chat_id)`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("error executing query %s: %v", query, err)
		}
	}

	if err := db.ensureBookingVersionColumn(); err != nil {
		return err
	}
	if err := db.ensureNewColumns(); err != nil {
		return err
	}
	return nil
}

func (db *DB) ensureBookingVersionColumn() error {
	_, err := db.Exec(`ALTER TABLE bookings ADD COLUMN version INTEGER NOT NULL DEFAULT 1`)
	if err != nil {
		// Ignore duplicate column error for SQLite
		if strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			return nil
		}
		return fmt.Errorf("failed to add version column: %w", err)
	}
	return nil
}

// ensureNewColumns adds new columns to existing tables if they don't exist
func (db *DB) ensureNewColumns() error {
	migrations := []string{
		`ALTER TABLE bookings ADD COLUMN reminder_sent BOOLEAN NOT NULL DEFAULT 0`,
		`ALTER TABLE bookings ADD COLUMN external_booking_id TEXT`,
		`ALTER TABLE items ADD COLUMN permanent_reserved BOOLEAN NOT NULL DEFAULT 0`,
	}

	for _, m := range migrations {
		_, err := db.Exec(m)
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "duplicate column") {
			// Log but don't fail - column might already exist
			if db.logger != nil {
				db.logger.Debug().Err(err).Str("migration", m).Msg("Migration skipped")
			}
		}
	}
	return nil
}

func (db *DB) Close() error {
	return db.DB.Close()
}

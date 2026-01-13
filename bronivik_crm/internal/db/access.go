package db

import (
	"context"
	"database/sql"
	"time"
)

// BlockedUser represents a blocked user record.
type BlockedUser struct {
	UserID    int64
	BlockedAt time.Time
	Reason    string
	BlockedBy int64
}

// Manager represents a manager record.
type Manager struct {
	UserID  int64
	ChatID  int64
	Name    string
	AddedAt time.Time
	AddedBy int64
}

// IsBlocked checks if a user is blocked.
func (db *DB) IsBlocked(ctx context.Context, userID int64) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM blocked_users WHERE user_id = ?",
		userID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetBlockedUser returns blocked user details.
func (db *DB) GetBlockedUser(ctx context.Context, userID int64) (*BlockedUser, error) {
	var bu BlockedUser
	err := db.QueryRowContext(ctx,
		"SELECT user_id, blocked_at, reason, blocked_by FROM blocked_users WHERE user_id = ?",
		userID,
	).Scan(&bu.UserID, &bu.BlockedAt, &bu.Reason, &bu.BlockedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &bu, nil
}

// BlockUser adds a user to the blocklist.
func (db *DB) BlockUser(ctx context.Context, userID int64, reason string, blockedBy int64) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO blocked_users (user_id, blocked_at, reason, blocked_by)
		VALUES (?, ?, ?, ?)`,
		userID, time.Now(), reason, blockedBy,
	)
	return err
}

// UnblockUser removes a user from the blocklist.
func (db *DB) UnblockUser(ctx context.Context, userID int64) error {
	_, err := db.ExecContext(ctx,
		"DELETE FROM blocked_users WHERE user_id = ?",
		userID,
	)
	return err
}

// ListBlockedUsers returns all blocked users.
func (db *DB) ListBlockedUsers(ctx context.Context) ([]BlockedUser, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT user_id, blocked_at, reason, blocked_by FROM blocked_users ORDER BY blocked_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []BlockedUser
	for rows.Next() {
		var bu BlockedUser
		if err := rows.Scan(&bu.UserID, &bu.BlockedAt, &bu.Reason, &bu.BlockedBy); err != nil {
			return nil, err
		}
		users = append(users, bu)
	}
	return users, rows.Err()
}

// IsManager checks if a user is a manager.
func (db *DB) IsManager(ctx context.Context, userID int64) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM managers WHERE user_id = ?",
		userID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// GetManager returns manager details.
func (db *DB) GetManager(ctx context.Context, userID int64) (*Manager, error) {
	var m Manager
	err := db.QueryRowContext(ctx,
		"SELECT user_id, chat_id, name, added_at, added_by FROM managers WHERE user_id = ?",
		userID,
	).Scan(&m.UserID, &m.ChatID, &m.Name, &m.AddedAt, &m.AddedBy)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// AddManager adds a new manager.
func (db *DB) AddManager(ctx context.Context, userID, chatID int64, name string, addedBy int64) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO managers (user_id, chat_id, name, added_at, added_by)
		VALUES (?, ?, ?, ?, ?)`,
		userID, chatID, name, time.Now(), addedBy,
	)
	return err
}

// RemoveManager removes a manager.
func (db *DB) RemoveManager(ctx context.Context, userID int64) error {
	_, err := db.ExecContext(ctx,
		"DELETE FROM managers WHERE user_id = ?",
		userID,
	)
	return err
}

// ListManagers returns all managers.
func (db *DB) ListManagers(ctx context.Context) ([]Manager, error) {
	rows, err := db.QueryContext(ctx,
		"SELECT user_id, chat_id, name, added_at, added_by FROM managers ORDER BY added_at",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var managers []Manager
	for rows.Next() {
		var m Manager
		if err := rows.Scan(&m.UserID, &m.ChatID, &m.Name, &m.AddedAt, &m.AddedBy); err != nil {
			return nil, err
		}
		managers = append(managers, m)
	}
	return managers, rows.Err()
}

// GetManagerChatIDs returns all manager chat IDs.
func (db *DB) GetManagerChatIDs(ctx context.Context) ([]int64, error) {
	rows, err := db.QueryContext(ctx, "SELECT chat_id FROM managers")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chatIDs []int64
	for rows.Next() {
		var chatID int64
		if err := rows.Scan(&chatID); err != nil {
			return nil, err
		}
		chatIDs = append(chatIDs, chatID)
	}
	return chatIDs, rows.Err()
}

package domain

import "context"

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByChatID(ctx context.Context, chatID int64) (*User, error)
}

type StateRepository interface {
	GetState(ctx context.Context, userID int64) (*UserState, error)
	SetState(ctx context.Context, state *UserState) error
	ClearState(ctx context.Context, userID int64) error
	CheckRateLimit(ctx context.Context, userID int64, limit int, window time.Duration) (bool, error)
}

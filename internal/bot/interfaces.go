package bot

import (
	"context"
	"time"

	"bronivik/internal/domain"
	"bronivik/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Repository interface {
	GetBooking(ctx context.Context, id int64) (*models.Booking, error)
	CreateBooking(ctx context.Context, booking *models.Booking) error
	UpdateBookingStatus(ctx context.Context, id int64, status string) error
	UpdateBookingStatusWithVersion(ctx context.Context, id int64, version int, status string) error
	GetBookingsByDateRange(ctx context.Context, start, end time.Time) ([]*models.Booking, error)
	CheckAvailability(ctx context.Context, itemID int64, date time.Time) (bool, error)
	GetAvailabilityForPeriod(ctx context.Context, itemID int64, startDate time.Time, days int) ([]models.AvailabilityInfo, error)
	GetActiveItems(ctx context.Context) ([]models.Item, error)
	GetItemByName(ctx context.Context, name string) (*models.Item, error)
	CreateItem(ctx context.Context, item *models.Item) error
	UpdateItem(ctx context.Context, item *models.Item) error
	DeactivateItem(ctx context.Context, id int64) error
	ReorderItem(ctx context.Context, id int64, newOrder int64) error
}

type StateManager interface {
	GetUserState(ctx context.Context, userID int64) (*domain.UserState, error)
	SetUserState(ctx context.Context, userID int64, step string, data map[string]interface{}) error
	ClearUserState(ctx context.Context, userID int64) error
	CheckRateLimit(ctx context.Context, userID int64, limit int, window time.Duration) (bool, error)
}

type EventPublisher interface {
	Publish(evType string, payload interface{})
}

type TelegramSender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

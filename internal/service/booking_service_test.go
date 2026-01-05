package service

import (
"context"
"io"
"testing"
"time"

"bronivik/internal/models"
"github.com/rs/zerolog"
"github.com/stretchr/testify/assert"
"github.com/stretchr/testify/mock"
)

type mockRepo struct {
mock.Mock
}

func (m *mockRepo) GetBooking(ctx context.Context, id int64) (*models.Booking, error) {
args := m.Called(ctx, id)
if args.Get(0) == nil { return nil, args.Error(1) }
return args.Get(0).(*models.Booking), args.Error(1)
}
func (m *mockRepo) CreateBooking(ctx context.Context, b *models.Booking) error { return m.Called(ctx, b).Error(0) }
func (m *mockRepo) CreateBookingWithLock(ctx context.Context, b *models.Booking) error { return m.Called(ctx, b).Error(0) }
func (m *mockRepo) UpdateBookingStatus(ctx context.Context, id int64, s string) error { return m.Called(ctx, id, s).Error(0) }
func (m *mockRepo) UpdateBookingStatusWithVersion(ctx context.Context, id int64, v int64, s string) error { return m.Called(ctx, id, v, s).Error(0) }
func (m *mockRepo) GetBookingsByDateRange(ctx context.Context, s, e time.Time) ([]models.Booking, error) {
args := m.Called(ctx, s, e)
return args.Get(0).([]models.Booking), args.Error(1)
}
func (m *mockRepo) CheckAvailability(ctx context.Context, id int64, d time.Time) (bool, error) {
args := m.Called(ctx, id, d)
return args.Bool(0), args.Error(1)
}
func (m *mockRepo) GetAvailabilityForPeriod(ctx context.Context, id int64, s time.Time, d int) ([]models.Availability, error) {
args := m.Called(ctx, id, s, d)
return args.Get(0).([]models.Availability), args.Error(1)
}
func (m *mockRepo) GetActiveItems(ctx context.Context) ([]models.Item, error) {
args := m.Called(ctx)
return args.Get(0).([]models.Item), args.Error(1)
}
func (m *mockRepo) GetItemByID(ctx context.Context, id int64) (*models.Item, error) {
args := m.Called(ctx, id)
if args.Get(0) == nil { return nil, args.Error(1) }
return args.Get(0).(*models.Item), args.Error(1)
}
func (m *mockRepo) GetItemByName(ctx context.Context, n string) (*models.Item, error) {
args := m.Called(ctx, n)
if args.Get(0) == nil { return nil, args.Error(1) }
return args.Get(0).(*models.Item), args.Error(1)
}
func (m *mockRepo) CreateItem(ctx context.Context, i *models.Item) error { return m.Called(ctx, i).Error(0) }
func (m *mockRepo) UpdateItem(ctx context.Context, i *models.Item) error { return m.Called(ctx, i).Error(0) }
func (m *mockRepo) DeactivateItem(ctx context.Context, id int64) error { return m.Called(ctx, id).Error(0) }
func (m *mockRepo) ReorderItem(ctx context.Context, id int64, o int64) error { return m.Called(ctx, id, o).Error(0) }
func (m *mockRepo) GetAllUsers(ctx context.Context) ([]models.User, error) {
args := m.Called(ctx)
return args.Get(0).([]models.User), args.Error(1)
}
func (m *mockRepo) GetUserByTelegramID(ctx context.Context, id int64) (*models.User, error) {
args := m.Called(ctx, id)
if args.Get(0) == nil { return nil, args.Error(1) }
return args.Get(0).(*models.User), args.Error(1)
}
func (m *mockRepo) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
args := m.Called(ctx, id)
if args.Get(0) == nil { return nil, args.Error(1) }
return args.Get(0).(*models.User), args.Error(1)
}
func (m *mockRepo) CreateOrUpdateUser(ctx context.Context, u *models.User) error { return m.Called(ctx, u).Error(0) }
func (m *mockRepo) UpdateUserActivity(ctx context.Context, id int64) error { return m.Called(ctx, id).Error(0) }
func (m *mockRepo) UpdateUserPhone(ctx context.Context, id int64, p string) error { return m.Called(ctx, id, p).Error(0) }
func (m *mockRepo) GetDailyBookings(ctx context.Context, s, e time.Time) (map[string][]models.Booking, error) {
args := m.Called(ctx, s, e)
return args.Get(0).(map[string][]models.Booking), args.Error(1)
}
func (m *mockRepo) GetBookedCount(ctx context.Context, id int64, d time.Time) (int, error) {
args := m.Called(ctx, id, d)
return args.Int(0), args.Error(1)
}
func (m *mockRepo) GetBookingWithAvailability(ctx context.Context, id int64, nid int64) (*models.Booking, bool, error) {
args := m.Called(ctx, id, nid)
if args.Get(0) == nil { return nil, args.Bool(1), args.Error(2) }
return args.Get(0).(*models.Booking), args.Bool(1), args.Error(2)
}
func (m *mockRepo) UpdateBookingItemAndStatusWithVersion(ctx context.Context, id int64, v int64, iid int64, in string, s string) error {
return m.Called(ctx, id, v, iid, in, s).Error(0)
}
func (m *mockRepo) SetItems(items []models.Item) { m.Called(items) }
func (m *mockRepo) GetActiveUsers(ctx context.Context, d int) ([]models.User, error) {
args := m.Called(ctx, d)
return args.Get(0).([]models.User), args.Error(1)
}
func (m *mockRepo) GetUsersByManagerStatus(ctx context.Context, im bool) ([]models.User, error) {
args := m.Called(ctx, im)
return args.Get(0).([]models.User), args.Error(1)
}
func (m *mockRepo) GetUserBookings(ctx context.Context, id int64) ([]models.Booking, error) {
args := m.Called(ctx, id)
return args.Get(0).([]models.Booking), args.Error(1)
}

type mockEventBus struct {
mock.Mock
}
func (m *mockEventBus) PublishJSON(et string, p interface{}) error { return m.Called(et, p).Error(0) }

type mockWorker struct {
mock.Mock
}
func (m *mockWorker) EnqueueTask(ctx context.Context, tt string, bid int64, b *models.Booking, s string) error {
return m.Called(ctx, tt, bid, b, s).Error(0)
}
func (m *mockWorker) EnqueueSyncSchedule(ctx context.Context, s, e time.Time) error {
return m.Called(ctx, s, e).Error(0)
}

func TestBookingService(t *testing.T) {
repo := new(mockRepo)
bus := new(mockEventBus)
worker := new(mockWorker)
logger := zerolog.New(io.Discard)
svc := NewBookingService(repo, bus, worker, 30, 2, &logger)
ctx := context.Background()

t.Run("ValidateBookingDate", func(t *testing.T) {
now := time.Now()

// Past date
err := svc.ValidateBookingDate(now.AddDate(0, 0, -1))
assert.Error(t, err)

// Too far
err = svc.ValidateBookingDate(now.AddDate(0, 0, 31))
assert.Error(t, err)

// Valid
err = svc.ValidateBookingDate(now.AddDate(0, 0, 5))
assert.NoError(t, err)
})

t.Run("CreateBooking", func(t *testing.T) {
date := time.Now().AddDate(0, 0, 5)
booking := &models.Booking{ItemID: 1, Date: date}

repo.On("CheckAvailability", ctx, int64(1), date).Return(true, nil).Once()
repo.On("CreateBookingWithLock", ctx, booking).Return(nil).Once()
bus.On("PublishJSON", mock.Anything, mock.Anything).Return(nil).Once()
worker.On("EnqueueTask", ctx, "upsert", int64(0), booking, "").Return(nil).Once()
worker.On("EnqueueSyncSchedule", ctx, mock.Anything, mock.Anything).Return(nil).Once()

err := svc.CreateBooking(ctx, booking)
assert.NoError(t, err)
repo.AssertExpectations(t)
})

t.Run("ConfirmBooking", func(t *testing.T) {
booking := &models.Booking{ID: 10, Status: models.StatusConfirmed}
repo.On("UpdateBookingStatusWithVersion", ctx, int64(10), int64(1), models.StatusConfirmed).Return(nil).Once()
repo.On("GetBooking", ctx, int64(10)).Return(booking, nil).Once()
bus.On("PublishJSON", mock.Anything, mock.Anything).Return(nil).Once()
worker.On("EnqueueTask", ctx, "update_status", int64(10), booking, models.StatusConfirmed).Return(nil).Once()
worker.On("EnqueueSyncSchedule", ctx, mock.Anything, mock.Anything).Return(nil).Once()

err := svc.ConfirmBooking(ctx, 10, 1, 100)
assert.NoError(t, err)
repo.AssertExpectations(t)
})
}

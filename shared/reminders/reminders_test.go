package reminders

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockReminderRepository implements ReminderRepository for testing.
type MockReminderRepository struct {
	mu        sync.Mutex
	reminders map[int64]*Reminder
	nextID    int64
}

func NewMockReminderRepository() *MockReminderRepository {
	return &MockReminderRepository{
		reminders: make(map[int64]*Reminder),
		nextID:    1,
	}
}

func (m *MockReminderRepository) Create(ctx context.Context, r *Reminder) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicates
	for _, existing := range m.reminders {
		if existing.UserID == r.UserID &&
			existing.BookingID == r.BookingID &&
			existing.ReminderType == r.ReminderType {
			return nil // Silently ignore duplicate
		}
	}

	r.ID = m.nextID
	m.nextID++
	m.reminders[r.ID] = r
	return nil
}

func (m *MockReminderRepository) GetByID(ctx context.Context, id int64) (*Reminder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if r, ok := m.reminders[id]; ok {
		return r, nil
	}
	return nil, nil
}

func (m *MockReminderRepository) GetPending(ctx context.Context, before time.Time) ([]*Reminder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*Reminder
	for _, r := range m.reminders {
		if r.Status == ReminderStatusPending && r.Enabled && !r.ScheduledAt.After(before) {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *MockReminderRepository) Update(ctx context.Context, r *Reminder) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.reminders[r.ID]; ok {
		m.reminders[r.ID] = r
	}
	return nil
}

func (m *MockReminderRepository) Delete(ctx context.Context, id int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.reminders, id)
	return nil
}

func (m *MockReminderRepository) GetByFilter(ctx context.Context, filter ReminderFilter) ([]*Reminder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*Reminder
	for _, r := range m.reminders {
		match := true
		if filter.UserID != nil && r.UserID != *filter.UserID {
			match = false
		}
		if filter.BookingID != nil && r.BookingID != *filter.BookingID {
			match = false
		}
		if len(filter.Status) > 0 {
			found := false
			for _, s := range filter.Status {
				if r.Status == s {
					found = true
					break
				}
			}
			if !found {
				match = false
			}
		}
		if match {
			result = append(result, r)
		}
	}
	return result, nil
}

func (m *MockReminderRepository) TryAcquireReminder(ctx context.Context, id int64) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	r, ok := m.reminders[id]
	if !ok {
		return false, nil
	}
	if r.Status != ReminderStatusPending {
		return false, nil
	}
	r.Status = ReminderStatusProcessing
	return true, nil
}

func (m *MockReminderRepository) DeleteOldReminders(ctx context.Context, sentBefore time.Time, failedBefore time.Time) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var count int64
	for id, r := range m.reminders {
		if r.Status == ReminderStatusSent && r.SentAt != nil && r.SentAt.Before(sentBefore) {
			delete(m.reminders, id)
			count++
		}
		if r.Status == ReminderStatusFailed && r.UpdatedAt.Before(failedBefore) {
			delete(m.reminders, id)
			count++
		}
	}
	return count, nil
}

func (m *MockReminderRepository) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.reminders)
}

func TestReminderDeduplication(t *testing.T) {
	repo := NewMockReminderRepository()
	ctx := context.Background()

	// Create first reminder
	r1 := &Reminder{
		UserID:       123,
		BookingID:    456,
		ReminderType: ReminderType24HBefore,
		ScheduledAt:  time.Now().Add(time.Hour),
		Status:       ReminderStatusPending,
		Enabled:      true,
	}
	err := repo.Create(ctx, r1)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.Count())

	// Try to create duplicate - should be ignored
	r2 := &Reminder{
		UserID:       123,
		BookingID:    456,
		ReminderType: ReminderType24HBefore,
		ScheduledAt:  time.Now().Add(2 * time.Hour),
		Status:       ReminderStatusPending,
		Enabled:      true,
	}
	err = repo.Create(ctx, r2)
	require.NoError(t, err)
	assert.Equal(t, 1, repo.Count(), "Duplicate should not be created")

	// Different reminder type - should be created
	r3 := &Reminder{
		UserID:       123,
		BookingID:    456,
		ReminderType: ReminderTypeDayOfBooking,
		ScheduledAt:  time.Now().Add(time.Hour),
		Status:       ReminderStatusPending,
		Enabled:      true,
	}
	err = repo.Create(ctx, r3)
	require.NoError(t, err)
	assert.Equal(t, 2, repo.Count(), "Different type should be created")

	// Different user - should be created
	r4 := &Reminder{
		UserID:       789,
		BookingID:    456,
		ReminderType: ReminderType24HBefore,
		ScheduledAt:  time.Now().Add(time.Hour),
		Status:       ReminderStatusPending,
		Enabled:      true,
	}
	err = repo.Create(ctx, r4)
	require.NoError(t, err)
	assert.Equal(t, 3, repo.Count(), "Different user should be created")
}

func TestTryAcquireReminder(t *testing.T) {
	repo := NewMockReminderRepository()
	ctx := context.Background()

	// Create a pending reminder
	r := &Reminder{
		UserID:       123,
		BookingID:    456,
		ReminderType: ReminderType24HBefore,
		ScheduledAt:  time.Now().Add(time.Hour),
		Status:       ReminderStatusPending,
		Enabled:      true,
	}
	err := repo.Create(ctx, r)
	require.NoError(t, err)

	// First acquire should succeed
	acquired, err := repo.TryAcquireReminder(ctx, r.ID)
	require.NoError(t, err)
	assert.True(t, acquired, "First acquire should succeed")

	// Second acquire should fail (already processing)
	acquired, err = repo.TryAcquireReminder(ctx, r.ID)
	require.NoError(t, err)
	assert.False(t, acquired, "Second acquire should fail")
}

func TestDeleteOldReminders(t *testing.T) {
	repo := NewMockReminderRepository()
	ctx := context.Background()

	now := time.Now()
	sentAt := now.Add(-48 * time.Hour) // 2 days ago

	// Create sent reminder (old)
	oldSent := &Reminder{
		UserID:       1,
		BookingID:    1,
		ReminderType: ReminderType24HBefore,
		ScheduledAt:  sentAt,
		SentAt:       &sentAt,
		Status:       ReminderStatusSent,
		Enabled:      true,
		UpdatedAt:    sentAt,
	}
	err := repo.Create(ctx, oldSent)
	require.NoError(t, err)

	// Create sent reminder (recent)
	recentSentAt := now.Add(-1 * time.Hour)
	recentSent := &Reminder{
		UserID:       2,
		BookingID:    2,
		ReminderType: ReminderType24HBefore,
		ScheduledAt:  recentSentAt,
		SentAt:       &recentSentAt,
		Status:       ReminderStatusSent,
		Enabled:      true,
		UpdatedAt:    recentSentAt,
	}
	err = repo.Create(ctx, recentSent)
	require.NoError(t, err)

	// Create failed reminder (old)
	oldFailed := &Reminder{
		UserID:       3,
		BookingID:    3,
		ReminderType: ReminderType24HBefore,
		ScheduledAt:  sentAt,
		Status:       ReminderStatusFailed,
		Enabled:      true,
		UpdatedAt:    sentAt,
	}
	err = repo.Create(ctx, oldFailed)
	require.NoError(t, err)

	assert.Equal(t, 3, repo.Count())

	// Delete old reminders (sent > 1 day, failed > 3 days)
	deleted, err := repo.DeleteOldReminders(ctx, now.Add(-24*time.Hour), now.Add(-72*time.Hour))
	require.NoError(t, err)

	// Only the old sent reminder should be deleted (sent > 24h ago)
	// The old failed one is only 48h old, not 72h
	assert.Equal(t, int64(1), deleted)
	assert.Equal(t, 2, repo.Count())
}

func TestGetPending(t *testing.T) {
	repo := NewMockReminderRepository()
	ctx := context.Background()

	now := time.Now()

	// Pending, scheduled in the past - should be returned
	past := &Reminder{
		UserID:       1,
		BookingID:    1,
		ReminderType: ReminderType24HBefore,
		ScheduledAt:  now.Add(-1 * time.Hour),
		Status:       ReminderStatusPending,
		Enabled:      true,
	}
	_ = repo.Create(ctx, past)

	// Pending, scheduled in future - should NOT be returned
	future := &Reminder{
		UserID:       2,
		BookingID:    2,
		ReminderType: ReminderType24HBefore,
		ScheduledAt:  now.Add(1 * time.Hour),
		Status:       ReminderStatusPending,
		Enabled:      true,
	}
	_ = repo.Create(ctx, future)

	// Already sent - should NOT be returned
	sent := &Reminder{
		UserID:       3,
		BookingID:    3,
		ReminderType: ReminderType24HBefore,
		ScheduledAt:  now.Add(-1 * time.Hour),
		Status:       ReminderStatusSent,
		Enabled:      true,
	}
	_ = repo.Create(ctx, sent)

	// Disabled - should NOT be returned
	disabled := &Reminder{
		UserID:       4,
		BookingID:    4,
		ReminderType: ReminderType24HBefore,
		ScheduledAt:  now.Add(-1 * time.Hour),
		Status:       ReminderStatusPending,
		Enabled:      false,
	}
	_ = repo.Create(ctx, disabled)

	pending, err := repo.GetPending(ctx, now)
	require.NoError(t, err)
	assert.Len(t, pending, 1, "Only one pending reminder should be returned")
	assert.Equal(t, int64(1), pending[0].UserID)
}

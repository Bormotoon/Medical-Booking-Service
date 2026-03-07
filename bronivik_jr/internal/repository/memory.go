package repository

import (
	"context"
	"sync"
	"time"

	"bronivik/internal/models"
)

type MemoryStateRepository struct {
	mu         sync.Mutex
	states     map[int64]memoryStateEntry
	rateLimits map[int64]rateLimitEntry
	ttl        time.Duration
	now        func() time.Time
}

type memoryStateEntry struct {
	state     *models.UserState
	expiresAt time.Time
}

type rateLimitEntry struct {
	count     int
	expiresAt time.Time
}

func NewMemoryStateRepository(ttl time.Duration) *MemoryStateRepository {
	return &MemoryStateRepository{
		states:     make(map[int64]memoryStateEntry),
		rateLimits: make(map[int64]rateLimitEntry),
		ttl:        ttl,
		now:        time.Now,
	}
}

func (r *MemoryStateRepository) GetState(ctx context.Context, userID int64) (*models.UserState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cleanupExpiredLocked(r.now())
	entry, ok := r.states[userID]
	if !ok {
		return nil, nil
	}
	return cloneUserState(entry.state), nil
}

func (r *MemoryStateRepository) SetState(ctx context.Context, state *models.UserState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	r.cleanupExpiredLocked(now)

	var expiresAt time.Time
	if r.ttl > 0 {
		expiresAt = now.Add(r.ttl)
	}
	r.states[state.UserID] = memoryStateEntry{
		state:     cloneUserState(state),
		expiresAt: expiresAt,
	}
	return nil
}

func (r *MemoryStateRepository) ClearState(ctx context.Context, userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.states, userID)
	return nil
}

func (r *MemoryStateRepository) CheckRateLimit(ctx context.Context, userID int64, limit int, window time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := r.now()
	r.cleanupExpiredLocked(now)

	entry, ok := r.rateLimits[userID]
	if !ok || !now.Before(entry.expiresAt) {
		entry = rateLimitEntry{
			count:     1,
			expiresAt: now.Add(window),
		}
	} else {
		entry.count++
	}

	r.rateLimits[userID] = entry
	return entry.count <= limit, nil
}

func (r *MemoryStateRepository) rateLimitSnapshot(userID int64) (rateLimitEntry, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cleanupExpiredLocked(r.now())
	entry, ok := r.rateLimits[userID]
	return entry, ok
}

func (r *MemoryStateRepository) reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.states = make(map[int64]memoryStateEntry)
	r.rateLimits = make(map[int64]rateLimitEntry)
}

func (r *MemoryStateRepository) cleanupExpiredLocked(now time.Time) {
	for userID, entry := range r.states {
		if !entry.expiresAt.IsZero() && !now.Before(entry.expiresAt) {
			delete(r.states, userID)
		}
	}
	for userID, entry := range r.rateLimits {
		if !now.Before(entry.expiresAt) {
			delete(r.rateLimits, userID)
		}
	}
}

func cloneUserState(state *models.UserState) *models.UserState {
	if state == nil {
		return nil
	}

	cloned := &models.UserState{
		UserID:      state.UserID,
		CurrentStep: state.CurrentStep,
	}
	if state.TempData != nil {
		cloned.TempData = make(map[string]interface{}, len(state.TempData))
		for key, value := range state.TempData {
			cloned.TempData[key] = value
		}
	}
	return cloned
}

package repository

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"bronivik/internal/domain"
	"bronivik/internal/models"

	"github.com/rs/zerolog"
)

type FailoverStateRepository struct {
	primary  domain.StateRepository
	fallback domain.StateRepository
	logger   *zerolog.Logger

	mu                 sync.Mutex
	isDown             atomic.Bool
	lastCheck          time.Time
	recoveryInterval   time.Duration
	pendingStateUpsert map[int64]*models.UserState
	pendingStateDelete map[int64]struct{}
	pendingRateLimits  map[int64]rateLimitEntry
	now                func() time.Time
}

type rateLimitRestorer interface {
	RestoreRateLimit(ctx context.Context, userID int64, count int, ttl time.Duration) error
}

func NewFailoverStateRepository(primary, fallback domain.StateRepository, logger *zerolog.Logger) *FailoverStateRepository {
	return &FailoverStateRepository{
		primary:            primary,
		fallback:           fallback,
		logger:             logger,
		recoveryInterval:   time.Minute,
		pendingStateUpsert: make(map[int64]*models.UserState),
		pendingStateDelete: make(map[int64]struct{}),
		pendingRateLimits:  make(map[int64]rateLimitEntry),
		now:                time.Now,
	}
}

func (r *FailoverStateRepository) checkHealth() {
}

func (r *FailoverStateRepository) GetState(ctx context.Context, userID int64) (*models.UserState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isDown.Load() {
		state, err := r.primary.GetState(ctx, userID)
		if err == nil {
			return state, nil
		}
		r.markPrimaryDownLocked(err)
	}

	if r.recoveryDueLocked() {
		if err := r.flushPendingLocked(ctx); err == nil {
			state, getErr := r.primary.GetState(ctx, userID)
			if getErr == nil {
				r.markRecoveredLocked()
				return state, nil
			}
			r.markRecoveryFailedLocked(getErr)
		} else {
			r.markRecoveryFailedLocked(err)
		}
	}

	return r.fallback.GetState(ctx, userID)
}

func (r *FailoverStateRepository) SetState(ctx context.Context, state *models.UserState) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isDown.Load() {
		if err := r.primary.SetState(ctx, state); err == nil {
			return nil
		} else {
			r.markPrimaryDownLocked(err)
		}
	}

	if r.recoveryDueLocked() {
		if err := r.flushPendingLocked(ctx); err == nil {
			if err := r.primary.SetState(ctx, state); err == nil {
				r.markRecoveredLocked()
				return nil
			} else {
				r.markRecoveryFailedLocked(err)
			}
		} else {
			r.markRecoveryFailedLocked(err)
		}
	}

	if err := r.fallback.SetState(ctx, state); err != nil {
		return err
	}
	r.pendingStateUpsert[state.UserID] = cloneUserState(state)
	delete(r.pendingStateDelete, state.UserID)
	return nil
}

func (r *FailoverStateRepository) ClearState(ctx context.Context, userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isDown.Load() {
		if err := r.primary.ClearState(ctx, userID); err == nil {
			return nil
		} else {
			r.markPrimaryDownLocked(err)
		}
	}

	if r.recoveryDueLocked() {
		if err := r.flushPendingLocked(ctx); err == nil {
			if err := r.primary.ClearState(ctx, userID); err == nil {
				r.markRecoveredLocked()
				return nil
			} else {
				r.markRecoveryFailedLocked(err)
			}
		} else {
			r.markRecoveryFailedLocked(err)
		}
	}

	if err := r.fallback.ClearState(ctx, userID); err != nil {
		return err
	}
	delete(r.pendingStateUpsert, userID)
	r.pendingStateDelete[userID] = struct{}{}
	return nil
}

func (r *FailoverStateRepository) CheckRateLimit(ctx context.Context, userID int64, limit int, window time.Duration) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isDown.Load() {
		allowed, err := r.primary.CheckRateLimit(ctx, userID, limit, window)
		if err == nil {
			return allowed, nil
		}
		r.markPrimaryDownLocked(err)
	}

	if r.recoveryDueLocked() {
		if err := r.flushPendingLocked(ctx); err == nil {
			allowed, err := r.primary.CheckRateLimit(ctx, userID, limit, window)
			if err == nil {
				r.markRecoveredLocked()
				return allowed, nil
			}
			r.markRecoveryFailedLocked(err)
		} else {
			r.markRecoveryFailedLocked(err)
		}
	}

	allowed, err := r.fallback.CheckRateLimit(ctx, userID, limit, window)
	if err != nil {
		return false, err
	}
	r.captureRateLimitLocked(userID)
	return allowed, nil
}

func (r *FailoverStateRepository) recoveryDueLocked() bool {
	return r.isDown.Load() && !r.lastCheck.IsZero() && r.now().Sub(r.lastCheck) >= r.recoveryInterval
}

func (r *FailoverStateRepository) flushPendingLocked(ctx context.Context) error {
	if len(r.pendingStateDelete) == 0 && len(r.pendingStateUpsert) == 0 && len(r.pendingRateLimits) == 0 {
		r.resetFallbackLocked()
		return nil
	}

	for userID := range r.pendingStateDelete {
		if err := r.primary.ClearState(ctx, userID); err != nil {
			return err
		}
	}

	for _, state := range r.pendingStateUpsert {
		if err := r.primary.SetState(ctx, cloneUserState(state)); err != nil {
			return err
		}
	}

	if len(r.pendingRateLimits) > 0 {
		restorer, ok := r.primary.(rateLimitRestorer)
		if !ok {
			return fmt.Errorf("primary repository does not support rate limit restore")
		}

		now := r.now()
		for userID, entry := range r.pendingRateLimits {
			ttl := entry.expiresAt.Sub(now)
			if ttl <= 0 {
				continue
			}
			if err := restorer.RestoreRateLimit(ctx, userID, entry.count, ttl); err != nil {
				return err
			}
		}
	}

	r.pendingStateUpsert = make(map[int64]*models.UserState)
	r.pendingStateDelete = make(map[int64]struct{})
	r.pendingRateLimits = make(map[int64]rateLimitEntry)
	r.resetFallbackLocked()
	return nil
}

func (r *FailoverStateRepository) captureRateLimitLocked(userID int64) {
	fallback, ok := r.fallback.(*MemoryStateRepository)
	if !ok {
		return
	}

	entry, ok := fallback.rateLimitSnapshot(userID)
	if !ok {
		delete(r.pendingRateLimits, userID)
		return
	}
	r.pendingRateLimits[userID] = entry
}

func (r *FailoverStateRepository) resetFallbackLocked() {
	fallback, ok := r.fallback.(*MemoryStateRepository)
	if !ok {
		return
	}
	fallback.reset()
}

func (r *FailoverStateRepository) markPrimaryDownLocked(err error) {
	if r.logger != nil {
		r.logger.Error().Err(err).Msg("Primary state repository failed, falling back to memory")
	}
	r.isDown.Store(true)
	r.lastCheck = r.now()
}

func (r *FailoverStateRepository) markRecoveryFailedLocked(err error) {
	if r.logger != nil {
		r.logger.Warn().Err(err).Msg("Primary state repository recovery failed")
	}
	r.isDown.Store(true)
	r.lastCheck = r.now()
}

func (r *FailoverStateRepository) markRecoveredLocked() {
	r.isDown.Store(false)
	r.lastCheck = time.Time{}
}

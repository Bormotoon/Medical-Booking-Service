package bot

import (
	"context"
	"time"
)

func (b *Bot) withRecovery(handler func()) {
	defer func() {
		if r := recover(); r != nil {
			if b.metrics != nil {
				b.metrics.ErrorsTotal.Inc()
			}
			b.logger.Error().Interface("panic", r).Msg("Recovered from panic in update handler")
		}
	}()
	handler()
}

func (b *Bot) trackActivity(userID int64) {
	scheduledAt, ok := b.beginActivityUpdate(userID)
	if !ok {
		return
	}

	// Update asynchronously only when the debounce window allows a new write.
	go b.performActivityUpdate(context.Background(), userID, scheduledAt)
}

func (b *Bot) beginActivityUpdate(userID int64) (time.Time, bool) {
	if b == nil || b.userService == nil || userID == 0 {
		return time.Time{}, false
	}

	nowFn := b.activityNow
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()

	window := b.activityWindow
	if window <= 0 {
		window = defaultActivityUpdateInterval
	}

	b.activityMu.Lock()
	defer b.activityMu.Unlock()

	if b.activityLast == nil {
		b.activityLast = make(map[int64]time.Time)
	}

	if last, ok := b.activityLast[userID]; ok && now.Sub(last) < window {
		return time.Time{}, false
	}

	b.activityLast[userID] = now
	return now, true
}

func (b *Bot) performActivityUpdate(parentCtx context.Context, userID int64, scheduledAt time.Time) {
	if userID == 0 {
		return
	}

	ctx := parentCtx
	if ctx == nil {
		ctx = context.Background()
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := b.userService.UpdateUserActivity(ctx, userID); err != nil {
		b.rollbackActivityUpdate(userID, scheduledAt)
		b.logger.Error().Err(err).Int64("user_id", userID).Msg("Failed to update user activity")
	}
}

func (b *Bot) rollbackActivityUpdate(userID int64, scheduledAt time.Time) {
	if b == nil || userID == 0 {
		return
	}

	b.activityMu.Lock()
	defer b.activityMu.Unlock()

	if last, ok := b.activityLast[userID]; ok && last.Equal(scheduledAt) {
		delete(b.activityLast, userID)
	}
}

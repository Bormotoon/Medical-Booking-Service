package repository

import (
	"context"
	"sync"
	"testing"
	"time"

	"bronivik/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStateRepository(t *testing.T) {
	repo := NewMemoryStateRepository(time.Hour)
	ctx := context.Background()

	t.Run("SetAndGetState", func(t *testing.T) {
		state := &models.UserState{UserID: 123, CurrentStep: "test"}
		err := repo.SetState(ctx, state)
		require.NoError(t, err)

		got, err := repo.GetState(ctx, 123)
		require.NoError(t, err)
		assert.Equal(t, state, got)
	})

	t.Run("ClearState", func(t *testing.T) {
		err := repo.ClearState(ctx, 123)
		require.NoError(t, err)
		got, _ := repo.GetState(ctx, 123)
		assert.Nil(t, got)
	})

	t.Run("RateLimit", func(t *testing.T) {
		userID := int64(456)
		allowed, _ := repo.CheckRateLimit(ctx, userID, 2, time.Second)
		assert.True(t, allowed)
		allowed, _ = repo.CheckRateLimit(ctx, userID, 2, time.Second)
		assert.True(t, allowed)
		allowed, _ = repo.CheckRateLimit(ctx, userID, 2, time.Second)
		assert.False(t, allowed)

		// Wait for expiry
		time.Sleep(time.Second + 10*time.Millisecond)
		allowed, _ = repo.CheckRateLimit(ctx, userID, 2, time.Second)
		assert.True(t, allowed)
	})

	t.Run("StateTTL", func(t *testing.T) {
		shortRepo := NewMemoryStateRepository(25 * time.Millisecond)
		state := &models.UserState{UserID: 789, CurrentStep: "ttl"}
		require.NoError(t, shortRepo.SetState(ctx, state))

		got, err := shortRepo.GetState(ctx, 789)
		require.NoError(t, err)
		require.NotNil(t, got)

		time.Sleep(40 * time.Millisecond)

		got, err = shortRepo.GetState(ctx, 789)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("RateLimitConcurrent", func(t *testing.T) {
		concurrentRepo := NewMemoryStateRepository(time.Hour)
		const attempts = 64

		var wg sync.WaitGroup
		results := make(chan bool, attempts)
		for i := 0; i < attempts; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				allowed, err := concurrentRepo.CheckRateLimit(ctx, 999, attempts, time.Second)
				if err != nil {
					t.Errorf("CheckRateLimit: %v", err)
					return
				}
				results <- allowed
			}()
		}
		wg.Wait()
		close(results)

		allowedCount := 0
		for allowed := range results {
			if allowed {
				allowedCount++
			}
		}
		assert.Equal(t, attempts, allowedCount)

		allowed, err := concurrentRepo.CheckRateLimit(ctx, 999, attempts, time.Second)
		require.NoError(t, err)
		assert.False(t, allowed)
	})
}

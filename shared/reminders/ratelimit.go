package reminders

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"
)

// RateLimiterConfig holds configuration for the rate limiter.
type RateLimiterConfig struct {
	// Rate is the number of tokens added per second.
	Rate float64
	// Burst is the maximum number of tokens in the bucket.
	Burst int
	// JitterMin is the minimum jitter delay in milliseconds.
	JitterMin int
	// JitterMax is the maximum jitter delay in milliseconds.
	JitterMax int
}

// DefaultRateLimiterConfig returns the default configuration.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Rate:      20.0, // 20 messages per second
		Burst:     30,   // burst of 30 messages
		JitterMin: 50,   // 50ms minimum jitter
		JitterMax: 150,  // 150ms maximum jitter
	}
}

// RateLimiter implements a token bucket rate limiter with jitter.
type RateLimiter struct {
	config   RateLimiterConfig
	tokens   float64
	lastTime time.Time
	mu       sync.Mutex
	rng      *rand.Rand
}

// NewRateLimiter creates a new rate limiter with the given configuration.
func NewRateLimiter(config RateLimiterConfig) *RateLimiter {
	return &RateLimiter{
		config:   config,
		tokens:   float64(config.Burst),
		lastTime: time.Now(),
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Wait blocks until a token is available or the context is cancelled.
// Returns nil on success, context.Err() on cancellation.
func (r *RateLimiter) Wait(ctx context.Context) error {
	// Add jitter before rate limiting
	jitter := r.getJitter()
	if jitter > 0 {
		select {
		case <-time.After(jitter):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	r.mu.Lock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	r.tokens = math.Min(float64(r.config.Burst), r.tokens+elapsed*r.config.Rate)
	r.lastTime = now

	if r.tokens >= 1 {
		r.tokens--
		r.mu.Unlock()
		return nil
	}

	// Calculate wait time for next token
	waitTime := time.Duration((1 - r.tokens) / r.config.Rate * float64(time.Second))
	r.mu.Unlock()

	select {
	case <-time.After(waitTime):
		r.mu.Lock()
		r.tokens = 0
		r.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// getJitter returns a random jitter duration.
func (r *RateLimiter) getJitter() time.Duration {
	if r.config.JitterMax <= r.config.JitterMin {
		return time.Duration(r.config.JitterMin) * time.Millisecond
	}

	r.mu.Lock()
	jitterMs := r.config.JitterMin + r.rng.Intn(r.config.JitterMax-r.config.JitterMin)
	r.mu.Unlock()

	return time.Duration(jitterMs) * time.Millisecond
}

// TryAcquire attempts to acquire a token without blocking.
// Returns true if a token was acquired, false otherwise.
func (r *RateLimiter) TryAcquire() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	r.tokens = math.Min(float64(r.config.Burst), r.tokens+elapsed*r.config.Rate)
	r.lastTime = now

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// Available returns the current number of available tokens.
func (r *RateLimiter) Available() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastTime).Seconds()
	return math.Min(float64(r.config.Burst), r.tokens+elapsed*r.config.Rate)
}

package reminders

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// RetryConfig holds configuration for retry logic.
type RetryConfig struct {
	MaxRetries  int
	RetryDelays []time.Duration
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		RetryDelays: []time.Duration{
			1 * time.Second,
			5 * time.Second,
			30 * time.Second,
		},
	}
}

// TelegramError represents an error from Telegram API.
type TelegramError struct {
	Code       int
	Message    string
	RetryAfter int // seconds to wait before retrying (for 429 errors)
}

func (e *TelegramError) Error() string {
	return fmt.Sprintf("telegram error %d: %s", e.Code, e.Message)
}

// IsTelegramError checks if the error is a TelegramError.
func IsTelegramError(err error) (*TelegramError, bool) {
	var tgErr *TelegramError
	if errors.As(err, &tgErr) {
		return tgErr, true
	}
	return nil, false
}

// ReminderSender handles sending reminders with rate limiting and retry logic.
type ReminderSender struct {
	notifier    Notifier
	repo        ReminderRepository
	bookings    BookingStore
	rateLimiter *RateLimiter
	retryConfig RetryConfig
	logger      Logger
}

// ReminderSenderConfig holds configuration for the sender.
type ReminderSenderConfig struct {
	RateLimiter RateLimiterConfig
	Retry       RetryConfig
}

// DefaultReminderSenderConfig returns the default configuration.
func DefaultReminderSenderConfig() ReminderSenderConfig {
	return ReminderSenderConfig{
		RateLimiter: DefaultRateLimiterConfig(),
		Retry:       DefaultRetryConfig(),
	}
}

// NewReminderSender creates a new reminder sender.
func NewReminderSender(
	notifier Notifier,
	repo ReminderRepository,
	bookings BookingStore,
	config ReminderSenderConfig,
	logger Logger,
) *ReminderSender {
	return &ReminderSender{
		notifier:    notifier,
		repo:        repo,
		bookings:    bookings,
		rateLimiter: NewRateLimiter(config.RateLimiter),
		retryConfig: config.Retry,
		logger:      logger,
	}
}

// SendWithRetry sends a reminder with retry logic and rate limiting.
func (s *ReminderSender) SendWithRetry(ctx context.Context, r *Reminder, booking Booking) error {
	// Wait for rate limiter
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limiter: %w", err)
	}

	var lastErr error
	maxRetries := s.retryConfig.MaxRetries
	delays := s.retryConfig.RetryDelays

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Try to send
		err := s.notifier.SendReminder(ctx, r.UserID, booking)
		if err == nil {
			// Success - mark as sent
			return s.markAsSent(ctx, r)
		}

		lastErr = err

		// Check error type
		if tgErr, ok := IsTelegramError(err); ok {
			switch tgErr.Code {
			case 429: // Too Many Requests
				waitTime := time.Duration(tgErr.RetryAfter) * time.Second
				if waitTime == 0 && attempt < len(delays) {
					waitTime = delays[attempt]
				}
				s.logger.Info("rate limited by Telegram, waiting",
					"retry_after", waitTime,
					"attempt", attempt,
					"reminder_id", r.ID)

				select {
				case <-time.After(waitTime):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}

			case 403: // Bot blocked by user
				s.logger.Info("user blocked bot",
					"user_id", r.UserID,
					"reminder_id", r.ID)
				return s.markAsFailed(ctx, r, "user_blocked")

			case 400: // Bad Request
				s.logger.Error("bad request to Telegram",
					"error", err,
					"reminder_id", r.ID)
				return s.markAsFailed(ctx, r, "bad_request")
			}
		}

		// For other errors, retry with backoff
		if attempt < maxRetries {
			delay := delays[attempt]
			s.logger.Info("retrying reminder send",
				"attempt", attempt+1,
				"max_retries", maxRetries,
				"delay", delay,
				"error", err)

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	// All retries exhausted
	s.logger.Error("max retries exceeded for reminder",
		"reminder_id", r.ID,
		"user_id", r.UserID,
		"error", lastErr)

	return s.markAsFailed(ctx, r, "max_retries_exceeded")
}

// markAsSent marks a reminder as successfully sent.
func (s *ReminderSender) markAsSent(ctx context.Context, r *Reminder) error {
	now := time.Now()
	r.Status = ReminderStatusSent
	r.SentAt = &now
	r.UpdatedAt = now

	if err := s.repo.UpdateReminder(ctx, r); err != nil {
		s.logger.Error("failed to mark reminder as sent",
			"reminder_id", r.ID,
			"error", err)
		return err
	}

	s.logger.Info("reminder sent successfully",
		"reminder_id", r.ID,
		"user_id", r.UserID,
		"booking_id", r.BookingID)

	return nil
}

// markAsFailed marks a reminder as failed.
func (s *ReminderSender) markAsFailed(ctx context.Context, r *Reminder, reason string) error {
	r.Status = ReminderStatusFailed
	r.LastError = reason
	r.UpdatedAt = time.Now()

	if err := s.repo.UpdateReminder(ctx, r); err != nil {
		s.logger.Error("failed to mark reminder as failed",
			"reminder_id", r.ID,
			"error", err)
		return err
	}

	s.logger.Info("reminder marked as failed",
		"reminder_id", r.ID,
		"user_id", r.UserID,
		"reason", reason)

	return nil
}

// IncrementRetryCount increments the retry count for a reminder.
func (s *ReminderSender) IncrementRetryCount(ctx context.Context, r *Reminder) error {
	r.RetryCount++
	r.UpdatedAt = time.Now()
	return s.repo.UpdateReminder(ctx, r)
}

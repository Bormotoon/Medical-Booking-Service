package reminders

import (
	"context"
	"sync"
	"time"
)

// SchedulerConfig holds configuration for the reminder scheduler.
type SchedulerConfig struct {
	// Timezone for scheduling (e.g., "Europe/Moscow")
	Timezone string
	// DailyHour is the hour (0-23) when daily reminders are processed.
	DailyHour int
	// DailyMinute is the minute (0-59) when daily reminders are processed.
	DailyMinute int
	// CheckInterval is how often to check if it's time to run.
	CheckInterval time.Duration
	// CleanupEnabled enables automatic cleanup of old reminders.
	CleanupEnabled bool
	// CleanupRetentionDays is how many days to keep sent reminders.
	CleanupRetentionDays int
}

// DefaultSchedulerConfig returns the default scheduler configuration.
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		Timezone:             "Europe/Moscow",
		DailyHour:            12,
		DailyMinute:          0,
		CheckInterval:        1 * time.Minute,
		CleanupEnabled:       true,
		CleanupRetentionDays: 1,
	}
}

// Scheduler manages the reminder sending schedule.
type Scheduler struct {
	config      SchedulerConfig
	service     *Service
	sender      *ReminderSender
	location    *time.Location
	logger      Logger
	mu          sync.Mutex
	lastRunDate string // YYYY-MM-DD of last run
	running     bool
	stopCh      chan struct{}
}

// NewScheduler creates a new reminder scheduler.
func NewScheduler(
	config SchedulerConfig,
	service *Service,
	sender *ReminderSender,
	logger Logger,
) (*Scheduler, error) {
	loc, err := time.LoadLocation(config.Timezone)
	if err != nil {
		return nil, err
	}

	return &Scheduler{
		config:   config,
		service:  service,
		sender:   sender,
		location: loc,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}, nil
}

// Start begins the scheduler loop.
func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.logger.Info("reminder scheduler started",
		"timezone", s.config.Timezone,
		"daily_time", s.formatTime())

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("reminder scheduler stopped by context")
			return
		case <-s.stopCh:
			s.logger.Info("reminder scheduler stopped")
			return
		case <-ticker.C:
			s.checkAndRun(ctx)
		}
	}
}

// Stop stops the scheduler.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.running {
		s.running = false
		close(s.stopCh)
	}
	s.mu.Unlock()
}

// checkAndRun checks if it's time to run and executes if needed.
func (s *Scheduler) checkAndRun(ctx context.Context) {
	now := time.Now().In(s.location)
	today := now.Format("2006-01-02")

	s.mu.Lock()
	alreadyRan := s.lastRunDate == today
	s.mu.Unlock()

	if alreadyRan {
		return
	}

	// Check if it's the right time
	if now.Hour() != s.config.DailyHour || now.Minute() != s.config.DailyMinute {
		return
	}

	s.logger.Info("starting daily reminder processing",
		"date", today,
		"time", now.Format("15:04:05"))

	s.mu.Lock()
	s.lastRunDate = today
	s.mu.Unlock()

	s.processDailyReminders(ctx)

	if s.config.CleanupEnabled {
		s.cleanupOldReminders(ctx)
	}
}

// processDailyReminders processes all pending reminders.
func (s *Scheduler) processDailyReminders(ctx context.Context) {
	start := time.Now()
	stats := struct {
		total   int
		sent    int
		skipped int
		failed  int
	}{}

	now := time.Now()
	// Get pending reminders
	filter := ReminderFilter{
		ScheduledAtBefore: &now,
		Status:            []ReminderStatus{ReminderStatusPending, ReminderStatusScheduled},
	}
	enabled := true
	filter.Enabled = &enabled

	reminders, err := s.service.repo.FindReminders(ctx, filter)
	if err != nil {
		s.logger.Error("failed to fetch pending reminders", "error", err)
		return
	}

	stats.total = len(reminders)
	s.logger.Info("found pending reminders", "count", stats.total)

	for i := range reminders {
		// Check context cancellation
		select {
		case <-ctx.Done():
			s.logger.Info("reminder processing interrupted",
				"processed", stats.sent+stats.failed,
				"remaining", stats.total-stats.sent-stats.failed)
			return
		default:
		}

		// Process single reminder
		r := &reminders[i]
		if err := s.processReminder(ctx, r); err != nil {
			stats.failed++
		} else {
			stats.sent++
		}
	}

	duration := time.Since(start)
	s.logger.Info("daily reminders processed",
		"total", stats.total,
		"sent", stats.sent,
		"skipped", stats.skipped,
		"failed", stats.failed,
		"duration", duration)
}

// processReminder processes a single reminder.
func (s *Scheduler) processReminder(ctx context.Context, r *Reminder) error {
	// Try to acquire the reminder (for distributed processing)
	acquired, err := s.service.repo.TryAcquireReminder(ctx, r.ID)
	if err != nil {
		s.logger.Error("failed to acquire reminder", "id", r.ID, "error", err)
		return err
	}
	if !acquired {
		s.logger.Debug("reminder already being processed", "id", r.ID)
		return nil
	}
	defer s.service.repo.ReleaseReminder(ctx, r.ID)

	// Check user settings
	settings, err := s.service.userSettings.GetUserSettings(ctx, r.UserID)
	if err != nil {
		s.logger.Error("failed to get user settings", "user_id", r.UserID, "error", err)
		return err
	}
	if !settings.RemindersEnabled {
		s.logger.Debug("reminders disabled for user", "user_id", r.UserID)
		r.Status = ReminderStatusCancelled
		r.UpdatedAt = time.Now()
		return s.service.repo.UpdateReminder(ctx, r)
	}

	// Get booking
	bookings, err := s.service.bookings.GetUpcomingBookings(ctx, 48*time.Hour)
	if err != nil {
		s.logger.Error("failed to get booking", "error", err)
		return err
	}

	// Find the booking for this reminder
	var booking Booking
	for _, b := range bookings {
		if b.GetID() == r.BookingID {
			booking = b
			break
		}
	}

	if booking == nil {
		s.logger.Info("booking not found for reminder", "booking_id", r.BookingID)
		r.Status = ReminderStatusCancelled
		r.UpdatedAt = time.Now()
		return s.service.repo.UpdateReminder(ctx, r)
	}

	// Send with retry
	return s.sender.SendWithRetry(ctx, r, booking)
}

// cleanupOldReminders removes old sent/failed reminders.
func (s *Scheduler) cleanupOldReminders(ctx context.Context) {
	retention := time.Duration(s.config.CleanupRetentionDays) * 24 * time.Hour
	cutoff := time.Now().Add(-retention)

	// Delete sent reminders older than retention period
	sentFilter := ReminderFilter{
		Status:     []ReminderStatus{ReminderStatusSent},
		SentBefore: &cutoff,
	}

	deleted, err := s.service.repo.DeleteReminders(ctx, sentFilter)
	if err != nil {
		s.logger.Error("failed to cleanup sent reminders", "error", err)
	} else if deleted > 0 {
		s.logger.Info("cleaned up old sent reminders", "deleted", deleted)
	}

	// Delete failed reminders older than 3 days
	failedCutoff := time.Now().Add(-3 * 24 * time.Hour)
	failedFilter := ReminderFilter{
		Status:     []ReminderStatus{ReminderStatusFailed},
		SentBefore: &failedCutoff,
	}

	deleted, err = s.service.repo.DeleteReminders(ctx, failedFilter)
	if err != nil {
		s.logger.Error("failed to cleanup failed reminders", "error", err)
	} else if deleted > 0 {
		s.logger.Info("cleaned up old failed reminders", "deleted", deleted)
	}
}

// RunNow forces an immediate run of the reminder processing.
func (s *Scheduler) RunNow(ctx context.Context) {
	s.logger.Info("manual reminder processing triggered")
	s.processDailyReminders(ctx)
	if s.config.CleanupEnabled {
		s.cleanupOldReminders(ctx)
	}
}

// formatTime returns the scheduled time as a string.
func (s *Scheduler) formatTime() string {
	return time.Date(2000, 1, 1, s.config.DailyHour, s.config.DailyMinute, 0, 0, time.UTC).Format("15:04")
}

// IsRunning returns whether the scheduler is currently running.
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

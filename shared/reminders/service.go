package reminders

import (
	"context"
	"sync"
	"time"
)

// Config holds configuration for the reminder service.
type Config struct {
	// CheckInterval is how often to check for upcoming bookings.
	// Default: 15 minutes.
	CheckInterval time.Duration

	// DefaultHoursBefore is the default number of hours before a booking
	// to send a reminder if user has no custom setting.
	// Default: 24 hours.
	DefaultHoursBefore int

	// MaxConcurrentNotifications limits parallel notification sends.
	// Default: 10.
	MaxConcurrentNotifications int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		CheckInterval:              15 * time.Minute,
		DefaultHoursBefore:         24,
		MaxConcurrentNotifications: 10,
	}
}

// Service handles sending booking reminders.
type Service struct {
	config       *Config
	bookings     BookingStore
	settings     UserSettingsStore
	notifier     Notifier
	logger       Logger
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.Mutex
	running      bool
}

// NewService creates a new reminder service.
func NewService(
	config *Config,
	bookings BookingStore,
	settings UserSettingsStore,
	notifier Notifier,
	logger Logger,
) *Service {
	if config == nil {
		config = DefaultConfig()
	}
	if config.CheckInterval == 0 {
		config.CheckInterval = 15 * time.Minute
	}
	if config.DefaultHoursBefore == 0 {
		config.DefaultHoursBefore = 24
	}
	if config.MaxConcurrentNotifications == 0 {
		config.MaxConcurrentNotifications = 10
	}

	return &Service{
		config:   config,
		bookings: bookings,
		settings: settings,
		notifier: notifier,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the reminder check loop.
func (s *Service) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.loop()

	if s.logger != nil {
		s.logger.Info("Reminder service started",
			"check_interval", s.config.CheckInterval,
			"default_hours_before", s.config.DefaultHoursBefore,
		)
	}
}

// Stop gracefully stops the reminder service.
func (s *Service) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()

	if s.logger != nil {
		s.logger.Info("Reminder service stopped")
	}
}

func (s *Service) loop() {
	defer s.wg.Done()

	// Run immediately on start
	s.checkAndSendReminders()

	ticker := time.NewTicker(s.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndSendReminders()
		}
	}
}

func (s *Service) checkAndSendReminders() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Look for bookings within the next 25 hours (to catch 24h reminders with some buffer)
	lookAhead := time.Duration(s.config.DefaultHoursBefore+1) * time.Hour

	bookings, err := s.bookings.GetUpcomingBookings(ctx, lookAhead)
	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to get upcoming bookings", "error", err)
		}
		return
	}

	if len(bookings) == 0 {
		return
	}

	if s.logger != nil {
		s.logger.Debug("Found bookings to check for reminders", "count", len(bookings))
	}

	// Use semaphore to limit concurrent notifications
	sem := make(chan struct{}, s.config.MaxConcurrentNotifications)
	var wg sync.WaitGroup

	for _, booking := range bookings {
		if booking.IsReminderSent() {
			continue
		}

		// Check user settings
		settings, err := s.settings.GetUserSettings(ctx, booking.GetUserID())
		if err != nil {
			if s.logger != nil {
				s.logger.Error("Failed to get user settings",
					"user_id", booking.GetUserID(),
					"error", err,
				)
			}
			// Use default settings on error
			settings = DefaultUserSettings(booking.GetUserID())
		}

		if !settings.RemindersEnabled {
			continue
		}

		// Check if it's time to send the reminder
		reminderTime := booking.GetStartTime().Add(-time.Duration(settings.ReminderHoursBefore) * time.Hour)
		if time.Now().Before(reminderTime) {
			continue
		}

		// Send reminder
		wg.Add(1)
		sem <- struct{}{} // acquire

		go func(b Booking) {
			defer wg.Done()
			defer func() { <-sem }() // release

			if err := s.sendReminder(ctx, b); err != nil {
				if s.logger != nil {
					s.logger.Error("Failed to send reminder",
						"booking_id", b.GetID(),
						"user_id", b.GetUserID(),
						"error", err,
					)
				}
			}
		}(booking)
	}

	wg.Wait()
}

func (s *Service) sendReminder(ctx context.Context, booking Booking) error {
	// Send notification
	if err := s.notifier.SendReminder(ctx, booking.GetUserID(), booking); err != nil {
		return err
	}

	// Mark as sent
	if err := s.bookings.MarkReminderSent(ctx, booking.GetID()); err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to mark reminder as sent (notification was sent)",
				"booking_id", booking.GetID(),
				"error", err,
			)
		}
		// Don't return error - notification was sent successfully
	}

	if s.logger != nil {
		s.logger.Info("Reminder sent",
			"booking_id", booking.GetID(),
			"user_id", booking.GetUserID(),
		)
	}

	return nil
}

// CheckNow triggers an immediate check for reminders (useful for testing).
func (s *Service) CheckNow() {
	s.checkAndSendReminders()
}

package bot

import (
	"context"
	"log"
	"time"

	"bronivik/internal/models"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// StartReminders schedules daily reminders for next-day bookings.
func (b *Bot) StartReminders(ctx context.Context) {
	if b == nil || b.db == nil || b.bot == nil {
		return
	}

	go func() {
		// First wait until next 09:00 local time, then tick every 24h.
		wait := timeUntilNextHour(9)
		timer := time.NewTimer(wait)
		defer timer.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				b.sendTomorrowReminders(ctx)
				timer.Reset(24 * time.Hour)
			}
		}
	}()
}

func (b *Bot) sendTomorrowReminders(ctx context.Context) {
	start := time.Now().Add(24 * time.Hour).Truncate(24 * time.Hour)
	end := start

	bookings, err := b.db.GetBookingsByDateRange(ctx, start, end)
	if err != nil {
		log.Printf("reminder: get bookings: %v", err)
		return
	}

	for _, booking := range bookings {
		if !shouldRemindStatus(booking.Status) {
			continue
		}

		user, err := b.db.GetUserByID(ctx, booking.UserID)
		if err != nil {
			log.Printf("reminder: load user %d: %v", booking.UserID, err)
			continue
		}
		if user.TelegramID == 0 {
			continue
		}

		msgText := formatReminderMessage(booking)
		msg := tgbotapi.NewMessage(user.TelegramID, msgText)
		if _, err := b.bot.Send(msg); err != nil {
			log.Printf("reminder: send to %d: %v", user.TelegramID, err)
		}
	}
}

func shouldRemindStatus(status string) bool {
	switch status {
	case "pending", "confirmed", "changed":
		return true
	default:
		return false
	}
}

func formatReminderMessage(b models.Booking) string {
	date := b.Date.Format("02.01.2006")
	return "Напоминание: завтра у вас бронь " + b.ItemName + " на " + date + ". Статус: " + b.Status
}

func timeUntilNextHour(hour int) time.Duration {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next.Sub(now)
}

package manager

import (
	"context"
	"fmt"
	"time"

	"bronivik/bronivik_crm/internal/model"
)

// BookingStatus represents booking status.
type BookingStatus string

const (
	StatusPending       BookingStatus = "pending"
	StatusApproved      BookingStatus = "approved"
	StatusRejected      BookingStatus = "rejected"
	StatusNeedsRevision BookingStatus = "needs_revision"
	StatusCanceled      BookingStatus = "canceled"
	StatusCompleted     BookingStatus = "completed"
)

// BookingRepository provides booking operations.
type BookingRepository interface {
	GetBooking(ctx context.Context, id int64) (*model.HourlyBooking, error)
	ListBookings(ctx context.Context, filter BookingFilter) ([]model.HourlyBooking, error)
	UpdateBookingStatus(ctx context.Context, id int64, status, comment string) error
	UpdateBooking(ctx context.Context, booking *model.HourlyBooking) error
	DeleteBooking(ctx context.Context, id int64) error
}

// DeviceClient provides device operations.
type DeviceClient interface {
	BookDeviceSimple(ctx context.Context, deviceID int64, date time.Time, externalID, clientName, clientPhone string) (int64, error)
	CancelDeviceBooking(ctx context.Context, externalID string) error
}

// NotificationSender sends notifications to users.
type NotificationSender interface {
	SendMessage(ctx context.Context, chatID int64, message string) error
}

// UserRepository provides user operations.
type UserRepository interface {
	GetUserChatID(ctx context.Context, userID int64) (int64, error)
}

// BookingFilter for filtering bookings.
type BookingFilter struct {
	Status    string
	DateFrom  time.Time
	DateTo    time.Time
	UserID    int64
	CabinetID int64
	Limit     int
	Offset    int
}

// Service provides manager operations.
type Service struct {
	bookings      BookingRepository
	devices       DeviceClient
	notifications NotificationSender
	users         UserRepository
}

// NewService creates a new manager service.
func NewService(
	bookings BookingRepository,
	devices DeviceClient,
	notifications NotificationSender,
	users UserRepository,
) *Service {
	return &Service{
		bookings:      bookings,
		devices:       devices,
		notifications: notifications,
		users:         users,
	}
}

// ListBookings returns bookings based on filter.
func (s *Service) ListBookings(ctx context.Context, filter BookingFilter) ([]model.HourlyBooking, error) {
	if filter.Limit <= 0 {
		filter.Limit = 10
	}
	return s.bookings.ListBookings(ctx, filter)
}

// GetBooking returns a booking by ID.
func (s *Service) GetBooking(ctx context.Context, id int64) (*model.HourlyBooking, error) {
	return s.bookings.GetBooking(ctx, id)
}

// ApproveBooking approves a booking.
func (s *Service) ApproveBooking(ctx context.Context, bookingID int64, managerComment string) error {
	booking, err := s.bookings.GetBooking(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("get booking: %w", err)
	}

	if booking.Status != string(StatusPending) {
		return fmt.Errorf("cannot approve booking with status %s", booking.Status)
	}

	if err := s.bookings.UpdateBookingStatus(ctx, bookingID, string(StatusApproved), managerComment); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Notify user
	if s.notifications != nil && s.users != nil {
		chatID, err := s.users.GetUserChatID(ctx, booking.UserID)
		if err == nil && chatID > 0 {
			msg := fmt.Sprintf("✅ Ваша заявка #%d подтверждена!\n\n📅 %s, %s – %s\n🔬 %s",
				bookingID,
				booking.StartTime.Format("02.01.2006"),
				booking.StartTime.Format("15:04"),
				booking.EndTime.Format("15:04"),
				booking.ItemName,
			)
			if managerComment != "" {
				msg += fmt.Sprintf("\n\n💬 Комментарий: %s", managerComment)
			}
			_ = s.notifications.SendMessage(ctx, chatID, msg)
		}
	}

	return nil
}

// RejectBooking rejects a booking.
func (s *Service) RejectBooking(ctx context.Context, bookingID int64, reason string) error {
	booking, err := s.bookings.GetBooking(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("get booking: %w", err)
	}

	if booking.Status == string(StatusRejected) || booking.Status == string(StatusCanceled) {
		return fmt.Errorf("booking already finalized")
	}

	// Cancel device booking if exists
	if s.devices != nil && booking.ExternalDeviceBookingID > 0 {
		externalID := fmt.Sprintf("crm-%d", booking.ID)
		_ = s.devices.CancelDeviceBooking(ctx, externalID)
	}

	if err := s.bookings.UpdateBookingStatus(ctx, bookingID, string(StatusRejected), reason); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Notify user
	if s.notifications != nil && s.users != nil {
		chatID, err := s.users.GetUserChatID(ctx, booking.UserID)
		if err == nil && chatID > 0 {
			msg := fmt.Sprintf("❌ Ваша заявка #%d отклонена.\n\n📅 %s, %s – %s",
				bookingID,
				booking.StartTime.Format("02.01.2006"),
				booking.StartTime.Format("15:04"),
				booking.EndTime.Format("15:04"),
			)
			if reason != "" {
				msg += fmt.Sprintf("\n\n📝 Причина: %s", reason)
			}
			_ = s.notifications.SendMessage(ctx, chatID, msg)
		}
	}

	return nil
}

// RequestRevision sends booking back for revision.
func (s *Service) RequestRevision(ctx context.Context, bookingID int64, comment string) error {
	booking, err := s.bookings.GetBooking(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("get booking: %w", err)
	}

	if booking.Status != string(StatusPending) {
		return fmt.Errorf("cannot request revision for status %s", booking.Status)
	}

	if err := s.bookings.UpdateBookingStatus(ctx, bookingID, string(StatusNeedsRevision), comment); err != nil {
		return fmt.Errorf("update status: %w", err)
	}

	// Notify user
	if s.notifications != nil && s.users != nil {
		chatID, err := s.users.GetUserChatID(ctx, booking.UserID)
		if err == nil && chatID > 0 {
			msg := fmt.Sprintf("⚠️ Заявка #%d требует доработки.\n\n💬 Комментарий менеджера:\n%s\n\nПожалуйста, внесите изменения и отправьте заново.",
				bookingID, comment,
			)
			_ = s.notifications.SendMessage(ctx, chatID, msg)
		}
	}

	return nil
}

// RescheduleBooking moves booking to a new date/time.
func (s *Service) RescheduleBooking(ctx context.Context, bookingID int64, newStart, newEnd time.Time, notifyUser bool) error {
	booking, err := s.bookings.GetBooking(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("get booking: %w", err)
	}

	oldStart := booking.StartTime
	oldEnd := booking.EndTime

	// Cancel old device booking
	if s.devices != nil && booking.ExternalDeviceBookingID > 0 {
		externalID := fmt.Sprintf("crm-%d", booking.ID)
		_ = s.devices.CancelDeviceBooking(ctx, externalID)
	}

	// Update booking times
	booking.StartTime = newStart
	booking.EndTime = newEnd
	booking.UpdatedAt = time.Now()

	if err := s.bookings.UpdateBooking(ctx, booking); err != nil {
		return fmt.Errorf("update booking: %w", err)
	}

	// Book device for new time if needed
	if s.devices != nil && booking.ItemID > 0 {
		externalID := fmt.Sprintf("crm-%d", booking.ID)
		_, _ = s.devices.BookDeviceSimple(ctx, booking.ItemID, newStart, externalID, booking.ClientName, booking.ClientPhone)
	}

	// Notify user
	if notifyUser && s.notifications != nil && s.users != nil {
		chatID, err := s.users.GetUserChatID(ctx, booking.UserID)
		if err == nil && chatID > 0 {
			msg := fmt.Sprintf("📅 Ваша заявка #%d перенесена.\n\nБыло: %s, %s – %s\nСтало: %s, %s – %s",
				bookingID,
				oldStart.Format("02.01.2006"),
				oldStart.Format("15:04"),
				oldEnd.Format("15:04"),
				newStart.Format("02.01.2006"),
				newStart.Format("15:04"),
				newEnd.Format("15:04"),
			)
			_ = s.notifications.SendMessage(ctx, chatID, msg)
		}
	}

	return nil
}

// ChangeDevice changes the device for a booking.
func (s *Service) ChangeDevice(ctx context.Context, bookingID int64, newDeviceID int64, newDeviceName string) error {
	booking, err := s.bookings.GetBooking(ctx, bookingID)
	if err != nil {
		return fmt.Errorf("get booking: %w", err)
	}

	// Cancel old device booking
	if s.devices != nil && booking.ExternalDeviceBookingID > 0 {
		externalID := fmt.Sprintf("crm-%d", booking.ID)
		_ = s.devices.CancelDeviceBooking(ctx, externalID)
	}

	// Update booking
	booking.ItemID = newDeviceID
	booking.ItemName = newDeviceName
	booking.UpdatedAt = time.Now()

	if err := s.bookings.UpdateBooking(ctx, booking); err != nil {
		return fmt.Errorf("update booking: %w", err)
	}

	// Book new device
	if s.devices != nil {
		externalID := fmt.Sprintf("crm-%d", booking.ID)
		_, _ = s.devices.BookDeviceSimple(ctx, newDeviceID, booking.StartTime, externalID, booking.ClientName, booking.ClientPhone)
	}

	return nil
}

// FormatBookingList formats bookings for display.
func FormatBookingList(bookings []model.HourlyBooking) string {
	if len(bookings) == 0 {
		return "Заявок не найдено."
	}

	result := "📋 *Список заявок:*\n\n"
	for _, b := range bookings {
		statusEmoji := statusToEmoji(BookingStatus(b.Status))
		result += fmt.Sprintf("%s *#%d* | %s\n👤 %s\n📅 %s, %s – %s\n🔬 %s\n\n",
			statusEmoji,
			b.ID,
			b.Status,
			b.ClientName,
			b.StartTime.Format("02.01.2006"),
			b.StartTime.Format("15:04"),
			b.EndTime.Format("15:04"),
			b.ItemName,
		)
	}
	return result
}

func statusToEmoji(status BookingStatus) string {
	switch status {
	case StatusPending:
		return "⏳"
	case StatusApproved:
		return "✅"
	case StatusRejected:
		return "❌"
	case StatusNeedsRevision:
		return "⚠️"
	case StatusCanceled:
		return "🚫"
	case StatusCompleted:
		return "✓"
	default:
		return "❓"
	}
}

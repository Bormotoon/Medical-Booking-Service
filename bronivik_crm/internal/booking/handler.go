package booking

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DefaultHandler implements Handler interface.
type DefaultHandler struct {
	fsm           *FSM
	deviceClient  DeviceClient
	slotChecker   SlotChecker
}

// SlotChecker provides slot availability information.
type SlotChecker interface {
	GetAvailableSlots(ctx context.Context, cabinetID int64, date time.Time) ([]SlotInfo, error)
	IsSlotAvailable(ctx context.Context, cabinetID int64, start, end time.Time) (bool, error)
}

// SlotInfo represents a time slot.
type SlotInfo struct {
	Start     string
	End       string
	Available bool
}

// NewDefaultHandler creates a new handler.
func NewDefaultHandler(deviceClient DeviceClient, slotChecker SlotChecker) *DefaultHandler {
	return &DefaultHandler{
		fsm:          NewFSM(),
		deviceClient: deviceClient,
		slotChecker:  slotChecker,
	}
}

// HandleInput processes user input based on current state.
func (h *DefaultHandler) HandleInput(ctx context.Context, session *Session, input string) TransitionResult {
	session.mu.Lock()
	defer session.mu.Unlock()

	input = strings.TrimSpace(input)
	session.UpdatedAt = time.Now()

	// Handle cancel command
	if strings.ToLower(input) == "/cancel" || strings.ToLower(input) == "отмена" {
		session.State = StateCanceled
		return TransitionResult{
			NewState: StateCanceled,
			Message:  StatePrompts[StateCanceled],
		}
	}

	// Handle back command
	if strings.ToLower(input) == "/back" || strings.ToLower(input) == "назад" {
		return h.handleBack(session)
	}

	switch session.State {
	case StateAskName:
		return h.handleName(session, input)
	case StateAskDate:
		return h.handleDate(session, input)
	case StateAskStartTime:
		return h.handleStartTime(ctx, session, input)
	case StateAskDuration:
		return h.handleDuration(ctx, session, input)
	case StateAskDevice:
		return h.handleDevice(ctx, session, input)
	case StateConfirm:
		return h.handleConfirm(session, input)
	default:
		return TransitionResult{
			NewState: session.State,
			Message:  "Неизвестное состояние. Начните бронирование заново с /book",
			Error:    fmt.Errorf("unknown state: %s", session.State),
		}
	}
}

func (h *DefaultHandler) handleBack(session *Session) TransitionResult {
	var newState State
	switch session.State {
	case StateAskDate:
		newState = StateAskName
	case StateAskStartTime:
		newState = StateAskDate
	case StateAskDuration:
		newState = StateAskStartTime
	case StateAskDevice:
		newState = StateAskDuration
	case StateConfirm:
		newState = StateAskDevice
	default:
		newState = session.State
	}

	session.State = newState
	return TransitionResult{
		NewState: newState,
		Message:  StatePrompts[newState],
	}
}

func (h *DefaultHandler) handleName(session *Session, input string) TransitionResult {
	// Validate name (at least 2 words)
	words := strings.Fields(input)
	if len(words) < 2 {
		return TransitionResult{
			NewState: StateAskName,
			Message:  "Пожалуйста, введите полное ФИО (минимум имя и фамилия).",
		}
	}

	// Basic validation - only letters and spaces
	if !isValidName(input) {
		return TransitionResult{
			NewState: StateAskName,
			Message:  "ФИО может содержать только буквы и пробелы.",
		}
	}

	session.Data.ClientName = input
	session.State = StateAskDate

	return TransitionResult{
		NewState: StateAskDate,
		Message:  StatePrompts[StateAskDate],
	}
}

func (h *DefaultHandler) handleDate(session *Session, input string) TransitionResult {
	// Parse date in various formats
	date, err := parseDate(input)
	if err != nil {
		return TransitionResult{
			NewState: StateAskDate,
			Message:  "Неверный формат даты. Используйте ДД.ММ.ГГГГ или выберите из календаря.",
		}
	}

	// Validate date is not in the past
	today := time.Now().Truncate(24 * time.Hour)
	if date.Before(today) {
		return TransitionResult{
			NewState: StateAskDate,
			Message:  "Нельзя забронировать на прошедшую дату.",
		}
	}

	// Validate date is not too far in the future (e.g., 60 days)
	maxDate := today.AddDate(0, 2, 0)
	if date.After(maxDate) {
		return TransitionResult{
			NewState: StateAskDate,
			Message:  "Бронирование доступно максимум на 60 дней вперёд.",
		}
	}

	session.Data.Date = date
	session.State = StateAskStartTime

	return TransitionResult{
		NewState: StateAskStartTime,
		Message:  StatePrompts[StateAskStartTime],
	}
}

func (h *DefaultHandler) handleStartTime(ctx context.Context, session *Session, input string) TransitionResult {
	// Parse time in HH:MM format
	startTime, err := parseTime(session.Data.Date, input)
	if err != nil {
		return TransitionResult{
			NewState: StateAskStartTime,
			Message:  "Неверный формат времени. Используйте ЧЧ:ММ (например, 14:30).",
		}
	}

	// Check if slot is available
	if h.slotChecker != nil {
		available, err := h.slotChecker.IsSlotAvailable(ctx, session.Data.CabinetID, startTime, startTime.Add(30*time.Minute))
		if err != nil || !available {
			return TransitionResult{
				NewState: StateAskStartTime,
				Message:  "Это время уже занято. Выберите другое время.",
			}
		}
	}

	session.Data.StartTime = startTime
	session.State = StateAskDuration

	return TransitionResult{
		NewState: StateAskDuration,
		Message:  StatePrompts[StateAskDuration],
	}
}

func (h *DefaultHandler) handleDuration(ctx context.Context, session *Session, input string) TransitionResult {
	// Parse duration
	duration := parseDuration(input)
	if duration <= 0 {
		return TransitionResult{
			NewState: StateAskDuration,
			Message:  "Выберите длительность из предложенных вариантов.",
		}
	}

	endTime := session.Data.StartTime.Add(time.Duration(duration) * time.Minute)

	// Check if all slots in range are available
	if h.slotChecker != nil {
		available, err := h.slotChecker.IsSlotAvailable(ctx, session.Data.CabinetID, session.Data.StartTime, endTime)
		if err != nil || !available {
			return TransitionResult{
				NewState: StateAskDuration,
				Message:  "Выбранный период недоступен. Попробуйте меньшую длительность.",
			}
		}
	}

	session.Data.Duration = duration
	session.Data.EndTime = endTime
	session.State = StateAskDevice

	return TransitionResult{
		NewState: StateAskDevice,
		Message:  StatePrompts[StateAskDevice],
	}
}

func (h *DefaultHandler) handleDevice(ctx context.Context, session *Session, input string) TransitionResult {
	// Parse device selection
	deviceID, deviceName, err := h.parseDeviceSelection(ctx, session.Data.Date, input)
	if err != nil {
		return TransitionResult{
			NewState: StateAskDevice,
			Message:  "Выберите аппарат из списка или введите номер.",
		}
	}

	session.Data.DeviceID = deviceID
	session.Data.DeviceName = deviceName
	session.State = StateConfirm

	return TransitionResult{
		NewState: StateConfirm,
		Message:  FormatBookingConfirmation(&session.Data),
	}
}

func (h *DefaultHandler) handleConfirm(session *Session, input string) TransitionResult {
	input = strings.ToLower(input)

	if input == "да" || input == "подтвердить" || input == "✅" {
		session.State = StateComplete
		return TransitionResult{
			NewState: StateComplete,
			Message:  StatePrompts[StateComplete],
		}
	}

	if input == "нет" || input == "отмена" || input == "❌" {
		session.State = StateCanceled
		return TransitionResult{
			NewState: StateCanceled,
			Message:  StatePrompts[StateCanceled],
		}
	}

	return TransitionResult{
		NewState: StateConfirm,
		Message:  "Нажмите 'Да' для подтверждения или 'Нет' для отмены.",
	}
}

func (h *DefaultHandler) parseDeviceSelection(ctx context.Context, date time.Time, input string) (int64, string, error) {
	if h.deviceClient == nil {
		// Default device for testing
		return 1, input, nil
	}

	devices, err := h.deviceClient.GetAvailableDevices(ctx, date)
	if err != nil {
		return 0, "", err
	}

	// Try to match by number
	var num int
	if _, err := fmt.Sscanf(input, "%d", &num); err == nil && num > 0 && num <= len(devices) {
		d := devices[num-1]
		return d.ID, d.Name, nil
	}

	// Try to match by name
	inputLower := strings.ToLower(input)
	for _, d := range devices {
		if strings.ToLower(d.Name) == inputLower || strings.Contains(strings.ToLower(d.Name), inputLower) {
			return d.ID, d.Name, nil
		}
	}

	return 0, "", fmt.Errorf("device not found")
}

// GetPrompt returns the prompt for current state.
func (h *DefaultHandler) GetPrompt(state State, data *BookingData) string {
	return StatePrompts[state]
}

// GetKeyboard returns keyboard for current state.
func (h *DefaultHandler) GetKeyboard(ctx context.Context, state State, data *BookingData) interface{} {
	// This would return Telegram keyboard markup
	// Actual implementation depends on telegram bot library
	return nil
}

// Helper functions

var nameRegex = regexp.MustCompile(`^[\p{L}\s\-']+$`)

func isValidName(name string) bool {
	return nameRegex.MatchString(name)
}

func parseDate(input string) (time.Time, error) {
	formats := []string{
		"02.01.2006",
		"2.1.2006",
		"02-01-2006",
		"2006-01-02",
		"02/01/2006",
	}

	input = strings.TrimSpace(input)

	for _, format := range formats {
		if t, err := time.Parse(format, input); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse date: %s", input)
}

func parseTime(date time.Time, input string) (time.Time, error) {
	input = strings.TrimSpace(input)

	// Try HH:MM format
	var hour, minute int
	if _, err := fmt.Sscanf(input, "%d:%d", &hour, &minute); err == nil {
		if hour >= 0 && hour < 24 && minute >= 0 && minute < 60 {
			return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, date.Location()), nil
		}
	}

	// Try H:MM format
	if _, err := fmt.Sscanf(input, "%d:%d", &hour, &minute); err == nil {
		if hour >= 0 && hour < 24 && minute >= 0 && minute < 60 {
			return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, date.Location()), nil
		}
	}

	return time.Time{}, fmt.Errorf("cannot parse time: %s", input)
}

func parseDuration(input string) int {
	input = strings.TrimSpace(strings.ToLower(input))

	// Common patterns
	switch input {
	case "30", "30 мин", "30 минут", "полчаса":
		return 30
	case "60", "1 час", "час", "1час":
		return 60
	case "90", "1.5", "1.5 часа", "1,5 часа", "полтора часа":
		return 90
	case "120", "2", "2 часа", "два часа":
		return 120
	case "150", "2.5", "2.5 часа", "2,5 часа":
		return 150
	case "180", "3", "3 часа", "три часа":
		return 180
	}

	// Try to parse number directly
	var num int
	if _, err := fmt.Sscanf(input, "%d", &num); err == nil && num > 0 && num <= 480 {
		return num
	}

	return 0
}

// Package booking provides FSM-based booking dialog implementation.
package booking

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// State represents the current state of booking dialog.
type State string

const (
	StateIdle         State = "idle"
	StateAskName      State = "ask_name"
	StateAskDate      State = "ask_date"
	StateAskStartTime State = "ask_start_time"
	StateAskDuration  State = "ask_duration"
	StateAskDevice    State = "ask_device"
	StateConfirm      State = "confirm"
	StateComplete     State = "complete"
	StateCanceled     State = "canceled"
)

// BookingData holds the data collected during booking dialog.
type BookingData struct {
	UserID      int64
	CabinetID   int64
	ClientName  string
	ClientPhone string
	Date        time.Time
	StartTime   time.Time
	EndTime     time.Time
	Duration    int // minutes
	DeviceID    int64
	DeviceName  string
	Comment     string
	CreatedAt   time.Time
}

// Session represents a booking dialog session.
type Session struct {
	State     State
	Data      BookingData
	StartedAt time.Time
	UpdatedAt time.Time
	mu        sync.Mutex
}

// NewSession creates a new booking session.
func NewSession(userID int64) *Session {
	now := time.Now()
	return &Session{
		State: StateAskName,
		Data: BookingData{
			UserID:    userID,
			CreatedAt: now,
		},
		StartedAt: now,
		UpdatedAt: now,
	}
}

// SetState updates the session state.
func (s *Session) SetState(state State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.State = state
	s.UpdatedAt = time.Now()
}

// GetState returns current state.
func (s *Session) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.State
}

// IsExpired checks if session has expired.
func (s *Session) IsExpired(timeout time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return time.Since(s.UpdatedAt) > timeout
}

// SessionStore manages booking sessions.
type SessionStore struct {
	sessions map[int64]*Session
	mu       sync.RWMutex
	timeout  time.Duration
}

// NewSessionStore creates a new session store.
func NewSessionStore(timeout time.Duration) *SessionStore {
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	return &SessionStore{
		sessions: make(map[int64]*Session),
		timeout:  timeout,
	}
}

// Get returns a session for user.
func (ss *SessionStore) Get(userID int64) *Session {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return ss.sessions[userID]
}

// GetOrCreate returns existing or creates new session.
func (ss *SessionStore) GetOrCreate(userID int64) *Session {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	session, ok := ss.sessions[userID]
	if ok && !session.IsExpired(ss.timeout) {
		return session
	}

	session = NewSession(userID)
	ss.sessions[userID] = session
	return session
}

// Delete removes a session.
func (ss *SessionStore) Delete(userID int64) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	delete(ss.sessions, userID)
}

// Reset resets a session to initial state.
func (ss *SessionStore) Reset(userID int64) *Session {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	session := NewSession(userID)
	ss.sessions[userID] = session
	return session
}

// Cleanup removes expired sessions.
func (ss *SessionStore) Cleanup() int {
	ss.mu.Lock()
	defer ss.mu.Unlock()

	removed := 0
	for userID, session := range ss.sessions {
		if session.IsExpired(ss.timeout) {
			delete(ss.sessions, userID)
			removed++
		}
	}
	return removed
}

// Transition represents allowed state transitions.
type Transition struct {
	From State
	To   State
}

// FSM manages state transitions for booking dialog.
type FSM struct {
	transitions map[State][]State
}

// NewFSM creates a new FSM with predefined transitions.
func NewFSM() *FSM {
	return &FSM{
		transitions: map[State][]State{
			StateIdle:         {StateAskName},
			StateAskName:      {StateAskDate, StateCanceled},
			StateAskDate:      {StateAskStartTime, StateAskName, StateCanceled},
			StateAskStartTime: {StateAskDuration, StateAskDate, StateCanceled},
			StateAskDuration:  {StateAskDevice, StateAskStartTime, StateCanceled},
			StateAskDevice:    {StateConfirm, StateAskDuration, StateCanceled},
			StateConfirm:      {StateComplete, StateAskDevice, StateCanceled},
			StateComplete:     {StateIdle},
			StateCanceled:     {StateIdle},
		},
	}
}

// CanTransition checks if transition is allowed.
func (f *FSM) CanTransition(from, to State) bool {
	allowed, ok := f.transitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// Transition updates the session state if the transition is allowed.
func (f *FSM) Transition(session *Session, to State) bool {
	if f.CanTransition(session.GetState(), to) {
		session.SetState(to)
		return true
	}
	return false
}

// TransitionResult contains the result of processing input.
type TransitionResult struct {
	NewState State
	Message  string
	Keyboard interface{} // Telegram keyboard markup
	Error    error
}

// Handler processes user input and manages transitions.
type Handler interface {
	// HandleInput processes user input and returns transition result.
	HandleInput(ctx context.Context, session *Session, input string) TransitionResult

	// GetPrompt returns the prompt for current state.
	GetPrompt(state State, data *BookingData) string

	// GetKeyboard returns keyboard for current state.
	GetKeyboard(ctx context.Context, state State, data *BookingData) interface{}
}

// BookingService creates bookings.
type BookingService interface {
	CreateBooking(ctx context.Context, data BookingData) (int64, error)
}

// DeviceClient fetches available devices.
type DeviceClient interface {
	GetAvailableDevices(ctx context.Context, date time.Time) ([]Device, error)
	BookDevice(ctx context.Context, deviceID int64, date time.Time, externalID string) (int64, error)
}

// Device represents an available device.
type Device struct {
	ID        int64
	Name      string
	Available bool
}

// Prompts for different states.
var StatePrompts = map[State]string{
	StateAskName:      "Введите ваше ФИО:",
	StateAskDate:      "Выберите дату:",
	StateAskStartTime: "Выберите время начала:",
	StateAskDuration:  "Выберите длительность сеанса:",
	StateAskDevice:    "Выберите аппарат:",
	StateConfirm:      "Проверьте данные бронирования.\n\n⚠️ Окончательное подтверждение зависит от звонка менеджера и доступности аппаратов.",
	StateComplete:     "✅ Заявка успешно создана! Ожидайте звонка менеджера для подтверждения.",
	StateCanceled:     "❌ Бронирование отменено.",
}

// FormatBookingConfirmation formats booking data for confirmation.
func FormatBookingConfirmation(data *BookingData) string {
	return fmt.Sprintf(`📋 *Данные бронирования:*

👤 *ФИО:* %s
📅 *Дата:* %s
⏰ *Время:* %s – %s
⏱ *Длительность:* %d мин
🔬 *Аппарат:* %s

Подтвердить?`,
		data.ClientName,
		data.Date.Format("02.01.2006"),
		data.StartTime.Format("15:04"),
		data.EndTime.Format("15:04"),
		data.Duration,
		data.DeviceName,
	)
}

// FormatBookingComplete formats completed booking message.
func FormatBookingComplete(data *BookingData, bookingID int64) string {
	return fmt.Sprintf(`✅ *Заявка #%d создана!*

📋 *Детали:*
👤 %s
📅 %s, %s – %s
🔬 %s

⏳ Ожидайте звонка менеджера для подтверждения.`,
		bookingID,
		data.ClientName,
		data.Date.Format("02.01.2006"),
		data.StartTime.Format("15:04"),
		data.EndTime.Format("15:04"),
		data.DeviceName,
	)
}

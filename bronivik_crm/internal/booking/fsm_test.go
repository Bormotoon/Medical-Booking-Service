package booking

import (
	"testing"
	"time"
)

func TestFSMTransitions(t *testing.T) {
	fsm := NewFSM()

	tests := []struct {
		name        string
		from        State
		to          State
		shouldAllow bool
	}{
		{"idle to ask cabinet", StateIdle, StateAskCabinet, true},
		{"ask cabinet to ask date", StateAskCabinet, StateAskDate, true},
		{"ask date to ask start time", StateAskDate, StateAskStartTime, true},
		{"ask start time to ask duration", StateAskStartTime, StateAskDuration, true},
		{"ask duration to ask device", StateAskDuration, StateAskDevice, true},
		{"ask device to ask name", StateAskDevice, StateAskName, true},
		{"ask name to ask phone", StateAskName, StateAskPhone, true},
		{"ask phone to confirm", StateAskPhone, StateConfirm, true},
		{"confirm to complete", StateConfirm, StateComplete, true},
		// Back transitions
		{"ask date back to ask cabinet", StateAskDate, StateAskCabinet, true},
		{"ask start time back to ask date", StateAskStartTime, StateAskDate, true},
		{"ask name back to ask device", StateAskName, StateAskDevice, true},
		// Invalid transitions
		{"idle to complete", StateIdle, StateComplete, false},
		{"ask cabinet to confirm", StateAskCabinet, StateConfirm, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed := fsm.CanTransition(tt.from, tt.to)
			if allowed != tt.shouldAllow {
				t.Errorf("transition %s -> %s: expected allowed=%v, got %v",
					tt.from, tt.to, tt.shouldAllow, allowed)
			}
		})
	}
}

func TestSessionStore(t *testing.T) {
	store := NewSessionStore(30 * time.Minute)

	// Test Get non-existent session
	session := store.Get(123)
	if session != nil {
		t.Error("expected nil for non-existent session")
	}

	// Test GetOrCreate
	created := store.GetOrCreate(123)
	if created == nil {
		t.Fatal("expected created session")
	}
	if created.Data.UserID != 123 {
		t.Errorf("expected UserID 123, got %d", created.Data.UserID)
	}
	if created.State != StateAskCabinet {
		t.Errorf("expected initial state StateAskCabinet, got %s", created.State)
	}

	// Test Get existing session
	retrieved := store.Get(123)
	if retrieved == nil {
		t.Fatal("expected to retrieve session")
	}
	if retrieved != created {
		t.Error("expected same session object")
	}

	// Test GetOrCreate for existing
	existing := store.GetOrCreate(123)
	if existing != created {
		t.Error("GetOrCreate should return existing session")
	}

	// Test GetOrCreate for new
	newSession := store.GetOrCreate(456)
	if newSession == nil {
		t.Fatal("expected new session")
	}
	if newSession.Data.UserID != 456 {
		t.Errorf("expected UserID 456, got %d", newSession.Data.UserID)
	}

	// Test Delete
	store.Delete(123)
	if store.Get(123) != nil {
		t.Error("session should be deleted")
	}
}

func TestSessionStateTransitions(t *testing.T) {
	store := NewSessionStore(30 * time.Minute)
	session := store.GetOrCreate(123)
	fsm := NewFSM()

	// Initial state (NewSession starts with StateAskCabinet)
	if session.State != StateAskCabinet {
		t.Errorf("expected StateAskCabinet, got %s", session.State)
	}

	// Set state to Idle to test transition
	session.SetState(StateIdle)

	// Valid transition
	if !fsm.Transition(session, StateAskCabinet) {
		t.Error("transition to StateAskCabinet should succeed")
	}
	if session.State != StateAskCabinet {
		t.Errorf("expected StateAskCabinet, got %s", session.State)
	}

	// Set cabinet and transition
	session.Data.CabinetID = 1
	if !fsm.Transition(session, StateAskDate) {
		t.Error("transition to StateAskDate should succeed")
	}

	// Invalid transition
	if fsm.Transition(session, StateComplete) {
		t.Error("transition from StateAskDate to StateComplete should fail")
	}
	if session.State != StateAskDate {
		t.Error("state should remain StateAskDate after failed transition")
	}
}

func TestSessionDataStorage(t *testing.T) {
	session := &Session{
		State:     StateAskName,
		StartedAt: time.Now(),
		Data: BookingData{
			UserID: 123,
		},
	}

	// Test client data
	session.Data.ClientName = "Иванов Иван Иванович"
	session.Data.ClientPhone = "+79991234567"

	if session.Data.ClientName != "Иванов Иван Иванович" {
		t.Error("client name not stored correctly")
	}
	if session.Data.ClientPhone != "+79991234567" {
		t.Error("client phone not stored correctly")
	}

	// Test booking data
	session.Data.CabinetID = 1
	session.Data.Date = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	session.Data.Duration = 60
	session.Data.DeviceID = 5

	if session.Data.CabinetID != 1 {
		t.Error("cabinet ID not stored correctly")
	}
	if session.Data.Duration != 60 {
		t.Error("duration not stored correctly")
	}
}

func TestStatePrompts(t *testing.T) {
	// Verify all states have prompts
	states := []State{
		StateAskCabinet,
		StateAskDate,
		StateAskStartTime,
		StateAskDuration,
		StateAskDevice,
		StateAskName,
		StateAskPhone,
		StateConfirm,
	}

	for _, state := range states {
		prompt, ok := StatePrompts[state]
		if !ok {
			t.Errorf("missing prompt for state %s", state)
		}
		if prompt == "" {
			t.Errorf("empty prompt for state %s", state)
		}
	}
}

func TestIsValidState(t *testing.T) {
	validStates := []State{
		StateIdle,
		StateAskCabinet,
		StateAskDate,
		StateAskStartTime,
		StateAskDuration,
		StateAskDevice,
		StateAskName,
		StateAskPhone,
		StateConfirm,
		StateComplete,
		StateCanceled,
	}

	for _, state := range validStates {
		if !isValidState(state) {
			t.Errorf("state %s should be valid", state)
		}
	}

	// Invalid state
	if isValidState(State("invalid_state")) {
		t.Error("invalid_state should not be valid")
	}
}

// Helper function to check if state is valid
func isValidState(s State) bool {
	switch s {
	case StateIdle, StateAskCabinet, StateAskDate, StateAskStartTime,
		StateAskDuration, StateAskDevice, StateAskName, StateAskPhone,
		StateConfirm, StateComplete, StateCanceled:
		return true
	}
	return false
}

func TestSessionTimeout(t *testing.T) {
	session := &Session{
		State:     StateAskName,
		StartedAt: time.Now().Add(-25 * time.Hour),
		UpdatedAt: time.Now().Add(-25 * time.Hour),
		Data: BookingData{
			UserID: 123,
		},
	}

	timeout := 24 * time.Hour
	isExpired := time.Since(session.UpdatedAt) > timeout

	if !isExpired {
		t.Error("session should be expired after 24 hours")
	}

	// Fresh session
	freshSession := &Session{
		State:     StateAskName,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
		Data: BookingData{
			UserID: 456,
		},
	}

	isFreshExpired := time.Since(freshSession.UpdatedAt) > timeout
	if isFreshExpired {
		t.Error("fresh session should not be expired")
	}
}

func TestBackNavigation(t *testing.T) {
	fsm := NewFSM()

	// Define back navigation paths
	backPaths := map[State]State{
		StateAskDate:      StateAskCabinet,
		StateAskStartTime: StateAskDate,
		StateAskDuration:  StateAskStartTime,
		StateAskDevice:    StateAskDuration,
		StateAskName:      StateAskDevice,
		StateAskPhone:     StateAskName,
		StateConfirm:      StateAskPhone,
	}

	for from, to := range backPaths {
		if !fsm.CanTransition(from, to) {
			t.Errorf("back navigation from %s to %s should be allowed", from, to)
		}
	}
}

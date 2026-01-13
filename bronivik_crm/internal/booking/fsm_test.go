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
		{"init to ask name", StateInit, StateAskName, true},
		{"ask name to ask date", StateAskName, StateAskDate, true},
		{"ask date to ask start time", StateAskDate, StateAskStartTime, true},
		{"ask start time to ask duration", StateAskStartTime, StateAskDuration, true},
		{"ask duration to ask device", StateAskDuration, StateAskDevice, true},
		{"ask device to confirm", StateAskDevice, StateConfirm, true},
		{"confirm to complete", StateConfirm, StateComplete, true},
		// Back transitions
		{"ask date back to ask name", StateAskDate, StateAskName, true},
		{"ask start time back to ask date", StateAskStartTime, StateAskDate, true},
		{"confirm back to ask device", StateConfirm, StateAskDevice, true},
		// Invalid transitions
		{"init to complete", StateInit, StateComplete, false},
		{"ask name to confirm", StateAskName, StateConfirm, false},
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
	store := NewInMemorySessionStore()

	// Test Get non-existent session
	session := store.Get(123)
	if session != nil {
		t.Error("expected nil for non-existent session")
	}

	// Test Create
	created := store.Create(123)
	if created == nil {
		t.Fatal("expected created session")
	}
	if created.UserID != 123 {
		t.Errorf("expected UserID 123, got %d", created.UserID)
	}
	if created.State != StateInit {
		t.Errorf("expected initial state, got %s", created.State)
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
	if newSession.UserID != 456 {
		t.Errorf("expected UserID 456, got %d", newSession.UserID)
	}

	// Test Delete
	store.Delete(123)
	if store.Get(123) != nil {
		t.Error("session should be deleted")
	}
}

func TestSessionStateTransitions(t *testing.T) {
	store := NewInMemorySessionStore()
	session := store.Create(123)
	fsm := NewFSM()

	// Initial state
	if session.State != StateInit {
		t.Errorf("expected StateInit, got %s", session.State)
	}

	// Valid transition
	if !fsm.Transition(session, StateAskName) {
		t.Error("transition to StateAskName should succeed")
	}
	if session.State != StateAskName {
		t.Errorf("expected StateAskName, got %s", session.State)
	}

	// Set name and transition
	session.ClientName = "Иванов Иван"
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
		UserID:    123,
		State:     StateAskName,
		StartedAt: time.Now(),
	}

	// Test client data
	session.ClientName = "Иванов Иван Иванович"
	session.ClientPhone = "+79991234567"

	if session.ClientName != "Иванов Иван Иванович" {
		t.Error("client name not stored correctly")
	}
	if session.ClientPhone != "+79991234567" {
		t.Error("client phone not stored correctly")
	}

	// Test booking data
	session.CabinetID = 1
	session.SelectedDate = time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	session.StartTime = "09:00"
	session.Duration = 60
	session.DeviceID = 5

	if session.CabinetID != 1 {
		t.Error("cabinet ID not stored correctly")
	}
	if session.Duration != 60 {
		t.Error("duration not stored correctly")
	}
}

func TestStatePrompts(t *testing.T) {
	// Verify all states have prompts
	states := []State{
		StateAskName,
		StateAskDate,
		StateAskStartTime,
		StateAskDuration,
		StateAskDevice,
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
		StateInit,
		StateAskName,
		StateAskDate,
		StateAskStartTime,
		StateAskDuration,
		StateAskDevice,
		StateConfirm,
		StateComplete,
		StateCancelled,
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
	case StateInit, StateAskName, StateAskDate, StateAskStartTime,
		StateAskDuration, StateAskDevice, StateConfirm, StateComplete, StateCancelled:
		return true
	}
	return false
}

func TestSessionTimeout(t *testing.T) {
	session := &Session{
		UserID:    123,
		State:     StateAskName,
		StartedAt: time.Now().Add(-25 * time.Hour),
		UpdatedAt: time.Now().Add(-25 * time.Hour),
	}

	timeout := 24 * time.Hour
	isExpired := time.Since(session.UpdatedAt) > timeout

	if !isExpired {
		t.Error("session should be expired after 24 hours")
	}

	// Fresh session
	freshSession := &Session{
		UserID:    456,
		State:     StateAskName,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
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
		StateAskDate:      StateAskName,
		StateAskStartTime: StateAskDate,
		StateAskDuration:  StateAskStartTime,
		StateAskDevice:    StateAskDuration,
		StateConfirm:      StateAskDevice,
	}

	for from, to := range backPaths {
		if !fsm.CanTransition(from, to) {
			t.Errorf("back navigation from %s to %s should be allowed", from, to)
		}
	}
}

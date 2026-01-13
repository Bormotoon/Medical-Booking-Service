package bot

import "sync"

type bookingStep string

const (
	stepNone        bookingStep = "none"
	stepCabinet     bookingStep = "cabinet"
	stepDate        bookingStep = "date"
	stepTime        bookingStep = "time"
	stepDuration    bookingStep = "duration"
	stepItem        bookingStep = "item"
	stepClientName  bookingStep = "client_name"
	stepClientPhone bookingStep = "client_phone"
	stepConfirm     bookingStep = "confirm"
)

type BookingDraft struct {
	CabinetID   int64
	CabinetName string
	ItemName    string
	Date        string // YYYY-MM-DD
	TimeLabel   string // HH:MM-HH:MM (calculated from start+duration)
	StartTime   string // HH:MM
	Duration    int    // minutes
	ClientName  string
	ClientPhone string
}

type userState struct {
	Step           bookingStep
	Draft          BookingDraft
	APIUnreachable bool
	IsManual       bool // Created by manager manually
}

type stateStore struct {
	mu sync.Mutex
	m  map[int64]*userState
}

func newStateStore() *stateStore {
	return &stateStore{m: make(map[int64]*userState)}
}

func (s *stateStore) get(userID int64) *userState {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.m[userID]
	if st == nil {
		st = &userState{Step: stepNone}
		s.m[userID] = st
	}
	return st
}

func (s *stateStore) reset(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, userID)
}

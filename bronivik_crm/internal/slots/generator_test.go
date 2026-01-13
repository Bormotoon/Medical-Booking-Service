package slots

import (
	"context"
	"testing"
	"time"
)

// mockChecker implements BookingChecker for testing
type mockChecker struct {
	bookedSlots map[string]bool // key: "HH:MM"
}

func (m *mockChecker) IsSlotBooked(ctx context.Context, cabinetID int64, start, end time.Time) (bool, error) {
	if m.bookedSlots == nil {
		return false, nil
	}
	key := start.Format("15:04")
	return m.bookedSlots[key], nil
}

func TestGenerateSlots(t *testing.T) {
	// Set time to past so slots aren't considered "past"
	baseDate := time.Now().AddDate(0, 0, 7) // 7 days in future

	tests := []struct {
		name          string
		schedule      ScheduleInfo
		bookedSlots   map[string]bool
		expectedCount int
		expectClosed  bool
	}{
		{
			name: "full day no bookings",
			schedule: ScheduleInfo{
				StartTime:    "09:00",
				EndTime:      "18:00",
				LunchStart:   "13:00",
				LunchEnd:     "14:00",
				SlotDuration: 30,
			},
			bookedSlots:   nil,
			expectedCount: 16, // 9 hours - 1 lunch hour = 8 hours * 2 slots = 16
		},
		{
			name: "with some bookings",
			schedule: ScheduleInfo{
				StartTime:    "09:00",
				EndTime:      "18:00",
				LunchStart:   "13:00",
				LunchEnd:     "14:00",
				SlotDuration: 30,
			},
			bookedSlots: map[string]bool{
				"09:00": true,
				"09:30": true,
				"10:00": true,
			},
			expectedCount: 16, // All slots generated, but some marked unavailable
		},
		{
			name: "closed day",
			schedule: ScheduleInfo{
				IsClosed: true,
			},
			expectedCount: 0,
			expectClosed:  true,
		},
		{
			name: "no lunch break",
			schedule: ScheduleInfo{
				StartTime:    "10:00",
				EndTime:      "12:00",
				SlotDuration: 30,
			},
			expectedCount: 4,
		},
		{
			name: "60 minute slots",
			schedule: ScheduleInfo{
				StartTime:    "09:00",
				EndTime:      "12:00",
				SlotDuration: 60,
			},
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &mockChecker{bookedSlots: tt.bookedSlots}
			generator := NewGenerator(checker)

			slots, err := generator.GenerateSlots(context.Background(), 1, baseDate, tt.schedule)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectClosed && len(slots) != 0 {
				t.Errorf("expected no slots for closed day, got %d", len(slots))
				return
			}

			if len(slots) != tt.expectedCount {
				t.Errorf("expected %d slots, got %d", tt.expectedCount, len(slots))
			}

			// Check no lunch slots if lunch is defined
			if tt.schedule.LunchStart != "" && tt.schedule.LunchEnd != "" {
				for _, slot := range slots {
					slotTime := slot.StartTime.Format("15:04")
					if slotTime >= tt.schedule.LunchStart && slotTime < tt.schedule.LunchEnd {
						t.Errorf("lunch slot %s should not be generated", slotTime)
					}
				}
			}
		})
	}
}

func TestFindConsecutiveSlots(t *testing.T) {
	baseDate := time.Now().AddDate(0, 0, 7)

	tests := []struct {
		name     string
		slots    []Slot
		expected int // number of consecutive groups
	}{
		{
			name: "all available - one group",
			slots: []Slot{
				{StartTime: baseDate.Add(9 * time.Hour), EndTime: baseDate.Add(9*time.Hour + 30*time.Minute), Available: true},
				{StartTime: baseDate.Add(9*time.Hour + 30*time.Minute), EndTime: baseDate.Add(10 * time.Hour), Available: true},
				{StartTime: baseDate.Add(10 * time.Hour), EndTime: baseDate.Add(10*time.Hour + 30*time.Minute), Available: true},
			},
			expected: 1,
		},
		{
			name: "gap in middle - two groups",
			slots: []Slot{
				{StartTime: baseDate.Add(9 * time.Hour), EndTime: baseDate.Add(9*time.Hour + 30*time.Minute), Available: true},
				{StartTime: baseDate.Add(9*time.Hour + 30*time.Minute), EndTime: baseDate.Add(10 * time.Hour), Available: false},
				{StartTime: baseDate.Add(10 * time.Hour), EndTime: baseDate.Add(10*time.Hour + 30*time.Minute), Available: true},
			},
			expected: 2,
		},
		{
			name:     "empty slots",
			slots:    nil,
			expected: 0,
		},
		{
			name: "all unavailable",
			slots: []Slot{
				{StartTime: baseDate.Add(9 * time.Hour), EndTime: baseDate.Add(9*time.Hour + 30*time.Minute), Available: false},
				{StartTime: baseDate.Add(9*time.Hour + 30*time.Minute), EndTime: baseDate.Add(10 * time.Hour), Available: false},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groups := FindConsecutiveSlots(tt.slots)

			if len(groups) != tt.expected {
				t.Errorf("expected %d groups, got %d", tt.expected, len(groups))
			}
		})
	}
}

func TestCanBookConsecutive(t *testing.T) {
	baseDate := time.Now().AddDate(0, 0, 7)
	slot1Start := baseDate.Add(9 * time.Hour)
	slot2Start := baseDate.Add(9*time.Hour + 30*time.Minute)
	slot3Start := baseDate.Add(10 * time.Hour)

	tests := []struct {
		name      string
		slots     []Slot
		startTime time.Time
		count     int
		expected  bool
	}{
		{
			name: "can book 2 consecutive slots",
			slots: []Slot{
				{StartTime: slot1Start, EndTime: slot2Start, Available: true},
				{StartTime: slot2Start, EndTime: slot3Start, Available: true},
				{StartTime: slot3Start, EndTime: slot3Start.Add(30 * time.Minute), Available: true},
			},
			startTime: slot1Start,
			count:     2,
			expected:  true,
		},
		{
			name: "cannot book if next slot unavailable",
			slots: []Slot{
				{StartTime: slot1Start, EndTime: slot2Start, Available: true},
				{StartTime: slot2Start, EndTime: slot3Start, Available: false},
			},
			startTime: slot1Start,
			count:     2,
			expected:  false,
		},
		{
			name: "cannot book more than available",
			slots: []Slot{
				{StartTime: slot1Start, EndTime: slot2Start, Available: true},
				{StartTime: slot2Start, EndTime: slot3Start, Available: true},
			},
			startTime: slot1Start,
			count:     5,
			expected:  false,
		},
		{
			name:      "invalid count",
			slots:     nil,
			startTime: slot1Start,
			count:     0,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CanBookConsecutive(tt.slots, tt.startTime, tt.count)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetDurationOptions(t *testing.T) {
	baseDate := time.Now().AddDate(0, 0, 7)
	slot1Start := baseDate.Add(9 * time.Hour)
	slot2Start := baseDate.Add(9*time.Hour + 30*time.Minute)
	slot3Start := baseDate.Add(10 * time.Hour)
	slot4Start := baseDate.Add(10*time.Hour + 30*time.Minute)

	tests := []struct {
		name         string
		slots        []Slot
		startTime    time.Time
		slotDuration int
		expected     []int // durations in minutes
	}{
		{
			name: "all options from first slot",
			slots: []Slot{
				{StartTime: slot1Start, EndTime: slot2Start, Available: true},
				{StartTime: slot2Start, EndTime: slot3Start, Available: true},
				{StartTime: slot3Start, EndTime: slot4Start, Available: true},
			},
			startTime:    slot1Start,
			slotDuration: 30,
			expected:     []int{30, 60, 90},
		},
		{
			name: "limited by unavailable slot",
			slots: []Slot{
				{StartTime: slot1Start, EndTime: slot2Start, Available: true},
				{StartTime: slot2Start, EndTime: slot3Start, Available: false},
				{StartTime: slot3Start, EndTime: slot4Start, Available: true},
			},
			startTime:    slot1Start,
			slotDuration: 30,
			expected:     []int{30},
		},
		{
			name: "no options if start unavailable",
			slots: []Slot{
				{StartTime: slot1Start, EndTime: slot2Start, Available: false},
			},
			startTime:    slot1Start,
			slotDuration: 30,
			expected:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := GetDurationOptions(tt.slots, tt.startTime, tt.slotDuration)

			if len(options) != len(tt.expected) {
				t.Errorf("expected %d options, got %d: %v", len(tt.expected), len(options), options)
				return
			}

			for i, opt := range options {
				if opt != tt.expected[i] {
					t.Errorf("option %d: expected %d, got %d", i, tt.expected[i], opt)
				}
			}
		})
	}
}

func TestToSlotInfo(t *testing.T) {
	baseDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	slots := []Slot{
		{
			StartTime: baseDate.Add(9 * time.Hour),
			EndTime:   baseDate.Add(9*time.Hour + 30*time.Minute),
			Available: true,
		},
		{
			StartTime: baseDate.Add(9*time.Hour + 30*time.Minute),
			EndTime:   baseDate.Add(10 * time.Hour),
			Available: false,
		},
	}

	infos := ToSlotInfo(slots)

	if len(infos) != 2 {
		t.Fatalf("expected 2 slot infos, got %d", len(infos))
	}

	if infos[0].Start != "09:00" || infos[0].End != "09:30" {
		t.Errorf("unexpected first slot: %v", infos[0])
	}

	if infos[0].Available != true {
		t.Error("first slot should be available")
	}

	if infos[1].Available != false {
		t.Error("second slot should be unavailable")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		minutes  int
		expected string
	}{
		{30, "30 мин"},
		{60, "1 час"},
		{90, "1 ч 30 мин"},
		{120, "2 часа"},
		{150, "2 ч 30 мин"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatDuration(tt.minutes)
			if result != tt.expected {
				t.Errorf("FormatDuration(%d): expected %q, got %q", tt.minutes, tt.expected, result)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"30 мин", 30},
		{"1 час", 60},
		{"1.5 часа", 90},
		{"2 часа", 120},
		{"1 ч 30 мин", 90},
		{"2 ч 15 мин", 135},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseDuration(tt.input)
			if result != tt.expected {
				t.Errorf("ParseDuration(%q): expected %d, got %d", tt.input, tt.expected, result)
			}
		})
	}
}

func TestGetAvailableSlots(t *testing.T) {
	baseDate := time.Now()

	slots := []Slot{
		{StartTime: baseDate, EndTime: baseDate.Add(30 * time.Minute), Available: true},
		{StartTime: baseDate.Add(30 * time.Minute), EndTime: baseDate.Add(60 * time.Minute), Available: false},
		{StartTime: baseDate.Add(60 * time.Minute), EndTime: baseDate.Add(90 * time.Minute), Available: true},
	}

	available := GetAvailableSlots(slots)

	if len(available) != 2 {
		t.Errorf("expected 2 available slots, got %d", len(available))
	}

	for _, s := range available {
		if !s.Available {
			t.Error("GetAvailableSlots returned unavailable slot")
		}
	}
}

package slots

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Slot represents a time slot.
type Slot struct {
	StartTime time.Time
	EndTime   time.Time
	Available bool
}

// SlotInfo is a simplified representation for UI.
type SlotInfo struct {
	Start     string `json:"start"` // "10:00"
	End       string `json:"end"`   // "10:30"
	Available bool   `json:"available"`
}

// ScheduleInfo contains schedule parameters for a day.
type ScheduleInfo struct {
	StartTime    string // "10:00"
	EndTime      string // "22:00"
	LunchStart   string // "13:00" (optional)
	LunchEnd     string // "14:00" (optional)
	SlotDuration int    // minutes
	IsClosed     bool
}

// BookingChecker checks if a slot is booked.
type BookingChecker interface {
	IsSlotBooked(ctx context.Context, cabinetID int64, start, end time.Time) (bool, error)
}

// Generator generates available slots for a date.
type Generator struct {
	checker BookingChecker
}

// NewGenerator creates a new slot generator.
func NewGenerator(checker BookingChecker) *Generator {
	return &Generator{checker: checker}
}

// GenerateSlots generates all slots for a date based on schedule.
func (g *Generator) GenerateSlots(ctx context.Context, cabinetID int64, date time.Time, schedule ScheduleInfo) ([]Slot, error) {
	if schedule.IsClosed {
		return nil, nil
	}

	if schedule.SlotDuration <= 0 {
		schedule.SlotDuration = 30
	}

	startTime, err := parseTimeOnDate(date, schedule.StartTime)
	if err != nil {
		return nil, fmt.Errorf("parse start time: %w", err)
	}

	endTime, err := parseTimeOnDate(date, schedule.EndTime)
	if err != nil {
		return nil, fmt.Errorf("parse end time: %w", err)
	}

	var lunchStart, lunchEnd time.Time
	hasLunch := schedule.LunchStart != "" && schedule.LunchEnd != ""
	if hasLunch {
		lunchStart, _ = parseTimeOnDate(date, schedule.LunchStart)
		lunchEnd, _ = parseTimeOnDate(date, schedule.LunchEnd)
	}

	slotDuration := time.Duration(schedule.SlotDuration) * time.Minute
	var slots []Slot

	for cursor := startTime; cursor.Add(slotDuration).Before(endTime) || cursor.Add(slotDuration).Equal(endTime); cursor = cursor.Add(slotDuration) {
		slotStart := cursor
		slotEnd := cursor.Add(slotDuration)

		// Skip lunch break
		if hasLunch && isOverlapping(slotStart, slotEnd, lunchStart, lunchEnd) {
			continue
		}

		// Check if slot is booked
		booked := false
		if g.checker != nil {
			booked, err = g.checker.IsSlotBooked(ctx, cabinetID, slotStart, slotEnd)
			if err != nil {
				return nil, fmt.Errorf("check slot: %w", err)
			}
		}

		// Skip past slots
		isPast := slotStart.Before(time.Now())

		slots = append(slots, Slot{
			StartTime: slotStart,
			EndTime:   slotEnd,
			Available: !booked && !isPast,
		})
	}

	return slots, nil
}

// ToSlotInfo converts slots to SlotInfo for UI.
func ToSlotInfo(slots []Slot) []SlotInfo {
	result := make([]SlotInfo, len(slots))
	for i, s := range slots {
		result[i] = SlotInfo{
			Start:     s.StartTime.Format("15:04"),
			End:       s.EndTime.Format("15:04"),
			Available: s.Available,
		}
	}
	return result
}

// GetAvailableSlots returns only available slots.
func GetAvailableSlots(slots []Slot) []Slot {
	var available []Slot
	for _, s := range slots {
		if s.Available {
			available = append(available, s)
		}
	}
	return available
}

// FindConsecutiveSlots finds groups of consecutive available slots.
func FindConsecutiveSlots(slots []Slot) [][]Slot {
	available := GetAvailableSlots(slots)
	if len(available) == 0 {
		return nil
	}

	sort.Slice(available, func(i, j int) bool {
		return available[i].StartTime.Before(available[j].StartTime)
	})

	var groups [][]Slot
	currentGroup := []Slot{available[0]}

	for i := 1; i < len(available); i++ {
		if available[i].StartTime.Equal(currentGroup[len(currentGroup)-1].EndTime) {
			currentGroup = append(currentGroup, available[i])
		} else {
			groups = append(groups, currentGroup)
			currentGroup = []Slot{available[i]}
		}
	}
	groups = append(groups, currentGroup)

	return groups
}

// CanBookConsecutive checks if N consecutive slots starting from given slot are available.
func CanBookConsecutive(slots []Slot, startTime time.Time, count int) bool {
	if count <= 0 {
		return false
	}

	// Find start index
	startIdx := -1
	for i, s := range slots {
		if s.StartTime.Equal(startTime) {
			startIdx = i
			break
		}
	}

	if startIdx < 0 || startIdx+count > len(slots) {
		return false
	}

	// Check all slots are available and consecutive
	for i := 0; i < count; i++ {
		idx := startIdx + i
		if !slots[idx].Available {
			return false
		}
		if i > 0 && !slots[idx].StartTime.Equal(slots[idx-1].EndTime) {
			return false
		}
	}

	return true
}

// GetDurationOptions returns available duration options based on consecutive slots.
func GetDurationOptions(slots []Slot, startTime time.Time, slotDuration int) []int {
	// Find max consecutive available slots from start
	startIdx := -1
	for i, s := range slots {
		if s.StartTime.Equal(startTime) && s.Available {
			startIdx = i
			break
		}
	}

	if startIdx < 0 {
		return nil
	}

	maxSlots := 0
	for i := startIdx; i < len(slots); i++ {
		if !slots[i].Available {
			break
		}
		if i > startIdx && !slots[i].StartTime.Equal(slots[i-1].EndTime) {
			break
		}
		maxSlots++
	}

	// Generate duration options (in minutes)
	var options []int
	for i := 1; i <= maxSlots; i++ {
		options = append(options, i*slotDuration)
	}

	return options
}

func parseTimeOnDate(date time.Time, timeStr string) (time.Time, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid hour: %w", err)
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid minute: %w", err)
	}

	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, date.Location()), nil
}

func isOverlapping(start1, end1, start2, end2 time.Time) bool {
	return start1.Before(end2) && start2.Before(end1)
}

// FormatDuration formats duration in minutes to human-readable string.
func FormatDuration(minutes int) string {
	if minutes < 60 {
		return fmt.Sprintf("%d мин", minutes)
	}
	hours := minutes / 60
	mins := minutes % 60
	if mins == 0 {
		if hours == 1 {
			return "1 час"
		}
		return fmt.Sprintf("%d часа", hours)
	}
	return fmt.Sprintf("%d ч %d мин", hours, mins)
}

// ParseDuration parses duration string to minutes.
func ParseDuration(s string) int {
	// Simple cases
	switch s {
	case "30 мин":
		return 30
	case "1 час":
		return 60
	case "1.5 часа", "1,5 часа":
		return 90
	case "2 часа":
		return 120
	}

	// Try to parse "X ч Y мин" format
	s = strings.TrimSpace(s)
	total := 0

	if strings.Contains(s, "ч") {
		parts := strings.Split(s, "ч")
		if len(parts) >= 1 {
			h, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			total += h * 60
		}
		if len(parts) >= 2 && strings.Contains(parts[1], "мин") {
			mPart := strings.Replace(parts[1], "мин", "", 1)
			m, _ := strconv.Atoi(strings.TrimSpace(mPart))
			total += m
		}
	} else if strings.Contains(s, "мин") {
		mPart := strings.Replace(s, "мин", "", 1)
		m, _ := strconv.Atoi(strings.TrimSpace(mPart))
		total += m
	}

	return total
}

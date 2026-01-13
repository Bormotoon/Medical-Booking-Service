package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// CabinetConfig represents a single cabinet configuration.
type CabinetConfig struct {
	ID              int                      `yaml:"id"`
	Name            string                   `yaml:"name"`
	Number          string                   `yaml:"number"`
	Address         string                   `yaml:"address"`
	Description     string                   `yaml:"description"`
	Floor           int                      `yaml:"floor"`
	Capacity        int                      `yaml:"capacity"`
	IsActive        bool                     `yaml:"is_active"`
	DefaultSchedule *CabinetScheduleConfig   `yaml:"default_schedule,omitempty"`
}

// CabinetScheduleConfig represents schedule configuration.
type CabinetScheduleConfig struct {
	StartTime           string `yaml:"start_time"`            // "10:00"
	EndTime             string `yaml:"end_time"`              // "22:00"
	SlotDurationMinutes int    `yaml:"slot_duration_minutes"` // 30
	LunchStart          string `yaml:"lunch_start,omitempty"` // "13:00"
	LunchEnd            string `yaml:"lunch_end,omitempty"`   // "14:00"
}

// HolidayConfig represents a holiday configuration.
type HolidayConfig struct {
	Date string `yaml:"date"` // "2026-01-01"
	Name string `yaml:"name"` // "Новый год"
}

// DefaultsConfig represents global default settings.
type DefaultsConfig struct {
	Schedule *CabinetScheduleConfig `yaml:"schedule"`
	DaysOff  []int                  `yaml:"days_off"` // 1=Mon, 7=Sun
}

// CabinetsConfig is the root configuration for cabinets.yaml.
type CabinetsConfig struct {
	Cabinets []CabinetConfig `yaml:"cabinets"`
	Defaults DefaultsConfig  `yaml:"defaults"`
	Holidays []HolidayConfig `yaml:"holidays"`
}

// LoadCabinetsConfig loads and validates cabinets configuration from YAML file.
func LoadCabinetsConfig(path string) (*CabinetsConfig, error) {
	if path == "" {
		path = "configs/cabinets.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cabinets config: %w", err)
	}

	var cfg CabinetsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse cabinets config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate cabinets config: %w", err)
	}

	// Apply defaults to cabinets without explicit schedule
	cfg.applyDefaults()

	return &cfg, nil
}

// Validate checks the configuration for errors.
func (c *CabinetsConfig) Validate() error {
	if len(c.Cabinets) == 0 {
		return fmt.Errorf("no cabinets defined")
	}

	ids := make(map[int]bool)
	names := make(map[string]bool)

	for i, cab := range c.Cabinets {
		if cab.ID <= 0 {
			return fmt.Errorf("cabinet[%d]: id must be positive, got %d", i, cab.ID)
		}
		if ids[cab.ID] {
			return fmt.Errorf("cabinet[%d]: duplicate id %d", i, cab.ID)
		}
		ids[cab.ID] = true

		if cab.Name == "" {
			return fmt.Errorf("cabinet[%d]: name is required", i)
		}
		if names[cab.Name] {
			return fmt.Errorf("cabinet[%d]: duplicate name '%s'", i, cab.Name)
		}
		names[cab.Name] = true

		if cab.Capacity < 0 {
			return fmt.Errorf("cabinet[%d]: capacity cannot be negative", i)
		}

		// Validate schedule if present
		if cab.DefaultSchedule != nil {
			if err := validateSchedule(cab.DefaultSchedule, fmt.Sprintf("cabinet[%d].default_schedule", i)); err != nil {
				return err
			}
		}
	}

	// Validate default schedule
	if c.Defaults.Schedule != nil {
		if err := validateSchedule(c.Defaults.Schedule, "defaults.schedule"); err != nil {
			return err
		}
	}

	// Validate holidays
	for i, h := range c.Holidays {
		if h.Date == "" {
			return fmt.Errorf("holiday[%d]: date is required", i)
		}
		if _, err := time.Parse("2006-01-02", h.Date); err != nil {
			return fmt.Errorf("holiday[%d]: invalid date format '%s', expected YYYY-MM-DD", i, h.Date)
		}
	}

	// Validate days off
	for i, d := range c.Defaults.DaysOff {
		if d < 1 || d > 7 {
			return fmt.Errorf("defaults.days_off[%d]: invalid day %d, must be 1-7 (1=Mon, 7=Sun)", i, d)
		}
	}

	return nil
}

// validateSchedule checks a schedule configuration for errors.
func validateSchedule(s *CabinetScheduleConfig, prefix string) error {
	if s.StartTime == "" {
		return fmt.Errorf("%s.start_time is required", prefix)
	}
	if s.EndTime == "" {
		return fmt.Errorf("%s.end_time is required", prefix)
	}

	startTime, err := time.Parse("15:04", s.StartTime)
	if err != nil {
		return fmt.Errorf("%s.start_time: invalid format '%s', expected HH:MM", prefix, s.StartTime)
	}

	endTime, err := time.Parse("15:04", s.EndTime)
	if err != nil {
		return fmt.Errorf("%s.end_time: invalid format '%s', expected HH:MM", prefix, s.EndTime)
	}

	if !endTime.After(startTime) {
		return fmt.Errorf("%s: end_time must be after start_time", prefix)
	}

	if s.SlotDurationMinutes <= 0 {
		return fmt.Errorf("%s.slot_duration_minutes must be positive", prefix)
	}

	// Validate lunch times if present
	if s.LunchStart != "" && s.LunchEnd != "" {
		lunchStart, err := time.Parse("15:04", s.LunchStart)
		if err != nil {
			return fmt.Errorf("%s.lunch_start: invalid format '%s', expected HH:MM", prefix, s.LunchStart)
		}

		lunchEnd, err := time.Parse("15:04", s.LunchEnd)
		if err != nil {
			return fmt.Errorf("%s.lunch_end: invalid format '%s', expected HH:MM", prefix, s.LunchEnd)
		}

		if !lunchEnd.After(lunchStart) {
			return fmt.Errorf("%s: lunch_end must be after lunch_start", prefix)
		}

		if lunchStart.Before(startTime) || lunchEnd.After(endTime) {
			return fmt.Errorf("%s: lunch break must be within working hours", prefix)
		}
	}

	return nil
}

// applyDefaults applies default values to cabinets without explicit configuration.
func (c *CabinetsConfig) applyDefaults() {
	for i := range c.Cabinets {
		// Apply default schedule if not set
		if c.Cabinets[i].DefaultSchedule == nil && c.Defaults.Schedule != nil {
			c.Cabinets[i].DefaultSchedule = c.Defaults.Schedule
		}

		// Apply default capacity if not set
		if c.Cabinets[i].Capacity == 0 {
			c.Cabinets[i].Capacity = 1
		}
	}
}

// GetCabinetByID returns cabinet config by ID.
func (c *CabinetsConfig) GetCabinetByID(id int) *CabinetConfig {
	for i := range c.Cabinets {
		if c.Cabinets[i].ID == id {
			return &c.Cabinets[i]
		}
	}
	return nil
}

// GetCabinetByName returns cabinet config by name.
func (c *CabinetsConfig) GetCabinetByName(name string) *CabinetConfig {
	for i := range c.Cabinets {
		if c.Cabinets[i].Name == name {
			return &c.Cabinets[i]
		}
	}
	return nil
}

// GetActiveCabinets returns only active cabinets.
func (c *CabinetsConfig) GetActiveCabinets() []CabinetConfig {
	result := make([]CabinetConfig, 0)
	for _, cab := range c.Cabinets {
		if cab.IsActive {
			result = append(result, cab)
		}
	}
	return result
}

// IsHoliday checks if a date is a holiday.
func (c *CabinetsConfig) IsHoliday(date time.Time) (bool, string) {
	dateStr := date.Format("2006-01-02")
	for _, h := range c.Holidays {
		if h.Date == dateStr {
			return true, h.Name
		}
	}
	return false, ""
}

// IsDayOff checks if a weekday is a day off.
func (c *CabinetsConfig) IsDayOff(weekday time.Weekday) bool {
	// Convert Go's weekday (0=Sun) to our format (1=Mon, 7=Sun)
	day := int(weekday)
	if day == 0 {
		day = 7
	}

	for _, d := range c.Defaults.DaysOff {
		if d == day {
			return true
		}
	}
	return false
}

// String returns a summary of the configuration.
func (c *CabinetsConfig) String() string {
	active := 0
	for _, cab := range c.Cabinets {
		if cab.IsActive {
			active++
		}
	}
	return fmt.Sprintf("CabinetsConfig: %d cabinets (%d active), %d holidays",
		len(c.Cabinets), active, len(c.Holidays))
}

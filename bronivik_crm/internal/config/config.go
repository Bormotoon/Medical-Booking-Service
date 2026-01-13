package config

import (
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Telegram struct {
		BotToken string `yaml:"bot_token"`
		Debug    bool   `yaml:"debug"`
	} `yaml:"telegram"`

	Database struct {
		Path string `yaml:"path"`
	} `yaml:"database"`

	Backup struct {
		Enabled       bool   `yaml:"enabled"`
		IntervalHours int    `yaml:"interval_hours"`
		Path          string `yaml:"path"`
		RetentionDays int    `yaml:"retention_days"`
	} `yaml:"backup"`

	Redis struct {
		Address  string `yaml:"address"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`

	API struct {
		Enabled         bool   `yaml:"enabled"`
		BaseURL         string `yaml:"base_url"`
		APIKey          string `yaml:"api_key"`
		APIExtra        string `yaml:"api_extra"`
		CacheTTLSeconds int    `yaml:"cache_ttl_seconds"`
	} `yaml:"api"`

	Monitoring struct {
		HealthCheckPort   int  `yaml:"health_check_port"`
		PrometheusEnabled bool `yaml:"prometheus_enabled"`
		PrometheusPort    int  `yaml:"prometheus_port"`
	} `yaml:"monitoring"`

	Booking struct {
		MinAdvanceMinutes int `yaml:"min_advance_minutes"`
		MaxAdvanceDays    int `yaml:"max_advance_days"`
		MaxActivePerUser  int `yaml:"max_active_per_user"`
	} `yaml:"booking"`

	Managers []int64 `yaml:"managers"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = "configs/config.yaml"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Support ${ENV_VAR} placeholders in YAML config.
	data = []byte(os.ExpandEnv(string(data)))

	var cfg Config
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Database.Path == "" {
		cfg.Database.Path = "data/bronivik_crm.db"
	}

	if err = os.MkdirAll(filepath.Dir(cfg.Database.Path), 0o755); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) BookingMinAdvance() time.Duration {
	if c.Booking.MinAdvanceMinutes <= 0 {
		return 60 * time.Minute
	}
	return time.Duration(c.Booking.MinAdvanceMinutes) * time.Minute
}

func (c *Config) BookingMaxAdvance() time.Duration {
	if c.Booking.MaxAdvanceDays <= 0 {
		return 30 * 24 * time.Hour
	}
	return time.Duration(c.Booking.MaxAdvanceDays) * 24 * time.Hour
}

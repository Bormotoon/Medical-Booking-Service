package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

var envPlaceholderPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)

type Config struct {
	Telegram struct {
		BotToken string `yaml:"bot_token"`
		Debug    bool   `yaml:"debug"`
	} `yaml:"telegram"`

	Database struct {
		Path     string `yaml:"path"`
		Postgres struct {
			Host string `yaml:"host"`
			Port int    `yaml:"port"`
			User string `yaml:"user"`
			Pass string `yaml:"password"`
			DB   string `yaml:"dbname"`
			SSL  string `yaml:"sslmode"`
		} `yaml:"postgres"`
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

	Logging struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"logging"`

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

	// CabinetsConfigPath is the path to cabinets.yaml configuration file
	CabinetsConfigPath string `yaml:"cabinets_config_path"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = "configs/config.yaml"
	}

	if err := loadDotEnvIfPresent(".env"); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	data = []byte(expandEnvWithDefaults(string(data)))

	var cfg Config
	if err = yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.Database.Path == "" {
		cfg.Database.Path = "data/bronivik_crm.db"
	}
	if cfg.UsePostgres() {
		return nil, fmt.Errorf("postgresql is not supported; configure sqlite database.path")
	}

	if err = os.MkdirAll(filepath.Dir(cfg.Database.Path), 0o755); err != nil {
		return nil, err
	}

	// Set default cabinets config path
	if cfg.CabinetsConfigPath == "" {
		cfg.CabinetsConfigPath = "configs/cabinets.yaml"
	}

	return &cfg, nil
}

func loadDotEnvIfPresent(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return godotenv.Load(path)
}

func expandEnvWithDefaults(input string) string {
	return envPlaceholderPattern.ReplaceAllStringFunc(input, func(match string) string {
		parts := envPlaceholderPattern.FindStringSubmatch(match)
		if len(parts) == 0 {
			return match
		}

		key := parts[1]
		if value, ok := os.LookupEnv(key); ok && value != "" {
			return value
		}
		if strings.Contains(match, ":-") {
			return parts[3]
		}
		return os.Getenv(key)
	})
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

func (c *Config) UsePostgres() bool {
	p := c.Database.Postgres
	return strings.TrimSpace(p.Host) != "" && strings.TrimSpace(p.User) != "" && strings.TrimSpace(p.DB) != ""
}

func (c *Config) DatabaseDriver() string {
	if c.UsePostgres() {
		return "postgres"
	}
	return "sqlite3"
}

func (c *Config) PostgresDSN() string {
	p := c.Database.Postgres
	port := p.Port
	if port == 0 {
		port = 5432
	}
	ssl := p.SSL
	if ssl == "" {
		ssl = "disable"
	}
	return "host=" + p.Host +
		" port=" + strconv.Itoa(port) +
		" user=" + p.User +
		" password=" + p.Pass +
		" dbname=" + p.DB +
		" sslmode=" + ssl
}

// LoadCabinets loads cabinets configuration from the configured path.
func (c *Config) LoadCabinets() (*CabinetsConfig, error) {
	return LoadCabinetsConfig(c.CabinetsConfigPath)
}

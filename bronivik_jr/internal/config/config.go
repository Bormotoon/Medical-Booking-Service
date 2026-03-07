package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"bronivik/internal/models"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

var envPlaceholderPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(:-([^}]*))?\}`)

type Config struct {
	App              AppConfig        `yaml:"app"`
	Telegram         TelegramConfig   `yaml:"telegram"`
	Database         DatabaseConfig   `yaml:"database"`
	Redis            RedisConfig      `yaml:"redis"`
	Backup           BackupConfig     `yaml:"backup"`
	Monitoring       MonitoringConfig `yaml:"monitoring"`
	Logging          LoggingConfig    `yaml:"logging"`
	API              APIConfig        `yaml:"api"`
	Managers         []int64          `yaml:"managers"`
	ManagersContacts []string         `yaml:"managers_contacts"`
	Blacklist        []int64          `yaml:"blacklist"`
	Items            []models.Item    `yaml:"items"`
	Exports          ExportConfig     `yaml:"exports"`
	Google           GoogleConfig     `yaml:"google"`
	Bot              BotConfig        `yaml:"bot"`
}

type BotConfig struct {
	ReminderTime      string `yaml:"reminder_time"`
	PaginationSize    int    `yaml:"pagination_size"`
	MaxBookingDays    int    `yaml:"max_booking_days"`
	MinBookingAdvance int    `yaml:"min_booking_advance"`
	RateLimitMessages int    `yaml:"rate_limit_messages"`
	RateLimitWindow   int    `yaml:"rate_limit_window"`
}

type APIConfig struct {
	Enabled   bool               `yaml:"enabled"`
	HTTP      APIHTTPConfig      `yaml:"http"`
	GRPC      APIGRPCConfig      `yaml:"grpc"`
	Auth      APIAuthConfig      `yaml:"auth"`
	RateLimit APIRateLimitConfig `yaml:"rate_limit"`
}

type APIHTTPConfig struct {
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
}

type APIGRPCConfig struct {
	Port       int          `yaml:"port"`
	Reflection bool         `yaml:"reflection"`
	TLS        APITLSConfig `yaml:"tls"`
}

type APITLSConfig struct {
	Enabled           bool   `yaml:"enabled"`
	CertFile          string `yaml:"cert_file"`
	KeyFile           string `yaml:"key_file"`
	ClientCAFile      string `yaml:"client_ca_file"`
	RequireClientCert bool   `yaml:"require_client_cert"`
}

type APIAuthConfig struct {
	Enabled      bool           `yaml:"enabled"`
	HeaderAPIKey string         `yaml:"header_api_key"`
	HeaderExtra  string         `yaml:"header_extra"`
	APIKeys      []APIClientKey `yaml:"api_keys"`
	enabledSet   bool
}

type APIClientKey struct {
	Key         string   `yaml:"key"`
	Extra       string   `yaml:"extra"`
	Name        string   `yaml:"name"`
	Permissions []string `yaml:"permissions"`
}

type APIRateLimitConfig struct {
	RPS   float64 `yaml:"rps"`
	Burst int     `yaml:"burst"`
}

type ExportConfig struct {
	Path string `yaml:"path"`
}

type AppConfig struct {
	Name        string `yaml:"name"`
	Environment string `yaml:"environment"`
	Version     string `yaml:"version"`
}

type TelegramConfig struct {
	BotToken   string `yaml:"bot_token"`
	WebhookURL string `yaml:"webhook_url"`
	Debug      bool   `yaml:"debug"`
}

type DatabaseConfig struct {
	Path     string         `yaml:"path"`
	Postgres PostgresConfig `yaml:"postgres"`
}

type PostgresConfig struct {
	Host           string `yaml:"host"`
	Port           int    `yaml:"port"`
	User           string `yaml:"user"`
	Password       string `yaml:"password"`
	DBName         string `yaml:"dbname"`
	SSLMode        string `yaml:"sslmode"`
	MaxConnections int    `yaml:"max_connections"`
	MigrationTable string `yaml:"migration_table"`
}

type RedisConfig struct {
	Address  string `yaml:"address"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
	PoolSize int    `yaml:"pool_size"`
}

type BackupConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Schedule      string `yaml:"schedule"`
	RetentionDays int    `yaml:"retention_days"`
	StoragePath   string `yaml:"storage_path"`
}

type MonitoringConfig struct {
	PrometheusEnabled bool   `yaml:"prometheus_enabled"`
	PrometheusPort    int    `yaml:"prometheus_port"`
	HealthCheckPort   int    `yaml:"health_check_port"`
	LogLevel          string `yaml:"log_level"`
}

type LoggingConfig struct {
	Level    string `yaml:"level"`
	Format   string `yaml:"format"`
	Output   string `yaml:"output"`
	FilePath string `yaml:"file_path"`
}

type GoogleConfig struct {
	GoogleCredentialsFile string `yaml:"credentials_file"`
	UsersSpreadSheetID    string `yaml:"users_spreadsheet_id"`
	BookingSpreadSheetID  string `yaml:"bookings_spreadsheet_id"`
}

func Load(configPath string) (*Config, error) {
	if err := loadDotEnvIfPresent(".env"); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	expandedData := []byte(expandEnvWithDefaults(string(data)))

	var config Config
	if err := yaml.Unmarshal(expandedData, &config); err != nil {
		return nil, err
	}

	config.applyDefaults()

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
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

func (c *Config) Validate() error {
	if c.Telegram.BotToken == "" || c.Telegram.BotToken == "YOUR_BOT_TOKEN_HERE" {
		return errors.New("telegram bot token is required")
	}

	if !c.UsePostgres() && c.Database.Path == "" {
		return errors.New("database path is required")
	}

	if c.UsePostgres() {
		return errors.New("postgresql is not supported; configure sqlite database.path")
	}

	return ValidateItems(c.Items)
}

func (c *Config) UsePostgres() bool {
	p := c.Database.Postgres
	return strings.TrimSpace(p.Host) != "" && strings.TrimSpace(p.User) != "" && strings.TrimSpace(p.DBName) != ""
}

func (c *Config) DatabaseDriver() string {
	if c.UsePostgres() {
		return "postgres"
	}
	return "sqlite3"
}

func (c *Config) PostgresDSN() string {
	p := c.Database.Postgres
	sslmode := p.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	port := p.Port
	if port == 0 {
		port = 5432
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		p.Host, port, p.User, p.Password, p.DBName, sslmode)
}

func ValidateItems(items []models.Item) error {
	// Check for duplicate item IDs
	itemIDs := make(map[int64]bool)
	for _, item := range items {
		if item.ID == 0 {
			return fmt.Errorf("item '%s' has invalid ID 0", item.Name)
		}
		if itemIDs[item.ID] {
			return fmt.Errorf("duplicate item ID found: %d", item.ID)
		}
		itemIDs[item.ID] = true
	}
	return nil
}

func (c *APIAuthConfig) UnmarshalYAML(value *yaml.Node) error {
	type rawAPIAuthConfig APIAuthConfig

	var raw rawAPIAuthConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}

	*c = APIAuthConfig(raw)
	c.enabledSet = hasYAMLKey(value, "enabled")
	return nil
}

func hasYAMLKey(node *yaml.Node, key string) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return true
		}
	}
	return false
}

func (c *Config) applyDefaults() {
	if c.API.GRPC.Port == 0 {
		c.API.GRPC.Port = 8081
	}
	if c.API.HTTP.Port == 0 {
		c.API.HTTP.Port = 8080
	}
	if c.Monitoring.PrometheusEnabled && c.Monitoring.PrometheusPort == 0 {
		c.Monitoring.PrometheusPort = 9090
	}
	// auth enabled by default when API is enabled
	if !c.API.Auth.enabledSet {
		c.API.Auth.Enabled = true
	}
	if !c.API.HTTP.Enabled && c.API.Enabled {
		c.API.HTTP.Enabled = true
	}
	if c.API.Auth.HeaderAPIKey == "" {
		c.API.Auth.HeaderAPIKey = "x-api-key"
	}
	if c.API.Auth.HeaderExtra == "" {
		c.API.Auth.HeaderExtra = "x-api-extra"
	}

	// Bot defaults
	if c.Bot.ReminderTime == "" {
		c.Bot.ReminderTime = fmt.Sprintf("%02d:00", models.ReminderHour)
	}
	if c.Bot.PaginationSize == 0 {
		c.Bot.PaginationSize = models.DefaultPaginationSize
	}
	if c.Bot.MaxBookingDays == 0 {
		c.Bot.MaxBookingDays = 365
	}
	if c.Bot.RateLimitMessages == 0 {
		c.Bot.RateLimitMessages = models.RateLimitMessages
	}
	if c.Bot.RateLimitWindow == 0 {
		c.Bot.RateLimitWindow = models.RateLimitWindow
	}
}

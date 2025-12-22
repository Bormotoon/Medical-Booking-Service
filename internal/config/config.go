package config

import (
	"os"

	"bronivik/internal/models"
	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

type Config struct {
	App              AppConfig        `yaml:"app"`
	Telegram         TelegramConfig   `yaml:"telegram"`
	Database         DatabaseConfig   `yaml:"database"`
	Redis            RedisConfig      `yaml:"redis"`
	Backup           BackupConfig     `yaml:"backup"`
	Monitoring       MonitoringConfig `yaml:"monitoring"`
	Logging          LoggingConfig    `yaml:"logging"`
	Managers         []int64          `yaml:"managers"`
	ManagersContacts []string         `yaml:"managers_contacts"`
	Blacklist        []int64          `yaml:"blacklist"`
	Items            []models.Item    `yaml:"items"`
	Exports          ExportConfig     `yaml:"exports"`
	Google           GoogleConfig     `yaml:"google"`
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
	UsersSpreadSheetId    string `yaml:"users_spreadsheet_id"`
	BookingSpreadSheetId  string `yaml:"bookings_spreadsheet_id"`
}

func Load(configPath string) (*Config, error) {
	// Загружаем .env файл если существует
	err := godotenv.Load(".env")
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// Предварительная замена переменных окружения в YAML
	expandedData := []byte(os.ExpandEnv(string(data)))

	var config Config
	if err := yaml.Unmarshal(expandedData, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

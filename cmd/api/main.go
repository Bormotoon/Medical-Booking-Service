package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bronivik/internal/api"
	"bronivik/internal/config"
	"bronivik/internal/database"
	"bronivik/internal/google"
	"bronivik/internal/logging"
	"bronivik/internal/metrics"
	"bronivik/internal/models"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"
)

func main() {
	// Загрузка конфигурации
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	baseLogger, closer, err := logging.New(cfg.Logging, cfg.App)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}
	if closer != nil {
		defer closer.Close()
	}
	logger := baseLogger.With().Str("component", "api-main").Logger()

	// Загрузка позиций из отдельного файла
	itemsPath := os.Getenv("ITEMS_PATH")
	if itemsPath == "" {
		itemsPath = "configs/items.yaml"
	}
	itemsData, err := os.ReadFile(itemsPath)
	if err != nil {
		logger.Fatal().Err(err).Str("items_path", itemsPath).Msg("read items")
	}

	var itemsConfig struct {
		Items []models.Item `yaml:"items"`
	}
	if err := yaml.Unmarshal(itemsData, &itemsConfig); err != nil {
		logger.Fatal().Err(err).Str("items_path", itemsPath).Msg("parse items")
	}

	// Инициализация базы данных
	db, err := database.NewDB(cfg.Database.Path, &logger)
	if err != nil {
		logger.Fatal().Err(err).Str("db_path", cfg.Database.Path).Msg("init database")
	}
	defer db.Close()

	// Устанавливаем items в базу данных
	db.SetItems(itemsConfig.Items)

	if !cfg.API.Enabled {
		logger.Warn().Msg("API is disabled in config, but starting API application. Check your config.")
	}

	// Инициализация Redis (опционально для health checks)
	var redisClient *redis.Client
	if cfg.Redis.Address != "" {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Address,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
			PoolSize: cfg.Redis.PoolSize,
		})
		if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
			logger.Warn().Err(err).Msg("redis connection failed, continuing without redis")
			redisClient = nil
		} else {
			defer redisClient.Close()
			logger.Info().Str("addr", cfg.Redis.Address).Msg("redis connected")
		}
	}

	// Инициализация Google Sheets (опционально для health checks)
	var sheetsService *google.SheetsService
	if cfg.Google.GoogleCredentialsFile != "" && cfg.Google.BookingSpreadSheetId != "" {
		sheetsService, err = google.NewSimpleSheetsService(
			cfg.Google.GoogleCredentialsFile,
			cfg.Google.UsersSpreadSheetId,
			cfg.Google.BookingSpreadSheetId,
		)
		if err != nil {
			logger.Warn().Err(err).Msg("google sheets init failed, continuing without sheets")
			sheetsService = nil
		} else {
			logger.Info().Msg("google sheets connected")
		}
	}

	grpcServer, err := api.NewGRPCServer(cfg.API, db, &logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("create grpc server")
	}

	httpServer := api.NewHTTPServer(cfg.API, db, redisClient, sheetsService, &logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.Monitoring.PrometheusEnabled {
		metrics.Register()
		if cfg.Monitoring.PrometheusPort == 0 {
			cfg.Monitoring.PrometheusPort = 9090
		}
		go startMetricsServer(ctx, cfg.Monitoring.PrometheusPort, &logger)
	}

	go func() {
		if err := grpcServer.Serve(); err != nil {
			logger.Error().Err(err).Msg("grpc server stopped")
		}
	}()

	go func() {
		if !cfg.API.HTTP.Enabled {
			return
		}
		if err := httpServer.Start(); err != nil {
			logger.Error().Err(err).Msg("http server stopped")
		}
	}()

	logger.Info().Str("grpc_addr", grpcServer.Addr()).Int("http_port", cfg.API.HTTP.Port).Msg("API server started")

	<-ctx.Done()
	logger.Info().Msg("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcServer.Shutdown(shutdownCtx)
	_ = httpServer.Shutdown(shutdownCtx)
	logger.Info().Msg("API server stopped")
}

func startMetricsServer(ctx context.Context, port int, logger *zerolog.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	go func() {
		<-ctx.Done()
		ctxShutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctxShutdown)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error().Err(err).Msg("metrics server error")
	}
}

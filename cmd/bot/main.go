package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	crmapi "bronivik/bronivik_crm/internal/crmapi"
	"bronivik/bronivik_crm/internal/bot"
	"bronivik/bronivik_crm/internal/config"
	"bronivik/bronivik_crm/internal/db"
	"bronivik/bronivik_crm/internal/metrics"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

func main() {
	// Initialize logger
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	logger := zerolog.New(output).With().Timestamp().Logger()

	cfg, err := config.Load(os.Getenv("CRM_CONFIG_PATH"))
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	if cfg.Telegram.BotToken == "" || cfg.Telegram.BotToken == "YOUR_BOT_TOKEN_HERE" {
		logger.Fatal().Msg("set telegram.bot_token in config")
	}

	database, err := db.NewDB(cfg.Database.Path)
	if err != nil {
		logger.Fatal().Err(err).Msg("open db error")
	}
	defer database.Close()

	client := crmapi.NewBronivikClient(cfg.API.BaseURL, cfg.API.APIKey, cfg.API.APIExtra)
	var rdb *redis.Client
	if cfg.Redis.Address != "" && cfg.API.CacheTTLSeconds > 0 {
		rdb = redis.NewClient(&redis.Options{Addr: cfg.Redis.Address, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
		client.UseRedisCache(rdb, time.Duration(cfg.API.CacheTTLSeconds)*time.Second)
	}

	rules := bot.BookingRules{
		MinAdvance:       cfg.BookingMinAdvance(),
		MaxAdvance:       cfg.BookingMaxAdvance(),
		MaxActivePerUser: cfg.Booking.MaxActivePerUser,
	}
	b, err := bot.New(cfg.Telegram.BotToken, client, database, cfg.Managers, rules, &logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("create bot error")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.Monitoring.HealthCheckPort == 0 {
		cfg.Monitoring.HealthCheckPort = 8090
	}
	go startHealthServer(ctx, cfg.Monitoring.HealthCheckPort, database, rdb, &logger)

	if cfg.Monitoring.PrometheusEnabled {
		if cfg.Monitoring.PrometheusPort == 0 {
			cfg.Monitoring.PrometheusPort = 9090
		}
		metrics.Register()
		go startMetricsServer(ctx, cfg.Monitoring.PrometheusPort, &logger)
	}

	logger.Info().Msg("CRM bot started")
	b.Start(ctx)
}

func startHealthServer(ctx context.Context, port int, database *db.DB, rdb *redis.Client, logger *zerolog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		ctxPing, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if err := database.PingContext(ctxPing); err != nil {
			http.Error(w, "db not ready", http.StatusServiceUnavailable)
			return
		}
		if rdb != nil {
			if err := rdb.Ping(ctxPing).Err(); err != nil {
				http.Error(w, "redis not ready", http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	srv := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	go func() {
		<-ctx.Done()
		ctxShutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctxShutdown)
	}()
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error().Err(err).Msg("health server error")
	}
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

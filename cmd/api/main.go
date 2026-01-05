package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"bronivik/internal/api"
	"bronivik/internal/config"
	"bronivik/internal/database"
	"bronivik/internal/metrics"
	"bronivik/internal/models"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
		log.Fatalf("Error loading config: %v", err)
	}

	// Загрузка позиций из отдельного файла
	itemsPath := os.Getenv("ITEMS_PATH")
	if itemsPath == "" {
		itemsPath = "configs/items.yaml"
	}
	itemsData, err := os.ReadFile(itemsPath)
	if err != nil {
		log.Fatalf("Ошибка чтения %s: %v", itemsPath, err)
	}

	var itemsConfig struct {
		Items []models.Item `yaml:"items"`
	}
	if err := yaml.Unmarshal(itemsData, &itemsConfig); err != nil {
		log.Fatalf("Ошибка парсинга %s: %v", itemsPath, err)
	}

	// Инициализация базы данных
	db, err := database.NewDB(cfg.Database.Path)
	if err != nil {
		log.Fatalf("Ошибка инициализации базы данных: %v", err)
	}
	defer db.Close()

	// Устанавливаем items в базу данных
	db.SetItems(itemsConfig.Items)

	if !cfg.API.Enabled {
		log.Println("API is disabled in config, but starting API application. Check your config.")
	}

	grpcServer, err := api.NewGRPCServer(cfg.API, db)
	if err != nil {
		log.Fatalf("Failed to create gRPC API server: %v", err)
	}

	httpServer := api.NewHTTPServer(cfg.API, db)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if cfg.Monitoring.PrometheusEnabled {
		metrics.Register()
		if cfg.Monitoring.PrometheusPort == 0 {
			cfg.Monitoring.PrometheusPort = 9090
		}
		go startMetricsServer(ctx, cfg.Monitoring.PrometheusPort)
	}

	go func() {
		if err := grpcServer.Serve(); err != nil {
			log.Printf("gRPC server stopped: %v", err)
		}
	}()

	go func() {
		if !cfg.API.HTTP.Enabled {
			return
		}
		if err := httpServer.Start(); err != nil {
			log.Printf("HTTP server stopped: %v", err)
		}
	}()

	log.Printf("API server started on %s", grpcServer.Addr())

	<-ctx.Done()
	log.Println("Shutdown signal received...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcServer.Shutdown(shutdownCtx)
	_ = httpServer.Shutdown(shutdownCtx)
	log.Println("API server stopped")
}

func startMetricsServer(ctx context.Context, port int) {
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
		log.Printf("metrics server error: %v", err)
	}
}

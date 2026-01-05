package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"bronivik/internal/bot"
	"bronivik/internal/config"
	"bronivik/internal/database"
	"bronivik/internal/events"
	"bronivik/internal/google"
	"bronivik/internal/models"
	"bronivik/internal/repository"
	"bronivik/internal/worker"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v2"
)

func main() {
	// Загрузка конфигурации
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		log.Fatalf("Config file does not exist: %s", configPath)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	if _, err := os.Stat("configs/items.yaml"); os.IsNotExist(err) {
		log.Fatalf("Config file does not exist: %s", "configs/items.yaml")
	}

	// Загрузка позиций из отдельного файла
	itemsData, err := os.ReadFile("configs/items.yaml")
	if err != nil {
		log.Fatal("Ошибка чтения items.yaml:", err)
	}

	var itemsConfig struct {
		Items []models.Item `yaml:"items"`
	}
	if err := yaml.Unmarshal(itemsData, &itemsConfig); err != nil {
		log.Fatal("Ошибка парсинга items.yaml:", err)
	}

	// Создаем необходимые директории
	if cfg == nil {
		log.Fatal("Cfg configuration is missing in config")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Database.Path), 0755); err != nil {
		log.Fatal("Ошибка создания директории для базы данных:", err)
	}

	if err := os.MkdirAll(cfg.Exports.Path, 0755); err != nil {
		log.Fatal("Ошибка создания директории для экспорта:", err)
	}

	// Инициализация базы данных
	db, err := database.NewDB(cfg.Database.Path)
	if err != nil {
		log.Fatal("Ошибка инициализации базы данных:", err)
	}
	defer db.Close()

	// Устанавливаем items в базу данных
	db.SetItems(itemsConfig.Items)

	if cfg.Telegram.BotToken == "YOUR_BOT_TOKEN_HERE" {
		log.Fatal("Задайте токен бота в config.yaml")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Инициализация Google Sheets через API Key
	var sheetsService *google.SheetsService
	if cfg.Google.GoogleCredentialsFile == "" || cfg.Google.UsersSpreadSheetId == "" || cfg.Google.BookingSpreadSheetId == "" {
		log.Fatal("Нехватает переменных для подключения к Гуглу", err)
	}

	service, err := google.NewSimpleSheetsService(
		cfg.Google.GoogleCredentialsFile,
		cfg.Google.UsersSpreadSheetId,
		cfg.Google.BookingSpreadSheetId,
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize Google Sheets service: %v", err)
	}

	// Тестируем подключение
	if err := service.TestConnection(); err != nil {
		log.Fatalf("Warning: Google Sheets connection test failed: %v", err)
	} else {
		sheetsService = service
		log.Println("Google Sheets service initialized successfully")
	}

	// Инициализация Redis (необязательно)
	var redisClient *redis.Client
	if cfg.Redis.Address != "" {
		redisClient = repository.NewRedisClient(cfg.Redis)
		if err := repository.Ping(ctx, redisClient); err != nil {
			log.Printf("Redis unavailable, falling back to in-memory queue: %v", err)
			redisClient = nil
		}
	}

	// Запускаем воркер синхронизации Google Sheets
	var sheetsWorker *worker.SheetsWorker
	if sheetsService != nil {
		retryPolicy := worker.RetryPolicy{MaxRetries: 5, InitialDelay: 2 * time.Second, MaxDelay: time.Minute, BackoffFactor: 2}
		sheetsWorker = worker.NewSheetsWorker(db, sheetsService, redisClient, retryPolicy, log.Default())
		go sheetsWorker.Start(ctx)
	}

	eventBus := events.NewEventBus()
	subscribeBookingEvents(ctx, eventBus, db, sheetsWorker, log.Default())

	// Создание и запуск бота
	telegramBot, err := bot.NewBot(cfg.Telegram.BotToken, cfg, itemsConfig.Items, db, sheetsService, sheetsWorker, eventBus)
	if err != nil {
		log.Fatal("Ошибка создания бота:", err)
	}

	log.Println("Бот запущен...")
	go telegramBot.Start()
	telegramBot.StartReminders(ctx)

	<-ctx.Done()
	log.Println("Shutdown signal received...")

	telegramBot.Stop()
}

func subscribeBookingEvents(ctx context.Context, bus *events.EventBus, db *database.DB, sheetsWorker *worker.SheetsWorker, logger *log.Logger) {
	if bus == nil || sheetsWorker == nil || db == nil {
		return
	}
	if logger == nil {
		logger = log.Default()
	}

	decode := func(ev events.Event) (events.BookingEventPayload, error) {
		var payload events.BookingEventPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return payload, err
		}
		return payload, nil
	}

	upsertHandler := func(ev events.Event) error {
		payload, err := decode(ev)
		if err != nil {
			logger.Printf("event bus: decode payload for %s: %v", ev.Type, err)
			return nil
		}

		booking, err := db.GetBooking(ctx, payload.BookingID)
		if err != nil {
			logger.Printf("event bus: load booking %d: %v", payload.BookingID, err)
			return nil
		}

		if err := sheetsWorker.EnqueueTask(ctx, worker.SheetTask{Type: worker.TaskUpsert, BookingID: booking.ID, Booking: booking}); err != nil {
			logger.Printf("event bus: enqueue upsert %d: %v", booking.ID, err)
		}
		return nil
	}

	statusHandler := func(ev events.Event) error {
		payload, err := decode(ev)
		if err != nil {
			logger.Printf("event bus: decode payload for %s: %v", ev.Type, err)
			return nil
		}

		status := payload.Status
		if status == "" {
			booking, err := db.GetBooking(ctx, payload.BookingID)
			if err == nil {
				status = booking.Status
			}
		}

		if status == "" {
			logger.Printf("event bus: missing status for booking %d", payload.BookingID)
			return nil
		}

		if err := sheetsWorker.EnqueueTask(ctx, worker.SheetTask{Type: worker.TaskUpdateStatus, BookingID: payload.BookingID, Status: status}); err != nil {
			logger.Printf("event bus: enqueue status %d: %v", payload.BookingID, err)
		}
		return nil
	}

	bus.Subscribe(events.EventBookingCreated, upsertHandler)
	bus.Subscribe(events.EventBookingItemChange, upsertHandler)
	bus.Subscribe(events.EventBookingConfirmed, statusHandler)
	bus.Subscribe(events.EventBookingCancelled, statusHandler)
	bus.Subscribe(events.EventBookingCompleted, statusHandler)
}

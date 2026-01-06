package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"bronivik/internal/api"
	"bronivik/internal/bot"
	"bronivik/internal/config"
	"bronivik/internal/database"
	"bronivik/internal/events"
	"bronivik/internal/google"
	"bronivik/internal/logging"
	"bronivik/internal/models"
	"bronivik/internal/repository"
	"bronivik/internal/service"
	"bronivik/internal/worker"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v2"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("Fatal error: %v", err)
	}
}

func run() error {
	// Загрузка конфигурации
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "configs/config.yaml"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return err
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	// Инициализация логгера
	baseLogger, closer, err := logging.New(cfg.Logging, cfg.App)
	if err != nil {
		return err
	}
	if closer != nil {
		defer (func(c io.Closer) {
			_ = c.Close()
		})(closer)
	}
	logger := baseLogger.With().Str("component", "bot-main").Logger()

	if _, errStat := os.Stat("configs/items.yaml"); os.IsNotExist(errStat) {
		logger.Error().Msgf("Config file does not exist: %s", "configs/items.yaml")
		return errStat
	}

	// Загрузка позиций из отдельного файла
	itemsData, errRead := os.ReadFile("configs/items.yaml")
	if errRead != nil {
		logger.Error().Err(errRead).Msg("Ошибка чтения items.yaml")
		return errRead
	}

	var itemsConfig struct {
		Items []models.Item `yaml:"items"`
	}
	if errUnmarshal := yaml.Unmarshal(itemsData, &itemsConfig); errUnmarshal != nil {
		logger.Error().Err(errUnmarshal).Msg("Ошибка парсинга items.yaml")
		return errUnmarshal
	}

	if errValidate := config.ValidateItems(itemsConfig.Items); errValidate != nil {
		logger.Error().Err(errValidate).Msg("Items validation failed")
		return errValidate
	}

	// Создаем необходимые директории
	if cfg == nil {
		return os.ErrInvalid
	}
	dbPath := cfg.Database.Path
	if errMkdirDB := os.MkdirAll(filepath.Dir(dbPath), 0o755); errMkdirDB != nil {
		logger.Error().Err(errMkdirDB).Msg("Ошибка создания директории для базы данных")
		return errMkdirDB
	}

	exportPath := cfg.Exports.Path
	if errMkdirExport := os.MkdirAll(exportPath, 0o755); errMkdirExport != nil {
		logger.Error().Err(errMkdirExport).Msg("Ошибка создания директории для экспорта")
		return errMkdirExport
	}

	// Инициализация базы данных
	db, errDB := database.NewDB(dbPath, &logger)
	if errDB != nil {
		logger.Error().Err(errDB).Msg("Ошибка инициализации базы данных")
		return errDB
	}
	defer db.Close()

	// Синхронизируем items с базой данных
	if errSync := db.SyncItems(context.Background(), itemsConfig.Items); errSync != nil {
		logger.Error().Err(errSync).Msg("Ошибка синхронизации позиций")
	}

	if cfg.Telegram.BotToken == "YOUR_BOT_TOKEN_HERE" {
		logger.Error().Msg("Задайте токен бота в config.yaml")
		return os.ErrInvalid
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Инициализация Google Sheets через API Key
	var sheetsService *google.SheetsService
	if cfg.Google.GoogleCredentialsFile == "" || cfg.Google.UsersSpreadSheetID == "" || cfg.Google.BookingSpreadSheetID == "" {
		logger.Error().Msg("Нехватает переменных для подключения к Гуглу")
		return os.ErrInvalid
	}

	sheetsSvc, errSvc := google.NewSimpleSheetsService(
		cfg.Google.GoogleCredentialsFile,
		cfg.Google.UsersSpreadSheetID,
		cfg.Google.BookingSpreadSheetID,
	)
	if errSvc != nil {
		logger.Warn().Err(errSvc).Msg("Failed to initialize Google Sheets service")
	}

	// Тестируем подключение
	if errConn := sheetsSvc.TestConnection(ctx); errConn != nil {
		logger.Error().Err(errConn).Msg("Google Sheets connection test failed")
		return errConn
	}
	sheetsService = sheetsSvc
	logger.Info().Msg("Google Sheets service initialized successfully")

	// Инициализация Redis
	var redisClient *redis.Client
	if cfg.Redis.Address != "" {
		redisClient = repository.NewRedisClient(cfg.Redis)
		if errPing := repository.Ping(ctx, redisClient); errPing != nil {
			logger.Warn().Err(errPing).Msg("Redis unavailable")
		}
	}

	// Инициализация сервиса состояний
	primaryRepo := repository.NewRedisStateRepository(redisClient, time.Duration(models.DefaultRedisTTL)*time.Second)
	fallbackRepo := repository.NewMemoryStateRepository(time.Duration(models.DefaultRedisTTL) * time.Second)
	stateRepo := repository.NewFailoverStateRepository(primaryRepo, fallbackRepo, &logger)
	stateService := service.NewStateService(stateRepo, &logger)

	// Запускаем воркер синхронизации Google Sheets
	var sheetsWorker *worker.SheetsWorker
	if sheetsService != nil {
		retryPolicy := worker.RetryPolicy{MaxRetries: 5, InitialDelay: 2 * time.Second, MaxDelay: time.Minute, BackoffFactor: 2}
		sheetsWorker = worker.NewSheetsWorker(db, sheetsService, redisClient, retryPolicy, &logger)
		go sheetsWorker.Start(ctx)
	}

	eventBus := events.NewEventBus()
	subscribeBookingEvents(ctx, eventBus, db, sheetsWorker, &logger)

	// Инициализация бизнес-сервисов
	bookingService := service.NewBookingService(db, eventBus, sheetsWorker, cfg.Bot.MaxBookingDays, cfg.Bot.MinBookingAdvance, &logger)
	userService := service.NewUserService(db, cfg, &logger)
	itemService := service.NewItemService(db, &logger)
	metrics := bot.NewMetrics()

	// Инициализация API сервера
	if cfg.API.Enabled {
		apiServer := api.NewHTTPServer(&cfg.API, db, redisClient, sheetsService, &logger)
		go func() {
			if errApi := apiServer.Start(); errApi != nil {
				logger.Error().Err(errApi).Msg("API server error")
			}
		}()
		defer func() {
			_ = apiServer.Shutdown(context.Background())
		}()
	}

	// Инициализация сервиса бэкапов
	if cfg.Backup.Enabled {
		backupService := database.NewBackupService(cfg.Database.Path, cfg.Backup, &logger)
		go backupService.Start(ctx)
	}

	// Создание и запуск бота
	botAPI, errBotAPI := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if errBotAPI != nil {
		logger.Error().Err(errBotAPI).Msg("Ошибка создания BotAPI")
		return errBotAPI
	}
	botWrapper := bot.NewBotWrapper(botAPI)
	tgService := service.NewTelegramService(botWrapper)

	telegramBot, errBot := bot.NewBot(tgService, cfg, stateService, sheetsService, sheetsWorker, eventBus, bookingService, userService, itemService, metrics, &logger)
	if errBot != nil {
		logger.Error().Err(errBot).Msg("Ошибка создания бота")
		return errBot
	}

	logger.Info().Msg("Бот запущен...")

	// Запускаем напоминания
	telegramBot.StartReminders(ctx)

	// Запускаем бота (блокирующий вызов)
	telegramBot.Start(ctx)

	logger.Info().Msg("Shutdown complete.")
	return nil
}

func subscribeBookingEvents(ctx context.Context, bus *events.EventBus, db *database.DB, sheetsWorker *worker.SheetsWorker, logger *zerolog.Logger) {
	if bus == nil || sheetsWorker == nil || db == nil {
		return
	}

	decode := func(ev *events.Event) (events.BookingEventPayload, error) {
		var payload events.BookingEventPayload
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return payload, err
		}
		return payload, nil
	}

	upsertHandler := func(ev *events.Event) error {
		payload, err := decode(ev)
		if err != nil {
			logger.Error().Err(err).Str("event", ev.Type).Msg("event bus: decode payload")
			return nil
		}

		booking, err := db.GetBooking(ctx, payload.BookingID)
		if err != nil {
			logger.Error().Err(err).Int64("booking_id", payload.BookingID).Msg("event bus: load booking")
			return nil
		}

		if err := sheetsWorker.EnqueueTask(ctx, "upsert", booking.ID, booking, ""); err != nil {
			logger.Error().Err(err).Int64("booking_id", booking.ID).Msg("event bus: enqueue upsert")
		}
		return nil
	}

	statusHandler := func(ev *events.Event) error {
		payload, err := decode(ev)
		if err != nil {
			logger.Error().Err(err).Str("event", ev.Type).Msg("event bus: decode payload")
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
			logger.Error().Int64("booking_id", payload.BookingID).Msg("event bus: missing status")
			return nil
		}

		if err := sheetsWorker.EnqueueTask(ctx, "update_status", payload.BookingID, nil, status); err != nil {
			logger.Error().Err(err).Int64("booking_id", payload.BookingID).Msg("event bus: enqueue status")
		}
		return nil
	}

	bus.Subscribe(events.EventBookingCreated, upsertHandler)
	bus.Subscribe(events.EventBookingItemChange, upsertHandler)
	bus.Subscribe(events.EventBookingConfirmed, statusHandler)
	bus.Subscribe(events.EventBookingCanceled, statusHandler)
	bus.Subscribe(events.EventBookingCompleted, statusHandler)
}

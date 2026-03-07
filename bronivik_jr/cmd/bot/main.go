package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

const (
	defaultConfigPath = "configs/config.yaml"
	defaultItemsPath  = "configs/items.yaml"

	modeBot    = "bot"
	modeWorker = "worker"

	workerJobReminders = "reminders"
)

type appCommand struct {
	mode      string
	workerJob string
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("Fatal error: %v", err)
	}
}

func run() error {
	cmd, err := parseCommand(os.Args[1:])
	if err != nil {
		return err
	}

	switch cmd.mode {
	case modeBot:
		return runBotMode()
	case modeWorker:
		return runWorkerMode(cmd.workerJob)
	default:
		return fmt.Errorf("unsupported mode: %s", cmd.mode)
	}
}

func parseCommand(args []string) (appCommand, error) {
	if len(args) == 0 {
		return appCommand{mode: modeBot}, nil
	}

	switch args[0] {
	case modeWorker:
		fs := flag.NewFlagSet(modeWorker, flag.ContinueOnError)
		fs.SetOutput(io.Discard)

		job := fs.String("job", "", "worker job to run")
		if err := fs.Parse(args[1:]); err != nil {
			return appCommand{}, fmt.Errorf("parse worker flags: %w", err)
		}
		if fs.NArg() > 0 {
			return appCommand{}, fmt.Errorf("unexpected worker arguments: %v", fs.Args())
		}
		if *job == "" {
			return appCommand{}, fmt.Errorf("worker job is required")
		}
		if *job != workerJobReminders {
			return appCommand{}, fmt.Errorf("unsupported worker job: %s", *job)
		}
		return appCommand{mode: modeWorker, workerJob: *job}, nil
	default:
		return appCommand{}, fmt.Errorf("unknown command: %s", args[0])
	}
}

func runBotMode() error {
	cfg, logger, closer, err := loadConfigAndLogger()
	if err != nil {
		return err
	}
	if closer != nil {
		defer (func(c io.Closer) { _ = c.Close() })(closer)
	}

	items, err := loadItems(&logger)
	if err != nil {
		return err
	}

	if err := prepareDirectories(cfg, &logger, true); err != nil {
		return err
	}

	db, err := initDatabase(cfg, &logger)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := syncItems(db, items, &logger); err != nil {
		logger.Error().Err(err).Msg("Ошибка синхронизации позиций")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sheetsService := initGoogleSheets(ctx, cfg, &logger)
	redisClient, stateService := initStateService(ctx, cfg, &logger)
	if redisClient != nil {
		defer redisClient.Close()
	}

	sheetsWorker := startSheetsWorker(ctx, db, sheetsService, redisClient, &logger)

	eventBus := events.NewEventBus()
	subscribeBookingEvents(ctx, eventBus, db, sheetsWorker, &logger)

	bookingService := service.NewBookingService(db, eventBus, sheetsWorker, cfg.Bot.MaxBookingDays, cfg.Bot.MinBookingAdvance, &logger)
	userService := service.NewUserService(db, cfg, &logger)
	itemService := service.NewItemService(db, &logger)
	metrics := bot.NewMetrics()

	if cfg.Backup.Enabled {
		startBackupLoop(ctx, cfg, &logger)
	}

	telegramBot, err := newApplicationBot(
		cfg,
		stateService,
		sheetsService,
		sheetsWorker,
		eventBus,
		bookingService,
		userService,
		itemService,
		metrics,
		&logger,
	)
	if err != nil {
		return err
	}

	logger.Info().Msg("Bot mode enabled: starting interactive Telegram bot")
	telegramBot.Start(ctx)
	logger.Info().Msg("Shutdown complete.")
	return nil
}

func runWorkerMode(job string) error {
	switch job {
	case workerJobReminders:
		return runReminderWorkerMode()
	default:
		return fmt.Errorf("unsupported worker job: %s", job)
	}
}

func runReminderWorkerMode() error {
	cfg, logger, closer, err := loadConfigAndLogger()
	if err != nil {
		return err
	}
	if closer != nil {
		defer (func(c io.Closer) { _ = c.Close() })(closer)
	}

	if err := prepareDirectories(cfg, &logger, false); err != nil {
		return err
	}

	db, err := initDatabase(cfg, &logger)
	if err != nil {
		return err
	}
	defer db.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	bookingService := service.NewBookingService(db, nil, nil, cfg.Bot.MaxBookingDays, cfg.Bot.MinBookingAdvance, &logger)
	userService := service.NewUserService(db, cfg, &logger)

	telegramBot, err := newReminderBot(cfg, bookingService, userService, &logger)
	if err != nil {
		return err
	}

	logger.Info().Str("job", workerJobReminders).Msg("Worker mode enabled: starting single worker job")
	telegramBot.StartReminders(ctx)
	<-ctx.Done()
	logger.Info().Str("job", workerJobReminders).Msg("Worker shutdown complete.")
	return nil
}

func loadConfigAndLogger() (*config.Config, zerolog.Logger, io.Closer, error) {
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = defaultConfigPath
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, zerolog.Logger{}, nil, err
	}

	baseLogger, closer, err := logging.New(cfg.Logging, cfg.App)
	if err != nil {
		return nil, zerolog.Logger{}, nil, err
	}
	logger := baseLogger.With().Str("component", "bot-main").Logger()

	return cfg, logger, closer, nil
}

func loadItems(logger *zerolog.Logger) ([]models.Item, error) {
	itemsPath := os.Getenv("ITEMS_PATH")
	if itemsPath == "" {
		itemsPath = defaultItemsPath
	}

	itemsData, err := os.ReadFile(itemsPath)
	if err != nil {
		logger.Error().Err(err).Msgf("Ошибка чтения %s", itemsPath)
		return nil, err
	}

	var itemsConfig struct {
		Items []models.Item `yaml:"items"`
	}
	if err := yaml.Unmarshal(itemsData, &itemsConfig); err != nil {
		logger.Error().Err(err).Msg("Ошибка парсинга items.yaml")
		return nil, err
	}

	if err := config.ValidateItems(itemsConfig.Items); err != nil {
		logger.Error().Err(err).Msg("Items validation failed")
		return nil, err
	}

	return itemsConfig.Items, nil
}

func prepareDirectories(cfg *config.Config, logger *zerolog.Logger, withExports bool) error {
	if cfg == nil {
		return os.ErrInvalid
	}
	if !cfg.UsePostgres() {
		if err := os.MkdirAll(filepath.Dir(cfg.Database.Path), 0o755); err != nil {
			logger.Error().Err(err).Msg("Ошибка создания директории для базы данных")
			return err
		}
	}
	if withExports {
		if err := os.MkdirAll(cfg.Exports.Path, 0o755); err != nil {
			logger.Error().Err(err).Msg("Ошибка создания директории для экспорта")
			return err
		}
	}
	return nil
}

func initDatabase(cfg *config.Config, logger *zerolog.Logger) (*database.DB, error) {
	var (
		db  *database.DB
		err error
	)
	if cfg.UsePostgres() {
		db, err = database.NewDBWithDriver(cfg.DatabaseDriver(), cfg.Database.Path, cfg.PostgresDSN(), logger)
	} else {
		db, err = database.NewDB(cfg.Database.Path, logger)
	}
	if err != nil {
		logger.Error().Err(err).Msg("Ошибка инициализации базы данных")
		return nil, err
	}
	return db, nil
}

func syncItems(db *database.DB, items []models.Item, logger *zerolog.Logger) error {
	if db == nil {
		return os.ErrInvalid
	}
	if err := db.SyncItems(context.Background(), items); err != nil {
		logger.Error().Err(err).Msg("Ошибка синхронизации позиций")
		return err
	}
	return nil
}

func initGoogleSheets(ctx context.Context, cfg *config.Config, logger *zerolog.Logger) *google.SheetsService {
	if cfg.Google.GoogleCredentialsFile == "" || cfg.Google.UsersSpreadSheetID == "" || cfg.Google.BookingSpreadSheetID == "" {
		logger.Warn().Msg("Google Sheets is not configured; continuing without sheets integration")
		return nil
	}

	sheetsSvc, err := google.NewSimpleSheetsService(
		cfg.Google.GoogleCredentialsFile,
		cfg.Google.UsersSpreadSheetID,
		cfg.Google.BookingSpreadSheetID,
	)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to initialize Google Sheets service")
		return nil
	}

	if err := sheetsSvc.TestConnection(ctx); err != nil {
		logger.Warn().Err(err).Msg("Google Sheets connection test failed; continuing without sheets integration")
		return nil
	}

	logger.Info().Msg("Google Sheets service initialized successfully")
	return sheetsSvc
}

func initStateService(ctx context.Context, cfg *config.Config, logger *zerolog.Logger) (*redis.Client, *service.StateService) {
	var redisClient *redis.Client
	if cfg.Redis.Address != "" {
		redisClient = repository.NewRedisClient(cfg.Redis)
		if errPing := repository.Ping(ctx, redisClient); errPing != nil {
			logger.Warn().Err(errPing).Msg("Redis unavailable")
		}
	}

	primaryRepo := repository.NewRedisStateRepository(redisClient, time.Duration(models.DefaultRedisTTL)*time.Second)
	fallbackRepo := repository.NewMemoryStateRepository(time.Duration(models.DefaultRedisTTL) * time.Second)
	stateRepo := repository.NewFailoverStateRepository(primaryRepo, fallbackRepo, logger)
	return redisClient, service.NewStateService(stateRepo, logger)
}

func startSheetsWorker(
	ctx context.Context,
	db *database.DB,
	sheetsService *google.SheetsService,
	redisClient *redis.Client,
	logger *zerolog.Logger,
) *worker.SheetsWorker {
	if sheetsService == nil {
		return nil
	}

	retryPolicy := worker.RetryPolicy{
		MaxRetries:    5,
		InitialDelay:  2 * time.Second,
		MaxDelay:      time.Minute,
		BackoffFactor: 2,
	}
	sheetsWorker := worker.NewSheetsWorker(db, sheetsService, redisClient, retryPolicy, logger)
	go sheetsWorker.Start(ctx)
	return sheetsWorker
}

func startBackupLoop(ctx context.Context, cfg *config.Config, logger *zerolog.Logger) {
	backupService := database.NewBackupServiceWithDriver(
		cfg.DatabaseDriver(),
		cfg.Database.Path,
		cfg.Backup,
		logger,
	)
	go backupService.Start(ctx)
}

func newApplicationBot(
	cfg *config.Config,
	stateService *service.StateService,
	sheetsService *google.SheetsService,
	sheetsWorker *worker.SheetsWorker,
	eventBus *events.EventBus,
	bookingService *service.BookingService,
	userService *service.UserService,
	itemService *service.ItemService,
	metrics *bot.Metrics,
	logger *zerolog.Logger,
) (*bot.Bot, error) {
	tgService, err := initTelegramService(cfg, logger)
	if err != nil {
		return nil, err
	}

	return bot.NewBot(
		tgService,
		cfg,
		stateService,
		sheetsService,
		sheetsWorker,
		eventBus,
		bookingService,
		userService,
		itemService,
		metrics,
		logger,
	)
}

func newReminderBot(
	cfg *config.Config,
	bookingService *service.BookingService,
	userService *service.UserService,
	logger *zerolog.Logger,
) (*bot.Bot, error) {
	tgService, err := initTelegramService(cfg, logger)
	if err != nil {
		return nil, err
	}

	return bot.NewBot(
		tgService,
		cfg,
		nil,
		nil,
		nil,
		nil,
		bookingService,
		userService,
		nil,
		nil,
		logger,
	)
}

func initTelegramService(cfg *config.Config, logger *zerolog.Logger) (*service.TelegramService, error) {
	if cfg.Telegram.BotToken == "YOUR_BOT_TOKEN_HERE" || cfg.Telegram.BotToken == "" {
		logger.Error().Msg("Задайте токен бота в config.yaml")
		return nil, os.ErrInvalid
	}

	botAPI, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		logger.Error().Err(err).Msg("Ошибка создания BotAPI")
		return nil, err
	}

	botWrapper := bot.NewBotWrapper(botAPI)
	return service.NewTelegramService(botWrapper), nil
}

func subscribeBookingEvents(
	ctx context.Context,
	bus *events.EventBus,
	db *database.DB,
	sheetsWorker *worker.SheetsWorker,
	logger *zerolog.Logger,
) {
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

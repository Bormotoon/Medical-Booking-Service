package bot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	crmapi "bronivik/bronivik_crm/internal/crmapi"
	"bronivik/bronivik_crm/internal/db"
	"bronivik/bronivik_crm/internal/metrics"
	"bronivik/bronivik_crm/internal/model"
	slotutil "bronivik/bronivik_crm/internal/slots"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
)

type telegramClient interface {
	Send(tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	GetUpdatesChan(tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	SelfUser() tgbotapi.User
}

type realTelegramClient struct {
	api *tgbotapi.BotAPI
}

func (c *realTelegramClient) Send(msg tgbotapi.Chattable) (tgbotapi.Message, error) {
	return c.api.Send(msg)
}

func (c *realTelegramClient) Request(msg tgbotapi.Chattable) (*tgbotapi.APIResponse, error) {
	return c.api.Request(msg)
}

func (c *realTelegramClient) GetUpdatesChan(cfg tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return c.api.GetUpdatesChan(cfg)
}

func (c *realTelegramClient) SelfUser() tgbotapi.User {
	return c.api.Self
}

const itemNone = "Без аппарата"

const helpText = "Доступные команды: /book, /my_bookings, /help"

// Bot is a thin Telegram bot wrapper for CRM flow.
type Bot struct {
	api        *crmapi.BronivikClient
	apiEnabled bool
	db         *db.DB
	managers   map[int64]struct{}
	tg         telegramClient
	state      *stateStore
	rules      *BookingRules
	logger     *zerolog.Logger
}

var errActiveLimit = errors.New("active bookings limit reached")

type BookingRules struct {
	MinAdvance       time.Duration
	MaxAdvance       time.Duration
	MaxActivePerUser int
}

func New(
	token string,
	apiClient *crmapi.BronivikClient,
	apiEnabled bool,
	db *db.DB,
	managers []int64,
	rules *BookingRules,
	logger *zerolog.Logger,
) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return newBot(&realTelegramClient{api: api}, apiClient, apiEnabled, db, managers, rules, logger)
}

// NewWithTelegramClient allows injecting a mocked Telegram client for tests.
func NewWithTelegramClient(
	tg telegramClient,
	apiClient *crmapi.BronivikClient,
	apiEnabled bool,
	db *db.DB,
	managers []int64,
	rules *BookingRules,
	logger *zerolog.Logger,
) (*Bot, error) {
	return newBot(tg, apiClient, apiEnabled, db, managers, rules, logger)
}

func newBot(
	tg telegramClient,
	apiClient *crmapi.BronivikClient,
	apiEnabled bool,
	db *db.DB,
	managers []int64,
	rules *BookingRules,
	logger *zerolog.Logger,
) (*Bot, error) {
	if tg == nil {
		return nil, fmt.Errorf("telegram client is nil")
	}
	mgrs := make(map[int64]struct{})
	for _, id := range managers {
		mgrs[id] = struct{}{}
	}
	if rules.MinAdvance <= 0 {
		rules.MinAdvance = 60 * time.Minute
	}
	if rules.MaxAdvance <= 0 {
		rules.MaxAdvance = 30 * 24 * time.Hour
	}
	if rules.MaxActivePerUser < 0 {
		rules.MaxActivePerUser = 0
	}
	return &Bot{
		api:        apiClient,
		apiEnabled: apiEnabled,
		db:         db,
		managers:   mgrs,
		tg:         tg,
		state:      newStateStore(),
		rules:      rules,
		logger:     logger,
	}, nil
}

// Start begins polling updates and handles commands.
var (
	mainMenu = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("🗓 Записаться"),
			tgbotapi.NewKeyboardButton("📌 Мои записи"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("ℹ️ Помощь"),
		),
	)

	managerMenu = tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📥 Заявки"),
			tgbotapi.NewKeyboardButton("➕ Создать запись"),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("📅 Расписание"),
			tgbotapi.NewKeyboardButton("⚙️ Админка"),
		),
	)
)

func (b *Bot) sendMainMenu(chatID int64, userID int64) {
	msg := tgbotapi.NewMessage(chatID, "Выберите действие:")
	if b.isManager(userID) {
		msg.ReplyMarkup = managerMenu
	} else {
		msg.ReplyMarkup = mainMenu
	}
	_, _ = b.tg.Send(msg)
}

func (b *Bot) Start(ctx context.Context) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.tg.GetUpdatesChan(u)
	b.logger.Info().Str("username", b.tg.SelfUser().UserName).Msg("CRM bot authorized")

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			requestID := uuid.New().String()
			l := b.logger.With().Str("request_id", requestID).Logger()
			updateCtx := l.WithContext(ctx)
			b.handleUpdate(updateCtx, &update)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update *tgbotapi.Update) {
	l := zerolog.Ctx(ctx)
	if update.CallbackQuery != nil {
		l.Debug().
			Int64("user_id", update.CallbackQuery.From.ID).
			Str("data", update.CallbackQuery.Data).
			Msg("Handling callback query")
		b.handleCallback(ctx, update.CallbackQuery)
		return
	}
	if update.Message != nil {
		l.Debug().
			Int64("user_id", update.Message.From.ID).
			Str("text", update.Message.Text).
			Msg("Handling message")
		b.handleMessage(ctx, update.Message)
		return
	}
}

func (b *Bot) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	if msg == nil {
		return
	}
	text := strings.TrimSpace(msg.Text)

	l := zerolog.Ctx(ctx)
	l.Debug().
		Int64("user_id", msg.From.ID).
		Str("text", text).
		Msg("handleMessage: start")

	// Handle keyboard button presses (non-command text)
	switch {
	case text == "🗓 Записаться":
		b.startBookingFlow(ctx, msg)
		return
	case text == "📌 Мои записи":
		b.handleMyBookings(ctx, msg)
		return
	case text == "ℹ️ Помощь":
		b.reply(msg.Chat.ID, helpText)
		return
	case text == "📥 Заявки" && b.isManager(msg.From.ID):
		b.handlePendingBookings(ctx, msg.Chat.ID)
		return
	case text == "➕ Создать запись" && b.isManager(msg.From.ID):
		b.startManualBookingFlow(ctx, msg)
		return
	case text == "📅 Расписание" && b.isManager(msg.From.ID):
		b.handleTodaySchedule(ctx, msg.Chat.ID)
		return
	case text == "⚙️ Админка" && b.isManager(msg.From.ID):
		b.sendAdminPanel(msg.Chat.ID)
		return
	}

	// Slash commands take priority and interrupt any active flow
	if strings.HasPrefix(text, "/") {
		switch {
		case strings.HasPrefix(text, "/start"):
			b.state.reset(msg.From.ID)
			b.sendMainMenu(msg.Chat.ID, msg.From.ID)
			return
		case strings.HasPrefix(text, "/help"):
			b.reply(msg.Chat.ID, helpText)
			return
		case text == "/admin" && b.isManager(msg.From.ID):
			b.sendAdminPanel(msg.Chat.ID)
			return
		case strings.HasPrefix(text, "/book"):
			b.startBookingFlow(ctx, msg)
			return
		case strings.HasPrefix(text, "/my_bookings"):
			b.handleMyBookings(ctx, msg)
			return
		case strings.HasPrefix(text, "/cancel_booking"):
			b.handleCancelBooking(ctx, msg)
			return
		case strings.HasPrefix(text, "/cancel"):
			b.state.reset(msg.From.ID)
			b.reply(msg.Chat.ID, "Операция отменена.")
			b.sendMainMenu(msg.Chat.ID, msg.From.ID)
			return
		}

		if b.isManager(msg.From.ID) {
			if b.handleManagerCommands(msg) {
				return
			}
		}
	}

	st := b.state.get(msg.From.ID)
	switch st.Step {
	case stepClientName:
		st.Draft.ClientName = text
		st.Step = stepClientPhone
		msg := tgbotapi.NewMessage(msg.Chat.ID, "Введите телефон клиента (в любом формате):")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", "back:name"),
			),
		)
		_, _ = b.tg.Send(msg)
		return
	case stepClientPhone:
		phone, ok := normalizeAndValidatePhone(text)
		if !ok {
			b.reply(msg.Chat.ID, "Некорректный телефон. Пример: +7 999 123-45-67")
			return
		}
		st.Draft.ClientPhone = phone
		st.Step = stepConfirm
		b.sendConfirm(msg.Chat.ID, msg.From.ID)
		return
	}
}

func (b *Bot) handleCallback(ctx context.Context, cq *tgbotapi.CallbackQuery) {
	if cq == nil {
		return
	}
	data := cq.Data

	l := zerolog.Ctx(ctx)
	l.Debug().Int64("user_id", cq.From.ID).Str("data", data).Msg("handleCallback: start")

	_ = b.answerCallback(cq.ID)
	if data == "noop" {
		return
	}

	userID := cq.From.ID
	chatID := cq.Message.Chat.ID
	st := b.state.get(userID)

	switch {
	case strings.HasPrefix(data, "cab:"):
		b.handleCabCallback(ctx, chatID, userID, st, data)
	case strings.HasPrefix(data, "item:"):
		b.handleItemCallback(ctx, chatID, st, data)
	case strings.HasPrefix(data, "date:"):
		b.handleDateCallback(ctx, chatID, userID, st, data)
	case strings.HasPrefix(data, "back:"):
		b.handleBack(ctx, chatID, userID, st, data)
	case strings.HasPrefix(data, "slot:"):
		b.handleSlotCallback(ctx, chatID, userID, st, data)
	case strings.HasPrefix(data, "dur:"):
		b.handleDurationCallback(ctx, chatID, userID, st, data)
	case data == "confirm":
		b.handleConfirmCallback(ctx, chatID, userID, cq, st)
	case data == "cancel":
		b.handleCancelCallback(chatID, userID)
	case strings.HasPrefix(data, "mgr:"):
		b.handleManagerDecision(ctx, chatID, userID, data)
	}
}

func (b *Bot) handleCabCallback(ctx context.Context, chatID int64, _userID int64, st *userState, data string) {
	idStr := strings.TrimPrefix(data, "cab:")
	cabID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		b.reply(chatID, "Некорректный кабинет")
		return
	}
	cab, err := b.db.GetCabinet(ctx, cabID)
	if err != nil {
		b.reply(chatID, "Не удалось загрузить кабинет")
		return
	}
	st.Draft.CabinetID = cabID
	st.Draft.CabinetName = cab.Name
	st.Step = stepDate
	b.sendCalendar(chatID)
}

func (b *Bot) handleItemCallback(_ctx context.Context, chatID int64, st *userState, data string) {
	name := strings.TrimPrefix(data, "item:")
	if name == "none" {
		name = ""
	}
	st.Draft.ItemName = name
	st.Step = stepClientName
	msg := tgbotapi.NewMessage(chatID, "Введите ФИО клиента:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", "back:item"),
		),
	)
	_, _ = b.tg.Send(msg)
}

func (b *Bot) handleBack(ctx context.Context, chatID, userID int64, st *userState, data string) {
	step := strings.TrimPrefix(data, "back:")
	switch step {
	case "cab":
		st.Step = stepCabinet
		b.sendCabinets(ctx, chatID)
	case "date":
		st.Step = stepDate
		b.sendCalendar(chatID)
	case "time":
		st.Step = stepTime
		b.sendTimeSlots(ctx, chatID, userID)
	case "duration":
		st.Step = stepDuration
		b.sendDurations(ctx, chatID, userID)
	case "item":
		st.Step = stepItem
		b.sendItems(ctx, chatID, st.Draft.Date)
	case "name":
		st.Step = stepClientName
		msg := tgbotapi.NewMessage(chatID, "Введите ФИО клиента:")
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", "back:item"),
			),
		)
		_, _ = b.tg.Send(msg)
	default:
		b.startBookingFlow(ctx, &tgbotapi.Message{From: &tgbotapi.User{ID: userID}, Chat: &tgbotapi.Chat{ID: chatID}})
	}
}

func (b *Bot) handleDateCallback(ctx context.Context, chatID, userID int64, st *userState, data string) {
	dateStr := strings.TrimPrefix(data, "date:")
	st.Draft.Date = dateStr
	st.Step = stepTime
	b.sendTimeSlots(ctx, chatID, userID)
}

func (b *Bot) handleSlotCallback(ctx context.Context, chatID, userID int64, st *userState, data string) {
	label := strings.TrimPrefix(data, "slot:")
	if st.Draft.Date == "" {
		b.reply(chatID, "Сначала выберите дату")
		return
	}
	date, err := time.Parse("2006-01-02", st.Draft.Date)
	if err != nil {
		b.reply(chatID, "Некорректная дата")
		return
	}
	start, _, err := parseTimeLabel(date, label)
	if err != nil {
		b.reply(chatID, "Некорректный слот")
		return
	}
	if vErr := b.validateBookingTime(start); vErr != nil {
		b.reply(chatID, vErr.Error())
		b.sendTimeSlots(ctx, chatID, userID)
		return
	}

	st.Draft.StartTime = start.Format("15:04")
	st.Step = stepDuration
	b.sendDurations(ctx, chatID, userID)
}

func (b *Bot) handleDurationCallback(ctx context.Context, chatID int64, userID int64, st *userState, data string) {
	durStr := strings.TrimPrefix(data, "dur:")
	dur, err := strconv.Atoi(durStr)
	if err != nil {
		b.reply(chatID, "Некорректная длительность")
		return
	}

	date, _ := time.Parse("2006-01-02", st.Draft.Date)
	start, _ := time.Parse("15:04", st.Draft.StartTime)
	startDT := time.Date(date.Year(), date.Month(), date.Day(), start.Hour(), start.Minute(), 0, 0, time.Local)
	endDT := startDT.Add(time.Duration(dur) * time.Minute)

	options, err := b.availableDurationOptions(ctx, st.Draft.CabinetID, date, startDT)
	if err != nil {
		b.reply(chatID, "Не удалось получить доступные длительности")
		return
	}
	if !containsDuration(options, dur) {
		b.reply(chatID, "Выбранная длительность недоступна для этого времени. Выберите одну из предложенных.")
		b.sendDurations(ctx, chatID, userID)
		return
	}

	// Final availability check for the whole range
	ok, err := b.db.CheckSlotAvailability(ctx, st.Draft.CabinetID, date, startDT, endDT)
	if err != nil {
		b.reply(chatID, "Не удалось проверить доступность")
		return
	}
	if !ok {
		b.reply(chatID, "Выбранный период времени занят. Выберите меньшую длительность или другое время.")
		b.sendDurations(ctx, chatID, userID)
		return
	}

	st.Draft.Duration = dur
	st.Draft.TimeLabel = fmt.Sprintf("%s-%s", st.Draft.StartTime, endDT.Format("15:04"))
	st.Step = stepItem
	b.sendItems(ctx, chatID, st.Draft.Date)
}

func (b *Bot) sendDurations(ctx context.Context, chatID, userID int64) {
	st := b.state.get(userID)
	if st.Draft.CabinetID == 0 || st.Draft.Date == "" || st.Draft.StartTime == "" {
		b.reply(chatID, "Сначала выберите время")
		return
	}

	date, err := time.Parse("2006-01-02", st.Draft.Date)
	if err != nil {
		b.reply(chatID, "Некорректная дата")
		return
	}
	start, err := time.Parse("15:04", st.Draft.StartTime)
	if err != nil {
		b.reply(chatID, "Некорректное время")
		return
	}
	startDT := time.Date(date.Year(), date.Month(), date.Day(), start.Hour(), start.Minute(), 0, 0, time.Local)

	durations, err := b.availableDurationOptions(ctx, st.Draft.CabinetID, date, startDT)
	if err != nil {
		b.reply(chatID, "Не удалось получить доступные длительности")
		return
	}
	if len(durations) == 0 {
		b.reply(chatID, "Для выбранного времени нет доступных длительностей. Выберите другой слот.")
		st.Step = stepTime
		b.sendTimeSlots(ctx, chatID, userID)
		return
	}

	rows := make([][]tgbotapi.InlineKeyboardButton, 0)
	for _, d := range durations {
		label := slotutil.FormatDuration(d)
		rows = append(rows, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("dur:%d", d)),
		})
	}
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", "back:time"),
	})

	msg := tgbotapi.NewMessage(chatID, "Выберите длительность приема:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	_, _ = b.tg.Send(msg)
}

func (b *Bot) availableDurationOptions(ctx context.Context, cabinetID int64, date, startTime time.Time) ([]int, error) {
	slots, err := b.db.GetAvailableSlots(ctx, cabinetID, date)
	if err != nil {
		return nil, err
	}

	_, _, slotDuration, err := b.db.GetScheduleWindow(ctx, cabinetID, date)
	if err != nil {
		return nil, err
	}
	if slotDuration <= 0 {
		slotDuration = 60
	}

	converted, err := convertDBSlots(date, slots)
	if err != nil {
		return nil, err
	}

	options := slotutil.GetDurationOptions(converted, startTime, slotDuration)
	if len(options) == 0 {
		return nil, nil
	}

	filtered := make([]int, 0, len(options))
	for _, duration := range options {
		if duration > 180 {
			break
		}
		filtered = append(filtered, duration)
	}

	return filtered, nil
}

func convertDBSlots(date time.Time, slots []db.TimeSlot) ([]slotutil.Slot, error) {
	converted := make([]slotutil.Slot, 0, len(slots))
	for _, slot := range slots {
		start, end, err := parseTimeLabel(date, slot.StartTime+"-"+slot.EndTime)
		if err != nil {
			return nil, err
		}
		converted = append(converted, slotutil.Slot{
			StartTime: start,
			EndTime:   end,
			Available: slot.Available,
		})
	}
	return converted, nil
}

func containsDuration(options []int, duration int) bool {
	for _, option := range options {
		if option == duration {
			return true
		}
	}
	return false
}

func (b *Bot) handleConfirmCallback(ctx context.Context, chatID, userID int64, cq *tgbotapi.CallbackQuery, st *userState) {
	if st.Step != stepConfirm {
		b.reply(chatID, "Сценарий устарел, начните заново: /book")
		return
	}
	if err := b.finalizeBooking(ctx, cq, st); err != nil {
		if errors.Is(err, db.ErrSlotNotAvailable) {
			b.reply(chatID, "Слот уже занят. Выберите другое время.")
			st.Step = stepTime
			b.sendTimeSlots(ctx, chatID, userID)
			return
		}
		if errors.Is(err, db.ErrItemNotAvailable) {
			b.reply(chatID, "Аппарат недоступен на эту дату. Выберите другой аппарат или 'Без аппарата'.")
			st.Step = stepItem
			b.sendItems(ctx, chatID, st.Draft.Date)
			return
		}
		if errors.Is(err, db.ErrSlotMisaligned) {
			b.reply(chatID, "Слот не совпадает с расписанием. Выберите другое время.")
			st.Step = stepTime
			b.sendTimeSlots(ctx, chatID, userID)
			return
		}
		if errors.Is(err, errActiveLimit) {
			b.reply(chatID, "Достигнут лимит активных бронирований. Отмените существующее или свяжитесь с менеджером.")
			return
		}
		b.reply(chatID, "Не удалось создать бронирование")
		return
	}
	b.state.reset(userID)
}

func (b *Bot) handleCancelCallback(chatID, userID int64) {
	b.state.reset(userID)
	b.reply(chatID, "Ок, отменено. /book чтобы начать заново")
}

func (b *Bot) handleManagerDecision(ctx context.Context, chatID, userID int64, data string) {
	if !b.isManager(userID) {
		return
	}
	b.logger.Debug().Int64("user_id", userID).Str("data", data).Msg("handleManagerDecision: start")
	switch {
	case strings.HasPrefix(data, "mgr:approve:"):
		idStr := strings.TrimPrefix(data, "mgr:approve:")
		bid, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return
		}
		_ = b.db.UpdateHourlyBookingStatus(ctx, bid, "approved", "")
		metrics.IncManagerDecision("approved")
		b.reply(chatID, fmt.Sprintf("Бронирование #%d подтверждено", bid))
		b.notifyBookingStatus(ctx, bid, "approved")
	case strings.HasPrefix(data, "mgr:reject:"):
		idStr := strings.TrimPrefix(data, "mgr:reject:")
		bid, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return
		}
		_ = b.db.UpdateHourlyBookingStatus(ctx, bid, "rejected", "")
		metrics.IncManagerDecision("rejected")
		b.reply(chatID, fmt.Sprintf("Бронирование #%d отклонено", bid))
		b.notifyBookingStatus(ctx, bid, "rejected")
	}
}

func (b *Bot) validateBookingTime(start time.Time) error {
	now := time.Now()
	if start.Before(now.Add(b.rules.MinAdvance)) {
		minMins := int(b.rules.MinAdvance.Minutes())
		return fmt.Errorf("Слишком близко по времени. Минимум за %d минут до начала.", minMins)
	}
	if start.After(now.Add(b.rules.MaxAdvance)) {
		days := int(b.rules.MaxAdvance.Hours() / 24)
		if days <= 0 {
			days = 30
		}
		return fmt.Errorf("Слишком далеко по времени. Доступно бронирование максимум на %d дней вперед.", days)
	}
	return nil
}

func (b *Bot) handleManagerCommands(msg *tgbotapi.Message) bool {
	text := msg.Text
	switch {
	case strings.HasPrefix(text, "/add_cabinet"):
		b.reply(msg.Chat.ID, "(stub) Добавить кабинет")
	case strings.HasPrefix(text, "/list_cabinets"):
		b.reply(msg.Chat.ID, "(stub) Список кабинетов")
	case strings.HasPrefix(text, "/cabinet_schedule"):
		b.reply(msg.Chat.ID, "(stub) Расписание кабинета")
	case strings.HasPrefix(text, "/set_schedule"):
		b.reply(msg.Chat.ID, "(stub) Установить расписание")
	case strings.HasPrefix(text, "/close_cabinet"):
		b.reply(msg.Chat.ID, "(stub) Закрыть кабинет на дату")
	case strings.HasPrefix(text, "/pending"):
		b.reply(msg.Chat.ID, "(stub) Ожидающие подтверждения")
	case strings.HasPrefix(text, "/approve"):
		b.reply(msg.Chat.ID, "(stub) Подтвердить бронирование")
	case strings.HasPrefix(text, "/reject"):
		b.reply(msg.Chat.ID, "(stub) Отклонить бронирование")
	case strings.HasPrefix(text, "/today_schedule"):
		b.reply(msg.Chat.ID, "(stub) Расписание на сегодня")
	case strings.HasPrefix(text, "/tomorrow_schedule"):
		b.reply(msg.Chat.ID, "(stub) Расписание на завтра")
	default:
		return false
	}
	return true
}

func (b *Bot) handleMyBookings(ctx context.Context, msg *tgbotapi.Message) {
	if msg == nil || msg.From == nil {
		return
	}
	b.logger.Debug().Int64("user_id", msg.From.ID).Msg("handleMyBookings: start")
	u, err := b.db.GetOrCreateUserByTelegramID(ctx, msg.From.ID, msg.From.UserName, msg.From.FirstName, msg.From.LastName, "")
	if err != nil {
		b.reply(msg.Chat.ID, "Не удалось загрузить пользователя")
		return
	}

	bookings, err := b.db.ListUserBookings(ctx, u.ID, 10, false)
	if err != nil {
		b.reply(msg.Chat.ID, "Не удалось получить бронирования")
		return
	}
	if len(bookings) == 0 {
		b.reply(msg.Chat.ID, "У вас нет активных бронирований")
		return
	}

	var sb strings.Builder
	sb.WriteString("Ваши бронирования:\n")
	for i := range bookings {
		bk := &bookings[i]
		cabName := fmt.Sprintf("Кабинет #%d", bk.CabinetID)
		if cab, err := b.db.GetCabinet(ctx, bk.CabinetID); err == nil && cab != nil {
			cabName = cab.Name
		}
		item := bk.ItemName
		if item == "" {
			item = itemNone
		}
		line := fmt.Sprintf("#%d %s %s-%s | %s | %s | %s\n",
			bk.ID,
			bk.StartTime.Format("02.01"),
			bk.StartTime.Format("15:04"),
			bk.EndTime.Format("15:04"),
			cabName,
			item,
			bk.Status,
		)
		sb.WriteString(line)
	}
	b.reply(msg.Chat.ID, sb.String())
}

func (b *Bot) handleCancelBooking(ctx context.Context, msg *tgbotapi.Message) {
	if msg == nil || msg.From == nil {
		return
	}
	b.logger.Debug().Int64("user_id", msg.From.ID).Str("text", msg.Text).Msg("handleCancelBooking: start")
	parts := strings.Fields(msg.Text)
	if len(parts) < 2 {
		b.reply(msg.Chat.ID, "Формат: /cancel_booking <id>")
		return
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		b.reply(msg.Chat.ID, "Некорректный id бронирования")
		return
	}

	u, err := b.db.GetOrCreateUserByTelegramID(ctx, msg.From.ID, msg.From.UserName, msg.From.FirstName, msg.From.LastName, "")
	if err != nil {
		b.reply(msg.Chat.ID, "Не удалось загрузить пользователя")
		return
	}

	switch err := b.db.CancelUserBooking(ctx, id, u.ID); {
	case err == nil:
		b.reply(msg.Chat.ID, fmt.Sprintf("Бронирование #%d отменено", id))
		metrics.IncBookingCanceled()
	case errors.Is(err, db.ErrBookingNotFound):
		b.reply(msg.Chat.ID, "Бронирование не найдено")
	case errors.Is(err, db.ErrBookingForbidden):
		b.reply(msg.Chat.ID, "Нельзя отменить чужое бронирование")
	case errors.Is(err, db.ErrBookingTooLate):
		b.reply(msg.Chat.ID, "Нельзя отменить уже начавшееся бронирование")
	case errors.Is(err, db.ErrBookingFinalized):
		b.reply(msg.Chat.ID, "Бронирование уже завершено или отменено")
	default:
		b.reply(msg.Chat.ID, "Не удалось отменить бронирование")
	}
}

func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, _ = b.tg.Send(msg)
}

func (b *Bot) handlePendingBookings(ctx context.Context, chatID int64) {
	b.logger.Debug().Int64("chat_id", chatID).Msg("handlePendingBookings: start")
	bookings, err := b.db.ListPendingBookings(ctx)
	if err != nil {
		b.reply(chatID, "Ошибка получения заявок")
		return
	}
	if len(bookings) == 0 {
		b.reply(chatID, "Нет новых заявок")
		return
	}

	for _, bk := range bookings {
		text := b.formatBookingInfo(bk)
		b.sendManagerDecisionMessage(chatID, bk.ID, text)
	}
}

func (b *Bot) startManualBookingFlow(ctx context.Context, msg *tgbotapi.Message) {
	b.state.reset(msg.From.ID)
	st := b.state.get(msg.From.ID)
	st.IsManual = true
	st.Step = stepCabinet
	b.sendCabinets(ctx, msg.Chat.ID)
}

func (b *Bot) handleTodaySchedule(ctx context.Context, chatID int64) {
	b.logger.Debug().Int64("chat_id", chatID).Msg("handleTodaySchedule: start")
	now := time.Now().Format("2006-01-02")
	bookings, err := b.db.ListBookingsByDate(ctx, now)
	if err != nil {
		b.reply(chatID, "Ошибка получения расписания")
		return
	}
	if len(bookings) == 0 {
		b.reply(chatID, "Сегодня ( "+now+" ) записей нет")
		return
	}

	var sb strings.Builder
	sb.WriteString("🗓 Расписание на " + now + ":\n\n")
	for _, bk := range bookings {
		timeRange := fmt.Sprintf("%s-%s", bk.StartTime.Format("15:04"), bk.EndTime.Format("15:04"))
		sb.WriteString(fmt.Sprintf("🔹 %s | %s | %s | %s\n", timeRange, bk.CabinetName, bk.ClientName, bk.Status))
	}
	b.reply(chatID, sb.String())
}

func (b *Bot) sendAdminPanel(chatID int64) {
	text := "⚙️ Панель управления (Admin Panel)\n\n" +
		"/add_cabinet - Добавить кабинет\n" +
		"/list_cabinets - Список всех кабинетов\n" +
		"/cabinet_schedule <id> - Просмотр расписания\n" +
		"/close_cabinet <id> <date> - Закрыть кабинет\n"
	b.reply(chatID, text)
}

func (b *Bot) formatBookingInfo(bk model.HourlyBooking) string {
	item := bk.ItemName
	if item == "" {
		item = "Без аппарата"
	}
	return fmt.Sprintf(
		"🆕 ЗАЯВКА #%d\n"+
			"🚪 Кабинет: %s\n"+
			"📅 Дата: %s\n"+
			"⏱ Время: %s\n"+
			"🛠 Аппарат: %s\n"+
			"👤 Клиент: %s\n"+
			"📞 Телефон: %s\n"+
			"💬 Коммент: %s",
		bk.ID, bk.CabinetName, bk.StartTime.Format("2006-01-02"),
		fmt.Sprintf("%s-%s", bk.StartTime.Format("15:04"), bk.EndTime.Format("15:04")),
		item, bk.ClientName, bk.ClientPhone, bk.Comment,
	)
}

func (b *Bot) sendManagerDecisionMessage(chatID int64, bookingID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("✅ Подтвердить", fmt.Sprintf("mgr:approve:%d", bookingID)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отклонить", fmt.Sprintf("mgr:reject:%d", bookingID)),
		),
	)
	_, _ = b.tg.Send(msg)
}

func (b *Bot) sendCabinets(ctx context.Context, chatID int64) {
	cabs, err := b.db.ListActiveCabinets(ctx)
	if err != nil {
		b.reply(chatID, "Не удалось загрузить кабинеты")
		return
	}

	if len(cabs) == 0 {
		b.reply(chatID, "Нет доступных кабинетов")
		return
	}

	var rows [][]tgbotapi.InlineKeyboardButton
	for _, cab := range cabs {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(cab.Name, fmt.Sprintf("cab:%d", cab.ID)),
		))
	}

	msg := tgbotapi.NewMessage(chatID, "Выберите кабинет:")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
	_, _ = b.tg.Send(msg)
}

func (b *Bot) isManager(id int64) bool {
	_, ok := b.managers[id]
	return ok
}

func (b *Bot) answerCallback(id string) error {
	_, err := b.tg.Request(tgbotapi.NewCallback(id, ""))
	return err
}

func (b *Bot) startBookingFlow(ctx context.Context, msg *tgbotapi.Message) {
	if msg == nil {
		return
	}
	b.logger.Debug().Int64("user_id", msg.From.ID).Msg("startBookingFlow: start")
	b.state.reset(msg.From.ID)
	st := b.state.get(msg.From.ID)
	st.Step = stepCabinet
	b.sendCabinets(ctx, msg.Chat.ID)
}

func (b *Bot) sendItems(ctx context.Context, chatID int64, dateStr string) {
	rows := [][]tgbotapi.InlineKeyboardButton{
		{tgbotapi.NewInlineKeyboardButtonData(itemNone, "item:none")},
	}
	if b.apiEnabled && b.api != nil {
		apiCtx := ctx
		if apiCtx == nil {
			apiCtx = context.Background()
		}
		apiCtx, cancel := context.WithTimeout(apiCtx, 5*time.Second)
		defer cancel()

		items, err := b.api.ListItems(apiCtx)
		if err == nil {
			for _, it := range items {
				avail, availErr := b.api.GetAvailability(apiCtx, it.Name, dateStr)
				status := ""
				if availErr == nil && avail != nil {
					if avail.Available {
						status = "✅ Свободен"
					} else {
						status = "❌ Занят"
					}
				}

				label := it.Name
				if status != "" {
					label = fmt.Sprintf("%s (%s)", it.Name, status)
				}

				rows = append(rows, []tgbotapi.InlineKeyboardButton{
					tgbotapi.NewInlineKeyboardButtonData(label, fmt.Sprintf("item:%s", it.Name)),
				})
			}
		} else {
			b.logger.Warn().Err(err).Msg("failed to list items from API")
		}
	}
	rows = append(rows, []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", "back:duration"),
	})

	out := tgbotapi.NewMessage(chatID, fmt.Sprintf("Выберите аппарат на %s:", dateStr))
	if b.apiEnabled && b.api != nil && len(rows) <= 2 { // none + back
		out.Text = "⚠️ Внешняя система недоступна, список аппаратов может быть неполным.\n\n" + out.Text
	}
	out.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	_, _ = b.tg.Send(out)
}

func (b *Bot) sendCalendar(chatID int64) {
	now := time.Now()
	markup := GenerateCalendarKeyboard(now.Year(), int(now.Month()), nil)
	out := tgbotapi.NewMessage(chatID, "Выберите дату:")
	out.ReplyMarkup = markup
	_, _ = b.tg.Send(out)
}

func (b *Bot) sendTimeSlots(ctx context.Context, chatID, userID int64) {
	st := b.state.get(userID)
	if st.Draft.CabinetID == 0 || st.Draft.Date == "" {
		b.reply(chatID, "Сначала выберите кабинет и дату: /book")
		return
	}
	date, err := time.Parse("2006-01-02", st.Draft.Date)
	if err != nil {
		b.reply(chatID, "Некорректная дата")
		return
	}
	slots, err := b.db.GetAvailableSlots(ctx, st.Draft.CabinetID, date)
	if err != nil {
		b.reply(chatID, "Не удалось получить слоты")
		return
	}

	anyAvailable := false
	ui := make([]TimeSlot, 0, len(slots))
	for _, s := range slots {
		if s.Available {
			anyAvailable = true
		}
		label := fmt.Sprintf("%s-%s", s.StartTime, s.EndTime)
		ui = append(ui, TimeSlot{Label: label, CallbackData: fmt.Sprintf("slot:%s", label), Available: s.Available})
	}

	header := "Выберите время:"
	if !anyAvailable {
		nextDate := date.AddDate(0, 0, 1)
		nextSlots, _ := b.db.GetAvailableSlots(ctx, st.Draft.CabinetID, nextDate)

		var sb strings.Builder
		sb.WriteString("⚠️ Кабинет полностью занят на выбранную дату.\n\n")
		sb.WriteString(fmt.Sprintf("Расписание на выбранную дату (%s):\n", date.Format("02.01.2006")))
		for _, s := range slots {
			status := "✅"
			if !s.Available {
				status = "❌"
			}
			sb.WriteString(fmt.Sprintf("%s %s-%s\n", status, s.StartTime, s.EndTime))
		}

		sb.WriteString(fmt.Sprintf("\nРасписание на следующий день (%s):\n", nextDate.Format("02.01.2006")))
		if len(nextSlots) == 0 {
			sb.WriteString("Нет данных или выходной.")
		} else {
			for _, s := range nextSlots {
				status := "✅"
				if !s.Available {
					status = "❌"
				}
				sb.WriteString(fmt.Sprintf("%s %s-%s\n", status, s.StartTime, s.EndTime))
			}
		}
		header = sb.String() + "\nВсе равно выберите время (для записи в очередь или другое):"
	}

	out := tgbotapi.NewMessage(chatID, header)
	out.ReplyMarkup = GenerateTimeSlotsKeyboard(ui, st.Draft.Date)
	_, _ = b.tg.Send(out)
}

func (b *Bot) sendConfirm(chatID, userID int64) {
	st := b.state.get(userID)
	item := st.Draft.ItemName
	if item == "" {
		item = itemNone
	}
	text := fmt.Sprintf("Проверьте данные:\n\nКабинет: %s\nАппарат: %s\nДата: %s\nВремя: %s\nКлиент: %s\nТелефон: %s\n\nПодтвердить?",
		st.Draft.CabinetName, item, st.Draft.Date, st.Draft.TimeLabel, st.Draft.ClientName, st.Draft.ClientPhone)

	if st.APIUnreachable {
		text = "⚠️ ВНИМАНИЕ: Внешняя система (аппараты) не отвечает. Выбранный аппарат не подтверждён автоматически — менеджер уточнит и подтвердит запись.\n\n" + text
	}

	rows := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("✅ Подтвердить", "confirm"),
			tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel"),
		},
	}
	out := tgbotapi.NewMessage(chatID, text)
	out.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
	_, _ = b.tg.Send(out)
}

func (b *Bot) finalizeBooking(ctx context.Context, cq *tgbotapi.CallbackQuery, st *userState) error {
	if cq == nil || cq.Message == nil {
		return fmt.Errorf("missing callback message")
	}
	// ensure user exists
	u, err := b.db.GetOrCreateUserByTelegramID(ctx, cq.From.ID, cq.From.UserName, cq.From.FirstName, cq.From.LastName, "")
	if err != nil {
		return err
	}

	date, err := time.Parse("2006-01-02", st.Draft.Date)
	if err != nil {
		return err
	}
	var start, end time.Time
	if start, end, err = parseTimeLabel(date, st.Draft.TimeLabel); err != nil {
		return err
	}
	if err = b.validateBookingTime(start); err != nil {
		return err
	}
	if b.rules.MaxActivePerUser > 0 {
		var count int
		count, err = b.db.CountActiveUserBookings(ctx, u.ID)
		if err != nil {
			return err
		}
		if count >= b.rules.MaxActivePerUser {
			return errActiveLimit
		}
	}

	// If API was unreachable, we skip strict API check in DB to allow booking with a warning
	apiClient := b.api
	if !b.apiEnabled || st.APIUnreachable {
		apiClient = nil
	}

	status := "pending"
	if st.IsManual {
		status = "approved"
	}

	bk := &model.HourlyBooking{
		UserID:      u.ID,
		CabinetID:   st.Draft.CabinetID,
		ItemName:    st.Draft.ItemName,
		ClientName:  st.Draft.ClientName,
		ClientPhone: st.Draft.ClientPhone,
		StartTime:   start,
		EndTime:     end,
		Status:      status,
		Comment:     "",
	}

	if err := b.db.CreateHourlyBookingWithChecks(ctx, bk, apiClient); err != nil {
		return err
	}
	metrics.IncBookingCreated(bk.Status)

	item := bk.ItemName
	if item == "" {
		item = "Без аппарата"
	}
	msg := fmt.Sprintf("Заявка #%d создана. Статус: %s. Кабинет: %s, %s %s, %s",
		bk.ID, bk.Status, st.Draft.CabinetName, st.Draft.Date, st.Draft.TimeLabel, item)
	b.reply(cq.Message.Chat.ID, msg)
	if !st.IsManual {
		b.notifyManagersNewBooking(bk.ID, st.Draft.CabinetName, item, st.Draft.Date, st.Draft.TimeLabel, st.Draft.ClientName, st.Draft.ClientPhone)
	}
	return nil
}

func parseTimeLabel(date time.Time, label string) (startDT, endDT time.Time, err error) {
	parts := strings.Split(label, "-")
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid time label")
	}
	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])
	start, err := time.Parse("15:04", startStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := time.Parse("15:04", endStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	startDT = time.Date(date.Year(), date.Month(), date.Day(), start.Hour(), start.Minute(), 0, 0, time.Local)
	endDT = time.Date(date.Year(), date.Month(), date.Day(), end.Hour(), end.Minute(), 0, 0, time.Local)
	return startDT, endDT, nil
}

func normalizeAndValidatePhone(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	// allow + and digits; strip common separators
	repl := strings.NewReplacer(" ", "", "-", "", "(", "", ")", "", "\t", "")
	s = repl.Replace(s)
	if strings.HasPrefix(s, "+") {
		s = "+" + filterDigits(s[1:])
	} else {
		s = filterDigits(s)
	}
	// very rough length check; keeps UX simple
	digits := strings.TrimPrefix(s, "+")
	if len(digits) < 10 || len(digits) > 15 {
		return "", false
	}
	if s[0] != '+' {
		// assume local; keep digits-only
		return digits, true
	}
	return s, true
}

func filterDigits(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (b *Bot) notifyManagersNewBooking(id int64, cabinet, item, date, timeLabel, clientName, clientPhone string) {
	rows := [][]tgbotapi.InlineKeyboardButton{
		{
			tgbotapi.NewInlineKeyboardButtonData("✅ Approve", fmt.Sprintf("mgr:approve:%d", id)),
			tgbotapi.NewInlineKeyboardButtonData("❌ Reject", fmt.Sprintf("mgr:reject:%d", id)),
		},
	}
	text := fmt.Sprintf("Новая заявка #%d\nКабинет: %s\nАппарат: %s\nДата: %s\nВремя: %s\nКлиент: %s\nТелефон: %s",
		id, cabinet, item, date, timeLabel, clientName, clientPhone)
	for mgrID := range b.managers {
		msg := tgbotapi.NewMessage(mgrID, text)
		msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
		_, _ = b.tg.Send(msg)
	}
}

func (b *Bot) notifyBookingStatus(ctx context.Context, bookingID int64, status string) {
	// best effort: load booking + user telegram id
	row := b.db.QueryRowContext(ctx, `
			SELECT u.telegram_id FROM hourly_bookings hb 
			JOIN users u ON u.id = hb.user_id 
			WHERE hb.id = ?`, bookingID)
	var telegramID int64
	if err := row.Scan(&telegramID); err != nil {
		return
	}
	msg := tgbotapi.NewMessage(telegramID, fmt.Sprintf("Статус заявки #%d: %s", bookingID, status))
	_, _ = b.tg.Send(msg)
}

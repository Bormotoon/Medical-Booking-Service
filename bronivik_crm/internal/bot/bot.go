package bot

import (
    "context"
    "fmt"
    "log"
    "strings"

    crmapi "bronivik/bronivik_crm/internal/api"
    "bronivik/bronivik_crm/internal/database"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot is a thin Telegram bot wrapper for CRM flow.
type Bot struct {
    api      *crmapi.BronivikClient
    db       *database.DB
    managers map[int64]struct{}
    bot      *tgbotapi.BotAPI
}

func New(token string, apiClient *crmapi.BronivikClient, db *database.DB, managers []int64) (*Bot, error) {
    b, err := tgbotapi.NewBotAPI(token)
    if err != nil {
        return nil, err
    }
    mgrs := make(map[int64]struct{})
    for _, id := range managers {
        mgrs[id] = struct{}{}
    }
    return &Bot{api: apiClient, db: db, managers: mgrs, bot: b}, nil
}

// Start begins polling updates and handles commands.
func (b *Bot) Start(ctx context.Context) {
    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60
    updates := b.bot.GetUpdatesChan(u)
    log.Printf("CRM bot authorized as %s", b.bot.Self.UserName)

    for {
        select {
        case <-ctx.Done():
            return
        case update := <-updates:
            if update.Message != nil {
                b.handleMessage(update)
            }
        }
    }
}

func (b *Bot) handleMessage(update tgbotapi.Update) {
    msg := update.Message
    if msg == nil {
        return
    }
    text := msg.Text

    switch {
    case strings.HasPrefix(text, "/start"):
        b.reply(msg.Chat.ID, "Добро пожаловать в бронь кабинетов! Используйте /book для создания заявки.")
    case strings.HasPrefix(text, "/help"):
        b.reply(msg.Chat.ID, "Доступные команды: /book, /my_bookings, /cancel_booking <id>, /help")
    case strings.HasPrefix(text, "/book"):
        b.reply(msg.Chat.ID, "(stub) Запуск сценария бронирования кабинета")
    case strings.HasPrefix(text, "/my_bookings"):
        b.reply(msg.Chat.ID, "(stub) Ваши бронирования")
    case strings.HasPrefix(text, "/cancel_booking"):
        b.reply(msg.Chat.ID, "(stub) Отмена бронирования")
    default:
        if b.isManager(msg.From.ID) {
            if b.handleManagerCommands(msg) {
                return
            }
        }
    }
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

func (b *Bot) reply(chatID int64, text string) {
    msg := tgbotapi.NewMessage(chatID, text)
    b.bot.Send(msg)
}

func (b *Bot) isManager(id int64) bool {
    _, ok := b.managers[id]
    return ok
}

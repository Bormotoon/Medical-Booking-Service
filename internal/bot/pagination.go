package bot

import (
	"context"
	"fmt"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type PaginationParams struct {
	Ctx          context.Context
	ChatID       int64
	MessageID    int // 0 if new message
	Page         int
	Title        string
	ItemPrefix   string
	PagePrefix   string
	BackCallback string
	ShowCapacity bool
}

func (b *Bot) renderPaginatedItems(params PaginationParams) {
	itemsPerPage := 8
	startIdx := params.Page * itemsPerPage
	endIdx := startIdx + itemsPerPage
	if endIdx > len(b.items) {
		endIdx = len(b.items)
	}

	var message strings.Builder
	message.WriteString(fmt.Sprintf("%s\n\n", params.Title))
	message.WriteString(fmt.Sprintf("Страница %d из %d\n\n", params.Page+1, (len(b.items)+itemsPerPage-1)/itemsPerPage))

	currentItems := b.items[startIdx:endIdx]
	for i, item := range currentItems {
		message.WriteString(fmt.Sprintf("%d. *%s*\n", startIdx+i+1, item.Name))
		if item.Description != "" {
			message.WriteString(fmt.Sprintf("   📝 %s\n", item.Description))
		}
		if params.ShowCapacity {
			message.WriteString(fmt.Sprintf("   👥 Вместимость: %d чел.\n", item.TotalQuantity))
		}
		message.WriteString("\n")
	}

	var keyboard [][]tgbotapi.InlineKeyboardButton

	for i, item := range currentItems {
		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("%d. %s", startIdx+i+1, item.Name),
			fmt.Sprintf("%s%d", params.ItemPrefix, item.ID),
		)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{btn})
	}

	var navButtons []tgbotapi.InlineKeyboardButton
	if params.Page > 0 {
		navButtons = append(navButtons, tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад", fmt.Sprintf("%s%d", params.PagePrefix, params.Page-1)))
	}
	if endIdx < len(b.items) {
		navButtons = append(navButtons, tgbotapi.NewInlineKeyboardButtonData("Вперед ➡️", fmt.Sprintf("%s%d", params.PagePrefix, params.Page+1)))
	}
	if len(navButtons) > 0 {
		keyboard = append(keyboard, navButtons)
	}

	if params.BackCallback != "" {
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("⬅️ Назад в меню", params.BackCallback),
		})
	}

	markup := tgbotapi.NewInlineKeyboardMarkup(keyboard...)

	if params.MessageID != 0 {
		editMsg := tgbotapi.NewEditMessageTextAndMarkup(
			params.ChatID,
			params.MessageID,
			message.String(),
			markup,
		)
		editMsg.ParseMode = "Markdown"
		b.bot.Send(editMsg)
	} else {
		msg := tgbotapi.NewMessage(params.ChatID, message.String())
		msg.ReplyMarkup = markup
		msg.ParseMode = "Markdown"
		b.bot.Send(msg)
	}
}

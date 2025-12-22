package service

// import (
// 	"fmt"
//
// 	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
// 	"mega-trainer-go/pkg/logger"
// )
//
// // TelegramService интерфейс для работы с Telegram API
// type TelegramService interface {
// 	SendMessage(chatID int64, text string) error
// 	SendMessageWithKeyboard(chatID int64, text string, keyboard interface{}) error
// 	AnswerCallback(callbackID, text string) error
// 	SendPhoto(chatID int64, photo []byte, caption string) error
// 	// EditMessageText(chatID int64, messageID int, text string, keyboard interface{}) error
// }
//
// type telegramService struct {
// 	bot *tgbotapi.BotAPI
// 	log *logger.Logger
// }
//
// func NewTelegramService(bot *tgbotapi.BotAPI, log *logger.Logger) TelegramService {
// 	return &telegramService{
// 		bot: bot,
// 		log: log,
// 	}
// }
//
// func (s *telegramService) SendMessage(chatID int64, text string) error {
// 	msg := tgbotapi.NewMessage(chatID, text)
// 	msg.ParseMode = "Markdown"
// 	msg.DisableWebPagePreview = true
//
// 	_, err := s.bot.Send(msg)
// 	if err != nil {
// 		return fmt.Errorf("failed to send message: %w", err)
// 	}
//
// 	s.log.Debug().Msgf("message sent", "chat_id", chatID, "text_length", len(text))
// 	return nil
// }
//
// func (s *telegramService) SendMessageWithKeyboard(chatID int64, text string, keyboard interface{}) error {
// 	msg := tgbotapi.NewMessage(chatID, text)
// 	msg.ParseMode = "Markdown"
// 	msg.DisableWebPagePreview = true
// 	msg.ReplyMarkup = keyboard
//
// 	_, err := s.bot.Send(msg)
// 	if err != nil {
// 		return fmt.Errorf("failed to send message with keyboard: %w", err)
// 	}
//
// 	s.log.Debug().Msgf("message with keyboard sent", "chat_id", chatID)
// 	return nil
// }
//
// func (s *telegramService) AnswerCallback(callbackID, text string) error {
// 	callback := tgbotapi.NewCallback(callbackID, text)
// 	if _, err := s.bot.Request(callback); err != nil {
// 		return fmt.Errorf("failed to answer callback: %w", err)
// 	}
//
// 	s.log.Debug().Msgf("callback answered", "callback_id", callbackID)
// 	return nil
// }
//
// func (s *telegramService) SendPhoto(chatID int64, photo []byte, caption string) error {
// 	photoFile := tgbotapi.FileBytes{
// 		Name:  "chart.png",
// 		Bytes: photo,
// 	}
//
// 	msg := tgbotapi.NewPhoto(chatID, photoFile)
// 	msg.Caption = caption
// 	msg.ParseMode = "Markdown"
//
// 	_, err := s.bot.Send(msg)
// 	if err != nil {
// 		return fmt.Errorf("failed to send photo: %w", err)
// 	}
//
// 	s.log.Debug().Msgf("photo sent", "chat_id", chatID, "photo_size", len(photo))
// 	return nil
// }
//
// func (s *telegramService) EditMessageText(chatID int64, messageID int, text string, keyboard *tgbotapi.InlineKeyboardMarkup) error {
// 	msg := tgbotapi.NewEditMessageText(chatID, messageID, text)
// 	msg.ParseMode = "Markdown"
// 	msg.DisableWebPagePreview = true
//
// 	if keyboard != nil {
// 		msg.ReplyMarkup = keyboard
// 	}
//
// 	_, err := s.bot.Send(msg)
// 	if err != nil {
// 		return fmt.Errorf("failed to edit message: %w", err)
// 	}
//
// 	s.log.Debug().Msgf("message edited", "chat_id", chatID, "message_id", messageID)
// 	return nil
// }

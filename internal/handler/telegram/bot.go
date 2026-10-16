package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"family-expenses/internal/config"
	"family-expenses/internal/service"
)

type Bot struct {
	bot    *tgbotapi.BotAPI
	agent  *service.AIAgent
	config config.TelegramConfig
}

func NewBot(cfg config.TelegramConfig, agent *service.AIAgent) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, err
	}

	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("parse proxy url: %w", err)
		}

		pollTimeout := cfg.PollTimeout
		if pollTimeout <= 0 {
			pollTimeout = 60
		}

		dialer := &net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}

		bot.Client = &http.Client{
			Transport: &http.Transport{
				Proxy:       http.ProxyURL(proxyURL),
				DialContext: dialer.DialContext,
			},
			Timeout: time.Duration(pollTimeout)*time.Second + 30*time.Second,
		}
		slog.Info("Бот использует прокси", "proxy", cfg.ProxyURL)
	}

	slog.Info("Бот авторизован", "username", bot.Self.UserName)

	return &Bot{
		bot:    bot,
		agent:  agent,
		config: cfg,
	}, nil
}

func (b *Bot) Username() string {
	if b.bot != nil {
		return b.bot.Self.UserName
	}
	return ""
}

func (b *Bot) Start(ctx context.Context) error {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = b.config.PollTimeout

	updates := b.bot.GetUpdatesChan(updateConfig)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Бот остановлен")
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}
			go b.handleUpdate(ctx, update)
		}
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	if update.Message == nil {
		return
	}

	msg := update.Message
	userID := msg.From.ID
	text := msg.Text
	userName := msg.From.UserName

	if text == "" {
		return
	}

	slog.Info("Получено сообщение",
		"user_id", userID,
		"username", userName,
		"chat_id", msg.Chat.ID,
		"text", text)

	response, err := b.agent.ProcessMessage(ctx, int64(userID), text)
	if err != nil {
		slog.Error("Ошибка обработки сообщения", "user_id", userID, "error", err)
		response = "Произошла ошибка. Попробуйте ещё раз."
	}

	reply := tgbotapi.NewMessage(update.Message.Chat.ID, response)
	reply.ReplyToMessageID = update.Message.MessageID

	if _, err := b.bot.Send(reply); err != nil {
		slog.Error("Ошибка отправки ответа", "user_id", userID, "error", err)
	} else {
		slog.Info("Ответ отправлен", "user_id", userID, "chat_id", msg.Chat.ID)
	}
}
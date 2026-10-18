package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"family-expenses/internal/config"
	"family-expenses/internal/service"
)

// defaultMaxConcurrency — значение по умолчанию, если не задано в конфиге.
const defaultMaxConcurrency = 10

type Bot struct {
	bot    *tgbotapi.BotAPI
	agent  *service.AIAgent
	config config.TelegramConfig
	sem    chan struct{}  // семафор для ограничения горутин
	wg     sync.WaitGroup // отслеживание завершения горутин при graceful shutdown
}

func NewBot(cfg config.TelegramConfig, agent *service.AIAgent) (*Bot, error) {
	var bot *tgbotapi.BotAPI
	var err error

	if cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("parse proxy url: %w", err)
		}

		pollTimeout := cfg.PollTimeout
		if pollTimeout <= 0 {
			pollTimeout = 60
		}

		client := &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyURL(proxyURL),
				DialContext: (&net.Dialer{
					Timeout:   15 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
			Timeout: time.Duration(pollTimeout)*time.Second + 30*time.Second,
		}

		bot, err = tgbotapi.NewBotAPIWithClient(cfg.Token, tgbotapi.APIEndpoint, client)
		slog.Info("Бот использует прокси", "proxy", cfg.ProxyURL)
	} else {
		bot, err = tgbotapi.NewBotAPI(cfg.Token)
	}

	if err != nil {
		return nil, err
	}
	if bot == nil {
		return nil, fmt.Errorf("не удалось создать Telegram-бота")
	}

	slog.Info("Бот авторизован", "username", bot.Self.UserName)

	// Инициализация семафора
	maxConcurrency := cfg.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = defaultMaxConcurrency
	}

	return &Bot{
		bot:    bot,
		agent:  agent,
		config: cfg,
		sem:    make(chan struct{}, maxConcurrency),
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

	slog.Info("Бот запущен", "max_concurrency", cap(b.sem), "poll_timeout", b.config.PollTimeout)

	for {
		select {
		case <-ctx.Done():
			slog.Info("Бот остановлен, ожидание завершения обработки сообщений...")
			b.shutdown()
			return nil
		case update, ok := <-updates:
			if !ok {
				return nil
			}

			// Пытаемся занять слот в семафоре; при переполнении — отвечаем пользователю
			select {
			case b.sem <- struct{}{}:
				b.wg.Add(1)
				go func() {
					defer func() {
						<-b.sem // освобождаем слот
						b.wg.Done()
					}()
					b.handleUpdate(ctx, update)
				}()
			default:
				slog.Warn("Семафор переполнен, пропускаем сообщение",
					"user_id", update.Message.From.ID,
					"max_concurrency", cap(b.sem))
				// Пытаемся отправить предупреждение
				if update.Message != nil {
					reply := tgbotapi.NewMessage(update.Message.Chat.ID,
						"⚠️ Слишком много запросов. Подождите немного.")
					reply.ReplyToMessageID = update.Message.MessageID
					b.bot.Send(reply) // ошибку игнорируем
				}
			}
		}
	}
}

// shutdown ожидает завершения текущих горутин с таймаутом.
func (b *Bot) shutdown() {
	done := make(chan struct{})
	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Все горутины завершены")
	case <-time.After(15 * time.Second):
		slog.Warn("Тайм-аут при завершении горутин бота")
	}
}

func (b *Bot) handleUpdate(ctx context.Context, update tgbotapi.Update) {
	// Проверка, не отменён ли контекст
	select {
	case <-ctx.Done():
		return
	default:
	}

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

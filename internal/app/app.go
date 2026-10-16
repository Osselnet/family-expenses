package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"family-expenses/internal/config"
	httpHandler "family-expenses/internal/handler/http"
	"family-expenses/internal/handler/telegram"
	"family-expenses/internal/llm/openai"
	"family-expenses/internal/repository/postgres"
	"family-expenses/internal/service"
)

type App struct {
	Config         *config.Config
	DB             *pgxpool.Pool
	ExpenseService *service.ExpenseService
	AIAgent        *service.AIAgent
	TelegramBot    *telegram.Bot
	httpServer     *http.Server
	webFS          http.FileSystem
}

func New(ctx context.Context, cfg *config.Config) (*App, error) {
	// Подключение к БД
	slog.Info("Подключение к БД...", "url", cfg.Database.URL)
	db, err := pgxpool.New(ctx, cfg.Database.URL)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	slog.Info("Подключение к БД установлено")

	// Репозиторий
	expenseRepo := postgres.NewExpenseRepository(db)

	// Применение миграций
	slog.Info("Применение миграций...")
	if err := expenseRepo.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	slog.Info("Миграции применены")

	// LLM-клиент
	slog.Info("Инициализация LLM-клиента", "provider", cfg.LLM.Provider, "base_url", cfg.LLM.BaseURL, "model", cfg.LLM.Model)
	llmClient := openai.NewClient(
		cfg.LLM.BaseURL,
		cfg.LLM.APIKey,
		cfg.LLM.Model,
		cfg.LLM.Temperature,
		cfg.LLM.MaxTokens,
		cfg.LLM.Timeout,
	)

	// Сервисы
	expenseService := service.NewExpenseService(expenseRepo)
	aiAgent := service.NewAIAgent(llmClient, expenseService)

	// Telegram-бот — создаётся только если задан токен в .env
	var tgBot *telegram.Bot
	if cfg.Telegram.Token != "" {
		slog.Info("Создание Telegram-бота...")
		tgBot, err = telegram.NewBot(cfg.Telegram, aiAgent)
		if err != nil {
			return nil, fmt.Errorf("create telegram bot: %w", err)
		}
		slog.Info("Telegram-бот создан", "username", tgBot.Username())
	} else {
		slog.Warn("TELEGRAM_TOKEN не задан — бот отключён")
	}

	app := &App{
		Config:         cfg,
		DB:             db,
		ExpenseService: expenseService,
		AIAgent:        aiAgent,
		TelegramBot:    tgBot,
	}

	return app, nil
}

// SetWebFS устанавливает файловую систему для веб-панели.
func (a *App) SetWebFS(fs http.FileSystem) {
	a.webFS = fs
}

// RunAPI запускает HTTP API сервер (блокирующий вызов)
func (a *App) RunAPI() error {
	router := httpHandler.NewRouter(a.ExpenseService, a.webFS)

	a.httpServer = &http.Server{
		Addr:    a.Config.ServerAddr(),
		Handler: router,
	}

	slog.Info("API сервер запущен", "addr", a.Config.ServerAddr())
	if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}
	return nil
}

// RunBot запускает Telegram-бота (блокирующий вызов)
func (a *App) RunBot(ctx context.Context) error {
	if a.TelegramBot == nil {
		slog.Warn("Telegram-бот не создан — проверьте TELEGRAM_TOKEN")
		<-ctx.Done()
		return nil
	}
	slog.Info("Telegram-бот запущен, ожидание сообщений...")
	return a.TelegramBot.Start(ctx)
}

func (a *App) Close() {
	slog.Info("Завершение работы приложения...")
	if a.httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("Ошибка при остановке HTTP-сервера", "error", err)
		}
	}
	if a.DB != nil {
		a.DB.Close()
	}
	slog.Info("Приложение остановлено")
}
package main

import (
	"context"
	"embed"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"family-expenses/internal/app"
	"family-expenses/internal/config"
)

//go:embed web/*
var webAssets embed.FS

func main() {
	service := flag.String("service", "api", "Service to run: api, bot, all")
	flag.Parse()

	// Структурированное логирование (JSON для Docker, text для локальной разработки)
	logLevel := slog.LevelInfo
	if os.Getenv("LOG_LEVEL") == "debug" {
		logLevel = slog.LevelDebug
	}
	var handler slog.Handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	if os.Getenv("LOG_FORMAT") == "text" {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}
	slog.SetDefault(slog.New(handler))
	slog.Info("Запуск Family Expenses...", "service", *service)

	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		slog.Error("Загрузка конфига", "error", err)
		os.Exit(1)
	}
	slog.Info("Конфигурация загружена", "llm_provider", cfg.LLM.Provider, "llm_model", cfg.LLM.Model)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	application, err := app.New(ctx, cfg)
	if err != nil {
		slog.Error("Инициализация приложения", "error", err)
		os.Exit(1)
	}
	defer application.Close()

	// Встраиваемая веб-панель
	application.SetWebFS(http.FS(webAssets))

	// Запуск сервисов
	switch *service {
	case "api":
		go func() {
			if err := application.RunAPI(); err != nil {
				slog.Error("Ошибка API", "error", err)
				cancel()
			}
		}()
	case "bot":
		go func() {
			if err := application.RunBot(ctx); err != nil {
				slog.Error("Ошибка бота", "error", err)
				cancel()
			}
		}()
	case "all":
		go func() {
			if err := application.RunAPI(); err != nil {
				slog.Error("Ошибка API", "error", err)
				cancel()
			}
		}()
		go func() {
			if err := application.RunBot(ctx); err != nil {
				slog.Error("Ошибка бота", "error", err)
				cancel()
			}
		}()
	default:
		slog.Error("Неизвестный сервис", "service", *service)
		os.Exit(1)
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("Получен сигнал остановки", "signal", sig.String())
	cancel()
	slog.Info("Сервер остановлен")
}
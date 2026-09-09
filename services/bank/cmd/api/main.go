package main

import (
	"bank/internal/app"
	"bank/internal/config"
	"bank/internal/logger"
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// @title Bank API
// @version 1.0
// @description Учебный REST API банковского сервиса.
// @description Для защищённых endpoints сначала выполните POST /auth/login.
// @description Access JWT и rotating refresh token устанавливаются в HttpOnly cookies.
//
// @tag.name auth
// @tag.description Регистрация и аутентификация.
// @tag.name users
// @tag.description Профили пользователей. Требуется cookie access_token.
// @tag.name accounts
// @tag.description Банковский аккаунт, пополнение и списание средств. Требуется cookie access_token.
//
// @BasePath /
// @schemes http https
func main() {
	cfg := config.MustLoad()

	logger := logger.New(cfg.Env)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("Fail to start app:", slog.Any("error", err))
		os.Exit(1)
	}
	defer func() {
		if err := application.Close(); err != nil {
			logger.Error("Fail to close app:", slog.Any("error", err))
		}
	}()

	if err := application.Run(ctx); err != nil {
		logger.Error("App stopped", slog.Any("error", err))
	}
}

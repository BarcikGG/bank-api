package app

import (
	"bank/internal/accounts"
	"bank/internal/auth"
	"bank/internal/config"
	"bank/internal/outbox"
	"bank/internal/storage/postgres"
	"bank/internal/transfers"
	"bank/internal/users"
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"log/slog"

	_ "bank/docs"
	"bank/internal/logger"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type App struct {
	router  *echo.Echo
	storage *postgres.Storage
	config  *config.Config
	logger  *slog.Logger
}

func New(cfg *config.Config, slogger *slog.Logger) (*App, error) {
	storage, err := postgres.Connect(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("подключить PostgreSQL: %w", err)
	}

	tokenManager := auth.NewManager(cfg.JWTSecret, cfg.JWTTTL, "bank-api", "bank-client")
	authService := auth.NewService(storage.DB, tokenManager, cfg.RefreshTTL, slogger)
	eventWriter := outbox.Writer{}
	userService := users.NewService(storage.DB, slogger, eventWriter)
	userHandler := users.NewHandler(userService, authService)

	accountService := accounts.NewService(storage.DB, slogger)
	accountHandler := accounts.NewHandler(accountService)

	transferService := transfers.NewService(storage.DB, slogger)
	transferHandler := transfers.NewHandler(transferService)

	requireAuth := tokenManager.Middleware(auth.AccessCookieName)

	router := echo.New()
	router.Logger = slogger
	router.HTTPErrorHandler = newHTTPErrorHandler()
	router.Use(middleware.RequestID())
	router.Use(logger.RequestLogger(slogger))
	router.Use(middleware.Recover())

	if cfg.Env == "local" {
		router.GET("/swagger/*", echo.WrapHandler(httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
			httpSwagger.DocExpansion("list"),
		)))
	}

	userHandler.RegisterAuthRoutes(
		router.Group("/auth"),
		requireAuth,
	)
	userHandler.RegisterRoutes(
		router.Group("/users", requireAuth),
	)
	accountHandler.RegisterRoutes(
		router.Group("/accounts", requireAuth),
	)
	transferHandler.RegisterRoutes(
		router.Group("/transfers", requireAuth),
	)

	return &App{
		router:  router,
		storage: storage,
		config:  cfg,
		logger:  slogger,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	address := net.JoinHostPort(a.config.Host, strconv.Itoa(a.config.Port))

	server := &http.Server{
		Addr:         address,
		Handler:      a.router,
		ReadTimeout:  a.config.Timeout,
		WriteTimeout: a.config.Timeout,
		IdleTimeout:  a.config.IdleTimeout,
	}

	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}

	a.logger.Info(
		"HTTP server started",
		slog.String("address", listener.Addr().String()),
		slog.String("swagger", "/swagger/index.html"),
	)

	serveErr := make(chan error, 1)

	go func() {
		serveErr <- server.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}

		return fmt.Errorf("serve HTTP: %w", err)

	case <-ctx.Done():
		a.logger.Info("HTTP server shutdown started")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()

			return fmt.Errorf("shutdown HTTP server: %w", err)
		}

		err := <-serveErr
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP after shutdown: %w", err)
		}

		a.logger.Info("HTTP server stopped")
		return nil
	}
}

func (a *App) Close() error {
	sqlDB, err := a.storage.DB.DB()
	if err != nil {
		return fmt.Errorf("получение соединения с базой данных: %v", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("закрытие соединения с базой данных: %v", err)
	}

	return nil
}

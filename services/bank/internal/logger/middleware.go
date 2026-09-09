package logger

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"
	echoMiddleware "github.com/labstack/echo/v5/middleware"
)

func RequestLogger(
	logger *slog.Logger,
) echo.MiddlewareFunc {
	return echoMiddleware.RequestLoggerWithConfig(
		echoMiddleware.RequestLoggerConfig{
			HandleError:     true,
			LogRequestID:    true,
			LogMethod:       true,
			LogURIPath:      true,
			LogStatus:       true,
			LogResponseSize: true,
			LogLatency:      true,
			LogValuesFunc: func(
				c *echo.Context,
				values echoMiddleware.RequestLoggerValues,
			) error {
				status := values.Status
				if status == 0 {
					status = http.StatusOK
				}

				level := slog.LevelInfo
				if status >= 500 {
					level = slog.LevelError
				} else if status >= 400 {
					level = slog.LevelWarn
				}

				attrs := []slog.Attr{
					slog.String("request_id", values.RequestID),
					slog.String("method", values.Method),
					slog.String("path", values.URIPath),
					slog.Int("status", status),
					slog.Int64("bytes", values.ResponseSize),
					slog.Duration("duration", values.Latency),
				}
				if values.Error != nil {
					attrs = append(attrs, slog.Any("error", values.Error))
				}

				logger.LogAttrs(
					c.Request().Context(),
					level,
					"HTTP request",
					attrs...,
				)

				return nil
			},
		})
}

package app

import (
	"errors"
	"net/http"

	"bank/internal/accounts"
	"bank/internal/auth"
	"bank/internal/transfers"
	"bank/internal/users"

	"github.com/labstack/echo/v5"
)

func newHTTPErrorHandler() echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if response, unwrapErr := echo.UnwrapResponse(c.Response()); unwrapErr == nil && response.Committed {
			return
		}

		status, message := mapHTTPError(err)

		var responseErr error
		if c.Request().Method == http.MethodHead {
			responseErr = c.NoContent(status)
		} else {
			responseErr = c.JSON(status, map[string]string{"error": message})
		}

		if responseErr != nil {
			c.Logger().Error(
				"failed to write HTTP error response",
				"error",
				errors.Join(err, responseErr),
			)
		}
	}
}

func mapHTTPError(err error) (int, string) {
	switch {
	case errors.Is(err, users.ErrInvalidInput):
		return http.StatusBadRequest, users.ErrInvalidInput.Error()
	case errors.Is(err, users.ErrInvalidID):
		return http.StatusBadRequest, users.ErrInvalidID.Error()
	case errors.Is(err, accounts.ErrInvalidEmailInput):
		return http.StatusBadRequest, accounts.ErrInvalidEmailInput.Error()
	case errors.Is(err, accounts.ErrInvalidID):
		return http.StatusBadRequest, accounts.ErrInvalidID.Error()
	case errors.Is(err, accounts.ErrInvalidAmount):
		return http.StatusBadRequest, accounts.ErrInvalidAmount.Error()
	case errors.Is(err, transfers.ErrInvalidUserID):
		return http.StatusBadRequest, transfers.ErrInvalidUserID.Error()
	case errors.Is(err, accounts.ErrUnsupportedCur):
		return http.StatusBadRequest, accounts.ErrUnsupportedCur.Error()
	case errors.Is(err, accounts.ErrNotEnoughMoney):
		return http.StatusBadRequest, accounts.ErrNotEnoughMoney.Error()

	case errors.Is(err, users.ErrEmailTaken):
		return http.StatusConflict, users.ErrEmailTaken.Error()

	case errors.Is(err, users.ErrNotFound):
		return http.StatusNotFound, users.ErrNotFound.Error()
	case errors.Is(err, accounts.ErrNotFound):
		return http.StatusNotFound, accounts.ErrNotFound.Error()
	case errors.Is(err, transfers.ErrNotFound):
		return http.StatusNotFound, transfers.ErrNotFound.Error()

	case errors.Is(err, users.ErrInvalidCredentials):
		return http.StatusUnauthorized, users.ErrInvalidCredentials.Error()

	case errors.Is(err, auth.ErrInvalidRefreshToken), errors.Is(err, auth.ErrInvalidToken):
		return http.StatusUnauthorized, "unauthorized"
	}

	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.StatusCode(), httpErr.Message
	}

	var statusCoder echo.HTTPStatusCoder
	if errors.As(err, &statusCoder) {
		status := statusCoder.StatusCode()
		if status != 0 {
			return status, http.StatusText(status)
		}
	}

	return http.StatusInternalServerError, "internal server error"
}

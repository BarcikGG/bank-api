package app

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"bank/internal/accounts"
	"bank/internal/auth"
	"bank/internal/transfers"
	"bank/internal/users"

	"github.com/labstack/echo/v5"
)

func TestMapHTTPError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "user validation",
			err:         users.ErrInvalidInput,
			wantStatus:  http.StatusBadRequest,
			wantMessage: users.ErrInvalidInput.Error(),
		},
		{
			name:        "wrapped account not found",
			err:         fmt.Errorf("find account: %w", accounts.ErrNotFound),
			wantStatus:  http.StatusNotFound,
			wantMessage: accounts.ErrNotFound.Error(),
		},
		{
			name:        "wrapped transfer not found",
			err:         fmt.Errorf("find transfer: %w", transfers.ErrNotFound),
			wantStatus:  http.StatusNotFound,
			wantMessage: transfers.ErrNotFound.Error(),
		},
		{
			name:        "invalid refresh token",
			err:         auth.ErrInvalidRefreshToken,
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "unauthorized",
		},
		{
			name:        "echo HTTP error",
			err:         echo.NewHTTPError(http.StatusUnprocessableEntity, "invalid payload"),
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "invalid payload",
		},
		{
			name:        "echo not found sentinel",
			err:         echo.ErrNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: http.StatusText(http.StatusNotFound),
		},
		{
			name:        "unexpected error",
			err:         errors.New("database unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "internal server error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			status, message := mapHTTPError(test.err)
			if status != test.wantStatus {
				t.Fatalf("status = %d, want %d", status, test.wantStatus)
			}
			if message != test.wantMessage {
				t.Fatalf("message = %q, want %q", message, test.wantMessage)
			}
		})
	}
}

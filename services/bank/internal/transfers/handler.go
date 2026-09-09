package transfers

import (
	"bank/internal/auth"
	"net/http"

	"github.com/labstack/echo/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(group *echo.Group) {
	group.GET("/history", h.getTransferHistory)
	group.GET("/history/:id", h.getTransferHistoryByID)
}

// getTransferHistory godoc
//
// @Summary Get transfer history
// @Description Get transfer history for the current user
// @Tags transfers
// @Produce json
// @Success 200 {array} Transfer
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /transfers/history [get]
func (h *Handler) getTransferHistory(c *echo.Context) error {
	authUserID, ok := auth.UserIDFromContext(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	history, err := h.service.GetTransferHistory(c.Request().Context(), authUserID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, history)
}

// getTransferHistoryByID godoc
//
// @Summary Get transfer history by ID
// @Description Get transfer history for the current user by ID
// @Tags transfers
// @Produce json
// @Success 200 {object} Transfer
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /transfers/history/{id} [get]
func (h *Handler) getTransferHistoryByID(c *echo.Context) error {
	authUserID, ok := auth.UserIDFromContext(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	transferID := c.Param("id")
	transfer, err := h.service.GetTransferHistoryByID(c.Request().Context(), authUserID, transferID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, transfer)
}

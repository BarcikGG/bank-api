package accounts

import (
	"net/http"

	"bank/internal/auth"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

const accountRequestBodyLimit = 16 << 10

type Handler struct {
	service *Service
}

type accountResponse struct {
	ID       string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UserID   string `json:"user_id" example:"450e8400-e29b-41d4-a716-446655440001"`
	Balance  int64  `json:"balance" example:"200"`
	Currency string `json:"currency" example:"RUB"`
}

type moneyRequest struct {
	Amount int64 `json:"amount" example:"50025"`
}

type transferRequest struct {
	Amount      int64  `json:"amount" example:"50025"`
	ToAccountID string `json:"to_account_id" example:"550e8400-e29b-41d4-a716-446655440000"`
}

func makeAccountResponse(account *Account) accountResponse {
	return accountResponse{
		ID:       account.ID,
		UserID:   account.UserID,
		Balance:  account.Balance,
		Currency: account.Currency,
	}
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(group *echo.Group) {
	group.GET("/my", h.getUserAccount)
	group.POST("/deposit", h.accountDeposit, middleware.BodyLimit(accountRequestBodyLimit))
	group.POST("/withdraw", h.accountWithdraw, middleware.BodyLimit(accountRequestBodyLimit))
	group.POST("/transfer", h.accountTransfer, middleware.BodyLimit(accountRequestBodyLimit))
}

// getAccountByID godoc
//
// @Summary Получить свой аккаунт по id
// @Description Возвращает аккаунт пользователя, определённого по JWT cookie.
// @Tags accounts
// @Produce json
// @Success 200 {object} accountResponse
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /accounts/my [get]
func (h *Handler) getUserAccount(c *echo.Context) error {
	authUserID, ok := auth.UserIDFromContext(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	account, err := h.service.GetAccountByUserID(c.Request().Context(), authUserID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, makeAccountResponse(account))
}

// accountDeposit godoc
//
// @Summary Пополнить баланс
// @Description Пополняет баланс аккаунта текущего пользователя и сохраняет финансовую операцию. Требуется access JWT cookie.
// @Tags accounts
// @Accept json
// @Produce json
// @Param request body moneyRequest true "Сумма пополнения"
// @Success 200 {object} accountResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /accounts/deposit [post]
func (h *Handler) accountDeposit(c *echo.Context) error {
	authUserID, ok := auth.UserIDFromContext(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var request moneyRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body").Wrap(err)
	}

	account, err := h.service.Deposit(c.Request().Context(), authUserID, request.Amount)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, makeAccountResponse(account))
}

// accountWithdraw godoc
//
// @Summary Вывести средства
// @Description Списывает средства с баланса аккаунта текущего пользователя и сохраняет финансовую операцию. Требуется access JWT cookie.
// @Tags accounts
// @Accept json
// @Produce json
// @Param request body moneyRequest true "Сумма списания"
// @Success 200 {object} accountResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /accounts/withdraw [post]
func (h *Handler) accountWithdraw(c *echo.Context) error {
	authUserID, ok := auth.UserIDFromContext(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var request moneyRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body").Wrap(err)
	}

	account, err := h.service.Withdraw(c.Request().Context(), authUserID, request.Amount)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, makeAccountResponse(account))
}

// accountTransfer godoc
//
// @Summary Перевести средства
// @Description Переводит средства между аккаунтами текущего пользователя и другого пользователя. Требуется access JWT cookie.
// @Tags accounts
// @Accept json
// @Produce json
// @Param request body transferRequest true "Сумма перевода"
// @Success 200 {object} accountResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
func (h *Handler) accountTransfer(c *echo.Context) error {
	authUserID, ok := auth.UserIDFromContext(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var request transferRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body").Wrap(err)
	}

	account, err := h.service.Transfer(c.Request().Context(), authUserID, request.Amount, request.ToAccountID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, makeAccountResponse(account))
}

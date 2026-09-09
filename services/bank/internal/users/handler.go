package users

import (
	"errors"
	"net/http"
	"time"

	"bank/internal/auth"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

const authRequestBodyLimit = 1 << 20

type Handler struct {
	service     *Service
	authService *auth.Service
}

type createRequest struct {
	Name     string `json:"name" example:"Ivan"`
	Email    string `json:"email" example:"ivan@example.com"`
	Password string `json:"password" example:"strong-password"`
}

type userResponse struct {
	ID        string    `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	Name      string    `json:"name" example:"Ivan"`
	Email     string    `json:"email" example:"ivan@example.com"`
	CreatedAt time.Time `json:"created_at" example:"2026-08-27T12:00:00Z"`
}

type loginRequest struct {
	Email    string `json:"email" example:"ivan@example.com"`
	Password string `json:"password" example:"strong-password"`
}

func NewHandler(service *Service, authService *auth.Service) *Handler {
	return &Handler{service: service, authService: authService}
}

func (h *Handler) RegisterRoutes(group *echo.Group) {
	group.GET("/me", h.getMe)
	group.GET("/:id", h.getByID)
}

func (h *Handler) RegisterAuthRoutes(group *echo.Group, requireAuth echo.MiddlewareFunc) {
	group.POST("/register", h.register, middleware.BodyLimit(authRequestBodyLimit))
	group.POST("/login", h.login, middleware.BodyLimit(authRequestBodyLimit))
	group.POST("/refresh", h.refresh)
	group.POST("/logout", h.logout)
	group.POST("/logout-all", h.logoutAll, requireAuth)
}

func makeUserResponse(user *User) userResponse {
	return userResponse{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}
}

// register godoc
//
// @Summary Зарегистрировать пользователя
// @Description Создаёт пользователя и его банковский аккаунт в одной транзакции.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body createRequest true "Данные для регистрации"
// @Success 201 {object} userResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/register [post]
func (h *Handler) register(c *echo.Context) error {
	var request createRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body").Wrap(err)
	}

	user, err := h.service.Register(c.Request().Context(), CreateInput{
		Name:     request.Name,
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, makeUserResponse(user))
}

// login godoc
//
// @Summary Войти
// @Description Проверяет email и пароль, создаёт сессию и устанавливает access/refresh токены в HttpOnly cookies.
// @Tags auth
// @Accept json
// @Produce json
// @Param request body loginRequest true "Учётные данные"
// @Success 204 "Успешная аутентификация"
// @Header 204 {string} Set-Cookie "HttpOnly cookies access_token и refresh_token"
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/login [post]
func (h *Handler) login(c *echo.Context) error {
	var request loginRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body").Wrap(err)
	}

	user, err := h.service.Login(c.Request().Context(), LoginInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		return err
	}

	tokens, err := h.authService.StartSession(c.Request().Context(), user.ID)
	if err != nil {
		return err
	}

	setAuthCookies(c, tokens)
	return c.NoContent(http.StatusNoContent)
}

// refresh godoc
//
// @Summary Обновить токены
// @Description Проверяет и ротирует refresh token, затем устанавливает новую пару HttpOnly cookies.
// @Tags auth
// @Success 204 "Токены обновлены"
// @Header 204 {string} Set-Cookie "Новые HttpOnly cookies access_token и refresh_token"
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/refresh [post]
func (h *Handler) refresh(c *echo.Context) error {
	cookie, err := c.Cookie(auth.RefreshCookieName)
	if err != nil {
		clearAuthCookies(c)
		return auth.ErrInvalidRefreshToken
	}

	tokens, err := h.authService.Refresh(c.Request().Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidRefreshToken) {
			clearAuthCookies(c)
		}
		return err
	}

	setAuthCookies(c, tokens)
	return c.NoContent(http.StatusNoContent)
}

// logout godoc
//
// @Summary Выйти из текущей сессии
// @Description Отзывает текущую сессию по refresh cookie и удаляет обе auth cookies.
// @Tags auth
// @Success 204 "Выход выполнен"
// @Failure 500 {object} map[string]string
// @Router /auth/logout [post]
func (h *Handler) logout(c *echo.Context) error {
	var rawRefreshToken string
	if cookie, err := c.Cookie(auth.RefreshCookieName); err == nil {
		rawRefreshToken = cookie.Value
	}

	if err := h.authService.RevokeSession(c.Request().Context(), rawRefreshToken); err != nil {
		return err
	}

	clearAuthCookies(c)
	return c.NoContent(http.StatusNoContent)
}

// logoutAll godoc
//
// @Summary Выйти со всех устройств
// @Description Отзывает все refresh-сессии пользователя и удаляет auth cookies текущего устройства. Требуется access JWT.
// @Tags auth
// @Success 204 "Все сессии отозваны"
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/logout-all [post]
func (h *Handler) logoutAll(c *echo.Context) error {
	userID, ok := auth.UserIDFromContext(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	if err := h.authService.RevokeAllSessions(c.Request().Context(), userID); err != nil {
		return err
	}

	clearAuthCookies(c)
	return c.NoContent(http.StatusNoContent)
}

// getMe godoc
//
// @Summary Получить свой профиль
// @Description Возвращает профиль пользователя, определённого по JWT cookie.
// @Tags users
// @Produce json
// @Success 200 {object} userResponse
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/me [get]
func (h *Handler) getMe(c *echo.Context) error {
	authUserID, ok := auth.UserIDFromContext(c)
	if !ok {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	user, err := h.service.GetByID(c.Request().Context(), authUserID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, makeUserResponse(user))
}

// getByID godoc
//
// @Summary Получить профиль по ID
// @Description Возвращает профиль другого пользователя. Требуется JWT cookie.
// @Tags users
// @Produce json
// @Param id path string true "UUID пользователя"
// @Success 200 {object} userResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/{id} [get]
func (h *Handler) getByID(c *echo.Context) error {
	rawID := c.Param("id")
	if rawID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid user id")
	}

	user, err := h.service.GetByID(c.Request().Context(), rawID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, makeUserResponse(user))
}

func setAuthCookies(c *echo.Context, tokens *auth.TokenPair) {
	c.SetCookie(&http.Cookie{
		Name:     auth.AccessCookieName,
		Value:    tokens.AccessToken,
		Path:     auth.AccessCookiePath,
		Expires:  tokens.AccessExpiresAt,
		MaxAge:   cookieMaxAge(tokens.AccessExpiresAt),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	c.SetCookie(&http.Cookie{
		Name:     auth.RefreshCookieName,
		Value:    tokens.RefreshToken,
		Path:     auth.RefreshCookiePath,
		Expires:  tokens.RefreshExpiresAt,
		MaxAge:   cookieMaxAge(tokens.RefreshExpiresAt),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
}

func clearAuthCookies(c *echo.Context) {
	expiresAt := time.Unix(1, 0)

	c.SetCookie(&http.Cookie{
		Name:     auth.AccessCookieName,
		Value:    "",
		Path:     auth.AccessCookiePath,
		Expires:  expiresAt,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	c.SetCookie(&http.Cookie{
		Name:     auth.RefreshCookieName,
		Value:    "",
		Path:     auth.RefreshCookiePath,
		Expires:  expiresAt,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
}

func cookieMaxAge(expiresAt time.Time) int {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 1 {
		return 1
	}
	return maxAge
}

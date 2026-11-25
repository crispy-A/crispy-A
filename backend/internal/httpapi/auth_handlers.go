package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"crispy-a/backend/internal/auth"
)

type cookieSettings struct {
	name   string
	path   string
	secure bool
}

type authHandler struct {
	service *auth.Service
	cookie  cookieSettings
	logger  *slog.Logger
}

func newAuthHandler(service *auth.Service, cookie cookieSettings, logger *slog.Logger) *authHandler {
	return &authHandler{service: service, cookie: cookie, logger: logger}
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginResponse struct {
	AccessToken string    `json:"accessToken"`
	ExpiresIn   int64     `json:"expiresIn"`
	User        auth.User `json:"user"`
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := decodeJSON(w, r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "请求体必须是合法的 JSON 对象")
		return
	}
	request.Email = strings.TrimSpace(request.Email)
	if request.Email == "" || request.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "邮箱和密码不能为空")
		return
	}

	result, err := h.service.Login(r.Context(), request.Email, request.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "邮箱或密码不正确")
		return
	}
	if err != nil {
		h.logger.Error("login failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "登录失败，请稍后再试")
		return
	}

	h.setRefreshCookie(w, result.RefreshToken, result.RefreshExpiry)
	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken: result.AccessToken,
		ExpiresIn:   int64(h.service.AccessTokenTTL().Seconds()),
		User:        result.User,
	})
}

func (h *authHandler) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.cookie.name)
	if err != nil || cookie.Value == "" {
		writeError(w, http.StatusUnauthorized, "missing_refresh_token", "登录状态已失效，请重新登录")
		return
	}

	result, err := h.service.Refresh(r.Context(), cookie.Value)
	if errors.Is(err, auth.ErrInvalidRefreshToken) {
		h.clearRefreshCookie(w)
		writeError(w, http.StatusUnauthorized, "invalid_refresh_token", "登录状态已失效，请重新登录")
		return
	}
	if err != nil {
		h.logger.Error("refresh failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "刷新登录状态失败，请稍后再试")
		return
	}

	h.setRefreshCookie(w, result.RefreshToken, result.RefreshExpiry)
	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken: result.AccessToken,
		ExpiresIn:   int64(h.service.AccessTokenTTL().Seconds()),
		User:        result.User,
	})
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(h.cookie.name); err == nil && cookie.Value != "" {
		if err := h.service.Logout(r.Context(), cookie.Value); err != nil {
			h.logger.Error("logout failed", "error", err)
			writeError(w, http.StatusInternalServerError, "internal_error", "登出失败，请稍后再试")
			return
		}
	}
	h.clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

type meResponse struct {
	User auth.User `json:"user"`
}

func (h *authHandler) me(w http.ResponseWriter, r *http.Request) {
	identity, ok := identityFrom(r.Context())
	if !ok {
		unauthorized(w)
		return
	}

	user, err := h.service.Profile(r.Context(), identity.ID)
	if errors.Is(err, auth.ErrUserNotFound) {
		unauthorized(w)
		return
	}
	if err != nil {
		h.logger.Error("load profile failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "读取账号信息失败")
		return
	}

	writeJSON(w, http.StatusOK, meResponse{User: user})
}

func (h *authHandler) setRefreshCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 1 {
		maxAge = 1
	}
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookie.name,
		Value:    token,
		Path:     h.cookie.path,
		Expires:  expiresAt,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   h.cookie.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (h *authHandler) clearRefreshCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookie.name,
		Value:    "",
		Path:     h.cookie.path,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookie.secure,
		SameSite: http.SameSiteLaxMode,
	})
}

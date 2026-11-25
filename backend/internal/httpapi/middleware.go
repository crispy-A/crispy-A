package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"crispy-a/backend/internal/auth"
)

type contextKey string

const identityContextKey contextKey = "identity"

// requestLogger 统一输出结构化访问日志，敏感路径（登录）不记录请求体。
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startedAt := time.Now()
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(wrapped, r)
			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", wrapped.Status(),
				"bytes", wrapped.BytesWritten(),
				"duration_ms", time.Since(startedAt).Milliseconds(),
				"remote_ip", r.RemoteAddr,
				"request_id", middleware.GetReqID(r.Context()),
			)
		})
	}
}

// recoverer 把 panic 转成 500 JSON，避免 chi 默认日志绕过结构化日志。
func recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic recovered",
						"panic", recovered,
						"stack", string(debug.Stack()),
						"request_id", middleware.GetReqID(r.Context()))
					writeError(w, http.StatusInternalServerError, "internal_error", "服务内部错误")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// requireAuth 校验 Bearer access token，并把身份放入请求上下文。
func requireAuth(issuer *auth.TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawToken, ok := bearerToken(r)
			if !ok {
				unauthorized(w)
				return
			}
			identity, err := issuer.Parse(rawToken)
			if err != nil {
				unauthorized(w)
				return
			}
			ctx := context.WithValue(r.Context(), identityContextKey, identity)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	header := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	if token == "" {
		return "", false
	}
	return token, true
}

func identityFrom(ctx context.Context) (auth.Identity, bool) {
	identity, ok := ctx.Value(identityContextKey).(auth.Identity)
	return identity, ok
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="crispy-a"`)
	writeError(w, http.StatusUnauthorized, "unauthorized", "缺少或无效的访问令牌")
}

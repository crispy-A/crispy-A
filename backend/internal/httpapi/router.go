package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"crispy-a/backend/internal/auth"
	"crispy-a/backend/internal/config"
)

// Version 是脚手架版本号，由 /api/health 返回。
const Version = "0.1.0"

// NewRouter 组装全部 HTTP 路由与中间件。
func NewRouter(cfg config.Config, pool *pgxpool.Pool, service *auth.Service, issuer *auth.TokenIssuer, logger *slog.Logger) http.Handler {
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(requestLogger(logger))
	router.Use(recoverer(logger))
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSOrigins,
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	handler := newAuthHandler(service, cookieSettings{
		name:   "crispy_refresh",
		path:   "/api/auth",
		secure: cfg.CookieSecure,
	}, logger)

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "接口不存在")
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "请求方法不允许")
	})

	router.Route("/api", func(api chi.Router) {
		api.Get("/health", newHealthHandler(pool, logger))

		api.Route("/auth", func(authRouter chi.Router) {
			authRouter.Post("/login", handler.login)
			authRouter.Post("/refresh", handler.refresh)
			authRouter.Post("/logout", handler.logout)
		})

		api.Group(func(admin chi.Router) {
			admin.Use(requireAuth(issuer))
			admin.Get("/admin/me", handler.me)
		})
	})

	return router
}

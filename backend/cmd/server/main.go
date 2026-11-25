// Command server 是个人网站脚手架的后端入口。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"crispy-a/backend/internal/auth"
	"crispy-a/backend/internal/config"
	"crispy-a/backend/internal/database"
	"crispy-a/backend/internal/httpapi"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := newLogger(cfg.Env)
	slog.SetDefault(logger)

	// 收到 Ctrl+C / SIGTERM 后先停止接收新请求，再关闭连接池。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	applied, err := database.Migrate(ctx, pool)
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	if applied > 0 {
		logger.Info("database migrations applied", "count", applied)
	}

	issuer := auth.NewTokenIssuer(cfg.JWTSecret, "crispy-a", cfg.AccessTokenTTL)
	service, err := auth.NewService(auth.NewStore(pool), issuer, cfg.RefreshTokenTTL, logger)
	if err != nil {
		return fmt.Errorf("init auth service: %w", err)
	}

	if cfg.AdminEmail != "" {
		seeded, err := service.EnsureAdmin(ctx, cfg.AdminEmail, cfg.AdminPassword)
		if err != nil {
			return fmt.Errorf("seed admin account: %w", err)
		}
		if !seeded {
			logger.Info("admin account already exists; skip seeding", "email", cfg.AdminEmail)
		}
	} else {
		logger.Warn("ADMIN_EMAIL/ADMIN_PASSWORD not configured; no admin account seeded")
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewRouter(cfg, pool, service, issuer, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr, "env", cfg.Env, "version", httpapi.Version)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		logger.Info("server stopped")
	}
	return nil
}

func newLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	if env == "development" {
		level = slog.LevelDebug
	}
	options := &slog.HandlerOptions{Level: level}
	if env == "production" {
		return slog.New(slog.NewJSONHandler(os.Stdout, options))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, options))
}

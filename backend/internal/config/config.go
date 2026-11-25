// Package config 负责从环境变量加载并校验服务配置。
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config 是服务运行所需的全部外部配置。
// 所有值都来自环境变量；本地开发可写在 backend/.env 中（该文件不提交）。
type Config struct {
	Env             string
	HTTPAddr        string
	DatabaseURL     string
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	AdminEmail      string
	AdminPassword   string
	CORSOrigins     []string
	CookieSecure    bool
}

// Load 读取环境变量并做启动前校验，任何一项不合法都直接失败，避免带病运行。
func Load() (Config, error) {
	// .env 只服务本地开发；文件不存在时继续使用真实环境变量。
	if err := loadDotEnv(".env"); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Env:           strings.TrimSpace(getEnv("APP_ENV", "development")),
		HTTPAddr:      strings.TrimSpace(getEnv("HTTP_ADDR", ":8080")),
		DatabaseURL:   strings.TrimSpace(os.Getenv("DATABASE_URL")),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		AdminEmail:    strings.TrimSpace(os.Getenv("ADMIN_EMAIL")),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		CORSOrigins:   splitList(getEnv("CORS_ORIGINS", "http://localhost:5173")),
	}

	accessTTL, err := parseDuration("ACCESS_TOKEN_TTL", "15m")
	if err != nil {
		return Config{}, err
	}
	cfg.AccessTokenTTL = accessTTL

	refreshTTL, err := parseDuration("REFRESH_TOKEN_TTL", "168h")
	if err != nil {
		return Config{}, err
	}
	cfg.RefreshTokenTTL = refreshTTL

	cfg.CookieSecure = cfg.Env == "production"
	if raw := strings.TrimSpace(os.Getenv("COOKIE_SECURE")); raw != "" {
		secure, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("COOKIE_SECURE must be true or false: %w", err)
		}
		cfg.CookieSecure = secure
	}

	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, errors.New("JWT_SECRET must be at least 32 characters")
	}
	if isTemplateValue(cfg.JWTSecret) {
		return Config{}, errors.New("JWT_SECRET still looks like a template value; generate a random secret (openssl rand -base64 48)")
	}
	if (cfg.AdminEmail == "") != (cfg.AdminPassword == "") {
		return Config{}, errors.New("ADMIN_EMAIL and ADMIN_PASSWORD must be set together")
	}
	if cfg.AdminPassword != "" && len(cfg.AdminPassword) < 10 {
		return Config{}, errors.New("ADMIN_PASSWORD must be at least 10 characters")
	}
	if cfg.AdminPassword != "" && len(cfg.AdminPassword) > 72 {
		return Config{}, errors.New("ADMIN_PASSWORD must be at most 72 bytes (bcrypt limit)")
	}
	if cfg.AdminPassword != "" && isTemplateValue(cfg.AdminPassword) {
		return Config{}, errors.New("ADMIN_PASSWORD still looks like a template value; choose a real password")
	}
	if cfg.Env != "development" && cfg.Env != "test" && cfg.Env != "production" {
		return Config{}, fmt.Errorf("APP_ENV must be development, test or production, got %q", cfg.Env)
	}
	if len(cfg.CORSOrigins) == 0 {
		return Config{}, errors.New("CORS_ORIGINS must contain at least one origin")
	}

	return cfg, nil
}

// loadDotEnv 读取 .env 并注入尚未存在的环境变量。
// Windows 编辑器常写入 UTF-8 BOM，godotenv 会因此解析失败，所以先去 BOM；
// 解析错误直接上抛，避免“变量全空但服务照常启动”的静默故障。
func loadDotEnv(path string) error {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	values, err := godotenv.Unmarshal(strings.TrimPrefix(string(raw), "\uFEFF"))
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	for key, value := range values {
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("apply %s from .env: %w", key, err)
		}
	}
	return nil
}

// isTemplateValue 拒绝公开模板里的示例值，避免“复制 .env.example 就能上线”的弱配置。
func isTemplateValue(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	return strings.Contains(normalized, "change-me") ||
		strings.Contains(normalized, "changeme") ||
		strings.Contains(normalized, "replace-with") ||
		strings.Contains(normalized, "your-secret") ||
		strings.Contains(normalized, "your-password")
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseDuration(key, fallback string) (time.Duration, error) {
	value := getEnv(key, fallback)
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration such as 15m or 168h: %w", key, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return parsed, nil
}

func splitList(raw string) []string {
	var items []string
	for _, item := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(item); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

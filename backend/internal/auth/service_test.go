package auth_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"crispy-a/backend/internal/auth"
	"crispy-a/backend/internal/database"
)

// 集成测试使用真实 PostgreSQL，连接串由 TEST_DATABASE_URL 提供；未设置时跳过。
func newTestFixture(t *testing.T) (*auth.Service, *auth.Store, *auth.TokenIssuer, *pgxpool.Pool) {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set; skipping PostgreSQL integration test")
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)

	if _, err := database.Migrate(ctx, pool); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	store := auth.NewStore(pool)
	issuer := auth.NewTokenIssuer("test-secret-0123456789abcdef0123456789", "crispy-a-test", time.Minute)
	service, err := auth.NewService(store, issuer, time.Hour, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("create auth service: %v", err)
	}
	return service, store, issuer, pool
}

func TestLoginRefreshLogout(t *testing.T) {
	service, store, issuer, pool := newTestFixture(t)
	ctx := context.Background()

	email := fmt.Sprintf("tester-%d@example.com", time.Now().UnixNano())
	password := "super-secret-password"
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user, err := store.CreateUser(ctx, email, passwordHash, "admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM users WHERE id = $1", user.ID); err != nil {
			t.Logf("cleanup user %d: %v", user.ID, err)
		}
	})

	if _, err := service.Login(ctx, email, "wrong-password"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}

	login, err := service.Login(ctx, email, password)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	identity, err := issuer.Parse(login.AccessToken)
	if err != nil {
		t.Fatalf("parse access token: %v", err)
	}
	if identity.ID != user.ID || identity.Email != email {
		t.Fatalf("unexpected identity: %+v", identity)
	}
	if login.RefreshToken == "" {
		t.Fatal("expected refresh token")
	}

	refreshed, err := service.Refresh(ctx, login.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.RefreshToken == login.RefreshToken {
		t.Fatal("refresh token must rotate")
	}
	if _, err := service.Refresh(ctx, login.RefreshToken); !errors.Is(err, auth.ErrInvalidRefreshToken) {
		t.Fatalf("rotated refresh token must be rejected, got %v", err)
	}
	if _, err := issuer.Parse(refreshed.AccessToken); err != nil {
		t.Fatalf("parse refreshed access token: %v", err)
	}

	if err := service.Logout(ctx, refreshed.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if _, err := service.Refresh(ctx, refreshed.RefreshToken); !errors.Is(err, auth.ErrInvalidRefreshToken) {
		t.Fatalf("logged out refresh token must be rejected, got %v", err)
	}
}

func TestLoginUnknownEmail(t *testing.T) {
	service, _, _, _ := newTestFixture(t)

	_, err := service.Login(context.Background(), fmt.Sprintf("missing-%d@example.com", time.Now().UnixNano()), "whatever-password")
	if !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials for unknown email, got %v", err)
	}
}

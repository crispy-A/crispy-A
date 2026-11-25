package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrSessionNotFound = errors.New("session not found")
)

// User 是返回给客户端的账号视图，永远不包含密码哈希。
type User struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// Store 封装账号与会话的 SQL 访问。
type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// FindUserForLogin 返回账号及密码哈希；邮箱不存在时返回 ErrUserNotFound。
func (s *Store) FindUserForLogin(ctx context.Context, email string) (User, string, error) {
	var user User
	var passwordHash string
	err := s.pool.QueryRow(ctx,
		`SELECT id, email, role, password_hash FROM users WHERE lower(email) = lower($1)`, email).
		Scan(&user.ID, &user.Email, &user.Role, &passwordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrUserNotFound
	}
	if err != nil {
		return User{}, "", fmt.Errorf("query user by email: %w", err)
	}
	return user, passwordHash, nil
}

func (s *Store) FindUserByID(ctx context.Context, id int64) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `SELECT id, email, role FROM users WHERE id = $1`, id).
		Scan(&user.ID, &user.Email, &user.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("query user by id: %w", err)
	}
	return user, nil
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var count int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count users: %w", err)
	}
	return count, nil
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash, role string) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role) VALUES ($1, $2, $3) RETURNING id, email, role`,
		email, passwordHash, role).
		Scan(&user.ID, &user.Email, &user.Role)
	if err != nil {
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID int64, tokenHash []byte, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

// RotateSession 在一个事务里作废旧 refresh token 并写入新会话，避免并发复用。
func (s *Store) RotateSession(ctx context.Context, oldHash, newHash []byte, expiresAt time.Time) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin rotate session: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var user User
	err = tx.QueryRow(ctx, `
		SELECT s.user_id, u.email, u.role
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now()
		FOR UPDATE OF s`, oldHash).
		Scan(&user.ID, &user.Email, &user.Role)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrSessionNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("select session for rotation: %w", err)
	}

	if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at = now() WHERE token_hash = $1`, oldHash); err != nil {
		return User{}, fmt.Errorf("revoke rotated session: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		user.ID, newHash, expiresAt); err != nil {
		return User{}, fmt.Errorf("insert rotated session: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit rotate session: %w", err)
	}
	return user, nil
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

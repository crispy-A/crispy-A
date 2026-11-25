package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// Service 承载登录、refresh 轮换、登出与管理员初始化等业务规则。
type Service struct {
	store      *Store
	issuer     *TokenIssuer
	refreshTTL time.Duration
	dummyHash  string
	logger     *slog.Logger
}

// LoginResult 是登录/刷新的结果；RefreshToken 只允许写入 HttpOnly cookie。
type LoginResult struct {
	AccessToken       string
	AccessTokenExpiry time.Time
	RefreshToken      string
	RefreshExpiry     time.Time
	User              User
}

func NewService(store *Store, issuer *TokenIssuer, refreshTTL time.Duration, logger *slog.Logger) (*Service, error) {
	if store == nil || issuer == nil {
		return nil, errors.New("store and token issuer are required")
	}
	if refreshTTL <= 0 {
		return nil, errors.New("refresh token TTL must be positive")
	}
	if logger == nil {
		logger = slog.Default()
	}
	// 账号不存在时也做一次等成本校验，避免通过响应时间枚举账号是否存在。
	dummy, err := bcrypt.GenerateFromPassword([]byte("timing-equalizer-dummy-password"), passwordCost)
	if err != nil {
		return nil, fmt.Errorf("prepare dummy password hash: %w", err)
	}
	return &Service{store: store, issuer: issuer, refreshTTL: refreshTTL, dummyHash: string(dummy), logger: logger}, nil
}

// Login 校验邮箱密码，签发 access token 并落库一个 refresh 会话。
func (s *Service) Login(ctx context.Context, email, password string) (LoginResult, error) {
	user, passwordHash, err := s.store.FindUserForLogin(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		_ = verifyPassword(s.dummyHash, password)
		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf("find user for login: %w", err)
	}
	if !verifyPassword(passwordHash, password) {
		return LoginResult{}, ErrInvalidCredentials
	}

	refreshToken, refreshHash, refreshExpiry, err := newRefreshToken(s.refreshTTL)
	if err != nil {
		return LoginResult{}, err
	}
	if err := s.store.CreateSession(ctx, user.ID, refreshHash, refreshExpiry); err != nil {
		return LoginResult{}, err
	}

	accessToken, accessExpiry, err := s.issuer.Issue(user.ID, user.Email)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		AccessToken:       accessToken,
		AccessTokenExpiry: accessExpiry,
		RefreshToken:      refreshToken,
		RefreshExpiry:     refreshExpiry,
		User:              user,
	}, nil
}

// Refresh 轮换 refresh token：旧会话作废，新会话生效。
func (s *Service) Refresh(ctx context.Context, rawToken string) (LoginResult, error) {
	if rawToken == "" {
		return LoginResult{}, ErrInvalidRefreshToken
	}

	newToken, newHash, newExpiry, err := newRefreshToken(s.refreshTTL)
	if err != nil {
		return LoginResult{}, err
	}
	user, err := s.store.RotateSession(ctx, hashToken(rawToken), newHash, newExpiry)
	if errors.Is(err, ErrSessionNotFound) {
		return LoginResult{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return LoginResult{}, err
	}

	accessToken, accessExpiry, err := s.issuer.Issue(user.ID, user.Email)
	if err != nil {
		return LoginResult{}, err
	}

	return LoginResult{
		AccessToken:       accessToken,
		AccessTokenExpiry: accessExpiry,
		RefreshToken:      newToken,
		RefreshExpiry:     newExpiry,
		User:              user,
	}, nil
}

// Logout 作废当前 refresh 会话；token 不存在时保持幂等。
func (s *Service) Logout(ctx context.Context, rawToken string) error {
	if rawToken == "" {
		return nil
	}
	return s.store.RevokeSession(ctx, hashToken(rawToken))
}

// Profile 用 access token 中的用户 ID 读取最新账号信息。
func (s *Service) Profile(ctx context.Context, userID int64) (User, error) {
	return s.store.FindUserByID(ctx, userID)
}

// EnsureAdmin 仅在 users 表为空时创建管理员，重复启动不会覆盖已有账号。
func (s *Service) EnsureAdmin(ctx context.Context, email, password string) (bool, error) {
	count, err := s.store.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, nil
	}

	passwordHash, err := HashPassword(password)
	if err != nil {
		return false, err
	}
	user, err := s.store.CreateUser(ctx, email, passwordHash, "admin")
	if err != nil {
		return false, err
	}
	s.logger.Info("admin account created", "user_id", user.ID, "email", user.Email)
	return true, nil
}

func (s *Service) AccessTokenTTL() time.Duration { return s.issuer.TTL() }

func newRefreshToken(ttl time.Duration) (string, []byte, time.Time, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, time.Time{}, fmt.Errorf("generate refresh token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), time.Now().Add(ttl), nil
}

// hashToken 只保存 token 的 SHA-256，数据库泄露也无法直接复用。
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

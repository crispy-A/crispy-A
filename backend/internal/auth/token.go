package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Identity 是 access token 解析后的调用者身份。
type Identity struct {
	ID    int64
	Email string
}

// Claims 是 access token 的载荷，sub 存用户 ID。
type Claims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// TokenIssuer 负责签发与校验 HS256 access token。
type TokenIssuer struct {
	secret []byte
	issuer string
	ttl    time.Duration
}

func NewTokenIssuer(secret, issuer string, ttl time.Duration) *TokenIssuer {
	return &TokenIssuer{secret: []byte(secret), issuer: issuer, ttl: ttl}
}

// TTL 返回 access token 有效期，供接口返回 expiresIn。
func (t *TokenIssuer) TTL() time.Duration { return t.ttl }

// Issue 为指定用户签发 access token。
func (t *TokenIssuer) Issue(userID int64, email string) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(t.ttl)
	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			Issuer:    t.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// Parse 校验签名算法、签发方与过期时间，并返回调用者身份。
func (t *TokenIssuer) Parse(raw string) (Identity, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims,
		func(*jwt.Token) (any, error) { return t.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(t.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(2*time.Second),
	)
	if err != nil {
		return Identity{}, fmt.Errorf("parse access token: %w", err)
	}
	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return Identity{}, errors.New("access token subject is not a user id")
	}
	return Identity{ID: userID, Email: claims.Email}, nil
}

package auth

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// passwordCost 12 在本地登录耗时约 100-200ms，足以抬高离线爆破成本。
const passwordCost = 12

// HashPassword 生成 bcrypt 哈希，用于管理员初始化与后续改密。
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), passwordCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func verifyPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func GenerateAuthorizationCode() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate authorization code: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

func ValidateAuthorizationCode(code string) error {
	if len(code) != 32 {
		return fmt.Errorf("%w: authorizationCode must be 32 characters", ErrBadRequest)
	}
	for _, r := range code {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			continue
		}
		return fmt.Errorf("%w: authorizationCode only supports letters and numbers", ErrBadRequest)
	}
	return nil
}

func HashAuthorizationCode(code string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(code), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash authorization code: %w", err)
	}
	return string(hash), nil
}

func CheckAuthorizationCode(hash string, code string) bool {
	if hash == "" || code == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(code)) == nil
}

func MaskAuthorizationCode(code string) string {
	if len(code) <= 8 {
		return strings.Repeat("*", len(code))
	}
	return code[:4] + strings.Repeat("*", len(code)-8) + code[len(code)-4:]
}

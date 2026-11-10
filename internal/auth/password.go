package auth

import (
	"golang.org/x/crypto/bcrypt"
)

const (
	// DefaultCost is the bcrypt work factor (12 is OWASP recommended for password hashing).
	DefaultCost = 12
)

// HashPassword generates a bcrypt hash of the given plaintext password.
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword compares a plaintext password with a bcrypt hash.
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

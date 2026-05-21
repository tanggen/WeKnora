package utils

import (
	"crypto/rand"
	"encoding/base64"
	"regexp"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a plaintext password using bcrypt with the default cost.
// Returns the base64-encoded hash string.
func HashPassword(plainPassword string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword compares a plaintext password against a bcrypt hash.
// Returns nil on match, or an error if they don't match.
func VerifyPassword(hashedPassword, plainPassword string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(plainPassword))
}

// IsStrongPassword validates that a password meets the minimum strength requirements:
// - 8 to 128 characters long
// - Contains at least one uppercase letter (A-Z)
// - Contains at least one lowercase letter (a-z)
// - Contains at least one digit (0-9)
func IsStrongPassword(password string) bool {
	if len(password) < 8 || len(password) > 128 {
		return false
	}
	hasUpper := false
	hasLower := false
	hasDigit := false
	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

// usernamePattern matches the allowed characters in a username: a-z, A-Z, 0-9, underscore.
var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// IsValidUsername checks that a username is 2-64 characters long and contains
// only alphanumeric characters and underscores.
func IsValidUsername(username string) bool {
	if len(username) < 2 || len(username) > 64 {
		return false
	}
	return usernamePattern.MatchString(username)
}

// GenerateRandomPassword creates a cryptographically secure random password
// with the specified length. Uses base64 URL-safe characters.
func GenerateRandomPassword(length int) (string, error) {
	if length <= 0 {
		length = 16
	}
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

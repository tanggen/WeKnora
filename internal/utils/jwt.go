package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// jwtSecret is cached from the JWT_SECRET / JWT_SECRET_KEY environment variable.
var jwtSecret []byte
var jwtSecretLoaded bool

// GetJWTSecret returns the JWT signing key. It reads from JWT_SECRET_KEY first,
// then falls back to JWT_SECRET, and finally to a default. The key is loaded once
// and cached for the lifetime of the process.
func GetJWTSecret() []byte {
	if jwtSecretLoaded {
		return jwtSecret
	}
	jwtSecretLoaded = true
	key := strings.TrimSpace(os.Getenv("JWT_SECRET_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	}
	if key == "" {
		key = "weknora-jwt-secret-key-change-in-production"
	}
	jwtSecret = []byte(key)
	return jwtSecret
}

// CustomClaims extends the standard JWT claims with application-specific fields.
type CustomClaims struct {
	jwt.RegisteredClaims
	// Sub is the user ID
	// TenantID is the tenant the user belongs to
	TenantID uint64 `json:"tid"`
	// Role is the user's role within the tenant
	Role string `json:"role"`
	// Permissions is the list of permission codes
	Permissions []string `json:"perms"`
	// TokenType: access, refresh, or reset
	TokenType string `json:"type"`
}

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
	TokenTypeReset   = "reset"
)

// TokenExpiry defines the expiry durations for each token type.
var (
	AccessTokenExpiry  = 24 * time.Hour
	RefreshTokenExpiry = 7 * 24 * time.Hour
	ResetTokenExpiry   = 5 * time.Minute
)

// GenerateAccessToken creates a signed JWT access token with the supplied claims.
func GenerateAccessToken(userID string, tenantID uint64, role string, permissions []string) (string, string, error) {
	now := time.Now()
	jti := uuid.New().String()
	claims := &CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenExpiry)),
		},
		TenantID:    tenantID,
		Role:        role,
		Permissions: permissions,
		TokenType:   TokenTypeAccess,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(GetJWTSecret())
	if err != nil {
		return "", "", fmt.Errorf("failed to sign access token: %w", err)
	}
	return signed, jti, nil
}

// GenerateRefreshToken creates a signed JWT refresh token.
func GenerateRefreshToken(userID string) (string, string, error) {
	now := time.Now()
	jti := uuid.New().String()
	claims := &CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(RefreshTokenExpiry)),
		},
		TokenType: TokenTypeRefresh,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(GetJWTSecret())
	if err != nil {
		return "", "", fmt.Errorf("failed to sign refresh token: %w", err)
	}
	return signed, jti, nil
}

// GenerateResetToken creates a signed JWT reset token for first-login password changes.
// It is short-lived (5 minutes) and only carries the user ID and token type.
func GenerateResetToken(userID string) (string, string, error) {
	now := time.Now()
	jti := uuid.New().String()
	claims := &CustomClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ResetTokenExpiry)),
		},
		TokenType: TokenTypeReset,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(GetJWTSecret())
	if err != nil {
		return "", "", fmt.Errorf("failed to sign reset token: %w", err)
	}
	return signed, jti, nil
}

// ParseToken validates and parses a JWT token string, returning the custom claims.
// It accepts access, refresh, and reset tokens. The caller should verify the token
// type matches the expected use case.
func ParseToken(tokenString string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return GetJWTSecret(), nil
	})
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

// GenerateSecureRandomString creates a URL-safe random string of the given byte length.
func GenerateSecureRandomString(length int) (string, error) {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

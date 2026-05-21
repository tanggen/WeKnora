package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
)

type oidcAuthorizationState struct {
	Nonce       string `json:"nonce"`
	RedirectURI string `json:"redirect_uri,omitempty"`
}

var (
	jwtSecretOnce sync.Once
	jwtSecret     string
)

// getJwtSecret retrieves the JWT secret from the environment, falling back to a securely generated random secret.
func getJwtSecret() string {
	jwtSecretOnce.Do(func() {
		if envSecret := strings.TrimSpace(os.Getenv("JWT_SECRET")); envSecret != "" {
			jwtSecret = envSecret
			return
		}

		randomBytes := make([]byte, 32)
		if _, err := rand.Read(randomBytes); err != nil {
			panic(fmt.Sprintf("failed to generate JWT secret: %v", err))
		}
		jwtSecret = base64.StdEncoding.EncodeToString(randomBytes)
	})

	return jwtSecret
}

// userService implements the UserService interface
type userService struct {
	userRepo       interfaces.UserRepository
	tokenRepo      interfaces.AuthTokenRepository
	tenantService  interfaces.TenantService
	tenantUserRepo interfaces.TenantUserRepository
	statsRepo      interfaces.TenantStatsRepository
	redisClient    *redis.Client
	config         *config.Config
}

// NewUserService creates a new user service instance
func NewUserService(
	configInfo *config.Config,
	userRepo interfaces.UserRepository,
	tokenRepo interfaces.AuthTokenRepository,
	tenantService interfaces.TenantService,
	tenantUserRepo interfaces.TenantUserRepository,
	statsRepo interfaces.TenantStatsRepository,
	redisClient *redis.Client,
) interfaces.UserService {
	return &userService{
		userRepo:       userRepo,
		tokenRepo:      tokenRepo,
		tenantService:  tenantService,
		tenantUserRepo: tenantUserRepo,
		statsRepo:      statsRepo,
		redisClient:    redisClient,
		config:         configInfo,
	}
}

// Register creates a new user account with a trial tenant.
// It performs the following side effects within a transaction:
//   - Creates a users row (password_expired=false)
//   - Creates a tenants row (plan_id="trial", trial_expires_at=now+3days)
//   - Creates a tenant_users row (role="tenant_admin")
//   - Initializes a tenant_stats row (user_count=1)
// Returns the created user; tokens are generated separately by the handler.
func (s *userService) Register(ctx context.Context, req *types.RegisterRequest) (*types.User, error) {
	logger.Info(ctx, "Start user registration")

	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, errors.New("username, email and password are required")
	}

	// Validate password strength
	if !secutils.IsStrongPassword(req.Password) {
		return nil, errors.New("password must be 8-128 characters with at least one uppercase letter, one lowercase letter, and one digit")
	}

	// Check uniqueness (username, email, phone)
	if existing, _ := s.userRepo.GetUserByEmail(ctx, req.Email); existing != nil {
		return nil, errors.New("user with this email already exists")
	}
	if existing, _ := s.userRepo.GetUserByUsername(ctx, req.Username); existing != nil {
		return nil, errors.New("user with this username already exists")
	}
	if strings.TrimSpace(req.Phone) != "" {
		if existing, _ := s.userRepo.GetUserByPhone(ctx, strings.TrimSpace(req.Phone)); existing != nil {
			return nil, errors.New("user with this phone already exists")
		}
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		logger.Errorf(ctx, "Failed to hash password: %v", err)
		return nil, errors.New("failed to process password")
	}

	// Determine tenant name
	tenantName := strings.TrimSpace(req.TenantName)
	if tenantName == "" {
		tenantName = fmt.Sprintf("%s的知识空间", secutils.SanitizeForLog(req.Username))
	}

	// Trial expiry: now + 3 days
	trialExpiresAt := time.Now().Add(72 * time.Hour)

	// Create default tenant for the user with trial plan
	tenant := &types.Tenant{
		Name:           tenantName,
		Description:    "Default workspace",
		Status:         "active",
		PlanID:         "trial",
		TrialExpiresAt: &trialExpiresAt,
	}

	createdTenant, err := s.tenantService.CreateTenant(ctx, tenant)
	if err != nil {
		logger.Errorf(ctx, "Failed to create tenant")
		return nil, errors.New("failed to create workspace")
	}

	// Create user
	user := &types.User{
		ID:              uuid.New().String(),
		Username:        req.Username,
		Email:           req.Email,
		Phone:           strings.TrimSpace(req.Phone),
		PasswordHash:    string(hashedPassword),
		TenantID:        createdTenant.ID,
		PasswordExpired: false, // Self-registered users set their own password
		IsActive:        true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	err = s.userRepo.CreateUser(ctx, user)
	if err != nil {
		logger.Errorf(ctx, "Failed to create user: %v", err)
		return nil, errors.New("failed to create user")
	}

	// Create tenant_users row: self-registered user is tenant_admin
	if s.tenantUserRepo != nil {
		tenantUser := &types.TenantUser{
			TenantID:  createdTenant.ID,
			UserID:    user.ID,
			Role:      types.RoleTenantAdmin,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := s.tenantUserRepo.CreateTenantUser(ctx, tenantUser); err != nil {
			logger.Warnf(ctx, "Failed to create tenant_users row: %v", err)
		}
	}

	// Initialize tenant_stats
	if s.statsRepo != nil {
		if err := s.statsRepo.IncrementUserCount(ctx, createdTenant.ID, 1); err != nil {
			logger.Warnf(ctx, "Failed to initialize tenant_stats: %v", err)
		}
	}

	logger.Infof(ctx, "User registered successfully: %s (tenant: %d)", secutils.SanitizeForLog(user.Email), createdTenant.ID)
	return user, nil
}

// Login authenticates a user and returns tokens.
// Uses smart account routing: email (contains @), phone (11 digits), or username.
// If password_expired is true, returns code "FIRST_LOGIN_RESET" with a reset_token (HTTP 423).
func (s *userService) Login(ctx context.Context, req *types.LoginRequest) (*types.LoginResponse, error) {
	logger.Info(ctx, "Start user login")

	account := req.EffectiveAccount()
	if account == "" {
		return &types.LoginResponse{
			Success: false,
			Message: "Account is required",
		}, nil
	}

	// Get user by smart account routing
	user, err := s.userRepo.FindByAccount(ctx, account)
	if err != nil || user == nil {
		logger.Warnf(ctx, "User not found for account: %s", secutils.SanitizeForLog(account))
		return &types.LoginResponse{
			Success: false,
			Message: "Invalid account or password",
		}, nil
	}

	// Check if user is active
	if !user.IsActive {
		logger.Warn(ctx, "User account is disabled")
		return &types.LoginResponse{
			Success: false,
			Message: "Account is disabled",
		}, nil
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		logger.Warn(ctx, "Password verification failed")
		return &types.LoginResponse{
			Success: false,
			Message: "Invalid account or password",
		}, nil
	}
	logger.Info(ctx, "Password verification successful")

	// Check if password is expired (first login or admin-created users)
	if user.PasswordExpired {
		logger.Infof(ctx, "User %s password expired, generating reset token", user.ID)
		resetToken, _, genErr := secutils.GenerateResetToken(user.ID)
		if genErr != nil {
			logger.Errorf(ctx, "Failed to generate reset token: %v", genErr)
			return &types.LoginResponse{
				Success: false,
				Message: "Login failed",
			}, nil
		}
		return &types.LoginResponse{
			Success:    false,
			Code:       "FIRST_LOGIN_RESET",
			Message:    "首次登录或密码已过期，请修改密码",
			User:       user,
			ResetToken: resetToken,
		}, nil
	}

	// Generate tokens
	logger.Info(ctx, "Generating tokens")
	accessToken, refreshToken, err := s.GenerateTokens(ctx, user)
	if err != nil {
		logger.Errorf(ctx, "Failed to generate tokens: %v", err)
		return &types.LoginResponse{
			Success: false,
			Message: "Login failed",
		}, nil
	}
	logger.Info(ctx, "Tokens generated successfully")

	// Get tenant information
	tenant, err := s.tenantService.GetTenantByID(ctx, user.TenantID)
	if err != nil {
		logger.Warn(ctx, "Failed to get tenant info")
	} else {
		logger.Info(ctx, "Tenant information retrieved successfully")
	}

	logger.Infof(ctx, "User logged in successfully: %s", secutils.SanitizeForLog(user.Email))
	return &types.LoginResponse{
		Success:      true,
		Message:      "Login successful",
		User:         user,
		Tenant:       tenant,
		Token:        accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// GetOIDCAuthorizationURL builds the OIDC authorization URL.
func (s *userService) GetOIDCAuthorizationURL(ctx context.Context, redirectURI string) (*types.OIDCAuthURLResponse, error) {
	cfg, err := s.getOIDCConfig(ctx)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(redirectURI) == "" {
		return nil, errors.New("redirect_uri is required")
	}

	nonce, err := generateRandomString(24)
	if err != nil {
		return nil, fmt.Errorf("failed to generate state: %w", err)
	}

	state, err := encodeOIDCAuthorizationState(&oidcAuthorizationState{
		Nonce:       nonce,
		RedirectURI: strings.TrimSpace(redirectURI),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode OIDC state: %w", err)
	}

	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", cfg.ClientID)
	query.Set("redirect_uri", redirectURI)
	query.Set("scope", strings.Join(cfg.Scopes, " "))
	query.Set("state", state)

	authURL := cfg.AuthorizationEndpoint
	if strings.Contains(authURL, "?") {
		authURL += "&" + query.Encode()
	} else {
		authURL += "?" + query.Encode()
	}

	return &types.OIDCAuthURLResponse{
		Success:             true,
		ProviderDisplayName: cfg.ProviderDisplayName,
		AuthorizationURL:    authURL,
		State:               state,
	}, nil
}

// LoginWithOIDC exchanges code for tokens, loads user info, provisions user if needed, and returns local login tokens.
func (s *userService) LoginWithOIDC(ctx context.Context, code, redirectURI string) (*types.OIDCCallbackResponse, error) {
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("code is required")
	}
	if strings.TrimSpace(redirectURI) == "" {
		return nil, errors.New("redirect_uri is required")
	}

	cfg, err := s.getOIDCConfig(ctx)
	if err != nil {
		return nil, err
	}

	tokenResp, err := s.exchangeOIDCCode(ctx, cfg, code, redirectURI)
	if err != nil {
		return nil, err
	}

	userInfo, err := s.resolveOIDCUserInfo(ctx, cfg, tokenResp)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(userInfo.Email) == "" {
		return nil, errors.New("OIDC provider did not return email")
	}

	user, err := s.userRepo.GetUserByEmail(ctx, userInfo.Email)
	if err != nil && !isUserLookupNotFound(err) {
		return nil, fmt.Errorf("failed to query user by email: %w", err)
	}
	isNewUser := false
	if isUserLookupNotFound(err) || user == nil {
		user, err = s.provisionOIDCUser(ctx, userInfo)
		if err != nil {
			return nil, err
		}
		isNewUser = true
	}

	if !user.IsActive {
		return &types.OIDCCallbackResponse{Success: false, Message: "Account is disabled"}, nil
	}

	accessToken, refreshToken, err := s.GenerateTokens(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to generate local tokens: %w", err)
	}

	return &types.OIDCCallbackResponse{
		Success:      true,
		Message:      "登录成功",
		Token:        accessToken,
		RefreshToken: refreshToken,
		IsNewUser:    isNewUser,
	}, nil
}

// GetUserByID gets a user by ID
func (s *userService) GetUserByID(ctx context.Context, id string) (*types.User, error) {
	return s.userRepo.GetUserByID(ctx, id)
}

// GetUserByEmail gets a user by email
func (s *userService) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	return s.userRepo.GetUserByEmail(ctx, email)
}

// GetUserByUsername gets a user by username
func (s *userService) GetUserByUsername(ctx context.Context, username string) (*types.User, error) {
	return s.userRepo.GetUserByUsername(ctx, username)
}

// GetUserByTenantID gets the first user (owner) of a tenant
func (s *userService) GetUserByTenantID(ctx context.Context, tenantID uint64) (*types.User, error) {
	return s.userRepo.GetUserByTenantID(ctx, tenantID)
}

// UpdateUser updates user information
func (s *userService) UpdateUser(ctx context.Context, user *types.User) error {
	user.UpdatedAt = time.Now()
	return s.userRepo.UpdateUser(ctx, user)
}

// DeleteUser deletes a user
func (s *userService) DeleteUser(ctx context.Context, id string) error {
	return s.userRepo.DeleteUser(ctx, id)
}

// ChangePassword changes user password.
// Supports two modes:
//   - Normal mode: oldPassword + newPassword (standard password change)
//   - Reset mode: newPassword only, with reset_token in context (first-login forced change)
// In reset mode, the user ID is extracted from the reset token and old password is not verified.
func (s *userService) ChangePassword(ctx context.Context, userID string, oldPassword, newPassword string) error {
	// Validate password strength
	if !secutils.IsStrongPassword(newPassword) {
		return errors.New("password must be 8-128 characters with at least one uppercase letter, one lowercase letter, and one digit")
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	// In normal mode, verify old password unless it's a reset-token flow
	// (reset-token flow passes empty oldPassword, the token was already validated by middleware/handler)
	if oldPassword != "" {
		err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword))
		if err != nil {
			return errors.New("invalid old password")
		}
		// Check new password is different from old
		if oldPassword == newPassword {
			return errors.New("new password must be different from old password")
		}
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	user.PasswordExpired = false // Clear the expired flag after successful change
	user.UpdatedAt = time.Now()

	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return err
	}

	return nil
}

// ValidatePassword validates user password
func (s *userService) ValidatePassword(ctx context.Context, userID string, password string) error {
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}

	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
}

// GenerateTokens generates access and refresh tokens for user with extended JWT claims.
// The access token includes role, permissions, tenant_id, and jti for blacklist support.
// Tokens are stored in both the database (for backward compatibility) and
// a Redis-based blacklist is supported via the logout flow.
func (s *userService) GenerateTokens(
	ctx context.Context,
	user *types.User,
) (accessToken, refreshToken string, err error) {
	// Generate access token with extended claims
	accessToken, accessJTI, err := secutils.GenerateAccessToken(
		user.ID,
		user.TenantID,
		"",          // role will be set by caller if needed
		[]string{},  // permissions will be set by caller if needed
	)
	if err != nil {
		return "", "", err
	}

	// Generate refresh token
	refreshToken, refreshJTI, err := secutils.GenerateRefreshToken(user.ID)
	if err != nil {
		return "", "", err
	}

	// Store tokens in database (backward compatibility)
	accessTokenRecord := &types.AuthToken{
		ID:        accessJTI,
		UserID:    user.ID,
		Token:     accessToken,
		TokenType: "access_token",
		ExpiresAt: time.Now().Add(secutils.AccessTokenExpiry),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	refreshTokenRecord := &types.AuthToken{
		ID:        refreshJTI,
		UserID:    user.ID,
		Token:     refreshToken,
		TokenType: "refresh_token",
		ExpiresAt: time.Now().Add(secutils.RefreshTokenExpiry),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_ = s.tokenRepo.CreateToken(ctx, accessTokenRecord)
	_ = s.tokenRepo.CreateToken(ctx, refreshTokenRecord)

	return accessToken, refreshToken, nil
}

// blacklistToken adds a JWT token to the Redis blacklist with TTL equal to remaining validity.
func (s *userService) blacklistToken(ctx context.Context, tokenString string) {
	if s.redisClient == nil {
		return
	}
	claims, err := secutils.ParseToken(tokenString)
	if err != nil {
		logger.Warnf(ctx, "Failed to parse token for blacklisting: %v", err)
		return
	}
	jti := claims.ID
	if jti == "" {
		return
	}
	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl <= 0 {
		return
	}
	key := fmt.Sprintf("blacklist:token:%s", jti)
	if err := s.redisClient.Set(ctx, key, "1", ttl).Err(); err != nil {
		logger.Warnf(ctx, "Failed to blacklist token %s: %v", jti, err)
	}
}

// isTokenBlacklisted checks if a JWT token is in the Redis blacklist.
func (s *userService) isTokenBlacklisted(ctx context.Context, tokenString string) bool {
	if s.redisClient == nil {
		return false
	}
	claims, err := secutils.ParseToken(tokenString)
	if err != nil {
		return false
	}
	jti := claims.ID
	if jti == "" {
		return false
	}
	key := fmt.Sprintf("blacklist:token:%s", jti)
	exists, err := s.redisClient.Exists(ctx, key).Result()
	if err != nil {
		return false
	}
	return exists > 0
}

// ValidateToken validates an access token.
// Checks both the database and Redis blacklist. Returns the user if the token is valid.
func (s *userService) ValidateToken(ctx context.Context, tokenString string) (*types.User, error) {
	// Check Redis blacklist first (faster)
	if s.isTokenBlacklisted(ctx, tokenString) {
		return nil, errors.New("token is revoked")
	}

	claims, err := secutils.ParseToken(tokenString)
	if err != nil {
		return nil, errors.New("invalid token")
	}

	if claims.TokenType != secutils.TokenTypeAccess && claims.TokenType != "" {
		// Only accept access tokens for API auth; if TokenType is empty (legacy tokens),
		// fall through to backward-compatible DB check
		if claims.TokenType != secutils.TokenTypeAccess {
			return nil, errors.New("invalid token type")
		}
	}

	// Check if token is revoked in database (backward compatibility)
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, tokenString)
	if err != nil || tokenRecord == nil || tokenRecord.IsRevoked {
		return nil, errors.New("token is revoked")
	}

	return s.userRepo.GetUserByID(ctx, claims.Subject)
}

// RefreshToken refreshes access token using refresh token.
// Checks both Redis blacklist and database for token validity.
func (s *userService) RefreshToken(
	ctx context.Context,
	refreshTokenString string,
) (accessToken, newRefreshToken string, err error) {
	// Check Redis blacklist first
	if s.isTokenBlacklisted(ctx, refreshTokenString) {
		return "", "", errors.New("refresh token is revoked")
	}

	claims, err := secutils.ParseToken(refreshTokenString)
	if err != nil {
		return "", "", errors.New("invalid refresh token")
	}

	if claims.TokenType != secutils.TokenTypeRefresh {
		return "", "", errors.New("not a refresh token")
	}

	// Check if token is revoked in database
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, refreshTokenString)
	if err != nil || tokenRecord == nil || tokenRecord.IsRevoked {
		return "", "", errors.New("refresh token is revoked")
	}

	// Get user
	user, err := s.userRepo.GetUserByID(ctx, claims.Subject)
	if err != nil {
		return "", "", err
	}

	// Revoke old refresh token
	tokenRecord.IsRevoked = true
	_ = s.tokenRepo.UpdateToken(ctx, tokenRecord)

	// Generate new tokens
	return s.GenerateTokens(ctx, user)
}

// RevokeToken revokes a token by adding it to the Redis blacklist and marking it
// as revoked in the database.
func (s *userService) RevokeToken(ctx context.Context, tokenString string) error {
	// Add to Redis blacklist
	s.blacklistToken(ctx, tokenString)

	// Mark as revoked in database
	tokenRecord, err := s.tokenRepo.GetTokenByValue(ctx, tokenString)
	if err != nil {
		return err
	}

	tokenRecord.IsRevoked = true
	tokenRecord.UpdatedAt = time.Now()

	return s.tokenRepo.UpdateToken(ctx, tokenRecord)
}

// GetCurrentUser gets current user from context
func (s *userService) GetCurrentUser(ctx context.Context) (*types.User, error) {
	user, ok := ctx.Value(types.UserContextKey).(*types.User)
	if !ok {
		return nil, errors.New("user not found in context")
	}

	return user, nil
}

// SearchUsers searches users by username or email
func (s *userService) SearchUsers(ctx context.Context, query string, limit int) ([]*types.User, error) {
	if query == "" {
		return []*types.User{}, nil
	}
	return s.userRepo.SearchUsers(ctx, query, limit)
}

type oidcDiscoveryDocument struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserInfoEndpoint      string `json:"userinfo_endpoint"`
}

type oidcTokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
	TokenType   string `json:"token_type"`
}

func (s *userService) getOIDCConfig(ctx context.Context) (*config.OIDCAuthConfig, error) {
	if s.config == nil || s.config.OIDCAuth == nil || !s.config.OIDCAuth.Enable {
		return nil, errors.New("OIDC login is disabled")
	}
	cfg := *s.config.OIDCAuth
	if cfg.UserInfoMapping == nil {
		cfg.UserInfoMapping = &config.OIDCUserInfoMapping{Username: "name", Email: "email"}
	}
	if err := s.populateOIDCEndpoints(ctx, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *userService) populateOIDCEndpoints(ctx context.Context, cfg *config.OIDCAuthConfig) error {
	if strings.TrimSpace(cfg.AuthorizationEndpoint) != "" && strings.TrimSpace(cfg.TokenEndpoint) != "" {
		return nil
	}
	if strings.TrimSpace(cfg.DiscoveryURL) == "" {
		return errors.New("OIDC discovery_url or explicit endpoints are required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.DiscoveryURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create OIDC discovery request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to load OIDC discovery document: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("OIDC discovery request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var doc oidcDiscoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return fmt.Errorf("failed to decode OIDC discovery document: %w", err)
	}
	if cfg.AuthorizationEndpoint == "" {
		cfg.AuthorizationEndpoint = doc.AuthorizationEndpoint
	}
	if cfg.TokenEndpoint == "" {
		cfg.TokenEndpoint = doc.TokenEndpoint
	}
	if cfg.UserInfoEndpoint == "" {
		cfg.UserInfoEndpoint = doc.UserInfoEndpoint
	}
	if cfg.AuthorizationEndpoint == "" || cfg.TokenEndpoint == "" {
		return errors.New("OIDC discovery document missing required endpoints")
	}
	return nil
}

func (s *userService) exchangeOIDCCode(ctx context.Context, cfg *config.OIDCAuthConfig, code, redirectURI string) (*oidcTokenResponse, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", cfg.ClientID)
	form.Set("client_secret", cfg.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create OIDC token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange OIDC code: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("OIDC token exchange failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var tokenResp oidcTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode OIDC token response: %w", err)
	}
	if strings.TrimSpace(tokenResp.AccessToken) == "" && strings.TrimSpace(tokenResp.IDToken) == "" {
		return nil, errors.New("OIDC token response missing access_token and id_token")
	}
	return &tokenResp, nil
}

func (s *userService) resolveOIDCUserInfo(ctx context.Context, cfg *config.OIDCAuthConfig, tokenResp *oidcTokenResponse) (*types.OIDCUserInfo, error) {
	claims := map[string]interface{}{}

	if strings.TrimSpace(tokenResp.IDToken) != "" {
		idTokenClaims, err := decodeJWTClaims(tokenResp.IDToken)
		if err != nil {
			logger.Warnf(ctx, "Failed to decode OIDC id_token claims: %v", err)
		} else {
			for k, v := range idTokenClaims {
				claims[k] = v
			}
		}
	}

	if strings.TrimSpace(cfg.UserInfoEndpoint) != "" && strings.TrimSpace(tokenResp.AccessToken) != "" {
		userInfoClaims, err := s.fetchOIDCUserInfo(ctx, cfg.UserInfoEndpoint, tokenResp.AccessToken)
		if err != nil {
			logger.Warnf(ctx, "Failed to fetch OIDC userinfo, fallback to id_token claims: %v", err)
		} else {
			for k, v := range userInfoClaims {
				claims[k] = v
			}
		}
	}

	info := &types.OIDCUserInfo{Claims: claims}
	if sub, _ := claims["sub"].(string); sub != "" {
		info.Subject = sub
	}
	info.Username = extractClaimAsString(claims, cfg.UserInfoMapping.Username)
	info.Email = extractClaimAsString(claims, cfg.UserInfoMapping.Email)
	if info.Username == "" {
		info.Username = extractClaimAsString(claims, "preferred_username")
	}
	if info.Username == "" {
		info.Username = extractClaimAsString(claims, "name")
	}
	if info.Username == "" && info.Email != "" {
		info.Username = strings.Split(info.Email, "@")[0]
	}
	return info, nil
}

func (s *userService) fetchOIDCUserInfo(ctx context.Context, endpoint, accessToken string) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("userinfo request failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var claims map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func (s *userService) provisionOIDCUser(ctx context.Context, info *types.OIDCUserInfo) (*types.User, error) {
	username := s.generateOIDCUsername(ctx, info)
	randomPassword, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password for OIDC user: %w", err)
	}

	user, err := s.Register(ctx, &types.RegisterRequest{
		Username: username,
		Email:    info.Email,
		Password: randomPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to auto-provision OIDC user: %w", err)
	}
	return user, nil
}

func (s *userService) generateOIDCUsername(ctx context.Context, info *types.OIDCUserInfo) string {
	base := sanitizeUsernameCandidate(info.Username)
	if base == "" {
		base = sanitizeUsernameCandidate(strings.Split(info.Email, "@")[0])
	}
	if base == "" {
		base = "oidc-user"
	}

	candidate := base
	for i := 0; i < 20; i++ {
		existing, err := s.userRepo.GetUserByUsername(ctx, candidate)
		if isUserLookupNotFound(err) || (err == nil && existing == nil) {
			return candidate
		}
		if err != nil && !isUserLookupNotFound(err) {
			logger.Warnf(ctx, "Failed to check existing OIDC username %q: %v", candidate, err)
		}
		candidate = fmt.Sprintf("%s-%d", base, i+1)
	}
	return fmt.Sprintf("%s-%d", base, time.Now().Unix())
}

func generateRandomString(length int) (string, error) {
	buffer := make([]byte, length)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func encodeOIDCAuthorizationState(state *oidcAuthorizationState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeJWTClaims(token string) (map[string]interface{}, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return nil, errors.New("invalid JWT format")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}
	var claims map[string]interface{}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

func extractClaimAsString(claims map[string]interface{}, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	value, ok := claims[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func sanitizeUsernameCandidate(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '.' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(b.String(), "-._")
	if len(result) > 50 {
		result = strings.Trim(result[:50], "-._")
	}
	return result
}

func isUserLookupNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, apprepo.ErrUserNotFound) || strings.Contains(strings.ToLower(err.Error()), "user not found")
}

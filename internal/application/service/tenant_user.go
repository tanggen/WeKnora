package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// tenantUserService implements TenantUserService
type tenantUserService struct {
	repo        interfaces.TenantUserRepository
	userRepo    interfaces.UserRepository
	db          *gorm.DB
	menuService *MenuService
}

// NewTenantUserService creates a new tenant user service
func NewTenantUserService(
	repo interfaces.TenantUserRepository,
	userRepo interfaces.UserRepository,
	db *gorm.DB,
	menuService *MenuService,
) interfaces.TenantUserService {
	return &tenantUserService{
		repo:        repo,
		userRepo:    userRepo,
		db:          db,
		menuService: menuService,
	}
}

// ListUsers returns paginated users within a tenant
func (s *tenantUserService) ListUsers(ctx context.Context, tenantID uint64, keyword, role, status string, page, pageSize int) (*types.TenantUserListResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var isActive *bool
	if status == "active" {
		t := true
		isActive = &t
	} else if status == "disabled" {
		f := false
		isActive = &f
	}

	results, total, err := s.repo.ListByTenant(ctx, tenantID, keyword, role, isActive, page, pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	// Build response with user info
	items := make([]types.TenantUserInfo, 0, len(results))
	for _, tu := range results {
		user, err := s.userRepo.GetUserByID(ctx, tu.UserID)
		if err != nil || user == nil {
			continue
		}
		items = append(items, types.TenantUserInfo{
			UserID:      user.ID,
			Username:    user.Username,
			Email:       user.Email,
			Phone:       user.Phone,
			Role:        tu.Role,
			IsActive:    user.IsActive,
			Permissions: []string(tu.Permissions),
			JoinedAt:    tu.CreatedAt,
		})
	}

	return &types.TenantUserListResponse{
		Data:     items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// CreateUser creates a new user and adds them to the tenant
func (s *tenantUserService) CreateUser(ctx context.Context, tenantID uint64, req *types.CreateTenantUserRequest) (*types.TenantUserCreatedResponse, error) {
	// Validate username format
	if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(req.Username) {
		return nil, errors.New("VALIDATION_ERROR: username must contain only letters, digits, and underscores")
	}

	// Validate role
	if !types.IsTenantRole(req.Role) {
		return nil, errors.New("INVALID_ROLE: role must be one of tenant_admin, editor, viewer")
	}

	// Check username uniqueness (within the tenant)
	existingByUsername, _ := s.userRepo.GetUserByUsername(ctx, req.Username)
	if existingByUsername != nil && existingByUsername.IsActive {
		// Check if this user already belongs to this tenant
		existingTU, _ := s.repo.GetByTenantAndUser(ctx, tenantID, existingByUsername.ID)
		if existingTU != nil {
			return nil, errors.New("USERNAME_EXISTS: user already belongs to this tenant")
		}
		return nil, errors.New("USERNAME_EXISTS: username is already taken globally")
	}

	// Check email uniqueness (global)
	existingByEmail, _ := s.userRepo.GetUserByEmail(ctx, req.Email)
	if existingByEmail != nil {
		return nil, errors.New("EMAIL_EXISTS: email is already registered")
	}

	// Check phone uniqueness if provided
	if req.Phone != "" {
		existingByPhone, err := s.userRepo.GetUserByPhone(ctx, req.Phone)
		if err == nil && existingByPhone != nil {
			return nil, errors.New("PHONE_EXISTS: phone number is already registered")
		}
	}

	// Generate random password
	initialPassword, err := generateRandomPassword(8)
	if err != nil {
		return nil, fmt.Errorf("failed to generate password: %w", err)
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(initialPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user in a transaction
	var createdUser *types.User
	var createdTU *types.TenantUser
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the user
		user := &types.User{
			ID:              uuid.New().String(),
			Username:        req.Username,
			Email:           req.Email,
			Phone:           req.Phone,
			PasswordHash:    string(passwordHash),
			TenantID:        tenantID,
			IsActive:        true,
			PasswordExpired: true, // force password change on first login
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		createdUser = user

		// Create tenant-user association
		tu := &types.TenantUser{
			TenantID:    tenantID,
			UserID:      user.ID,
			Role:        req.Role,
			Permissions: types.DefaultPermissions(req.Role),
		}
		if err := tx.Create(tu).Error; err != nil {
			return err
		}
		createdTU = tu

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &types.TenantUserCreatedResponse{
		UserID:          createdUser.ID,
		Username:        createdUser.Username,
		Email:           createdUser.Email,
		Phone:           createdUser.Phone,
		Role:            createdTU.Role,
		InitialPassword: initialPassword,
		IsActive:        createdUser.IsActive,
		JoinedAt:        createdTU.CreatedAt,
	}, nil
}

// UpdateUser updates a user's profile
func (s *tenantUserService) UpdateUser(ctx context.Context, tenantID uint64, userID string, req *types.UpdateTenantUserRequest) (*types.TenantUserInfo, error) {
	// Check tenant-user association exists
	tu, err := s.repo.GetByTenantAndUser(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("RESOURCE_NOT_FOUND")
		}
		return nil, err
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("RESOURCE_NOT_FOUND")
	}

	// Check uniqueness for updated fields
	if req.Username != nil && *req.Username != user.Username {
		if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(*req.Username) {
			return nil, errors.New("VALIDATION_ERROR: invalid username format")
		}
		existing, _ := s.userRepo.GetUserByUsername(ctx, *req.Username)
		if existing != nil && existing.ID != userID {
			return nil, errors.New("USERNAME_EXISTS")
		}
		user.Username = *req.Username
	}
	if req.Email != nil && *req.Email != user.Email {
		existing, _ := s.userRepo.GetUserByEmail(ctx, *req.Email)
		if existing != nil && existing.ID != userID {
			return nil, errors.New("EMAIL_EXISTS")
		}
		user.Email = *req.Email
	}
	if req.Phone != nil && *req.Phone != user.Phone {
		if *req.Phone != "" {
			existing, _ := s.userRepo.GetUserByPhone(ctx, *req.Phone)
			if existing != nil && existing.ID != userID {
				return nil, errors.New("PHONE_EXISTS")
			}
		}
		user.Phone = *req.Phone
	}

	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return nil, err
	}

	return &types.TenantUserInfo{
		UserID:      user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Phone:       user.Phone,
		Role:        tu.Role,
		IsActive:    user.IsActive,
		Permissions: []string(tu.Permissions),
		JoinedAt:    tu.CreatedAt,
	}, nil
}

// SetUserStatus enables or disables a user
func (s *tenantUserService) SetUserStatus(ctx context.Context, tenantID uint64, userID string, isActive bool) error {
	tu, err := s.repo.GetByTenantAndUser(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("RESOURCE_NOT_FOUND")
		}
		return err
	}

	// If disabling, check last admin
	if !isActive && tu.Role == types.RoleTenantAdmin {
		count, err := s.repo.CountActiveAdmins(ctx, tenantID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return errors.New("LAST_ADMIN: cannot disable the last active tenant admin")
		}
	}

	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return errors.New("RESOURCE_NOT_FOUND")
	}

	user.IsActive = isActive
	return s.userRepo.UpdateUser(ctx, user)
}

// DeleteUser soft-deletes a user
func (s *tenantUserService) DeleteUser(ctx context.Context, tenantID uint64, userID string) error {
	tu, err := s.repo.GetByTenantAndUser(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("RESOURCE_NOT_FOUND")
		}
		return err
	}

	// Check last admin
	if tu.Role == types.RoleTenantAdmin {
		count, err := s.repo.CountActiveAdmins(ctx, tenantID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return errors.New("LAST_ADMIN: cannot delete the last tenant admin")
		}
	}

	// Soft delete user
	user, err := s.userRepo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return errors.New("RESOURCE_NOT_FOUND")
	}

	user.IsActive = false
	if err := s.userRepo.UpdateUser(ctx, user); err != nil {
		return err
	}

	// Soft delete the tenant-user association
	return s.repo.Delete(ctx, tenantID, userID)
}

// SetUserRole changes a user's role
func (s *tenantUserService) SetUserRole(ctx context.Context, tenantID uint64, userID string, req *types.SetUserRoleRequest) (*types.TenantUserInfo, error) {
	// Validate role
	if !types.IsValidRole(string(req.Role)) {
		return nil, errors.New("INVALID_ROLE: role must be one of tenant_admin, editor, viewer")
	}
	if req.Role == types.RoleSystemAdmin {
		return nil, errors.New("INVALID_ROLE: system_admin cannot be assigned through tenant API")
	}

	tu, err := s.repo.GetByTenantAndUser(ctx, tenantID, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("RESOURCE_NOT_FOUND")
		}
		return nil, err
	}

	// If downgrading from tenant_admin, check last admin
	if tu.Role == types.RoleTenantAdmin && req.Role != types.RoleTenantAdmin {
		count, err := s.repo.CountActiveAdmins(ctx, tenantID)
		if err != nil {
			return nil, err
		}
		if count <= 1 {
			return nil, errors.New("LAST_ADMIN: cannot downgrade the last tenant admin")
		}
	}

	// Update role and permissions
	tu.Role = req.Role
	tu.Permissions = types.DefaultPermissions(req.Role)
	if err := s.repo.Update(ctx, tu); err != nil {
		return nil, err
	}

	// Invalidate menu/permission cache for this user
	s.menuService.InvalidateCache(ctx, tenantID, userID)

	user, _ := s.userRepo.GetUserByID(ctx, userID)
	isActive := false
	if user != nil {
		isActive = user.IsActive
	}

	return &types.TenantUserInfo{
		UserID:      tu.UserID,
		Username:    func() string { if user != nil { return user.Username }; return "" }(),
		Email:       func() string { if user != nil { return user.Email }; return "" }(),
		Phone:       func() string { if user != nil { return user.Phone }; return "" }(),
		Role:        tu.Role,
		IsActive:    isActive,
		Permissions: []string(tu.Permissions),
		JoinedAt:    tu.CreatedAt,
	}, nil
}

// GetTenantUser retrieves the tenant-user association
func (s *tenantUserService) GetTenantUser(ctx context.Context, tenantID uint64, userID string) (*types.TenantUser, error) {
	return s.repo.GetByTenantAndUser(ctx, tenantID, userID)
}

// generateRandomPassword generates a random password of given length with mixed characters
func generateRandomPassword(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := make([]byte, length)
	for i := range result {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}

	// Ensure at least one uppercase, one lowercase, and one digit
	hasUpper := strings.ContainsAny(string(result), "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	hasLower := strings.ContainsAny(string(result), "abcdefghijklmnopqrstuvwxyz")
	hasDigit := strings.ContainsAny(string(result), "0123456789")

	if !hasUpper || !hasLower || !hasDigit {
		// Regenerate if requirements not met (rare but possible)
		return generateRandomPassword(length)
	}

	return string(result), nil
}

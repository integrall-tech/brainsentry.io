package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/integraltech/brainsentry/internal/domain"
	"github.com/integraltech/brainsentry/internal/dto"
	"github.com/integraltech/brainsentry/internal/repository/postgres"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrAccountDisabled    = errors.New("account is disabled")
	ErrUserNotFound       = errors.New("user not found")
)

// AuthService handles authentication logic.
type AuthService struct {
	userRepo   *postgres.UserRepository
	jwtService *JWTService
}

// NewAuthService creates a new AuthService.
func NewAuthService(userRepo *postgres.UserRepository, jwtService *JWTService) *AuthService {
	return &AuthService{
		userRepo:   userRepo,
		jwtService: jwtService,
	}
}

// Login authenticates a user and returns a JWT token.
func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !user.Active {
		return nil, ErrAccountDisabled
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	if err := s.userRepo.UpdateLastLogin(ctx, user.ID, now); err != nil {
		// Non-fatal: the login still succeeds; just record the failure.
		slog.Warn("failed to update last login timestamp", "userId", user.ID, "error", err)
	}

	token, err := s.jwtService.GenerateToken(user.ID, user.Email, user.TenantID, user.Roles)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID, user.Email, user.TenantID, user.Roles)
	if err != nil {
		return nil, err
	}

	return &dto.LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User: dto.UserResponse{
			ID:    user.ID,
			Email: user.Email,
			Name:  user.Name,
			Roles: user.Roles,
		},
		TenantID: user.TenantID,
	}, nil
}

// RefreshToken validates a refresh token and issues a new access token.
func (s *AuthService) RefreshToken(refreshToken string) (*dto.LoginResponse, error) {
	claims, err := s.jwtService.ValidateToken(refreshToken)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	newToken, err := s.jwtService.GenerateToken(claims.UserID, claims.Email, claims.TenantID, claims.Roles)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.jwtService.GenerateRefreshToken(claims.UserID, claims.Email, claims.TenantID, claims.Roles)
	if err != nil {
		return nil, err
	}

	return &dto.LoginResponse{
		Token:        newToken,
		RefreshToken: newRefreshToken,
		User: dto.UserResponse{
			ID:    claims.UserID,
			Email: claims.Email,
			Roles: claims.Roles,
		},
		TenantID: claims.TenantID,
	}, nil
}

// DemoLogin creates or retrieves the demo user and returns a login response.
func (s *AuthService) DemoLogin(ctx context.Context) (*dto.LoginResponse, error) {
	const demoEmail = "demo@example.com"
	const demoPassword = "demo123"
	const demoTenantID = "a9f814d2-4dae-41f3-851b-8aa3d4706561"

	// Try to find existing demo user
	user, err := s.userRepo.FindByEmail(ctx, demoEmail)
	if err != nil {
		// User doesn't exist, create it
		hash, hashErr := HashPassword(demoPassword, 12)
		if hashErr != nil {
			return nil, hashErr
		}

		user = &domain.User{
			Email:         demoEmail,
			Name:          "Demo User",
			PasswordHash:  hash,
			TenantID:      demoTenantID,
			Roles:         []string{"ADMIN", "USER"},
			Active:        true,
			EmailVerified: true,
			CreatedAt:     time.Now(),
		}
		if createErr := s.userRepo.Create(ctx, user); createErr != nil {
			return nil, createErr
		}
	}

	if !user.Active {
		return nil, ErrAccountDisabled
	}

	token, err := s.jwtService.GenerateToken(user.ID, user.Email, user.TenantID, user.Roles)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.jwtService.GenerateRefreshToken(user.ID, user.Email, user.TenantID, user.Roles)
	if err != nil {
		return nil, err
	}

	return &dto.LoginResponse{
		Token:        token,
		RefreshToken: refreshToken,
		User: dto.UserResponse{
			ID:    user.ID,
			Email: user.Email,
			Name:  user.Name,
			Roles: user.Roles,
		},
		TenantID: user.TenantID,
	}, nil
}

// HashPassword hashes a password using bcrypt.
func HashPassword(password string, cost int) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CreateUser creates a new user with a hashed password.
func (s *AuthService) CreateUser(ctx context.Context, req dto.CreateUserRequest, bcryptCost int) (*domain.User, error) {
	hash, err := HashPassword(req.Password, bcryptCost)
	if err != nil {
		return nil, err
	}

	roles := req.Roles
	if len(roles) == 0 {
		roles = []string{"USER"}
	}

	user := &domain.User{
		Email:        req.Email,
		Name:         req.Name,
		PasswordHash: hash,
		TenantID:     req.TenantID,
		Roles:        roles,
		Active:       true,
		CreatedAt:    time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

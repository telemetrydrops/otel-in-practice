package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/repositories"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UserService handles user business logic
type UserService struct {
	repo   *repositories.UserRepository
	logger *zap.Logger
}

// NewUserService creates a new user service
func NewUserService(repo *repositories.UserRepository, logger *zap.Logger) (*UserService, error) {
	return &UserService{
		repo:   repo,
		logger: logger,
	}, nil
}

// RegisterUser creates a new user account
func (s *UserService) RegisterUser(ctx context.Context, email, name, tier string) (*models.User, error) {
	s.logger.Info("Starting user registration",
		zap.String("email", email),
		zap.String("tier", tier))

	// Create new user
	user := &models.User{
		Email: email,
		Name:  name,
		Tier:  tier,
	}

	if user.Tier == "" {
		user.Tier = "free"
	}

	if err := s.repo.Create(ctx, user); err != nil {
		// Handle duplicate email specifically (atomic check)
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "already exists") {
			return nil, fmt.Errorf("user with email %s already exists", email)
		}

		// Handle context cancellation (phantom success detection)
		if errors.Is(err, context.Canceled) {
			s.logger.Warn("Context canceled during user creation, checking for phantom success",
				zap.String("email", email))

			// Use background context for the check since the request context is dead
			phantomUser, phantomErr := s.repo.GetByEmail(context.Background(), email)
			if phantomErr == nil && phantomUser != nil {
				s.logger.Info("Phantom success detected: user was created despite context cancellation",
					zap.String("user_id", phantomUser.ID),
					zap.String("email", email))
				user = phantomUser
				goto success
			}
		}

		return nil, fmt.Errorf("creating user: %w", err)
	}

success:
	s.logger.Info("User registered successfully",
		zap.String("user_id", user.ID),
		zap.String("email", email))

	return user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id string) (*models.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("user not found: %s", id)
		}
		return nil, fmt.Errorf("getting user: %w", err)
	}

	return user, nil
}

// ListUsers retrieves all users
func (s *UserService) ListUsers(ctx context.Context, limit int) ([]*models.User, error) {
	users, err := s.repo.List(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}

	return users, nil
}

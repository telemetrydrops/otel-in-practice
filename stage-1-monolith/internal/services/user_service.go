package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/repositories"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// UserService handles user business logic
type UserService struct {
	repo                *repositories.UserRepository
	logger              *slog.Logger
	tracer              trace.Tracer
	registrationCounter metric.Int64Counter
}

// NewUserService creates a new user service
func NewUserService(repo *repositories.UserRepository, logger *slog.Logger) (*UserService, error) {
	meter := otel.Meter(telemetry.Scope)
	registrationCounter, err := meter.Int64Counter(
		telemetry.EcommerceUsersRegistrationsName,
		metric.WithDescription("Total number of user registrations"),
		metric.WithUnit(telemetry.EcommerceUsersRegistrationsUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("creating registration counter: %w", err)
	}

	return &UserService{
		repo:                repo,
		logger:              logger,
		tracer:              otel.Tracer(telemetry.Scope),
		registrationCounter: registrationCounter,
	}, nil
}

// RegisterUser creates a new user account
func (s *UserService) RegisterUser(ctx context.Context, email, name, tier string) (*models.User, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceUserRegisterName,
		trace.WithAttributes(
			attribute.String("user.email_hash", telemetry.HashEmail(email)),
			attribute.String(telemetry.AttrEcommerceCustomerTier, tier),
		))
	defer span.End()

	s.logger.InfoContext(ctx, "Starting user registration",
		"email_hash", telemetry.HashEmail(email),
		"tier", tier)

	// Create new user
	user := &models.User{
		Email: email,
		Name:  name,
		Tier:  tier,
	}

	if user.Tier == "" {
		user.Tier = "free"
	}

	telemetry.EmitEvent(ctx, "creating user in database")
	if err := s.repo.Create(ctx, user); err != nil {
		// Handle duplicate email specifically (atomic check)
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "already exists") {
			telemetry.EmitEvent(ctx, "user already exists (atomic check)")
			span.SetStatus(codes.Error, "duplicate email")
			return nil, fmt.Errorf("user with email_hash %s already exists", telemetry.HashEmail(email))
		}

		// Handle context cancellation (phantom success detection)
		if errors.Is(err, context.Canceled) {
			s.logger.WarnContext(ctx, "Context canceled during user creation, checking for phantom success",
				"email_hash", telemetry.HashEmail(email))

			// Use background context for the check since the request context is dead
			recoveryCtx := trace.ContextWithSpanContext(context.Background(), trace.SpanContextFromContext(ctx))
			phantomUser, phantomErr := s.repo.GetByEmail(recoveryCtx, email)
			if phantomErr == nil && phantomUser != nil {
				s.logger.InfoContext(ctx, "Phantom success detected: user was created despite context cancellation",
					"user_id", phantomUser.ID,
					"email_hash", telemetry.HashEmail(email))
				user = phantomUser
				goto success
			}
		}

		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "user creation failed")
		return nil, fmt.Errorf("creating user: %w", err)
	}

success:
	// Record metric
	s.registrationCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("registration.source", "api"),
			attribute.String(telemetry.AttrEcommerceCustomerTier, user.Tier),
		))

	span.SetAttributes(attribute.String(telemetry.AttrEcommerceUserId, user.ID))
	telemetry.EmitEvent(ctx, "user registered successfully")

	s.logger.InfoContext(ctx, "User registered successfully",
		"user_id", user.ID,
		"email_hash", telemetry.HashEmail(email))

	return user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id string) (*models.User, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceUserGetName,
		trace.WithAttributes(
			attribute.String(telemetry.AttrEcommerceUserId, id),
		))
	defer span.End()

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Error, "user not found")
			return nil, fmt.Errorf("user not found: %s", id)
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "user retrieval failed")
		return nil, fmt.Errorf("getting user: %w", err)
	}

	span.SetAttributes(
		attribute.String(telemetry.AttrEcommerceCustomerTier, user.Tier),
	)

	return user, nil
}

// ListUsers retrieves all users
func (s *UserService) ListUsers(ctx context.Context, limit int) ([]*models.User, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceUserListName,
		trace.WithAttributes(
			attribute.Int("limit", limit),
		))
	defer span.End()

	users, err := s.repo.List(ctx, limit)
	if err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "user listing failed")
		return nil, fmt.Errorf("listing users: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(users)))
	return users, nil
}

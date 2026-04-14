package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/repositories"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// UserService handles user business logic
type UserService struct {
	repo                *repositories.UserRepository
	logger              *zap.Logger
	tracer              trace.Tracer
	registrationCounter metric.Int64Counter
}

// NewUserService creates a new user service
func NewUserService(repo *repositories.UserRepository, logger *zap.Logger) (*UserService, error) {
	meter := otel.Meter(telemetry.Scope)
	registrationCounter, err := meter.Int64Counter(
		telemetry.USER_REGISTRATIONS_TOTAL,
		metric.WithDescription("Total number of user registrations"),
		metric.WithUnit("{registrations}"),
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
	ctx, span := s.tracer.Start(ctx, telemetry.SPAN_USER_REGISTRATION,
		trace.WithAttributes(
			attribute.String("user.email", email),
			attribute.String(telemetry.ATTR_CUSTOMER_TIER, tier),
		))
	defer span.End()

	s.logger.Info("Starting user registration",
		zap.String("email_hash", telemetry.HashEmail(email)),
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

	span.AddEvent("creating user in database")
	if err := s.repo.Create(ctx, user); err != nil {
		// Handle duplicate email specifically (atomic check)
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "already exists") {
			span.AddEvent("user already exists (atomic check)")
			span.SetStatus(codes.Error, "duplicate email")
			return nil, fmt.Errorf("user with email %s already exists", email)
		}

		// Handle context cancellation (phantom success detection)
		if errors.Is(err, context.Canceled) {
			s.logger.Warn("Context canceled during user creation, checking for phantom success",
				zap.String("email_hash", telemetry.HashEmail(email)))

			// Use background context for the check since the request context is dead,
			// but inject the current span so the DB span is a child of SPAN_USER_REGISTRATION.
			phantomCtx := trace.ContextWithSpan(context.Background(), span)
			phantomUser, phantomErr := s.repo.GetByEmail(phantomCtx, email)
			if phantomErr == nil && phantomUser != nil {
				s.logger.Info("Phantom success detected: user was created despite context cancellation",
					zap.String("user_id", phantomUser.ID),
					zap.String("email_hash", telemetry.HashEmail(email)))
				user = phantomUser
				goto success
			}
		}

		span.RecordError(err)
		span.SetStatus(codes.Error, "user creation failed")
		return nil, fmt.Errorf("creating user: %w", err)
	}

success:
	// Record metric
	s.registrationCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("registration.source", "api"),
			attribute.String(telemetry.ATTR_CUSTOMER_TIER, user.Tier),
		))

	span.SetAttributes(attribute.String(telemetry.ATTR_USER_ID, user.ID))
	span.AddEvent("user registered successfully")

	s.logger.Info("User registered successfully",
		zap.String("user_id", user.ID),
		zap.String("email_hash", telemetry.HashEmail(email)))

	return user, nil
}

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id string) (*models.User, error) {
	ctx, span := s.tracer.Start(ctx, "get user",
		trace.WithAttributes(
			attribute.String(telemetry.ATTR_USER_ID, id),
		))
	defer span.End()

	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Error, "user not found")
			return nil, fmt.Errorf("user not found: %s", id)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "user retrieval failed")
		return nil, fmt.Errorf("getting user: %w", err)
	}

	span.SetAttributes(
		attribute.String(telemetry.ATTR_CUSTOMER_TIER, user.Tier),
	)

	return user, nil
}

// ListUsers retrieves all users
func (s *UserService) ListUsers(ctx context.Context, limit int) ([]*models.User, error) {
	ctx, span := s.tracer.Start(ctx, "list users",
		trace.WithAttributes(
			attribute.Int("limit", limit),
		))
	defer span.End()

	users, err := s.repo.List(ctx, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "user listing failed")
		return nil, fmt.Errorf("listing users: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(users)))
	return users, nil
}

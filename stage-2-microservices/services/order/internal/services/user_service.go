package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// ErrUserNotFound indicates a user id has no row.
var ErrUserNotFound = errors.New("user not found")

type userRepo interface {
	Create(ctx context.Context, u *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
}

// UserService implements user-management business logic.
type UserService struct {
	repo               userRepo
	logger             *slog.Logger
	tracer             trace.Tracer
	registrationsTotal metric.Int64Counter
}

// NewUserService creates a UserService.
func NewUserService(repo userRepo, logger *slog.Logger) (*UserService, error) {
	meter := otel.Meter("telemetrydrops.com/order-service")
	c, err := meter.Int64Counter(
		telemetry.EcommerceUsersRegistrationsName,
		metric.WithDescription("Total number of successful user registrations"),
		metric.WithUnit(telemetry.EcommerceUsersRegistrationsUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("creating registrations counter: %w", err)
	}
	return &UserService{
		repo:               repo,
		logger:             logger,
		tracer:             otel.Tracer("telemetrydrops.com/order-service"),
		registrationsTotal: c,
	}, nil
}

// Register creates a new user.
func (s *UserService) Register(ctx context.Context, email, tier string) (*models.User, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceUserRegisterName,
		trace.WithAttributes(attribute.String(telemetry.AttrEcommerceCustomerTier, tier)))
	defer span.End()

	u := &models.User{ID: uuid.New().String(), Email: email, Tier: tier}
	if err := s.repo.Create(ctx, u); err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "user registration failed")
		return nil, fmt.Errorf("registering user: %w", err)
	}
	s.registrationsTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String(telemetry.AttrEcommerceCustomerTier, tier)))
	if span.IsRecording() {
		span.SetAttributes(attribute.String(telemetry.AttrEcommerceUserId, u.ID))
	}
	return u, nil
}

// GetUser returns a user by id.
func (s *UserService) GetUser(ctx context.Context, id string) (*models.User, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceUserGetName,
		trace.WithAttributes(attribute.String(telemetry.AttrEcommerceUserId, id)))
	defer span.End()

	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "user lookup failed")
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return u, nil
}

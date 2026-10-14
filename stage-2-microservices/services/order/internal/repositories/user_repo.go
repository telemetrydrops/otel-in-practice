package repositories

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// UserRepository persists User rows.
type UserRepository struct {
	db     *gorm.DB
	tracer trace.Tracer
}

// NewUserRepository creates a UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db, tracer: otel.Tracer("telemetrydrops.com/order-service")}
}

// Create inserts a new user.
func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	ctx, span := r.tracer.Start(ctx, "INSERT users",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName("users"),
			attribute.String(telemetry.AttrEcommerceUserId, u.ID),
		))
	defer span.End()

	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "insert failed")
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

// GetByID returns a user by id.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	ctx, span := r.tracer.Start(ctx, "SELECT users",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName("users"),
			attribute.String(telemetry.AttrEcommerceUserId, id),
		))
	defer span.End()

	var u models.User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "select failed")
		return nil, fmt.Errorf("selecting user: %w", err)
	}
	return &u, nil
}

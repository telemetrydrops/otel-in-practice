package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith-programmatic/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith-programmatic/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// UserRepository handles user data operations
type UserRepository struct {
	db     *gorm.DB
	tracer trace.Tracer
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		db:     db,
		tracer: otel.Tracer(telemetry.Scope),
	}
}

// Create inserts a new user into the database
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	ctx, span := r.tracer.Start(ctx, "INSERT users",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName("users"),
			attribute.String(telemetry.AttrEcommerceUserId, user.ID),
		))
	defer span.End()

	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	telemetry.EmitEvent(ctx, "user created in database",
		log.String(telemetry.AttrEcommerceUserId, user.ID))

	return nil
}

// GetByID retrieves a user by ID
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

	var user models.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("finding user: %w", err)
	}

	return &user, nil
}

// GetByEmail retrieves a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	ctx, span := r.tracer.Start(ctx, "SELECT users",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName("users"),
			attribute.String("user.email_hash", telemetry.HashEmail(email)),
		))
	defer span.End()

	var user models.User
	if err := r.db.WithContext(ctx).First(&user, "email = ?", email).Error; err != nil {
		return nil, fmt.Errorf("finding user by email: %w", err)
	}

	return &user, nil
}

// List retrieves all users with optional limit
func (r *UserRepository) List(ctx context.Context, limit int) ([]*models.User, error) {
	ctx, span := r.tracer.Start(ctx, "SELECT users",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName("users"),
			attribute.Int("limit", limit),
		))
	defer span.End()

	var users []*models.User
	query := r.db.WithContext(ctx)
	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&users).Error; err != nil {
		return nil, fmt.Errorf("listing users: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(users)))
	return users, nil
}

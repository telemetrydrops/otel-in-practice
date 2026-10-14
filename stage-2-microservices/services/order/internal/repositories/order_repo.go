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

// OrderRepository persists Order rows.
type OrderRepository struct {
	db     *gorm.DB
	tracer trace.Tracer
}

// NewOrderRepository creates an OrderRepository.
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db, tracer: otel.Tracer("telemetrydrops.com/order-service")}
}

// Create inserts an order and its items in a single transaction (GORM does
// this implicitly via the foreign-key association).
func (r *OrderRepository) Create(ctx context.Context, o *models.Order) error {
	ctx, span := r.tracer.Start(ctx, "INSERT orders",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName("orders"),
			attribute.String(telemetry.AttrEcommerceOrderId, o.ID),
		))
	defer span.End()

	if err := r.db.WithContext(ctx).Create(o).Error; err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "insert failed")
		return fmt.Errorf("creating order: %w", err)
	}
	return nil
}

// GetByID returns an order by id, with its items eagerly loaded.
func (r *OrderRepository) GetByID(ctx context.Context, id string) (*models.Order, error) {
	ctx, span := r.tracer.Start(ctx, "SELECT orders",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName("orders"),
			attribute.String(telemetry.AttrEcommerceOrderId, id),
		))
	defer span.End()

	var o models.Order
	if err := r.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&o).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "select failed")
		return nil, fmt.Errorf("selecting order: %w", err)
	}
	return &o, nil
}

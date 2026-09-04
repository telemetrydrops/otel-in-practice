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

// OrderRepository handles order data operations
type OrderRepository struct {
	db     *gorm.DB
	tracer trace.Tracer
}

// NewOrderRepository creates a new order repository
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{
		db:     db,
		tracer: otel.Tracer(telemetry.Scope),
	}
}

// Create inserts a new order into the database
func (r *OrderRepository) Create(ctx context.Context, order *models.Order) error {
	ctx, span := r.tracer.Start(ctx, "INSERT orders",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("INSERT"),
			semconv.DBCollectionName("orders"),
			attribute.String(telemetry.AttrEcommerceOrderId, order.ID),
			attribute.String(telemetry.AttrEcommerceUserId, order.UserID),
			attribute.Float64(telemetry.AttrEcommerceOrderTotal, order.Total),
		))
	defer span.End()

	if order.ID == "" {
		order.ID = uuid.New().String()
	}

	// Generate IDs for order items
	for i := range order.Items {
		if order.Items[i].ID == "" {
			order.Items[i].ID = uuid.New().String()
		}
		order.Items[i].OrderID = order.ID
	}

	// Use transaction to ensure consistency
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("creating order: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	telemetry.EmitEvent(ctx, "order created successfully",
		log.String(telemetry.AttrEcommerceOrderId, order.ID),
		log.Int("items.count", len(order.Items)),
	)

	return nil
}

// GetByID retrieves an order by ID with its items
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

	var order models.Order
	if err := r.db.WithContext(ctx).Preload("Items").First(&order, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("finding order: %w", err)
	}

	return &order, nil
}

// ListByUserID retrieves orders for a specific user
func (r *OrderRepository) ListByUserID(ctx context.Context, userID string, limit int) ([]*models.Order, error) {
	ctx, span := r.tracer.Start(ctx, "SELECT orders",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName("orders"),
			attribute.String(telemetry.AttrEcommerceUserId, userID),
			attribute.Int("limit", limit),
		))
	defer span.End()

	var orders []*models.Order
	query := r.db.WithContext(ctx).Preload("Items").Where("user_id = ?", userID)

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("listing user orders: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(orders)))
	return orders, nil
}

// UpdateStatus updates the status of an order
func (r *OrderRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	ctx, span := r.tracer.Start(ctx, "UPDATE orders",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("UPDATE"),
			semconv.DBCollectionName("orders"),
			attribute.String(telemetry.AttrEcommerceOrderId, id),
			attribute.String("order.status", status),
		))
	defer span.End()

	result := r.db.WithContext(ctx).Model(&models.Order{}).
		Where("id = ?", id).
		Update("status", status)

	if result.Error != nil {
		return fmt.Errorf("updating order status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("order not found: %s", id)
	}

	telemetry.EmitEvent(ctx, "order status updated",
		log.String("new.status", status),
	)

	return nil
}

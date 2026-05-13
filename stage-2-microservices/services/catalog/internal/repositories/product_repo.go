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

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// ProductRepository persists Product rows.
type ProductRepository struct {
	db     *gorm.DB
	tracer trace.Tracer
}

// NewProductRepository creates a ProductRepository.
func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{
		db:     db,
		tracer: otel.Tracer("telemetrydrops.com/catalog-service"),
	}
}

// GetByID returns a single product by id.
func (r *ProductRepository) GetByID(ctx context.Context, id string) (*models.Product, error) {
	ctx, span := r.tracer.Start(ctx, "SELECT products",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName("products"),
			attribute.String(telemetry.AttrEcommerceProductId, id),
		))
	defer span.End()

	var product models.Product
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Error, "product not found")
			return nil, err
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "select failed")
		return nil, fmt.Errorf("selecting product: %w", err)
	}

	return &product, nil
}

// List returns products optionally filtered by category, capped by limit.
func (r *ProductRepository) List(ctx context.Context, category string, limit int) ([]*models.Product, error) {
	ctx, span := r.tracer.Start(ctx, "SELECT products",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemNamePostgreSQL,
			semconv.DBOperationName("SELECT"),
			semconv.DBCollectionName("products"),
		))
	defer span.End()

	q := r.db.WithContext(ctx).Model(&models.Product{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}

	var products []*models.Product
	if err := q.Find(&products).Error; err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "select failed")
		return nil, fmt.Errorf("listing products: %w", err)
	}

	return products, nil
}

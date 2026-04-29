package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// ProductRepository handles product data operations
type ProductRepository struct {
	db     *gorm.DB
	tracer trace.Tracer
}

// NewProductRepository creates a new product repository
func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{
		db:     db,
		tracer: otel.Tracer(telemetry.Scope),
	}
}

// Create inserts a new product into the database
func (r *ProductRepository) Create(ctx context.Context, product *models.Product) error {
	ctx, span := r.tracer.Start(ctx, "INSERT products",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "INSERT"),
			attribute.String("db.sql.table", "products"),
			attribute.String(telemetry.ATTR_PRODUCT_ID, product.ID),
		))
	defer span.End()

	if product.ID == "" {
		product.ID = uuid.New().String()
	}

	if err := r.db.WithContext(ctx).Create(product).Error; err != nil {
		return fmt.Errorf("creating product: %w", err)
	}

	return nil
}

// GetByID retrieves a product by ID
func (r *ProductRepository) GetByID(ctx context.Context, id string) (*models.Product, error) {
	ctx, span := r.tracer.Start(ctx, telemetry.SPAN_PRODUCT_SELECT,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.sql.table", "products"),
			attribute.String(telemetry.ATTR_PRODUCT_ID, id),
		))
	defer span.End()

	var product models.Product
	if err := r.db.WithContext(ctx).First(&product, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("finding product: %w", err)
	}

	return &product, nil
}

// List retrieves products with optional category filter and limit
// This method deliberately contains an N+1 query problem for exercise purposes
func (r *ProductRepository) List(ctx context.Context, category string, limit int) ([]*models.Product, error) {
	ctx, span := r.tracer.Start(ctx, telemetry.SPAN_PRODUCT_SELECT,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.sql.table", "products"),
			attribute.String(telemetry.ATTR_PRODUCT_CATEGORY, category),
			attribute.Int("limit", limit),
		))
	defer span.End()

	var products []*models.Product
	query := r.db.WithContext(ctx)

	if category != "" {
		query = query.Where("category = ?", category)
	}
	if limit > 0 {
		query = query.Limit(limit)
	}

	// Deliberately inefficient: Get product IDs first
	var productIDs []string
	if err := query.Model(&models.Product{}).Pluck("id", &productIDs).Error; err != nil {
		return nil, fmt.Errorf("getting product IDs: %w", err)
	}

	// N+1 problem: Individual query for each product (for exercise scenario)
	for _, id := range productIDs {
		var product models.Product
		if err := r.db.WithContext(ctx).First(&product, "id = ?", id).Error; err != nil {
			return nil, fmt.Errorf("finding product %s: %w", id, err)
		}
		products = append(products, &product)
	}

	span.SetAttributes(attribute.Int("result.count", len(products)))
	return products, nil
}

// UpdateStock updates the stock quantity for a product
func (r *ProductRepository) UpdateStock(ctx context.Context, id string, quantity int) error {
	ctx, span := r.tracer.Start(ctx, telemetry.SPAN_PRODUCT_UPDATE,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "UPDATE"),
			attribute.String("db.sql.table", "products"),
			attribute.String(telemetry.ATTR_PRODUCT_ID, id),
			attribute.Int("stock.change", quantity),
		))
	defer span.End()

	result := r.db.WithContext(ctx).Model(&models.Product{}).
		Where("id = ?", id).
		Update("stock", gorm.Expr("stock + ?", quantity))

	if result.Error != nil {
		return fmt.Errorf("updating product stock: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("product not found: %s", id)
	}

	return nil
}

// GetTotalInventoryValue returns the total value of all products in stock (sum of stock * price)
func (r *ProductRepository) GetTotalInventoryValue(ctx context.Context) (float64, error) {
	var total float64
	err := r.db.WithContext(ctx).Model(&models.Product{}).
		Select("COALESCE(SUM(stock * price), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, fmt.Errorf("calculating inventory value: %w", err)
	}
	return total, nil
}

// CheckStock verifies if a product has sufficient stock
func (r *ProductRepository) CheckStock(ctx context.Context, id string, requiredQuantity int) (bool, error) {
	ctx, span := r.tracer.Start(ctx, telemetry.SPAN_PRODUCT_SELECT,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.sql.table", "products"),
			attribute.String(telemetry.ATTR_PRODUCT_ID, id),
			attribute.Int("required.quantity", requiredQuantity),
		))
	defer span.End()

	var stock int
	err := r.db.WithContext(ctx).Model(&models.Product{}).
		Where("id = ?", id).
		Pluck("stock", &stock).Error

	if err != nil {
		return false, fmt.Errorf("checking stock: %w", err)
	}

	hasStock := stock >= requiredQuantity
	span.SetAttributes(
		attribute.Int("current.stock", stock),
		attribute.Bool("has.sufficient.stock", hasStock),
	)

	return hasStock, nil
}

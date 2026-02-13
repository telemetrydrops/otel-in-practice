package services

import (
	"context"
	"errors"
	"fmt"

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

// ProductService handles product business logic
type ProductService struct {
	repo           *repositories.ProductRepository
	logger         *zap.Logger
	tracer         trace.Tracer
	lookupCounter  metric.Int64Counter
	inventoryGauge metric.Float64ObservableGauge
}

// NewProductService creates a new product service
func NewProductService(repo *repositories.ProductRepository, logger *zap.Logger) (*ProductService, error) {
	meter := otel.Meter(telemetry.Scope)
	lookupCounter, err := meter.Int64Counter(
		telemetry.PRODUCT_LOOKUPS_TOTAL,
		metric.WithDescription("Total number of product lookups"),
		metric.WithUnit("{lookups}"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating lookup counter: %w", err)
	}

	return &ProductService{
		repo:          repo,
		logger:        logger,
		tracer:        otel.Tracer(telemetry.Scope),
		lookupCounter: lookupCounter,
	}, nil
}

// GetProduct retrieves a product by ID
func (s *ProductService) GetProduct(ctx context.Context, id string) (*models.Product, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SPAN_PRODUCT_LOOKUP,
		trace.WithAttributes(
			attribute.String(telemetry.ATTR_PRODUCT_ID, id),
		))
	defer span.End()

	product, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Error, "product not found")
			return nil, fmt.Errorf("product not found: %s", id)
		}
		span.RecordError(err)
		span.SetStatus(codes.Error, "product retrieval failed")
		return nil, fmt.Errorf("getting product: %w", err)
	}

	// Record metric
	s.lookupCounter.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String(telemetry.ATTR_PRODUCT_CATEGORY, product.Category),
		))

	// Use IsRecording to guard expensive attribute computation
	if span.IsRecording() {
		span.SetAttributes(
			attribute.String(telemetry.ATTR_PRODUCT_CATEGORY, product.Category),
			attribute.Float64("product.price", product.Price),
			attribute.Int("product.stock", product.Stock),
		)
	}

	return product, nil
}

// ListProducts retrieves products with optional filters
func (s *ProductService) ListProducts(ctx context.Context, category string, limit int) ([]*models.Product, error) {
	ctx, span := s.tracer.Start(ctx, "list products",
		trace.WithAttributes(
			attribute.String(telemetry.ATTR_PRODUCT_CATEGORY, category),
			attribute.Int("limit", limit),
		))
	defer span.End()

	// Refine span name based on actual operation
	if category != "" {
		span.SetName("list products by category")
	}

	s.logger.Info("Listing products",
		zap.String("category", category),
		zap.Int("limit", limit))

	products, err := s.repo.List(ctx, category, limit)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "product listing failed")
		return nil, fmt.Errorf("listing products: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(products)))
	return products, nil
}

// CreateProduct adds a new product to the catalog
func (s *ProductService) CreateProduct(ctx context.Context, product *models.Product) error {
	ctx, span := s.tracer.Start(ctx, "create product",
		trace.WithAttributes(
			attribute.String("product.name", product.Name),
			attribute.String(telemetry.ATTR_PRODUCT_CATEGORY, product.Category),
			attribute.Float64("product.price", product.Price),
		))
	defer span.End()

	if product.Price <= 0 {
		span.SetStatus(codes.Error, "invalid product price")
		return fmt.Errorf("product price must be positive")
	}

	if product.Stock < 0 {
		span.SetStatus(codes.Error, "invalid product stock")
		return fmt.Errorf("product stock cannot be negative")
	}

	if err := s.repo.Create(ctx, product); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "product creation failed")
		return fmt.Errorf("creating product: %w", err)
	}

	span.SetAttributes(attribute.String(telemetry.ATTR_PRODUCT_ID, product.ID))
	span.AddEvent("product created successfully")

	s.logger.Info("Product created",
		zap.String("product_id", product.ID),
		zap.String("name", product.Name))

	return nil
}

// CheckInventory verifies if a product has sufficient stock
func (s *ProductService) CheckInventory(ctx context.Context, productID string, quantity int) (bool, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SPAN_INVENTORY_CHECK,
		trace.WithAttributes(
			attribute.String(telemetry.ATTR_PRODUCT_ID, productID),
			attribute.Int("requested.quantity", quantity),
		))
	defer span.End()

	hasStock, err := s.repo.CheckStock(ctx, productID, quantity)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "inventory check failed")
		return false, fmt.Errorf("checking inventory: %w", err)
	}

	span.SetAttributes(attribute.Bool("inventory.available", hasStock))

	if !hasStock {
		span.AddEvent("insufficient inventory")
		s.logger.Warn("Insufficient inventory",
			zap.String("product_id", productID),
			zap.Int("requested", quantity))
	}

	return hasStock, nil
}

// UpdateStock adjusts the stock level of a product
func (s *ProductService) UpdateStock(ctx context.Context, productID string, quantityChange int) error {
	ctx, span := s.tracer.Start(ctx, "update stock",
		trace.WithAttributes(
			attribute.String(telemetry.ATTR_PRODUCT_ID, productID),
			attribute.Int("quantity.change", quantityChange),
		))
	defer span.End()

	if err := s.repo.UpdateStock(ctx, productID, quantityChange); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "stock update failed")
		return fmt.Errorf("updating stock: %w", err)
	}

	span.AddEvent("stock updated successfully")
	return nil
}

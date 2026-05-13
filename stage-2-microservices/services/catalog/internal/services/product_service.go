package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// ErrProductNotFound is returned when a product id does not exist.
var ErrProductNotFound = errors.New("product not found")

// productRepo is the subset of ProductRepository the service needs.
type productRepo interface {
	GetByID(ctx context.Context, id string) (*models.Product, error)
	List(ctx context.Context, category string, limit int) ([]*models.Product, error)
}

// ProductService implements the catalog product business logic.
type ProductService struct {
	repo          productRepo
	logger        *slog.Logger
	tracer        trace.Tracer
	lookupCounter metric.Int64Counter
}

// NewProductService creates a new ProductService.
func NewProductService(repo productRepo, logger *slog.Logger) (*ProductService, error) {
	meter := otel.Meter("telemetrydrops.com/catalog-service")
	lookupCounter, err := meter.Int64Counter(
		telemetry.EcommerceProductsLookupsName,
		metric.WithDescription("Total number of product lookups"),
		metric.WithUnit(telemetry.EcommerceProductsLookupsUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("creating lookup counter: %w", err)
	}

	return &ProductService{
		repo:          repo,
		logger:        logger,
		tracer:        otel.Tracer("telemetrydrops.com/catalog-service"),
		lookupCounter: lookupCounter,
	}, nil
}

// GetProduct returns a single product by id.
func (s *ProductService) GetProduct(ctx context.Context, id string) (*models.Product, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceProductLookupName,
		trace.WithAttributes(attribute.String(telemetry.AttrEcommerceProductId, id)))
	defer span.End()

	product, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProductNotFound
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "product lookup failed")
		return nil, fmt.Errorf("getting product: %w", err)
	}

	s.lookupCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String(telemetry.AttrEcommerceProductCategory, product.Category),
	))

	if span.IsRecording() {
		span.SetAttributes(
			attribute.String(telemetry.AttrEcommerceProductCategory, product.Category),
		)
	}

	return product, nil
}

// ListProducts returns products optionally filtered by category.
func (s *ProductService) ListProducts(ctx context.Context, category string, limit int) ([]*models.Product, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceProductListName,
		trace.WithAttributes(
			attribute.String(telemetry.AttrEcommerceProductCategory, category),
			attribute.Int("limit", limit),
		))
	defer span.End()

	products, err := s.repo.List(ctx, category, limit)
	if err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "product list failed")
		return nil, fmt.Errorf("listing products: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(products)))
	return products, nil
}

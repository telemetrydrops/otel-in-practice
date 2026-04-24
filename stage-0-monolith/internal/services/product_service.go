package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/repositories"
	"gorm.io/gorm"
)

// ProductService handles product business logic
type ProductService struct {
	repo   *repositories.ProductRepository
	logger *slog.Logger
}

// NewProductService creates a new product service
func NewProductService(repo *repositories.ProductRepository, logger *slog.Logger) (*ProductService, error) {
	return &ProductService{
		repo:   repo,
		logger: logger,
	}, nil
}

// GetProduct retrieves a product by ID
func (s *ProductService) GetProduct(ctx context.Context, id string) (*models.Product, error) {
	product, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("product not found: %s", id)
		}
		return nil, fmt.Errorf("getting product: %w", err)
	}

	return product, nil
}

// ListProducts retrieves products with optional filters
func (s *ProductService) ListProducts(ctx context.Context, category string, limit int) ([]*models.Product, error) {
	s.logger.InfoContext(ctx, "Listing products",
		"category", category,
		"limit", limit)

	products, err := s.repo.List(ctx, category, limit)
	if err != nil {
		return nil, fmt.Errorf("listing products: %w", err)
	}

	return products, nil
}

// CreateProduct adds a new product to the catalog
func (s *ProductService) CreateProduct(ctx context.Context, product *models.Product) error {
	if product.Price <= 0 {
		return fmt.Errorf("product price must be positive")
	}

	if product.Stock < 0 {
		return fmt.Errorf("product stock cannot be negative")
	}

	if err := s.repo.Create(ctx, product); err != nil {
		return fmt.Errorf("creating product: %w", err)
	}

	s.logger.InfoContext(ctx, "Product created",
		"product_id", product.ID,
		"name", product.Name)

	return nil
}

// CheckInventory verifies if a product has sufficient stock
func (s *ProductService) CheckInventory(ctx context.Context, productID string, quantity int) (bool, error) {
	hasStock, err := s.repo.CheckStock(ctx, productID, quantity)
	if err != nil {
		return false, fmt.Errorf("checking inventory: %w", err)
	}

	if !hasStock {
		s.logger.WarnContext(ctx, "Insufficient inventory",
			"product_id", productID,
			"requested", quantity)
	}

	return hasStock, nil
}

// UpdateStock adjusts the stock level of a product
func (s *ProductService) UpdateStock(ctx context.Context, productID string, quantityChange int) error {
	if err := s.repo.UpdateStock(ctx, productID, quantityChange); err != nil {
		return fmt.Errorf("updating stock: %w", err)
	}

	return nil
}

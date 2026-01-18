package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/models"
	"gorm.io/gorm"
)

// ProductRepository handles product data operations
type ProductRepository struct {
	db *gorm.DB
}

// NewProductRepository creates a new product repository
func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{
		db: db,
	}
}

// Create inserts a new product into the database
func (r *ProductRepository) Create(ctx context.Context, product *models.Product) error {
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
	var product models.Product
	if err := r.db.WithContext(ctx).First(&product, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("finding product: %w", err)
	}

	return &product, nil
}

// List retrieves products with optional category filter and limit
// This method deliberately contains an N+1 query problem for exercise purposes
func (r *ProductRepository) List(ctx context.Context, category string, limit int) ([]*models.Product, error) {
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

	return products, nil
}

// UpdateStock updates the stock quantity for a product
func (r *ProductRepository) UpdateStock(ctx context.Context, id string, quantity int) error {
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

// CheckStock verifies if a product has sufficient stock
func (r *ProductRepository) CheckStock(ctx context.Context, id string, requiredQuantity int) (bool, error) {
	var stock int
	err := r.db.WithContext(ctx).Model(&models.Product{}).
		Where("id = ?", id).
		Pluck("stock", &stock).Error

	if err != nil {
		return false, fmt.Errorf("checking stock: %w", err)
	}

	hasStock := stock >= requiredQuantity

	return hasStock, nil
}

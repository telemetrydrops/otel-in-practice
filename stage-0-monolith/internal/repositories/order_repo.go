package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/models"
	"gorm.io/gorm"
)

// OrderRepository handles order data operations
type OrderRepository struct {
	db *gorm.DB
}

// NewOrderRepository creates a new order repository
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{
		db: db,
	}
}

// Create inserts a new order into the database
func (r *OrderRepository) Create(ctx context.Context, order *models.Order) error {
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

	return nil
}

// GetByID retrieves an order by ID with its items
func (r *OrderRepository) GetByID(ctx context.Context, id string) (*models.Order, error) {
	var order models.Order
	if err := r.db.WithContext(ctx).Preload("Items").First(&order, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("finding order: %w", err)
	}

	return &order, nil
}

// ListByUserID retrieves orders for a specific user
func (r *OrderRepository) ListByUserID(ctx context.Context, userID string, limit int) ([]*models.Order, error) {
	var orders []*models.Order
	query := r.db.WithContext(ctx).Preload("Items").Where("user_id = ?", userID)

	if limit > 0 {
		query = query.Limit(limit)
	}

	if err := query.Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("listing user orders: %w", err)
	}

	return orders, nil
}

// UpdateStatus updates the status of an order
func (r *OrderRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	result := r.db.WithContext(ctx).Model(&models.Order{}).
		Where("id = ?", id).
		Update("status", status)

	if result.Error != nil {
		return fmt.Errorf("updating order status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("order not found: %s", id)
	}

	return nil
}

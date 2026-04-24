package services

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/repositories"
)

// OrderService handles order business logic
type OrderService struct {
	orderRepo       *repositories.OrderRepository
	productRepo     *repositories.ProductRepository
	userRepo        *repositories.UserRepository
	logger          *slog.Logger
	goroutineLeaker *goroutineLeaker // For exercise: goroutine leak
}

// goroutineLeaker deliberately creates a goroutine leak for the exercise
type goroutineLeaker struct {
	wg sync.WaitGroup
}

// NewOrderService creates a new order service
func NewOrderService(
	orderRepo *repositories.OrderRepository,
	productRepo *repositories.ProductRepository,
	userRepo *repositories.UserRepository,
	logger *slog.Logger,
) (*OrderService, error) {
	return &OrderService{
		orderRepo:       orderRepo,
		productRepo:     productRepo,
		userRepo:        userRepo,
		logger:          logger,
		goroutineLeaker: &goroutineLeaker{},
	}, nil
}

// CreateOrder processes a new order
func (s *OrderService) CreateOrder(ctx context.Context, userID string, items []OrderItemRequest, paymentMethod string) (*models.Order, error) {
	s.logger.InfoContext(ctx, "Processing order",
		"user_id", userID,
		"items", len(items))

	// Verify user exists
	_, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Process order items and calculate total
	var orderItems []models.OrderItem
	var total float64

	for _, item := range items {
		// Check product availability
		product, err := s.productRepo.GetByID(ctx, item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("product %s not found: %w", item.ProductID, err)
		}

		// Check inventory
		hasStock, err := s.productRepo.CheckStock(ctx, item.ProductID, item.Quantity)
		if err != nil {
			return nil, fmt.Errorf("checking stock for product %s: %w", item.ProductID, err)
		}
		if !hasStock {
			return nil, fmt.Errorf("insufficient stock for product %s", item.ProductID)
		}

		orderItem := models.OrderItem{
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			Price:     product.Price,
		}
		orderItems = append(orderItems, orderItem)
		total += product.Price * float64(item.Quantity)

		// Update inventory
		if err := s.productRepo.UpdateStock(ctx, item.ProductID, -item.Quantity); err != nil {
			return nil, fmt.Errorf("updating stock for product %s: %w", item.ProductID, err)
		}
	}

	// Create order
	order := &models.Order{
		UserID:        userID,
		Status:        "pending",
		Total:         total,
		PaymentMethod: paymentMethod,
		Items:         orderItems,
	}

	if err := s.orderRepo.Create(ctx, order); err != nil {
		// Rollback inventory changes would go here in production
		return nil, fmt.Errorf("creating order: %w", err)
	}

	// Deliberately leak a goroutine for exercise (background processing simulation)
	s.startBackgroundProcessing(order.ID)

	s.logger.InfoContext(ctx, "Order created successfully",
		"order_id", order.ID,
		"total", total)

	return order, nil
}

// startBackgroundProcessing deliberately creates a goroutine leak for the exercise
func (s *OrderService) startBackgroundProcessing(orderID string) {
	s.goroutineLeaker.wg.Add(1)
	go func() {
		// Deliberate bug: No defer wg.Done() and no context cancellation
		// This goroutine will leak and run forever
		ticker := time.NewTicker(10 * time.Second)
		for range ticker.C {
			// Simulate background work that never completes
			s.logger.Debug("Background processing for order",
				"order_id", orderID)
		}
	}()
}

// GetOrder retrieves an order by ID
func (s *OrderService) GetOrder(ctx context.Context, orderID string) (*models.Order, error) {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("getting order: %w", err)
	}

	return order, nil
}

// GetUserOrders retrieves orders for a user
func (s *OrderService) GetUserOrders(ctx context.Context, userID string, limit int) ([]*models.Order, error) {
	orders, err := s.orderRepo.ListByUserID(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("getting user orders: %w", err)
	}

	return orders, nil
}

// UpdateOrderStatus updates the status of an order
func (s *OrderService) UpdateOrderStatus(ctx context.Context, orderID, status string) error {
	validStatuses := map[string]bool{
		"pending":    true,
		"processing": true,
		"completed":  true,
		"cancelled":  true,
	}

	if !validStatuses[status] {
		return fmt.Errorf("invalid order status: %s", status)
	}

	if err := s.orderRepo.UpdateStatus(ctx, orderID, status); err != nil {
		return fmt.Errorf("updating order status: %w", err)
	}

	return nil
}

// OrderItemRequest represents a request to add an item to an order
type OrderItemRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

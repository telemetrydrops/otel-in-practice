package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/repositories"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// OrderService handles order business logic
type OrderService struct {
	orderRepo       *repositories.OrderRepository
	productRepo     *repositories.ProductRepository
	userRepo        *repositories.UserRepository
	logger          *zap.Logger
	tracer          trace.Tracer
	processingHist  metric.Float64Histogram
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
	logger *zap.Logger,
) (*OrderService, error) {
	meter := otel.Meter(telemetry.Scope)
	processingHist, err := meter.Float64Histogram(
		telemetry.ORDER_PROCESSING_DURATION,
		metric.WithDescription("Duration of order processing"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating processing histogram: %w", err)
	}

	return &OrderService{
		orderRepo:       orderRepo,
		productRepo:     productRepo,
		userRepo:        userRepo,
		logger:          logger,
		tracer:          otel.Tracer(telemetry.Scope),
		processingHist:  processingHist,
		goroutineLeaker: &goroutineLeaker{},
	}, nil
}

// CreateOrder processes a new order
func (s *OrderService) CreateOrder(ctx context.Context, userID string, items []OrderItemRequest, paymentMethod string) (*models.Order, error) {
	startTime := time.Now()
	ctx, span := s.tracer.Start(ctx, telemetry.SPAN_ORDER_PROCESSING,
		trace.WithAttributes(
			attribute.String(telemetry.ATTR_USER_ID, userID),
			attribute.String(telemetry.ATTR_PAYMENT_METHOD, paymentMethod),
			attribute.Int("items.count", len(items)),
		))
	defer span.End()
	defer func() {
		// Record processing duration
		duration := float64(time.Since(startTime).Milliseconds())
		s.processingHist.Record(ctx, duration,
			metric.WithAttributes(
				attribute.String(telemetry.ATTR_PAYMENT_METHOD, paymentMethod),
			))
	}()

	s.logger.Info("Processing order",
		zap.String("user_id", userID),
		zap.Int("items", len(items)))

	// Verify user exists
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	span.SetAttributes(attribute.String(telemetry.ATTR_CUSTOMER_TIER, user.Tier))

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

	span.AddEvent("creating order in database")
	if err := s.orderRepo.Create(ctx, order); err != nil {
		// Rollback inventory changes would go here in production
		return nil, fmt.Errorf("creating order: %w", err)
	}

	span.SetAttributes(
		attribute.String(telemetry.ATTR_ORDER_ID, order.ID),
		attribute.Float64(telemetry.ATTR_ORDER_TOTAL, total),
	)

	// Deliberately leak a goroutine for exercise (background processing simulation)
	s.startBackgroundProcessing(order.ID)

	span.AddEvent("order created successfully")
	s.logger.Info("Order created successfully",
		zap.String("order_id", order.ID),
		zap.Float64("total", total))

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
				zap.String("order_id", orderID))
		}
	}()
}

// GetOrder retrieves an order by ID
func (s *OrderService) GetOrder(ctx context.Context, orderID string) (*models.Order, error) {
	ctx, span := s.tracer.Start(ctx, "get order",
		trace.WithAttributes(
			attribute.String(telemetry.ATTR_ORDER_ID, orderID),
		))
	defer span.End()

	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, fmt.Errorf("getting order: %w", err)
	}

	span.SetAttributes(
		attribute.String(telemetry.ATTR_USER_ID, order.UserID),
		attribute.String("order.status", order.Status),
		attribute.Float64(telemetry.ATTR_ORDER_TOTAL, order.Total),
	)

	return order, nil
}

// GetUserOrders retrieves orders for a user
func (s *OrderService) GetUserOrders(ctx context.Context, userID string, limit int) ([]*models.Order, error) {
	ctx, span := s.tracer.Start(ctx, "get user orders",
		trace.WithAttributes(
			attribute.String(telemetry.ATTR_USER_ID, userID),
			attribute.Int("limit", limit),
		))
	defer span.End()

	orders, err := s.orderRepo.ListByUserID(ctx, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("getting user orders: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(orders)))
	return orders, nil
}

// UpdateOrderStatus updates the status of an order
func (s *OrderService) UpdateOrderStatus(ctx context.Context, orderID, status string) error {
	ctx, span := s.tracer.Start(ctx, "update order status",
		trace.WithAttributes(
			attribute.String(telemetry.ATTR_ORDER_ID, orderID),
			attribute.String("new.status", status),
		))
	defer span.End()

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

	span.AddEvent("order status updated")
	return nil
}

// OrderItemRequest represents a request to add an item to an order
type OrderItemRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required,min=1"`
}

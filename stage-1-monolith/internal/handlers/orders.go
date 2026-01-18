package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/services"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// OrderHandler handles order-related HTTP requests
type OrderHandler struct {
	service *services.OrderService
}

// NewOrderHandler creates a new order handler
func NewOrderHandler(service *services.OrderService) *OrderHandler {
	return &OrderHandler{
		service: service,
	}
}

// RegisterRoutes registers order routes
func (h *OrderHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/orders", h.CreateOrder)
	router.GET("/orders/:id", h.GetOrder)
	router.GET("/users/:user_id/orders", h.GetUserOrders)
	router.PUT("/orders/:id/status", h.UpdateOrderStatus)
}

// CreateOrderRequest represents an order creation request
type CreateOrderRequest struct {
	UserID        string                      `json:"user_id" binding:"required"`
	Items         []services.OrderItemRequest `json:"items" binding:"required,min=1,dive"`
	PaymentMethod string                      `json:"payment_method" binding:"required,oneof=credit_card debit_card paypal"`
}

// CreateOrder handles order creation
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	ctx := c.Request.Context()

	// Extract span from context
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("business.operation", "order_creation"),
	)

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		span.AddEvent("invalid_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	span.SetAttributes(
		attribute.String(telemetry.ATTR_USER_ID, req.UserID),
		attribute.String(telemetry.ATTR_PAYMENT_METHOD, req.PaymentMethod),
		attribute.Int("items.count", len(req.Items)),
	)

	order, err := h.service.CreateOrder(ctx, req.UserID, req.Items, req.PaymentMethod)
	if err != nil {
		span.RecordError(err)
		span.AddEvent("order_creation_failed", trace.WithAttributes(
			attribute.String(telemetry.ATTR_USER_ID, req.UserID),
		))
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	span.SetAttributes(
		attribute.String(telemetry.ATTR_ORDER_ID, order.ID),
		attribute.Float64(telemetry.ATTR_ORDER_TOTAL, order.Total),
	)

	c.JSON(http.StatusCreated, order)
}

// GetOrder retrieves an order by ID
func (h *OrderHandler) GetOrder(c *gin.Context) {
	ctx := c.Request.Context()
	orderID := c.Param("id")

	// Extract span from context
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("business.operation", "order_lookup"),
		attribute.String(telemetry.ATTR_ORDER_ID, orderID),
	)

	order, err := h.service.GetOrder(ctx, orderID)
	if err != nil {
		span.RecordError(err)
		span.AddEvent("order_not_found", trace.WithAttributes(
			attribute.String(telemetry.ATTR_ORDER_ID, orderID),
		))
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	c.JSON(http.StatusOK, order)
}

// GetUserOrders retrieves orders for a specific user
func (h *OrderHandler) GetUserOrders(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.Param("user_id")

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 10
	}

	// Extract span from context
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("business.operation", "user_orders_lookup"),
		attribute.String(telemetry.ATTR_USER_ID, userID),
		attribute.Int("limit", limit),
	)

	orders, err := h.service.GetUserOrders(ctx, userID, limit)
	if err != nil {
		span.RecordError(err)
		span.AddEvent("get_user_orders_failed", trace.WithAttributes(
			attribute.String(telemetry.ATTR_USER_ID, userID),
		))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user orders"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"count":  len(orders),
	})
}

// UpdateOrderStatusRequest represents an order status update request
type UpdateOrderStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=pending processing completed cancelled"`
}

// UpdateOrderStatus updates the status of an order
func (h *OrderHandler) UpdateOrderStatus(c *gin.Context) {
	ctx := c.Request.Context()
	orderID := c.Param("id")

	// Extract span from context
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("business.operation", "order_status_update"),
		attribute.String(telemetry.ATTR_ORDER_ID, orderID),
	)

	var req UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		span.AddEvent("invalid_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	span.SetAttributes(
		attribute.String("order.new_status", req.Status),
	)

	if err := h.service.UpdateOrderStatus(ctx, orderID, req.Status); err != nil {
		span.RecordError(err)
		span.AddEvent("status_update_failed", trace.WithAttributes(
			attribute.String(telemetry.ATTR_ORDER_ID, orderID),
		))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order status updated successfully"})
}

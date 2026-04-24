package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/services"
	"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.20.0"
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

	// Extract span from context (created by otelgin middleware)
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("business.operation", "order_creation"),
	)

	// Extract trace ID for log correlation and response headers
	if spanCtx := span.SpanContext(); spanCtx.HasTraceID() {
		c.Header("X-Trace-ID", spanCtx.TraceID().String())
	}

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.SetStatus(codes.Error, "invalid request body")
		telemetry.EmitEvent(ctx, "invalid_request", log.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Propagate payment method as baggage for downstream services
	paymentMember, _ := baggage.NewMember(telemetry.BAGGAGE_PAYMENT_METHOD, req.PaymentMethod)
	bag, _ := baggage.New(paymentMember)
	ctx = baggage.ContextWithBaggage(ctx, bag)

	span.SetAttributes(
		attribute.String(telemetry.ATTR_USER_ID, req.UserID),
		attribute.String(telemetry.ATTR_PAYMENT_METHOD, req.PaymentMethod),
		attribute.Int("items.count", len(req.Items)),
	)

	order, err := h.service.CreateOrder(ctx, req.UserID, req.Items, req.PaymentMethod)
	if err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "order creation failed")
		telemetry.EmitEvent(ctx, "order_creation_failed",
			log.String(telemetry.ATTR_USER_ID, req.UserID),
		)
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
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "order not found")
		telemetry.EmitEvent(ctx, "order_not_found",
			log.String(telemetry.ATTR_ORDER_ID, orderID),
		)
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
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "failed to get user orders")
		telemetry.EmitEvent(ctx, "get_user_orders_failed",
			log.String(telemetry.ATTR_USER_ID, userID),
		)
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
		semconv.K8SClusterName("td-prod-1"),
		attribute.String("business.operation", "order_status_update"),
		attribute.String(telemetry.ATTR_ORDER_ID, orderID),
	)

	var req UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.SetStatus(codes.Error, "invalid request body")
		telemetry.EmitEvent(ctx, "invalid_request", log.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	span.SetAttributes(
		attribute.String("order.new_status", req.Status),
	)

	if err := h.service.UpdateOrderStatus(ctx, orderID, req.Status); err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "failed to update order status")
		telemetry.EmitEvent(ctx, "status_update_failed",
			log.String(telemetry.ATTR_ORDER_ID, orderID),
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order status updated successfully"})
}

package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/services"
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

	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := h.service.CreateOrder(ctx, req.UserID, req.Items, req.PaymentMethod)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, order)
}

// GetOrder retrieves an order by ID
func (h *OrderHandler) GetOrder(c *gin.Context) {
	ctx := c.Request.Context()
	orderID := c.Param("id")

	order, err := h.service.GetOrder(ctx, orderID)
	if err != nil {
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

	orders, err := h.service.GetUserOrders(ctx, userID, limit)
	if err != nil {
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

	var req UpdateOrderStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateOrderStatus(ctx, orderID, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order status updated successfully"})
}

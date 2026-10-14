package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/baggage"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/services"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// OrderHandler exposes order endpoints.
type OrderHandler struct {
	svc *services.OrderService
}

// NewOrderHandler creates an OrderHandler.
func NewOrderHandler(svc *services.OrderService) *OrderHandler { return &OrderHandler{svc: svc} }

// RegisterRoutes registers order routes.
func (h *OrderHandler) RegisterRoutes(r gin.IRouter) {
	r.POST("/orders", h.create)
	r.GET("/orders/:id", h.get)
}

type createOrderRequest struct {
	UserID        string                   `json:"user_id" binding:"required"`
	PaymentMethod string                   `json:"payment_method" binding:"required,oneof=credit_card paypal apple_pay"`
	Items         []createOrderItemRequest `json:"items" binding:"required,min=1,dive"`
}

type createOrderItemRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Qty       int32  `json:"qty" binding:"required,min=1"`
}

func (h *OrderHandler) create(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Read X-Customer-Tier and put it in baggage so it propagates to catalog.
	tier := c.GetHeader("X-Customer-Tier")
	if tier == "" {
		tier = "standard"
	}
	if member, err := baggage.NewMember(telemetry.BaggageCustomerTier, tier); err == nil {
		if bag, err := baggage.FromContext(ctx).SetMember(member); err == nil {
			ctx = baggage.ContextWithBaggage(ctx, bag)
		}
	}

	items := make([]services.OrderItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, services.OrderItemInput{ProductID: it.ProductID, Qty: it.Qty})
	}
	o, err := h.svc.Create(ctx, services.CreateOrderInput{
		UserID:        req.UserID,
		PaymentMethod: req.PaymentMethod,
		Items:         items,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrOrderProductMissing):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, o)
}

func (h *OrderHandler) get(c *gin.Context) {
	o, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, o)
}

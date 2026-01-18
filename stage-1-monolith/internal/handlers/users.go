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

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	service *services.UserService
}

// NewUserHandler creates a new user handler
func NewUserHandler(service *services.UserService) *UserHandler {
	return &UserHandler{
		service: service,
	}
}

// RegisterRoutes registers user routes
func (h *UserHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/users", h.CreateUser)
	router.GET("/users", h.ListUsers)
	router.GET("/users/:user_id", h.GetUser)
}

// CreateUserRequest represents a user registration request
type CreateUserRequest struct {
	Email string `json:"email" binding:"required,email"`
	Name  string `json:"name" binding:"required"`
	Tier  string `json:"tier"`
}

// CreateUser handles user registration
func (h *UserHandler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()

	// Extract span from context (created by otelgin middleware)
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("business.operation", "user_registration"),
	)

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		span.AddEvent("invalid_request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	span.SetAttributes(
		attribute.String("user.email", req.Email),
		attribute.String(telemetry.ATTR_CUSTOMER_TIER, req.Tier),
	)

	user, err := h.service.RegisterUser(ctx, req.Email, req.Name, req.Tier)
	if err != nil {
		span.RecordError(err)
		span.AddEvent("user_creation_failed", trace.WithAttributes(
			attribute.String("user.email", req.Email),
		))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// GetUser retrieves a user by ID
func (h *UserHandler) GetUser(c *gin.Context) {
	ctx := c.Request.Context()
	userID := c.Param("user_id")

	// Extract span from context
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("business.operation", "user_lookup"),
		attribute.String(telemetry.ATTR_USER_ID, userID),
	)

	user, err := h.service.GetUser(ctx, userID)
	if err != nil {
		span.RecordError(err)
		span.AddEvent("user_not_found", trace.WithAttributes(
			attribute.String(telemetry.ATTR_USER_ID, userID),
		))
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// ListUsers retrieves all users
func (h *UserHandler) ListUsers(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 20
	}

	// Extract span from context
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(
		attribute.String("business.operation", "user_list"),
		attribute.Int("limit", limit),
	)

	users, err := h.service.ListUsers(ctx, limit)
	if err != nil {
		span.RecordError(err)
		span.AddEvent("list_users_failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"count": len(users),
	})
}

package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/services"
)

// UserHandler exposes user endpoints.
type UserHandler struct {
	svc *services.UserService
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(svc *services.UserService) *UserHandler { return &UserHandler{svc: svc} }

// RegisterRoutes registers user routes.
func (h *UserHandler) RegisterRoutes(r gin.IRouter) {
	r.POST("/users", h.create)
	r.GET("/users/:id", h.get)
}

type createUserRequest struct {
	Email string `json:"email" binding:"required,email"`
	Tier  string `json:"tier" binding:"required,oneof=standard premium"`
}

func (h *UserHandler) create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.svc.Register(c.Request.Context(), req.Email, req.Tier)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (h *UserHandler) get(c *gin.Context) {
	u, err := h.svc.GetUser(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}

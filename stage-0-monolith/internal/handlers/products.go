package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/services"
)

// ProductHandler handles product-related HTTP requests
type ProductHandler struct {
	service *services.ProductService
}

// NewProductHandler creates a new product handler
func NewProductHandler(service *services.ProductService) *ProductHandler {
	return &ProductHandler{
		service: service,
	}
}

// RegisterRoutes registers product routes
func (h *ProductHandler) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/products", h.CreateProduct)
	router.GET("/products/:id", h.GetProduct)
	router.GET("/products", h.ListProducts)
	router.POST("/products/:id/stock", h.UpdateStock)
}

// CreateProductRequest represents a product creation request
type CreateProductRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	Category    string  `json:"category" binding:"required"`
	Price       float64 `json:"price" binding:"required,gt=0"`
	Stock       int     `json:"stock" binding:"min=0"`
}

// CreateProduct adds a new product
func (h *ProductHandler) CreateProduct(c *gin.Context) {
	ctx := c.Request.Context()

	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product := &models.Product{
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Price:       req.Price,
		Stock:       req.Stock,
	}

	if err := h.service.CreateProduct(ctx, product); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	c.JSON(http.StatusCreated, product)
}

// GetProduct retrieves a product by ID
func (h *ProductHandler) GetProduct(c *gin.Context) {
	ctx := c.Request.Context()
	productID := c.Param("id")

	product, err := h.service.GetProduct(ctx, productID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	c.JSON(http.StatusOK, product)
}

// ListProducts retrieves products with optional filters
func (h *ProductHandler) ListProducts(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse query parameters
	category := c.Query("category")
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 20
	}

	products, err := h.service.ListProducts(ctx, category, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list products"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"products": products,
		"count":    len(products),
	})
}

// UpdateStockRequest represents a stock update request
type UpdateStockRequest struct {
	Quantity int `json:"quantity" binding:"required"` // Can be negative for reduction
}

// UpdateStock adjusts product stock
func (h *ProductHandler) UpdateStock(c *gin.Context) {
	ctx := c.Request.Context()
	productID := c.Param("id")

	var req UpdateStockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.UpdateStock(ctx, productID, req.Quantity); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update stock"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Stock updated successfully"})
}

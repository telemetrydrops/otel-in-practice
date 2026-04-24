package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/config"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/handlers"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/repositories"
	"github.com/telemetrydrops/otel-in-practice/stage-0-monolith/internal/services"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Load configuration
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize Logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	logger.Info("Starting ecommerce-monolith application",
		"version", version,
		"commit", commit,
		"build_date", date,
		"server_address", cfg.GetServerAddress())

	// Initialize database
	db, err := initDatabase(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize database", "error", err)
		os.Exit(1)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		logger.Error("Failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Seed initial data
	if err := seedData(db, logger); err != nil {
		logger.Warn("Failed to seed data", "error", err)
	}

	// Initialize repositories
	userRepo := repositories.NewUserRepository(db)
	productRepo := repositories.NewProductRepository(db)
	orderRepo := repositories.NewOrderRepository(db)

	// Initialize services
	userService, err := services.NewUserService(userRepo, logger)
	if err != nil {
		logger.Error("Failed to create user service", "error", err)
		os.Exit(1)
	}

	productService, err := services.NewProductService(productRepo, logger)
	if err != nil {
		logger.Error("Failed to create product service", "error", err)
		os.Exit(1)
	}

	orderService, err := services.NewOrderService(orderRepo, productRepo, userRepo, logger)
	if err != nil {
		logger.Error("Failed to create order service", "error", err)
		os.Exit(1)
	}

	// Initialize handlers
	userHandler := handlers.NewUserHandler(userService)
	productHandler := handlers.NewProductHandler(productService)
	orderHandler := handlers.NewOrderHandler(orderService)

	// Setup HTTP server
	app := setupHTTPServer(cfg, userHandler, productHandler, orderHandler, logger)

	// Start server
	if err := runServer(cfg, app, logger); err != nil {
		logger.Error("Server failed", "error", err)
		os.Exit(1)
	}
}

func initDatabase(cfg *config.Config, logger *slog.Logger) (*gorm.DB, error) {
	dsn := cfg.GetDatabaseDSN()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("opening database connection: %w", err)
	}

	logger.Info("Database connection established",
		"host", cfg.Database.PostgreSQL.Host,
		"port", cfg.Database.PostgreSQL.Port,
		"database", cfg.Database.PostgreSQL.Database)
	return db, nil
}

func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.Product{},
		&models.Order{},
		&models.OrderItem{},
	)
}

func seedData(db *gorm.DB, logger *slog.Logger) error {
	// Check if data already exists
	var count int64
	db.Model(&models.Product{}).Count(&count)
	if count > 0 {
		logger.Info("Data already seeded, skipping")
		return nil
	}

	// Seed sample products
	products := []models.Product{
		{
			ID:          uuid.New().String(),
			Name:        "Laptop",
			Description: "High-performance laptop for developers",
			Category:    "Electronics",
			Price:       1299.99,
			Stock:       50,
		},
		{
			ID:          uuid.New().String(),
			Name:        "Wireless Mouse",
			Description: "Ergonomic wireless mouse",
			Category:    "Electronics",
			Price:       39.99,
			Stock:       100,
		},
		{
			ID:          uuid.New().String(),
			Name:        "Mechanical Keyboard",
			Description: "RGB mechanical keyboard with Cherry MX switches",
			Category:    "Electronics",
			Price:       149.99,
			Stock:       75,
		},
		{
			ID:          uuid.New().String(),
			Name:        "Monitor Stand",
			Description: "Adjustable monitor stand with USB hub",
			Category:    "Accessories",
			Price:       79.99,
			Stock:       30,
		},
		{
			ID:          uuid.New().String(),
			Name:        "USB-C Hub",
			Description: "7-in-1 USB-C hub for laptops",
			Category:    "Accessories",
			Price:       49.99,
			Stock:       60,
		},
	}

	for _, product := range products {
		if err := db.Create(&product).Error; err != nil {
			return fmt.Errorf("seeding product %s: %w", product.Name, err)
		}
	}

	logger.Info("Sample data seeded successfully")
	return nil
}

func setupHTTPServer(
	cfg *config.Config,
	userHandler *handlers.UserHandler,
	productHandler *handlers.ProductHandler,
	orderHandler *handlers.OrderHandler,
	logger *slog.Logger,
) *gin.Engine {
	// Set Gin mode based on configuration
	switch cfg.Server.Mode {
	case "release":
		gin.SetMode(gin.ReleaseMode)
	case "test":
		gin.SetMode(gin.TestMode)
	default:
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()

	// Add recovery middleware
	router.Use(gin.Recovery())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	// API routes
	api := router.Group("/api/v1")
	{
		userHandler.RegisterRoutes(api)
		productHandler.RegisterRoutes(api)
		orderHandler.RegisterRoutes(api)
	}

	logger.Info("HTTP routes configured")
	return router
}

func runServer(cfg *config.Config, handler http.Handler, logger *slog.Logger) error {
	// Parse timeout values from config
	readTimeout, _ := time.ParseDuration(cfg.Server.ReadTimeout)
	writeTimeout, _ := time.ParseDuration(cfg.Server.WriteTimeout)
	idleTimeout, _ := time.ParseDuration(cfg.Server.IdleTimeout)

	// Use defaults if parsing failed
	if readTimeout == 0 {
		readTimeout = 15 * time.Second
	}
	if writeTimeout == 0 {
		writeTimeout = 15 * time.Second
	}
	if idleTimeout == 0 {
		idleTimeout = 60 * time.Second
	}

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      handler,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	// Start server in goroutine
	go func() {
		logger.Info("HTTP server starting", "address", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return server.Shutdown(ctx)
}

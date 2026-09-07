package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/clients"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/config"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/handlers"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/repositories"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/services"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

var version = "dev"

const scope = "telemetrydrops.com/order-service"

func main() {
	ctx := context.Background()

	cfg, err := config.Load("configs/order.yaml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	providers, err := telemetry.SetupTelemetry(ctx, scope, version, "configs/otel-order.yaml")
	if err != nil {
		log.Fatalf("telemetry: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := providers.Closer(shutdownCtx); err != nil {
			providers.Logger.ErrorContext(shutdownCtx, "shutdown telemetry", "error", err)
		}
	}()

	providers.Logger.Info("order-service starting", "version", version, "http_port", cfg.HTTPPort)

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		providers.Logger.Error("open db", "error", err)
		os.Exit(1)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Order{}, &models.OrderItem{}); err != nil {
		providers.Logger.Error("migrate", "error", err)
		os.Exit(1)
	}

	catClient, conn, err := clients.New(ctx, cfg.Catalog.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		providers.Logger.Error("dial catalog", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	userRepo := repositories.NewUserRepository(db)
	orderRepo := repositories.NewOrderRepository(db)

	userSvc, err := services.NewUserService(userRepo, providers.Logger)
	if err != nil {
		providers.Logger.Error("user service", "error", err)
		os.Exit(1)
	}
	orderSvc, err := services.NewOrderService(orderRepo, catClient, providers.Logger)
	if err != nil {
		providers.Logger.Error("order service", "error", err)
		os.Exit(1)
	}

	gin.SetMode(gin.DebugMode)
	router := gin.New()
	router.Use(otelgin.Middleware("order-service"), gin.Recovery())
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "healthy"}) })
	api := router.Group("/api/v1")
	handlers.NewUserHandler(userSvc).RegisterRoutes(api)
	handlers.NewOrderHandler(orderSvc).RegisterRoutes(api)

	srv := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		providers.Logger.Info("http server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			providers.Logger.Error("http server", "error", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	providers.Logger.Info("shutting down http server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}

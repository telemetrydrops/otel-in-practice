package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	catalogv1 "github.com/telemetrydrops/otel-in-practice/stage-2-microservices/proto/catalog/v1"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/config"
	catgrpc "github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/grpc"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/repositories"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/services"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

var version = "dev"

const scope = "telemetrydrops.com/catalog-service"

func main() {
	ctx := context.Background()

	cfg, err := config.Load("configs/catalog.yaml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	providers, err := telemetry.SetupTelemetry(ctx, scope, version, "configs/otel-catalog.yaml")
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

	providers.Logger.Info("catalog-service starting", "version", version, "grpc_port", cfg.GRPCPort)

	db, err := gorm.Open(postgres.Open(cfg.Database.DSN), &gorm.Config{})
	if err != nil {
		providers.Logger.Error("open db", "error", err)
		os.Exit(1)
	}
	if err := db.AutoMigrate(&models.Product{}); err != nil {
		providers.Logger.Error("migrate", "error", err)
		os.Exit(1)
	}

	productRepo := repositories.NewProductRepository(db)
	productSvc, err := services.NewProductService(productRepo, providers.Logger)
	if err != nil {
		providers.Logger.Error("product service", "error", err)
		os.Exit(1)
	}
	inventorySvc := services.NewInventoryService(productRepo, providers.Logger)

	server := grpc.NewServer(grpc.StatsHandler(otelgrpc.NewServerHandler()))
	catalogv1.RegisterCatalogServiceServer(server, catgrpc.NewServer(productSvc, inventorySvc))

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		providers.Logger.Error("listen", "error", err)
		os.Exit(1)
	}

	go func() {
		providers.Logger.Info("grpc server listening", "addr", lis.Addr().String())
		if err := server.Serve(lis); err != nil {
			providers.Logger.Error("grpc serve", "error", err)
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	providers.Logger.Info("shutting down grpc server")
	server.GracefulStop()
}

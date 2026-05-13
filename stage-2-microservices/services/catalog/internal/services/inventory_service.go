package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// InventoryService answers stock-availability questions.
type InventoryService struct {
	repo   productRepo
	logger *slog.Logger
	tracer trace.Tracer
}

// NewInventoryService creates a new InventoryService.
func NewInventoryService(repo productRepo, logger *slog.Logger) *InventoryService {
	return &InventoryService{
		repo:   repo,
		logger: logger,
		tracer: otel.Tracer("telemetrydrops.com/catalog-service"),
	}
}

// CheckInventory reports whether at least qty units of productID are in stock,
// along with the current stock level.
func (s *InventoryService) CheckInventory(ctx context.Context, productID string, qty int32) (bool, int32, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceInventoryCheckName,
		trace.WithAttributes(
			attribute.String(telemetry.AttrEcommerceProductId, productID),
			attribute.Int("requested.qty", int(qty)),
		))
	defer span.End()

	product, err := s.repo.GetByID(ctx, productID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, 0, ErrProductNotFound
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "inventory check failed")
		return false, 0, fmt.Errorf("checking inventory: %w", err)
	}

	available := product.StockQty >= qty
	span.SetAttributes(
		attribute.Bool("inventory.available", available),
		attribute.Int("inventory.stock_qty", int(product.StockQty)),
	)
	return available, product.StockQty, nil
}

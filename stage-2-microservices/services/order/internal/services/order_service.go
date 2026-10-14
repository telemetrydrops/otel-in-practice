package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/clients"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// ErrOrderNotFound indicates an order id has no row.
var ErrOrderNotFound = errors.New("order not found")

// ErrOrderProductMissing indicates a referenced product was not in the catalog.
var ErrOrderProductMissing = errors.New("order references unknown product")

// OrderItemInput is a single line on a CreateOrderInput.
type OrderItemInput struct {
	ProductID string
	Qty       int32
}

// CreateOrderInput is the request payload for OrderService.Create.
type CreateOrderInput struct {
	UserID        string
	PaymentMethod string
	Items         []OrderItemInput
}

type orderRepo interface {
	Create(ctx context.Context, o *models.Order) error
	GetByID(ctx context.Context, id string) (*models.Order, error)
}

// catalogClient is the surface OrderService needs from the catalog.
type catalogClient interface {
	GetProduct(ctx context.Context, id string) (*clients.Product, error)
}

// OrderService implements the order business logic.
type OrderService struct {
	repo            orderRepo
	catalog         catalogClient
	logger          *slog.Logger
	tracer          trace.Tracer
	processDuration metric.Float64Histogram
}

// NewOrderService creates an OrderService.
func NewOrderService(repo orderRepo, catalog catalogClient, logger *slog.Logger) (*OrderService, error) {
	meter := otel.Meter("telemetrydrops.com/order-service")
	h, err := meter.Float64Histogram(
		telemetry.EcommerceOrdersProcessingDurationName,
		metric.WithDescription("End-to-end duration of processing a single order"),
		metric.WithUnit(telemetry.EcommerceOrdersProcessingDurationUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("creating processing duration histogram: %w", err)
	}
	return &OrderService{
		repo:            repo,
		catalog:         catalog,
		logger:          logger,
		tracer:          otel.Tracer("telemetrydrops.com/order-service"),
		processDuration: h,
	}, nil
}

// Create processes a new order.
func (s *OrderService) Create(ctx context.Context, in CreateOrderInput) (*models.Order, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceOrderProcessName,
		trace.WithAttributes(
			attribute.String(telemetry.AttrEcommerceUserId, in.UserID),
			attribute.String(telemetry.AttrEcommercePaymentMethod, in.PaymentMethod),
		))
	defer span.End()

	start := time.Now()

	order := &models.Order{
		ID:            uuid.New().String(),
		UserID:        in.UserID,
		Status:        "created",
		PaymentMethod: in.PaymentMethod,
	}

	for _, item := range in.Items {
		product, err := s.catalog.GetProduct(ctx, item.ProductID)
		if err != nil {
			if errors.Is(err, clients.ErrCatalogNotFound) {
				// Handled outcome — do not mark span as Error.
				return nil, ErrOrderProductMissing
			}
			telemetry.EmitException(ctx, err)
			span.SetStatus(codes.Error, "catalog lookup failed")
			return nil, fmt.Errorf("looking up product %s: %w", item.ProductID, err)
		}
		order.Items = append(order.Items, models.OrderItem{
			ID:             uuid.New().String(),
			OrderID:        order.ID,
			ProductID:      product.ID,
			Qty:            item.Qty,
			UnitPriceCents: product.PriceCents,
		})
		order.TotalCents += int64(item.Qty) * product.PriceCents
	}

	if err := s.repo.Create(ctx, order); err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "order persist failed")
		return nil, fmt.Errorf("persisting order: %w", err)
	}

	s.processDuration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(
		attribute.String(telemetry.AttrEcommercePaymentMethod, in.PaymentMethod),
	))

	if span.IsRecording() {
		span.SetAttributes(
			attribute.String(telemetry.AttrEcommerceOrderId, order.ID),
			attribute.Float64(telemetry.AttrEcommerceOrderTotal, float64(order.TotalCents)/100),
		)
	}

	return order, nil
}

// Get returns an order by id.
func (s *OrderService) Get(ctx context.Context, id string) (*models.Order, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceOrderGetName,
		trace.WithAttributes(attribute.String(telemetry.AttrEcommerceOrderId, id)))
	defer span.End()

	o, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "order lookup failed")
		return nil, fmt.Errorf("getting order: %w", err)
	}
	return o, nil
}

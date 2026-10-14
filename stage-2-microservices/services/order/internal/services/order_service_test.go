package services

import (
	"context"
	"errors"
	"testing"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/clients"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/models"
)

type mockOrderRepo struct {
	createFn  func(ctx context.Context, o *models.Order) error
	getByIDFn func(ctx context.Context, id string) (*models.Order, error)
}

func (m *mockOrderRepo) Create(ctx context.Context, o *models.Order) error {
	return m.createFn(ctx, o)
}

func (m *mockOrderRepo) GetByID(ctx context.Context, id string) (*models.Order, error) {
	return m.getByIDFn(ctx, id)
}

type mockCatalogClient struct {
	getProductFn func(ctx context.Context, id string) (*clients.Product, error)
}

func (m *mockCatalogClient) GetProduct(ctx context.Context, id string) (*clients.Product, error) {
	return m.getProductFn(ctx, id)
}

func TestOrderService_Create_PriceSnapshottedFromCatalog(t *testing.T) {
	cat := &mockCatalogClient{getProductFn: func(_ context.Context, id string) (*clients.Product, error) {
		return &clients.Product{ID: id, Name: "Widget", PriceCents: 999, StockQty: 100}, nil
	}}
	repo := &mockOrderRepo{createFn: func(_ context.Context, _ *models.Order) error { return nil }}

	svc, err := NewOrderService(repo, cat, newTestLogger())
	if err != nil {
		t.Fatalf("NewOrderService: %v", err)
	}
	o, err := svc.Create(context.Background(), CreateOrderInput{
		UserID:        "user-1",
		PaymentMethod: "credit_card",
		Items:         []OrderItemInput{{ProductID: "prod-1", Qty: 2}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(o.Items) != 1 || o.Items[0].UnitPriceCents != 999 {
		t.Fatalf("unexpected snapshot: %+v", o.Items)
	}
	if o.TotalCents != 2*999 {
		t.Fatalf("got total=%d, want %d", o.TotalCents, 2*999)
	}
}

func TestOrderService_Create_ProductNotFound(t *testing.T) {
	cat := &mockCatalogClient{getProductFn: func(_ context.Context, _ string) (*clients.Product, error) {
		return nil, clients.ErrCatalogNotFound
	}}
	repo := &mockOrderRepo{createFn: func(_ context.Context, _ *models.Order) error { return nil }}

	svc, err := NewOrderService(repo, cat, newTestLogger())
	if err != nil {
		t.Fatalf("NewOrderService: %v", err)
	}
	_, err = svc.Create(context.Background(), CreateOrderInput{
		UserID:        "user-1",
		PaymentMethod: "credit_card",
		Items:         []OrderItemInput{{ProductID: "missing", Qty: 1}},
	})
	if !errors.Is(err, ErrOrderProductMissing) {
		t.Fatalf("got err=%v, want ErrOrderProductMissing", err)
	}
}

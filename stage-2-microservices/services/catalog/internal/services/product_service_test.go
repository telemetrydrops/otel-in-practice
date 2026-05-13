package services

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/models"
	"gorm.io/gorm"
)

type mockProductRepo struct {
	getByIDFn func(ctx context.Context, id string) (*models.Product, error)
	listFn    func(ctx context.Context, category string, limit int) ([]*models.Product, error)
}

func (m *mockProductRepo) GetByID(ctx context.Context, id string) (*models.Product, error) {
	return m.getByIDFn(ctx, id)
}

func (m *mockProductRepo) List(ctx context.Context, category string, limit int) ([]*models.Product, error) {
	return m.listFn(ctx, category, limit)
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

func TestProductService_GetProduct_ReturnsProduct(t *testing.T) {
	repo := &mockProductRepo{
		getByIDFn: func(_ context.Context, id string) (*models.Product, error) {
			return &models.Product{ID: id, Name: "Laptop", Category: "electronics", PriceCents: 129999, StockQty: 5}, nil
		},
	}
	svc, err := NewProductService(repo, newTestLogger())
	if err != nil {
		t.Fatalf("NewProductService: %v", err)
	}
	got, err := svc.GetProduct(context.Background(), "prod-1")
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if got.Name != "Laptop" {
		t.Fatalf("got name=%q, want Laptop", got.Name)
	}
}

func TestProductService_GetProduct_NotFound(t *testing.T) {
	repo := &mockProductRepo{
		getByIDFn: func(_ context.Context, _ string) (*models.Product, error) {
			return nil, gorm.ErrRecordNotFound
		},
	}
	svc, err := NewProductService(repo, newTestLogger())
	if err != nil {
		t.Fatalf("NewProductService: %v", err)
	}
	_, err = svc.GetProduct(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("got err=%v, want ErrProductNotFound", err)
	}
}

func TestProductService_ListProducts_ReturnsItems(t *testing.T) {
	repo := &mockProductRepo{
		listFn: func(_ context.Context, category string, limit int) ([]*models.Product, error) {
			return []*models.Product{{ID: "a", Category: category}, {ID: "b", Category: category}}, nil
		},
	}
	svc, err := NewProductService(repo, newTestLogger())
	if err != nil {
		t.Fatalf("NewProductService: %v", err)
	}
	got, err := svc.ListProducts(context.Background(), "books", 10)
	if err != nil {
		t.Fatalf("ListProducts: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d products, want 2", len(got))
	}
}

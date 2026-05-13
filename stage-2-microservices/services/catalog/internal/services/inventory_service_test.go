package services

import (
	"context"
	"testing"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/models"
)

func TestInventoryService_CheckInventory_Available(t *testing.T) {
	repo := &mockProductRepo{
		getByIDFn: func(_ context.Context, id string) (*models.Product, error) {
			return &models.Product{ID: id, StockQty: 10}, nil
		},
	}
	svc := NewInventoryService(repo, newTestLogger())
	avail, stock, err := svc.CheckInventory(context.Background(), "prod-1", 5)
	if err != nil {
		t.Fatalf("CheckInventory: %v", err)
	}
	if !avail {
		t.Fatal("expected available=true")
	}
	if stock != 10 {
		t.Fatalf("got stock=%d, want 10", stock)
	}
}

func TestInventoryService_CheckInventory_NotEnoughStock(t *testing.T) {
	repo := &mockProductRepo{
		getByIDFn: func(_ context.Context, id string) (*models.Product, error) {
			return &models.Product{ID: id, StockQty: 2}, nil
		},
	}
	svc := NewInventoryService(repo, newTestLogger())
	avail, stock, err := svc.CheckInventory(context.Background(), "prod-1", 5)
	if err != nil {
		t.Fatalf("CheckInventory: %v", err)
	}
	if avail {
		t.Fatal("expected available=false")
	}
	if stock != 2 {
		t.Fatalf("got stock=%d, want 2", stock)
	}
}

package clients

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	catalogv1 "github.com/telemetrydrops/otel-in-practice/stage-2-microservices/proto/catalog/v1"
)

type mockCatalogClient struct {
	getProductFn func(ctx context.Context, req *catalogv1.GetProductRequest, opts ...grpc.CallOption) (*catalogv1.GetProductResponse, error)
	checkInvFn   func(ctx context.Context, req *catalogv1.CheckInventoryRequest, opts ...grpc.CallOption) (*catalogv1.CheckInventoryResponse, error)
}

func (m *mockCatalogClient) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest, opts ...grpc.CallOption) (*catalogv1.GetProductResponse, error) {
	return m.getProductFn(ctx, req, opts...)
}

func (m *mockCatalogClient) ListProducts(ctx context.Context, req *catalogv1.ListProductsRequest, opts ...grpc.CallOption) (catalogv1.CatalogService_ListProductsClient, error) {
	return nil, nil
}

func (m *mockCatalogClient) CheckInventory(ctx context.Context, req *catalogv1.CheckInventoryRequest, opts ...grpc.CallOption) (*catalogv1.CheckInventoryResponse, error) {
	return m.checkInvFn(ctx, req, opts...)
}

func TestCatalogClient_GetProduct_ReturnsProduct(t *testing.T) {
	mc := &mockCatalogClient{
		getProductFn: func(_ context.Context, req *catalogv1.GetProductRequest, _ ...grpc.CallOption) (*catalogv1.GetProductResponse, error) {
			return &catalogv1.GetProductResponse{Product: &catalogv1.Product{Id: req.GetId(), Name: "Widget", PriceCents: 1500}}, nil
		},
	}
	cc := NewWithClient(mc)
	got, err := cc.GetProduct(context.Background(), "prod-1")
	if err != nil {
		t.Fatalf("GetProduct: %v", err)
	}
	if got.Name != "Widget" || got.PriceCents != 1500 {
		t.Fatalf("unexpected product: %+v", got)
	}
}

func TestCatalogClient_GetProduct_NotFound(t *testing.T) {
	mc := &mockCatalogClient{
		getProductFn: func(_ context.Context, _ *catalogv1.GetProductRequest, _ ...grpc.CallOption) (*catalogv1.GetProductResponse, error) {
			return nil, status.Error(codes.NotFound, "not found")
		},
	}
	cc := NewWithClient(mc)
	_, err := cc.GetProduct(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound, got %v", err)
	}
}

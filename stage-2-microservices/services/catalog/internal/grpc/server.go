package grpc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	catalogv1 "github.com/telemetrydrops/otel-in-practice/stage-2-microservices/proto/catalog/v1"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/services"
)

// Server adapts the business services to the catalog gRPC contract.
type Server struct {
	catalogv1.UnimplementedCatalogServiceServer
	products  *services.ProductService
	inventory *services.InventoryService
}

// NewServer creates a Server.
func NewServer(products *services.ProductService, inventory *services.InventoryService) *Server {
	return &Server{products: products, inventory: inventory}
}

// GetProduct handles GetProductRequest.
func (s *Server) GetProduct(ctx context.Context, req *catalogv1.GetProductRequest) (*catalogv1.GetProductResponse, error) {
	product, err := s.products.GetProduct(ctx, req.GetId())
	if err != nil {
		if errors.Is(err, services.ErrProductNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &catalogv1.GetProductResponse{Product: toProtoProduct(product)}, nil
}

// ListProducts streams products matching the filter.
func (s *Server) ListProducts(req *catalogv1.ListProductsRequest, stream catalogv1.CatalogService_ListProductsServer) error {
	products, err := s.products.ListProducts(stream.Context(), req.GetCategory(), int(req.GetLimit()))
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}
	for _, p := range products {
		if err := stream.Send(&catalogv1.ListProductsResponse{Product: toProtoProduct(p)}); err != nil {
			return err
		}
	}
	return nil
}

// CheckInventory handles CheckInventoryRequest.
func (s *Server) CheckInventory(ctx context.Context, req *catalogv1.CheckInventoryRequest) (*catalogv1.CheckInventoryResponse, error) {
	available, stock, err := s.inventory.CheckInventory(ctx, req.GetProductId(), req.GetQty())
	if err != nil {
		if errors.Is(err, services.ErrProductNotFound) {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &catalogv1.CheckInventoryResponse{Available: available, StockQty: stock}, nil
}

func toProtoProduct(p *models.Product) *catalogv1.Product {
	return &catalogv1.Product{
		Id:         p.ID,
		Name:       p.Name,
		Category:   p.Category,
		PriceCents: p.PriceCents,
		StockQty:   p.StockQty,
	}
}

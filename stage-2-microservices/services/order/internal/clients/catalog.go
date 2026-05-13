package clients

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	catalogv1 "github.com/telemetrydrops/otel-in-practice/stage-2-microservices/proto/catalog/v1"
)

// ErrCatalogNotFound is returned when the catalog reports the product/id is unknown.
var ErrCatalogNotFound = errors.New("catalog: not found")

// IsNotFound reports whether err is a not-found from the catalog.
func IsNotFound(err error) bool { return errors.Is(err, ErrCatalogNotFound) }

// Product is the order-service view of a catalog product.
type Product struct {
	ID         string
	Name       string
	Category   string
	PriceCents int64
	StockQty   int32
}

// CatalogClient is the small typed surface order-service uses.
type CatalogClient struct {
	rpc catalogv1.CatalogServiceClient
}

// NewWithClient wraps an existing CatalogServiceClient (test seam).
func NewWithClient(rpc catalogv1.CatalogServiceClient) *CatalogClient {
	return &CatalogClient{rpc: rpc}
}

// New dials addr and returns a CatalogClient. The caller owns the returned conn.
func New(ctx context.Context, addr string, dialOpts ...grpc.DialOption) (*CatalogClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("dial catalog %q: %w", addr, err)
	}
	return NewWithClient(catalogv1.NewCatalogServiceClient(conn)), conn, nil
}

// GetProduct fetches a product by id.
func (c *CatalogClient) GetProduct(ctx context.Context, id string) (*Product, error) {
	resp, err := c.rpc.GetProduct(ctx, &catalogv1.GetProductRequest{Id: id})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrCatalogNotFound
		}
		return nil, fmt.Errorf("catalog GetProduct: %w", err)
	}
	p := resp.GetProduct()
	return &Product{
		ID:         p.GetId(),
		Name:       p.GetName(),
		Category:   p.GetCategory(),
		PriceCents: p.GetPriceCents(),
		StockQty:   p.GetStockQty(),
	}, nil
}

// CheckInventory asks the catalog whether qty units of productID are available.
func (c *CatalogClient) CheckInventory(ctx context.Context, productID string, qty int32) (bool, int32, error) {
	resp, err := c.rpc.CheckInventory(ctx, &catalogv1.CheckInventoryRequest{ProductId: productID, Qty: qty})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return false, 0, ErrCatalogNotFound
		}
		return false, 0, fmt.Errorf("catalog CheckInventory: %w", err)
	}
	return resp.GetAvailable(), resp.GetStockQty(), nil
}

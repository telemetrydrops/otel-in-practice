# Stage 2: Microservices Split — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a working two-service reference implementation (`order-service` HTTP + `catalog-service` gRPC) that demonstrates cross-process OpenTelemetry patterns, mirroring stage-1's conventions.

**Architecture:** Single Go module under `stage-2-microservices/` with two services calling each other across an HTTP→gRPC boundary, both backed by their own Postgres database, both exporting OTLP/HTTP to a shared `grafana/otel-lgtm` container. A shared `telemetry` package wraps SDK setup, log-based events, and Weaver-generated constants. Internal business spans use verb-object names (`process order`, `lookup product`); HTTP/gRPC/DB spans come from instrumentation libraries.

**Tech Stack:** Go 1.25, Gin, GORM (Postgres driver), gRPC + Protobuf, Buf for codegen, OpenTelemetry SDK + otelconf 1.0, otelgin, otelgrpc, otelslog, Weaver for telemetry conventions.

**Spec:** [docs/superpowers/specs/2026-05-08-stage-2-microservices-design.md](../specs/2026-05-08-stage-2-microservices-design.md)

---

## Pre-flight Notes

- All paths in this plan are relative to the repo root unless prefixed with `/`.
- Module path is `github.com/telemetrydrops/otel-in-practice/stage-2-microservices`.
- Run `go fmt ./...`, `go build ./...`, and `go test ./...` from `stage-2-microservices/` after each task that touches Go code.
- Commit messages use conventional commits.
- When porting stage-1 files, **read the original first** (`/home/jpkroehling/Projects/src/github.com/telemetrydrops/otel-in-practice/stage-1-monolith/...`), do not paraphrase from memory.
- For mocks, use simple hand-written mock structs implementing the consumer's interface — do not bring in mockery/gomock.

---

### Task 1: Bootstrap stage-2 module skeleton

**Files:**

- Create: `stage-2-microservices/go.mod`
- Create: `stage-2-microservices/.gitignore`
- Create: `stage-2-microservices/README.md` (placeholder, expanded later)

- [ ] **Step 1: Create directory and initialize module**

```bash
mkdir -p stage-2-microservices
cd stage-2-microservices
go mod init github.com/telemetrydrops/otel-in-practice/stage-2-microservices
```

- [ ] **Step 2: Pin Go toolchain**

Edit `stage-2-microservices/go.mod` so the second line reads `go 1.25.0` (matches stage-1).

- [ ] **Step 3: Add .gitignore**

Write `stage-2-microservices/.gitignore`:

```gitignore
bin/
*.out
local/
```

- [ ] **Step 4: Add placeholder README**

Write `stage-2-microservices/README.md`:

```markdown
# Stage 2: Microservices Split

Two-service reference implementation: `order-service` (HTTP) and `catalog-service` (gRPC).

Status: under construction. See `docs/superpowers/plans/2026-05-08-stage-2-microservices.md`.
```

- [ ] **Step 5: Commit**

```bash
git add stage-2-microservices/go.mod stage-2-microservices/.gitignore stage-2-microservices/README.md
git commit -m "feat(stage-2): bootstrap module skeleton"
```

---

### Task 2: Define and generate the catalog gRPC proto

**Files:**

- Create: `stage-2-microservices/buf.yaml`
- Create: `stage-2-microservices/buf.gen.yaml`
- Create: `stage-2-microservices/proto/catalog/v1/catalog.proto`
- Create (generated): `stage-2-microservices/proto/catalog/v1/catalog.pb.go`
- Create (generated): `stage-2-microservices/proto/catalog/v1/catalog_grpc.pb.go`

- [ ] **Step 1: Verify buf is installed**

```bash
buf --version
```

If missing, install per <https://buf.build/docs/installation>. Do NOT proceed without buf.

- [ ] **Step 2: Write buf.yaml (workspace config)**

Write `stage-2-microservices/buf.yaml`:

```yaml
version: v2
modules:
  - path: proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

- [ ] **Step 3: Write buf.gen.yaml**

Write `stage-2-microservices/buf.gen.yaml`:

```yaml
version: v2
managed:
  enabled: true
  override:
    - file_option: go_package_prefix
      value: github.com/telemetrydrops/otel-in-practice/stage-2-microservices/proto
plugins:
  - remote: buf.build/protocolbuffers/go
    out: proto
    opt:
      - paths=source_relative
  - remote: buf.build/grpc/go
    out: proto
    opt:
      - paths=source_relative
      - require_unimplemented_servers=true
```

- [ ] **Step 4: Write the catalog proto**

Write `stage-2-microservices/proto/catalog/v1/catalog.proto`:

```protobuf
syntax = "proto3";

package catalog.v1;

service CatalogService {
  rpc GetProduct(GetProductRequest) returns (GetProductResponse);
  rpc ListProducts(ListProductsRequest) returns (stream Product);
  rpc CheckInventory(CheckInventoryRequest) returns (CheckInventoryResponse);
}

message Product {
  string id = 1;
  string name = 2;
  string category = 3;
  int64 price_cents = 4;
  int32 stock_qty = 5;
}

message GetProductRequest {
  string id = 1;
}

message GetProductResponse {
  Product product = 1;
}

message ListProductsRequest {
  string category = 1;
  int32 limit = 2;
}

message CheckInventoryRequest {
  string product_id = 1;
  int32 qty = 2;
}

message CheckInventoryResponse {
  bool available = 1;
  int32 stock_qty = 2;
}
```

- [ ] **Step 5: Lint and generate**

```bash
cd stage-2-microservices
buf lint
buf generate
```

Expected: lint passes, files appear at `proto/catalog/v1/catalog.pb.go` and `proto/catalog/v1/catalog_grpc.pb.go`.

- [ ] **Step 6: Wire google.golang.org/grpc into the module**

```bash
cd stage-2-microservices
go get google.golang.org/grpc@v1.80.0
go get google.golang.org/protobuf@v1.36.11
go mod tidy
go build ./...
```

Expected: build succeeds.

- [ ] **Step 7: Commit**

```bash
git add stage-2-microservices/buf.yaml stage-2-microservices/buf.gen.yaml stage-2-microservices/proto/ stage-2-microservices/go.mod stage-2-microservices/go.sum
git commit -m "feat(stage-2): define catalog v1 gRPC API"
```

---

### Task 3: Docker compose stack with two databases

**Files:**

- Create: `stage-2-microservices/docker-compose.yml`
- Create: `stage-2-microservices/scripts/init-databases.sh`

- [ ] **Step 1: Write the database init script**

Write `stage-2-microservices/scripts/init-databases.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    CREATE DATABASE orders;
    CREATE DATABASE catalog;
EOSQL
```

- [ ] **Step 2: Mark it executable**

```bash
chmod +x stage-2-microservices/scripts/init-databases.sh
```

- [ ] **Step 3: Write docker-compose.yml (services-only stub for now)**

Write `stage-2-microservices/docker-compose.yml`:

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: ecommerce
      POSTGRES_PASSWORD: ecommerce
      POSTGRES_DB: postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./scripts/init-databases.sh:/docker-entrypoint-initdb.d/init-databases.sh:z

  otel-lgtm:
    image: grafana/otel-lgtm:latest
    ports:
      - "3000:3000"
      - "4318:4318"

volumes:
  postgres-data:
```

Note: the `:z` SELinux relabel flag matches the user's environment.

- [ ] **Step 4: Smoke test the stack**

```bash
cd stage-2-microservices
docker compose up -d postgres otel-lgtm
sleep 5
docker compose exec postgres psql -U ecommerce -d postgres -c '\l' | grep -E '(orders|catalog)'
```

Expected: both `orders` and `catalog` appear in the database list.

- [ ] **Step 5: Tear down**

```bash
cd stage-2-microservices
docker compose down -v
```

- [ ] **Step 6: Commit**

```bash
git add stage-2-microservices/docker-compose.yml stage-2-microservices/scripts/
git commit -m "feat(stage-2): docker compose stack with postgres init"
```

---

### Task 4: Port Weaver registry with verb-object span names

**Files:**

- Create: `stage-2-microservices/telemetry/registry/manifest.yaml`
- Create: `stage-2-microservices/telemetry/registry/attributes.yaml`
- Create: `stage-2-microservices/telemetry/registry/spans.yaml`
- Create: `stage-2-microservices/telemetry/registry/metrics.yaml`
- Create: `stage-2-microservices/telemetry/templates/registry/go/{weaver.yaml,attributes.go.j2,metrics.go.j2,spans.go.j2}`

- [ ] **Step 1: Copy registry from stage-1 as the starting point**

```bash
cp -r stage-1-monolith/telemetry/registry stage-2-microservices/telemetry/
cp -r stage-1-monolith/telemetry/templates stage-2-microservices/telemetry/
```

- [ ] **Step 2: Update manifest.yaml**

Edit `stage-2-microservices/telemetry/registry/manifest.yaml`:

```yaml
name: ecommerce
description: "Telemetry conventions for the ecommerce two-service split"
semconv_version: 0.2.0
schema_url: https://telemetrydrops.com/schemas/ecommerce
```

(Bumped from 0.1.0 to 0.2.0 because the runtime span names change.)

- [ ] **Step 3: Update span name notes to verb-object form**

Edit `stage-2-microservices/telemetry/registry/spans.yaml`. For every entry, replace the `name.note` value with the verb-object form. Use this mapping:

| `type` | `name.note` (new) |
|---|---|
| `ecommerce.user.register` | `register user` |
| `ecommerce.user.get` | `get user` |
| `ecommerce.user.list` | `list users` |
| `ecommerce.order.process` | `process order` |
| `ecommerce.order.background.process` | `process order in background` |
| `ecommerce.order.get` | `get order` |
| `ecommerce.order.list` | `list orders` |
| `ecommerce.order.status.update` | `update order status` |
| `ecommerce.product.lookup` | `lookup product` |
| `ecommerce.product.list` | `list products` |
| `ecommerce.product.create` | `create product` |
| `ecommerce.product.stock.update` | `update product stock` |
| `ecommerce.inventory.check` | `check inventory` |

Leave `type`, `kind`, `stability`, `brief`, and `attributes` unchanged.

- [ ] **Step 4: Verify Weaver registry**

```bash
cd stage-2-microservices
weaver registry check -r telemetry/registry/
```

Expected: passes (the "definition/2 not yet stable" warning is normal).

- [ ] **Step 5: Commit**

```bash
git add stage-2-microservices/telemetry/
git commit -m "feat(stage-2): port telemetry registry with verb-object span names"
```

---

### Task 5: Generate Weaver constants into shared/telemetry

**Files:**

- Create (generated): `stage-2-microservices/shared/telemetry/attributes_gen.go`
- Create (generated): `stage-2-microservices/shared/telemetry/spans_gen.go`
- Create (generated): `stage-2-microservices/shared/telemetry/metrics_gen.go`

- [ ] **Step 1: Generate**

```bash
cd stage-2-microservices
mkdir -p shared/telemetry
weaver registry generate \
  --registry telemetry/registry/ \
  --templates telemetry/templates/ \
  go shared/telemetry
```

Expected: three `*_gen.go` files appear under `shared/telemetry/`.

- [ ] **Step 2: Format**

```bash
cd stage-2-microservices
gofmt -w shared/telemetry/
```

- [ ] **Step 3: Verify span constants use verb-object names**

```bash
grep "SpanEcommerceOrderProcessName" stage-2-microservices/shared/telemetry/spans_gen.go
```

Expected: `SpanEcommerceOrderProcessName = "process order"`. If it shows the dotted form, Step 3 of Task 4 was incomplete — fix and regenerate.

- [ ] **Step 4: Confirm package compiles**

```bash
cd stage-2-microservices
go build ./shared/telemetry/
```

Expected: builds clean (the generated package only declares constants).

- [ ] **Step 5: Commit**

```bash
git add stage-2-microservices/shared/telemetry/
git commit -m "feat(stage-2): generate weaver constants into shared/telemetry"
```

---

### Task 6: Port telemetry SDK setup and event helpers

**Files:**

- Create: `stage-2-microservices/shared/telemetry/setup.go`
- Create: `stage-2-microservices/shared/telemetry/events.go`
- Create: `stage-2-microservices/shared/telemetry/const.go`

- [ ] **Step 1: Read stage-1's telemetry files to understand the source**

Read these files in full before continuing:

- `stage-1-monolith/internal/telemetry/setup.go`
- `stage-1-monolith/internal/telemetry/events.go`
- `stage-1-monolith/internal/telemetry/const.go`

- [ ] **Step 2: Write shared/telemetry/const.go**

Write `stage-2-microservices/shared/telemetry/const.go`:

```go
package telemetry

// Baggage keys for cross-cutting context propagation.
const (
	BaggageCustomerTier  = "ecommerce.customer.tier"
	BaggagePaymentMethod = "ecommerce.payment.method"
)
```

Note: stage-1's `Scope`, `ServiceName`, and `ServiceVersion` constants are intentionally omitted — each service supplies its own scope at runtime via `SetupTelemetry`.

- [ ] **Step 3: Write shared/telemetry/setup.go**

Copy `stage-1-monolith/internal/telemetry/setup.go` to `stage-2-microservices/shared/telemetry/setup.go` and modify:

- The `serviceScope` package-level variable is set inside `SetupTelemetry` so `events.go` can read it. Add at the top of the file (after imports):
  ```go
  // serviceScope holds the instrumentation scope for log-based events emitted
  // by EmitEvent / EmitException. Set during SetupTelemetry.
  var serviceScope string
  ```
- In `SetupTelemetry`, set `serviceScope = serviceName` as the first statement after the function signature (before the `providersFromConfig` call).
- Everything else stays identical, including the `multiHandler` definition and `insertAttribute`.

- [ ] **Step 4: Write shared/telemetry/events.go**

Copy `stage-1-monolith/internal/telemetry/events.go` to `stage-2-microservices/shared/telemetry/events.go` and modify:

- Replace `eventLogger()` so it reads `serviceScope` instead of the const `Scope`:
  ```go
  func eventLogger() log.Logger {
      return global.GetLoggerProvider().Logger(serviceScope)
  }
  ```
- Everything else stays identical.

- [ ] **Step 5: Add OpenTelemetry dependencies**

```bash
cd stage-2-microservices
go get \
  github.com/google/uuid@v1.6.0 \
  go.opentelemetry.io/contrib/bridges/otelslog@v0.18.0 \
  go.opentelemetry.io/contrib/otelconf@v0.23.0 \
  go.opentelemetry.io/otel@v1.43.0 \
  go.opentelemetry.io/otel/log@v0.19.0 \
  go.opentelemetry.io/otel/metric@v1.43.0 \
  go.opentelemetry.io/otel/trace@v1.43.0
go mod tidy
```

- [ ] **Step 6: Verify shared package builds**

```bash
cd stage-2-microservices
go build ./shared/telemetry/
go vet ./shared/telemetry/
```

Expected: clean.

- [ ] **Step 7: Commit**

```bash
git add stage-2-microservices/shared/telemetry/setup.go stage-2-microservices/shared/telemetry/events.go stage-2-microservices/shared/telemetry/const.go stage-2-microservices/go.mod stage-2-microservices/go.sum
git commit -m "feat(stage-2): port telemetry setup and event helpers to shared package"
```

---

### Task 7: Catalog service — model and repository

**Files:**

- Create: `stage-2-microservices/services/catalog/internal/models/product.go`
- Create: `stage-2-microservices/services/catalog/internal/repositories/product_repo.go`

- [ ] **Step 1: Write the product model**

Write `stage-2-microservices/services/catalog/internal/models/product.go`:

```go
package models

import "time"

// Product is a sellable item in the catalog.
type Product struct {
	ID         string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Name       string    `gorm:"type:varchar(200);not null" json:"name"`
	Category   string    `gorm:"type:varchar(100);index" json:"category"`
	PriceCents int64     `gorm:"not null" json:"price_cents"`
	StockQty   int32     `gorm:"not null;default:0" json:"stock_qty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName returns the table name for Product.
func (Product) TableName() string { return "products" }
```

- [ ] **Step 2: Write the repository**

Write `stage-2-microservices/services/catalog/internal/repositories/product_repo.go`:

```go
package repositories

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// ProductRepository persists Product rows.
type ProductRepository struct {
	db     *gorm.DB
	tracer trace.Tracer
}

// NewProductRepository creates a ProductRepository.
func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{
		db:     db,
		tracer: otel.Tracer("telemetrydrops.com/catalog-service"),
	}
}

// GetByID returns a single product by id.
func (r *ProductRepository) GetByID(ctx context.Context, id string) (*models.Product, error) {
	ctx, span := r.tracer.Start(ctx, "SELECT products",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperation("SELECT"),
			semconv.DBSQLTable("products"),
			attribute.String(telemetry.AttrEcommerceProductId, id),
		))
	defer span.End()

	var product models.Product
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Error, "product not found")
			return nil, err
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "select failed")
		return nil, fmt.Errorf("selecting product: %w", err)
	}

	return &product, nil
}

// List returns products optionally filtered by category, capped by limit.
func (r *ProductRepository) List(ctx context.Context, category string, limit int) ([]*models.Product, error) {
	ctx, span := r.tracer.Start(ctx, "SELECT products",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperation("SELECT"),
			semconv.DBSQLTable("products"),
		))
	defer span.End()

	q := r.db.WithContext(ctx).Model(&models.Product{})
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}

	var products []*models.Product
	if err := q.Find(&products).Error; err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "select failed")
		return nil, fmt.Errorf("listing products: %w", err)
	}

	return products, nil
}
```

- [ ] **Step 3: Add gorm dependencies**

```bash
cd stage-2-microservices
go get gorm.io/gorm@v1.30.2 gorm.io/driver/postgres@v1.6.0
go mod tidy
```

- [ ] **Step 4: Verify build**

```bash
cd stage-2-microservices
go build ./services/catalog/...
```

Expected: clean. (semconv import path may need to be `semconv/v1.36.0` if v1.34.0 is removed in current contrib; if the import errors, switch to whichever version stage-1 uses by inspecting `stage-1-monolith/internal/repositories/product_repo.go`.)

- [ ] **Step 5: Commit**

```bash
git add stage-2-microservices/services/catalog/ stage-2-microservices/go.mod stage-2-microservices/go.sum
git commit -m "feat(catalog): product model and repository"
```

---

### Task 8: Catalog service — product service (TDD)

**Files:**

- Create: `stage-2-microservices/services/catalog/internal/services/product_service.go`
- Create: `stage-2-microservices/services/catalog/internal/services/product_service_test.go`

This is the first TDD-style task. Tests use a hand-written mock of `productRepo`.

- [ ] **Step 1: Define the repo interface that the service depends on**

The service will depend on a small interface, not the concrete `*ProductRepository`. Add to the same package as the service. (We define it in the service file itself for proximity, since this is the consumer.)

- [ ] **Step 2: Write the failing test**

Write `stage-2-microservices/services/catalog/internal/services/product_service_test.go`:

```go
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
```

- [ ] **Step 3: Run tests — they should fail to build**

```bash
cd stage-2-microservices
go test ./services/catalog/internal/services/...
```

Expected: build error — `services` package does not exist yet.

- [ ] **Step 4: Implement product_service.go**

Write `stage-2-microservices/services/catalog/internal/services/product_service.go`:

```go
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/catalog/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// ErrProductNotFound is returned when a product id does not exist.
var ErrProductNotFound = errors.New("product not found")

// productRepo is the subset of ProductRepository the service needs.
type productRepo interface {
	GetByID(ctx context.Context, id string) (*models.Product, error)
	List(ctx context.Context, category string, limit int) ([]*models.Product, error)
}

// ProductService implements the catalog product business logic.
type ProductService struct {
	repo          productRepo
	logger        *slog.Logger
	tracer        trace.Tracer
	lookupCounter metric.Int64Counter
}

// NewProductService creates a new ProductService.
func NewProductService(repo productRepo, logger *slog.Logger) (*ProductService, error) {
	meter := otel.Meter("telemetrydrops.com/catalog-service")
	lookupCounter, err := meter.Int64Counter(
		telemetry.EcommerceProductsLookupsName,
		metric.WithDescription("Total number of product lookups"),
		metric.WithUnit(telemetry.EcommerceProductsLookupsUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("creating lookup counter: %w", err)
	}

	return &ProductService{
		repo:          repo,
		logger:        logger,
		tracer:        otel.Tracer("telemetrydrops.com/catalog-service"),
		lookupCounter: lookupCounter,
	}, nil
}

// GetProduct returns a single product by id.
func (s *ProductService) GetProduct(ctx context.Context, id string) (*models.Product, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceProductLookupName,
		trace.WithAttributes(attribute.String(telemetry.AttrEcommerceProductId, id)))
	defer span.End()

	product, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Error, "product not found")
			return nil, ErrProductNotFound
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "product lookup failed")
		return nil, fmt.Errorf("getting product: %w", err)
	}

	s.lookupCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String(telemetry.AttrEcommerceProductCategory, product.Category),
	))

	if span.IsRecording() {
		span.SetAttributes(
			attribute.String(telemetry.AttrEcommerceProductCategory, product.Category),
		)
	}

	return product, nil
}

// ListProducts returns products optionally filtered by category.
func (s *ProductService) ListProducts(ctx context.Context, category string, limit int) ([]*models.Product, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceProductListName,
		trace.WithAttributes(
			attribute.String(telemetry.AttrEcommerceProductCategory, category),
			attribute.Int("limit", limit),
		))
	defer span.End()

	products, err := s.repo.List(ctx, category, limit)
	if err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "product list failed")
		return nil, fmt.Errorf("listing products: %w", err)
	}

	span.SetAttributes(attribute.Int("result.count", len(products)))
	return products, nil
}
```

- [ ] **Step 5: Run tests — they should pass**

```bash
cd stage-2-microservices
go test ./services/catalog/internal/services/...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add stage-2-microservices/services/catalog/internal/services/
git commit -m "feat(catalog): product service with TDD"
```

---

### Task 9: Catalog service — inventory service (TDD)

**Files:**

- Create: `stage-2-microservices/services/catalog/internal/services/inventory_service.go`
- Create: `stage-2-microservices/services/catalog/internal/services/inventory_service_test.go`

- [ ] **Step 1: Write the failing test**

Write `stage-2-microservices/services/catalog/internal/services/inventory_service_test.go`:

```go
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
```

- [ ] **Step 2: Run tests — should fail to build**

```bash
cd stage-2-microservices
go test ./services/catalog/internal/services/...
```

Expected: build error — `NewInventoryService` undefined.

- [ ] **Step 3: Implement inventory_service.go**

Write `stage-2-microservices/services/catalog/internal/services/inventory_service.go`:

```go
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
			span.SetStatus(codes.Error, "product not found")
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
```

- [ ] **Step 4: Run tests — should pass**

```bash
cd stage-2-microservices
go test ./services/catalog/internal/services/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add stage-2-microservices/services/catalog/internal/services/inventory_service.go stage-2-microservices/services/catalog/internal/services/inventory_service_test.go
git commit -m "feat(catalog): inventory service with TDD"
```

---

### Task 10: Catalog service — gRPC server handlers

**Files:**

- Create: `stage-2-microservices/services/catalog/internal/grpc/server.go`

- [ ] **Step 1: Write the gRPC server**

Write `stage-2-microservices/services/catalog/internal/grpc/server.go`:

```go
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
		if err := stream.Send(toProtoProduct(p)); err != nil {
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
```

- [ ] **Step 2: Verify build**

```bash
cd stage-2-microservices
go build ./services/catalog/...
```

Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add stage-2-microservices/services/catalog/internal/grpc/
git commit -m "feat(catalog): grpc server adapter"
```

---

### Task 11: Catalog service — main, config, otelconf YAML

**Files:**

- Create: `stage-2-microservices/services/catalog/cmd/main.go`
- Create: `stage-2-microservices/services/catalog/internal/config/config.go`
- Create: `stage-2-microservices/configs/otel-catalog.yaml`
- Create: `stage-2-microservices/configs/catalog.yaml`

- [ ] **Step 1: Write catalog config loader**

Write `stage-2-microservices/services/catalog/internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the catalog service configuration.
type Config struct {
	GRPCPort string `yaml:"grpc_port"`
	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`
}

// Load reads YAML from file, then overlays environment variables for fields
// that have known overrides (DATABASE_URL, GRPC_PORT).
func Load(file string) (*Config, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("GRPC_PORT"); v != "" {
		cfg.GRPCPort = v
	}
	return &cfg, nil
}
```

- [ ] **Step 2: Write catalog config YAML**

Write `stage-2-microservices/configs/catalog.yaml`:

```yaml
grpc_port: "9090"
database:
  dsn: "host=localhost port=5432 user=ecommerce password=ecommerce dbname=catalog sslmode=disable"
```

- [ ] **Step 3: Write the otelconf YAML**

Write `stage-2-microservices/configs/otel-catalog.yaml`:

```yaml
file_format: "1.0"
disabled: false
resource:
  attributes:
    - name: service.name
      value: catalog-service
    - name: service.namespace
      value: ecommerce
    - name: deployment.environment.name
      value: dev
propagator:
  composite:
    - tracecontext:
    - baggage:
tracer_provider:
  processors:
    - batch:
        exporter:
          otlp_http:
            endpoint: http://localhost:4318/v1/traces
meter_provider:
  readers:
    - periodic:
        interval: 15000
        exporter:
          otlp_http:
            endpoint: http://localhost:4318/v1/metrics
logger_provider:
  processors:
    - batch:
        exporter:
          otlp_http:
            endpoint: http://localhost:4318/v1/logs
```

- [ ] **Step 4: Write catalog main**

Write `stage-2-microservices/services/catalog/cmd/main.go`:

```go
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
```

- [ ] **Step 5: Add otelgrpc dependency**

```bash
cd stage-2-microservices
go get go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc
go get gopkg.in/yaml.v3
go mod tidy
```

- [ ] **Step 6: Verify build**

```bash
cd stage-2-microservices
go build ./services/catalog/...
```

Expected: clean.

- [ ] **Step 7: Smoke test catalog locally**

```bash
cd stage-2-microservices
docker compose up -d postgres otel-lgtm
sleep 5
cd services/catalog
go run ./cmd
```

Expected: log line "grpc server listening". Stop with Ctrl-C.

- [ ] **Step 8: Tear down**

```bash
cd stage-2-microservices
docker compose down -v
```

- [ ] **Step 9: Commit**

```bash
git add stage-2-microservices/services/catalog/cmd/ stage-2-microservices/services/catalog/internal/config/ stage-2-microservices/configs/ stage-2-microservices/go.mod stage-2-microservices/go.sum
git commit -m "feat(catalog): main entrypoint with telemetry and otel config"
```

---

### Task 12: Order service — models and repositories

**Files:**

- Create: `stage-2-microservices/services/order/internal/models/{user.go,order.go,order_item.go}`
- Create: `stage-2-microservices/services/order/internal/repositories/{user_repo.go,order_repo.go}`

- [ ] **Step 1: Write models**

Write `stage-2-microservices/services/order/internal/models/user.go`:

```go
package models

import "time"

// User is a customer.
type User struct {
	ID        string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Email     string    `gorm:"uniqueIndex;type:varchar(255);not null" json:"email"`
	Tier      string    `gorm:"type:varchar(32);not null;default:'standard'" json:"tier"`
	CreatedAt time.Time `json:"created_at"`
}

func (User) TableName() string { return "users" }
```

Write `stage-2-microservices/services/order/internal/models/order.go`:

```go
package models

import "time"

// Order represents a customer purchase.
type Order struct {
	ID            string      `gorm:"primaryKey;type:varchar(64)" json:"id"`
	UserID        string      `gorm:"type:varchar(64);index;not null" json:"user_id"`
	Status        string      `gorm:"type:varchar(32);not null" json:"status"`
	TotalCents    int64       `gorm:"not null" json:"total_cents"`
	PaymentMethod string      `gorm:"type:varchar(32)" json:"payment_method"`
	Items         []OrderItem `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE" json:"items"`
	CreatedAt     time.Time   `json:"created_at"`
}

func (Order) TableName() string { return "orders" }
```

Write `stage-2-microservices/services/order/internal/models/order_item.go`:

```go
package models

// OrderItem is a line item on an order. ProductID is a string reference to a
// product owned by the catalog service — there is no FK across the service
// boundary.
type OrderItem struct {
	ID             string `gorm:"primaryKey;type:varchar(64)" json:"id"`
	OrderID        string `gorm:"type:varchar(64);index;not null" json:"order_id"`
	ProductID      string `gorm:"type:varchar(64);not null" json:"product_id"`
	Qty            int32  `gorm:"not null" json:"qty"`
	UnitPriceCents int64  `gorm:"not null" json:"unit_price_cents"`
}

func (OrderItem) TableName() string { return "order_items" }
```

- [ ] **Step 2: Write user repository**

Write `stage-2-microservices/services/order/internal/repositories/user_repo.go`:

```go
package repositories

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// UserRepository persists User rows.
type UserRepository struct {
	db     *gorm.DB
	tracer trace.Tracer
}

// NewUserRepository creates a UserRepository.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db, tracer: otel.Tracer("telemetrydrops.com/order-service")}
}

// Create inserts a new user.
func (r *UserRepository) Create(ctx context.Context, u *models.User) error {
	ctx, span := r.tracer.Start(ctx, "INSERT users",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperation("INSERT"),
			semconv.DBSQLTable("users"),
			attribute.String(telemetry.AttrEcommerceUserId, u.ID),
		))
	defer span.End()

	if err := r.db.WithContext(ctx).Create(u).Error; err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "insert failed")
		return fmt.Errorf("creating user: %w", err)
	}
	return nil
}

// GetByID returns a user by id.
func (r *UserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	ctx, span := r.tracer.Start(ctx, "SELECT users",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperation("SELECT"),
			semconv.DBSQLTable("users"),
			attribute.String(telemetry.AttrEcommerceUserId, id),
		))
	defer span.End()

	var u models.User
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Error, "user not found")
			return nil, err
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "select failed")
		return nil, fmt.Errorf("selecting user: %w", err)
	}
	return &u, nil
}
```

- [ ] **Step 3: Write order repository**

Write `stage-2-microservices/services/order/internal/repositories/order_repo.go`:

```go
package repositories

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// OrderRepository persists Order rows.
type OrderRepository struct {
	db     *gorm.DB
	tracer trace.Tracer
}

// NewOrderRepository creates an OrderRepository.
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{db: db, tracer: otel.Tracer("telemetrydrops.com/order-service")}
}

// Create inserts an order and its items in a single transaction.
func (r *OrderRepository) Create(ctx context.Context, o *models.Order) error {
	ctx, span := r.tracer.Start(ctx, "INSERT orders",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperation("INSERT"),
			semconv.DBSQLTable("orders"),
			attribute.String(telemetry.AttrEcommerceOrderId, o.ID),
		))
	defer span.End()

	if err := r.db.WithContext(ctx).Create(o).Error; err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "insert failed")
		return fmt.Errorf("creating order: %w", err)
	}
	return nil
}

// GetByID returns an order by id, with its items.
func (r *OrderRepository) GetByID(ctx context.Context, id string) (*models.Order, error) {
	ctx, span := r.tracer.Start(ctx, "SELECT orders",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			semconv.DBSystemPostgreSQL,
			semconv.DBOperation("SELECT"),
			semconv.DBSQLTable("orders"),
			attribute.String(telemetry.AttrEcommerceOrderId, id),
		))
	defer span.End()

	var o models.Order
	if err := r.db.WithContext(ctx).Preload("Items").Where("id = ?", id).First(&o).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Error, "order not found")
			return nil, err
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "select failed")
		return nil, fmt.Errorf("selecting order: %w", err)
	}
	return &o, nil
}
```

- [ ] **Step 4: Verify build**

```bash
cd stage-2-microservices
go build ./services/order/...
```

Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add stage-2-microservices/services/order/internal/models/ stage-2-microservices/services/order/internal/repositories/
git commit -m "feat(order): models and repositories"
```

---

### Task 13: Order service — catalog gRPC client wrapper (TDD)

**Files:**

- Create: `stage-2-microservices/services/order/internal/clients/catalog.go`
- Create: `stage-2-microservices/services/order/internal/clients/catalog_test.go`

The wrapper provides a small typed surface (`GetProduct`, `CheckInventory`) so business logic doesn't import the generated proto package. It is constructed with a generated `CatalogServiceClient`, which is also what the test mocks.

- [ ] **Step 1: Write the failing test**

Write `stage-2-microservices/services/order/internal/clients/catalog_test.go`:

```go
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
```

- [ ] **Step 2: Run tests — should fail to build**

```bash
cd stage-2-microservices
go test ./services/order/internal/clients/...
```

Expected: build error.

- [ ] **Step 3: Implement the wrapper**

Write `stage-2-microservices/services/order/internal/clients/catalog.go`:

```go
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
```

- [ ] **Step 4: Run tests — should pass**

```bash
cd stage-2-microservices
go test ./services/order/internal/clients/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add stage-2-microservices/services/order/internal/clients/
git commit -m "feat(order): catalog grpc client wrapper with TDD"
```

---

### Task 14: Order service — user service (TDD)

**Files:**

- Create: `stage-2-microservices/services/order/internal/services/user_service.go`
- Create: `stage-2-microservices/services/order/internal/services/user_service_test.go`

- [ ] **Step 1: Write the failing test**

Write `stage-2-microservices/services/order/internal/services/user_service_test.go`:

```go
package services

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/models"
	"gorm.io/gorm"
)

type mockUserRepo struct {
	createFn  func(ctx context.Context, u *models.User) error
	getByIDFn func(ctx context.Context, id string) (*models.User, error)
}

func (m *mockUserRepo) Create(ctx context.Context, u *models.User) error {
	return m.createFn(ctx, u)
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	return m.getByIDFn(ctx, id)
}

func newTestLogger() *slog.Logger { return slog.New(slog.NewTextHandler(os.Stderr, nil)) }

func TestUserService_Register_AssignsIDAndPersists(t *testing.T) {
	var captured *models.User
	repo := &mockUserRepo{createFn: func(_ context.Context, u *models.User) error {
		captured = u
		return nil
	}}
	svc, err := NewUserService(repo, newTestLogger())
	if err != nil {
		t.Fatalf("NewUserService: %v", err)
	}
	got, err := svc.Register(context.Background(), "alice@example.com", "premium")
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if got.ID == "" {
		t.Fatal("expected ID to be assigned")
	}
	if captured == nil || captured.Email != "alice@example.com" || captured.Tier != "premium" {
		t.Fatalf("unexpected captured user: %+v", captured)
	}
}

func TestUserService_GetUser_NotFound(t *testing.T) {
	repo := &mockUserRepo{getByIDFn: func(_ context.Context, _ string) (*models.User, error) {
		return nil, gorm.ErrRecordNotFound
	}}
	svc, err := NewUserService(repo, newTestLogger())
	if err != nil {
		t.Fatalf("NewUserService: %v", err)
	}
	_, err = svc.GetUser(context.Background(), "missing")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("got err=%v, want ErrUserNotFound", err)
	}
}
```

- [ ] **Step 2: Run tests — should fail to build**

```bash
cd stage-2-microservices
go test ./services/order/internal/services/...
```

Expected: build error.

- [ ] **Step 3: Implement user_service.go**

Write `stage-2-microservices/services/order/internal/services/user_service.go`:

```go
package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/models"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// ErrUserNotFound indicates a user id has no row.
var ErrUserNotFound = errors.New("user not found")

type userRepo interface {
	Create(ctx context.Context, u *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
}

// UserService implements user-management business logic.
type UserService struct {
	repo               userRepo
	logger             *slog.Logger
	tracer             trace.Tracer
	registrationsTotal metric.Int64Counter
}

// NewUserService creates a UserService.
func NewUserService(repo userRepo, logger *slog.Logger) (*UserService, error) {
	meter := otel.Meter("telemetrydrops.com/order-service")
	c, err := meter.Int64Counter(
		telemetry.EcommerceUsersRegistrationsName,
		metric.WithDescription("Total number of successful user registrations"),
		metric.WithUnit(telemetry.EcommerceUsersRegistrationsUnit),
	)
	if err != nil {
		return nil, fmt.Errorf("creating registrations counter: %w", err)
	}
	return &UserService{
		repo:               repo,
		logger:             logger,
		tracer:             otel.Tracer("telemetrydrops.com/order-service"),
		registrationsTotal: c,
	}, nil
}

// Register creates a new user.
func (s *UserService) Register(ctx context.Context, email, tier string) (*models.User, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceUserRegisterName,
		trace.WithAttributes(attribute.String(telemetry.AttrEcommerceCustomerTier, tier)))
	defer span.End()

	u := &models.User{ID: uuid.New().String(), Email: email, Tier: tier}
	if err := s.repo.Create(ctx, u); err != nil {
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "user registration failed")
		return nil, fmt.Errorf("registering user: %w", err)
	}
	s.registrationsTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String(telemetry.AttrEcommerceCustomerTier, tier)))
	if span.IsRecording() {
		span.SetAttributes(attribute.String(telemetry.AttrEcommerceUserId, u.ID))
	}
	return u, nil
}

// GetUser returns a user by id.
func (s *UserService) GetUser(ctx context.Context, id string) (*models.User, error) {
	ctx, span := s.tracer.Start(ctx, telemetry.SpanEcommerceUserGetName,
		trace.WithAttributes(attribute.String(telemetry.AttrEcommerceUserId, id)))
	defer span.End()

	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			span.SetStatus(codes.Error, "user not found")
			return nil, ErrUserNotFound
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "user lookup failed")
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return u, nil
}
```

- [ ] **Step 4: Run tests — should pass**

```bash
cd stage-2-microservices
go test ./services/order/internal/services/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add stage-2-microservices/services/order/internal/services/user_service.go stage-2-microservices/services/order/internal/services/user_service_test.go
git commit -m "feat(order): user service with TDD"
```

---

### Task 15: Order service — order service (TDD)

**Files:**

- Create: `stage-2-microservices/services/order/internal/services/order_service.go`
- Create: `stage-2-microservices/services/order/internal/services/order_service_test.go`

- [ ] **Step 1: Write the failing test**

Write `stage-2-microservices/services/order/internal/services/order_service_test.go`:

```go
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
```

- [ ] **Step 2: Run tests — should fail to build**

```bash
cd stage-2-microservices
go test ./services/order/internal/services/...
```

Expected: build error.

- [ ] **Step 3: Implement order_service.go**

Write `stage-2-microservices/services/order/internal/services/order_service.go`:

```go
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
			span.SetStatus(codes.Error, "order not found")
			return nil, ErrOrderNotFound
		}
		telemetry.EmitException(ctx, err)
		span.SetStatus(codes.Error, "order lookup failed")
		return nil, fmt.Errorf("getting order: %w", err)
	}
	return o, nil
}
```

- [ ] **Step 4: Run tests — should pass**

```bash
cd stage-2-microservices
go test ./services/order/internal/services/...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add stage-2-microservices/services/order/internal/services/order_service.go stage-2-microservices/services/order/internal/services/order_service_test.go
git commit -m "feat(order): order service with TDD and price snapshotting"
```

---

### Task 16: Order service — HTTP handlers

**Files:**

- Create: `stage-2-microservices/services/order/internal/handlers/users.go`
- Create: `stage-2-microservices/services/order/internal/handlers/orders.go`

- [ ] **Step 1: Write user handler**

Write `stage-2-microservices/services/order/internal/handlers/users.go`:

```go
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/services"
)

// UserHandler exposes user endpoints.
type UserHandler struct {
	svc *services.UserService
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(svc *services.UserService) *UserHandler { return &UserHandler{svc: svc} }

// RegisterRoutes registers user routes.
func (h *UserHandler) RegisterRoutes(r gin.IRouter) {
	r.POST("/users", h.create)
	r.GET("/users/:id", h.get)
}

type createUserRequest struct {
	Email string `json:"email" binding:"required,email"`
	Tier  string `json:"tier" binding:"required,oneof=standard premium"`
}

func (h *UserHandler) create(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	u, err := h.svc.Register(c.Request.Context(), req.Email, req.Tier)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, u)
}

func (h *UserHandler) get(c *gin.Context) {
	u, err := h.svc.GetUser(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, services.ErrUserNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, u)
}
```

- [ ] **Step 2: Write order handler**

Write `stage-2-microservices/services/order/internal/handlers/orders.go`:

```go
package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/baggage"

	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/services/order/internal/services"
	"github.com/telemetrydrops/otel-in-practice/stage-2-microservices/shared/telemetry"
)

// OrderHandler exposes order endpoints.
type OrderHandler struct {
	svc *services.OrderService
}

// NewOrderHandler creates an OrderHandler.
func NewOrderHandler(svc *services.OrderService) *OrderHandler { return &OrderHandler{svc: svc} }

// RegisterRoutes registers order routes.
func (h *OrderHandler) RegisterRoutes(r gin.IRouter) {
	r.POST("/orders", h.create)
	r.GET("/orders/:id", h.get)
}

type createOrderRequest struct {
	UserID        string                  `json:"user_id" binding:"required"`
	PaymentMethod string                  `json:"payment_method" binding:"required,oneof=credit_card paypal apple_pay"`
	Items         []createOrderItemRequest `json:"items" binding:"required,min=1,dive"`
}

type createOrderItemRequest struct {
	ProductID string `json:"product_id" binding:"required"`
	Qty       int32  `json:"qty" binding:"required,min=1"`
}

func (h *OrderHandler) create(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Read X-Customer-Tier and put it in baggage so it propagates to catalog.
	tier := c.GetHeader("X-Customer-Tier")
	if tier == "" {
		tier = "standard"
	}
	if member, err := baggage.NewMember(telemetry.BaggageCustomerTier, tier); err == nil {
		if bag, err := baggage.FromContext(ctx).SetMember(member); err == nil {
			ctx = baggage.ContextWithBaggage(ctx, bag)
		}
	}

	items := make([]services.OrderItemInput, 0, len(req.Items))
	for _, it := range req.Items {
		items = append(items, services.OrderItemInput{ProductID: it.ProductID, Qty: it.Qty})
	}
	o, err := h.svc.Create(ctx, services.CreateOrderInput{
		UserID:        req.UserID,
		PaymentMethod: req.PaymentMethod,
		Items:         items,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrOrderProductMissing):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}
	c.JSON(http.StatusCreated, o)
}

func (h *OrderHandler) get(c *gin.Context) {
	o, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, services.ErrOrderNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, o)
}
```

- [ ] **Step 3: Add gin and otelgin dependencies**

```bash
cd stage-2-microservices
go get github.com/gin-gonic/gin@v1.10.1
go get go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin@v0.62.0
go mod tidy
```

- [ ] **Step 4: Verify build**

```bash
cd stage-2-microservices
go build ./services/order/...
```

Expected: clean.

- [ ] **Step 5: Commit**

```bash
git add stage-2-microservices/services/order/internal/handlers/ stage-2-microservices/go.mod stage-2-microservices/go.sum
git commit -m "feat(order): http handlers with baggage propagation"
```

---

### Task 17: Order service — main, config, otelconf YAML

**Files:**

- Create: `stage-2-microservices/services/order/cmd/main.go`
- Create: `stage-2-microservices/services/order/internal/config/config.go`
- Create: `stage-2-microservices/configs/otel-order.yaml`
- Create: `stage-2-microservices/configs/order.yaml`

- [ ] **Step 1: Write order config loader**

Write `stage-2-microservices/services/order/internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the order service configuration.
type Config struct {
	HTTPPort string `yaml:"http_port"`
	Database struct {
		DSN string `yaml:"dsn"`
	} `yaml:"database"`
	Catalog struct {
		Address string `yaml:"address"`
	} `yaml:"catalog"`
}

// Load reads YAML, then overlays env vars.
func Load(file string) (*Config, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.DSN = v
	}
	if v := os.Getenv("HTTP_PORT"); v != "" {
		cfg.HTTPPort = v
	}
	if v := os.Getenv("CATALOG_GRPC_ADDR"); v != "" {
		cfg.Catalog.Address = v
	}
	return &cfg, nil
}
```

- [ ] **Step 2: Write order config YAML**

Write `stage-2-microservices/configs/order.yaml`:

```yaml
http_port: "8080"
database:
  dsn: "host=localhost port=5432 user=ecommerce password=ecommerce dbname=orders sslmode=disable"
catalog:
  address: "localhost:9090"
```

- [ ] **Step 3: Write the otelconf YAML**

Write `stage-2-microservices/configs/otel-order.yaml`:

```yaml
file_format: "1.0"
disabled: false
resource:
  attributes:
    - name: service.name
      value: order-service
    - name: service.namespace
      value: ecommerce
    - name: deployment.environment.name
      value: dev
propagator:
  composite:
    - tracecontext:
    - baggage:
tracer_provider:
  processors:
    - batch:
        exporter:
          otlp_http:
            endpoint: http://localhost:4318/v1/traces
meter_provider:
  readers:
    - periodic:
        interval: 15000
        exporter:
          otlp_http:
            endpoint: http://localhost:4318/v1/metrics
logger_provider:
  processors:
    - batch:
        exporter:
          otlp_http:
            endpoint: http://localhost:4318/v1/logs
```

- [ ] **Step 4: Write order main**

Write `stage-2-microservices/services/order/cmd/main.go`:

```go
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
```

- [ ] **Step 5: Verify build**

```bash
cd stage-2-microservices
go build ./services/order/...
```

Expected: clean.

- [ ] **Step 6: Commit**

```bash
git add stage-2-microservices/services/order/cmd/ stage-2-microservices/services/order/internal/config/ stage-2-microservices/configs/order.yaml stage-2-microservices/configs/otel-order.yaml stage-2-microservices/go.mod stage-2-microservices/go.sum
git commit -m "feat(order): main entrypoint with otelgin and catalog grpc client"
```

---

### Task 18: Dockerfiles for both services

**Files:**

- Create: `stage-2-microservices/Dockerfile.catalog`
- Create: `stage-2-microservices/Dockerfile.order`

- [ ] **Step 1: Write catalog Dockerfile**

Write `stage-2-microservices/Dockerfile.catalog`:

```dockerfile
# syntax=docker/dockerfile:1.7

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/catalog ./services/catalog/cmd

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/catalog /app/catalog
COPY configs/catalog.yaml /app/configs/catalog.yaml
COPY configs/otel-catalog.yaml /app/configs/otel-catalog.yaml
USER nonroot:nonroot
ENTRYPOINT ["/app/catalog"]
```

- [ ] **Step 2: Write order Dockerfile**

Write `stage-2-microservices/Dockerfile.order`:

```dockerfile
# syntax=docker/dockerfile:1.7

FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/order ./services/order/cmd

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/order /app/order
COPY configs/order.yaml /app/configs/order.yaml
COPY configs/otel-order.yaml /app/configs/otel-order.yaml
USER nonroot:nonroot
ENTRYPOINT ["/app/order"]
```

- [ ] **Step 3: Build images locally**

```bash
cd stage-2-microservices
docker build -f Dockerfile.catalog -t otel-in-practice/stage-2-catalog:dev .
docker build -f Dockerfile.order -t otel-in-practice/stage-2-order:dev .
```

Expected: both images build successfully.

- [ ] **Step 4: Commit**

```bash
git add stage-2-microservices/Dockerfile.catalog stage-2-microservices/Dockerfile.order
git commit -m "feat(stage-2): dockerfiles for both services"
```

---

### Task 19: Wire compose, smoke verify, finalize README

**Files:**

- Modify: `stage-2-microservices/docker-compose.yml`
- Modify: `stage-2-microservices/README.md`

- [ ] **Step 1: Extend docker-compose.yml with both services**

Replace `stage-2-microservices/docker-compose.yml` with:

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: ecommerce
      POSTGRES_PASSWORD: ecommerce
      POSTGRES_DB: postgres
    ports:
      - "5432:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./scripts/init-databases.sh:/docker-entrypoint-initdb.d/init-databases.sh:z
    healthcheck:
      test: ["CMD", "pg_isready", "-U", "ecommerce", "-d", "postgres"]
      interval: 2s
      timeout: 5s
      retries: 20

  otel-lgtm:
    image: grafana/otel-lgtm:latest
    ports:
      - "3000:3000"
      - "4318:4318"

  catalog-service:
    build:
      context: .
      dockerfile: Dockerfile.catalog
    environment:
      DATABASE_URL: "host=postgres port=5432 user=ecommerce password=ecommerce dbname=catalog sslmode=disable"
      GRPC_PORT: "9090"
      OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-lgtm:4318"
    ports:
      - "9090:9090"
    depends_on:
      postgres:
        condition: service_healthy
      otel-lgtm:
        condition: service_started

  order-service:
    build:
      context: .
      dockerfile: Dockerfile.order
    environment:
      DATABASE_URL: "host=postgres port=5432 user=ecommerce password=ecommerce dbname=orders sslmode=disable"
      HTTP_PORT: "8080"
      CATALOG_GRPC_ADDR: "catalog-service:9090"
      OTEL_EXPORTER_OTLP_ENDPOINT: "http://otel-lgtm:4318"
    ports:
      - "8080:8080"
    depends_on:
      postgres:
        condition: service_healthy
      catalog-service:
        condition: service_started
      otel-lgtm:
        condition: service_started

volumes:
  postgres-data:
```

Note: the otelconf YAMLs use `localhost:4318` in their endpoints. Inside the container, the DNS name `otel-lgtm` is needed instead. Either:

(a) update the otelconf YAMLs to use `otel-lgtm:4318`, **OR**
(b) wrap the file with `os.ExpandEnv` and use `${OTEL_EXPORTER_OTLP_ENDPOINT}` in the YAML.

For this plan, choose (a). Edit the two YAMLs to swap `http://localhost:4318/...` → `http://otel-lgtm:4318/...`. Local-host `go run` flows still work because they connect to the host-mapped port via... they do not. For `go run` outside docker, also publish 4318 (already done). Document that running services outside compose requires editing the YAMLs back. Pragmatic: the published ports + `otel-lgtm` DNS approach is fine for compose; for local-only dev runs, set `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` to override (otelconf supports env expansion when callers expand the YAML; this is left as a follow-up).

- [ ] **Step 2: Edit otel-catalog.yaml endpoints**

In `stage-2-microservices/configs/otel-catalog.yaml`, replace the three occurrences of `http://localhost:4318` with `http://otel-lgtm:4318`.

- [ ] **Step 3: Edit otel-order.yaml endpoints**

In `stage-2-microservices/configs/otel-order.yaml`, replace the three occurrences of `http://localhost:4318` with `http://otel-lgtm:4318`.

- [ ] **Step 4: Bring stack up**

```bash
cd stage-2-microservices
docker compose up -d --build
sleep 10
docker compose logs catalog-service | tail -5
docker compose logs order-service | tail -5
```

Expected: both services log "starting" and "listening".

- [ ] **Step 5: Smoke-test the happy path**

```bash
# Create a user
curl -sS -X POST http://localhost:8080/api/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","tier":"premium"}'
```

Expected: 201 with the new user JSON. Capture the `id` field; call it `$USER_ID`.

```bash
# Seed a product directly into catalog DB (no admin endpoint exists)
docker compose exec postgres psql -U ecommerce -d catalog -c \
  "INSERT INTO products (id, name, category, price_cents, stock_qty, created_at, updated_at) VALUES ('prod-1','Widget','tools',1500,100,now(),now());"
```

```bash
# Create an order with the customer-tier baggage header
curl -sS -X POST http://localhost:8080/api/v1/orders \
  -H 'Content-Type: application/json' \
  -H 'X-Customer-Tier: premium' \
  -d "{\"user_id\":\"$USER_ID\",\"payment_method\":\"credit_card\",\"items\":[{\"product_id\":\"prod-1\",\"qty\":2}]}"
```

Expected: 201 with the new order, total_cents=3000.

- [ ] **Step 6: Verify trace in Grafana**

Open <http://localhost:3000> → Explore → Tempo. Search for traces from `order-service`. Expected span tree for the order creation:

1. `POST /api/v1/orders` — server, otelgin
2. `process order` — internal, order-service
3. `catalog.v1.CatalogService/GetProduct` — client, otelgrpc (order-service)
4. `catalog.v1.CatalogService/GetProduct` — server, otelgrpc (catalog-service)
5. `lookup product` — internal, catalog-service
6. `SELECT products` — client (catalog DB)
7. `INSERT orders` — client (orders DB)

The baggage `ecommerce.customer.tier=premium` should appear on `lookup product` (carried across services).

- [ ] **Step 7: Tear down**

```bash
cd stage-2-microservices
docker compose down -v
```

- [ ] **Step 8: Replace README with finalized version**

Write `stage-2-microservices/README.md`:

````markdown
# Stage 2: Microservices Split

Two-service reference implementation showing cross-process OpenTelemetry patterns:

- **order-service** — HTTP/Gin front door (port 8080). Owns `users` and `orders`. Calls catalog via gRPC during checkout.
- **catalog-service** — gRPC server (port 9090). Owns `products`. Provides product lookup and inventory check.

Both export OTLP/HTTP to a shared `grafana/otel-lgtm` container, exposed at <http://localhost:3000>.

## Run

```bash
docker compose up -d --build
```

## Smoke test

```bash
# Register a user
USER_ID=$(curl -sS -X POST http://localhost:8080/api/v1/users \
  -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","tier":"premium"}' | jq -r .id)

# Seed a product into the catalog DB
docker compose exec postgres psql -U ecommerce -d catalog -c \
  "INSERT INTO products (id, name, category, price_cents, stock_qty, created_at, updated_at) VALUES ('prod-1','Widget','tools',1500,100,now(),now());"

# Place an order
curl -sS -X POST http://localhost:8080/api/v1/orders \
  -H 'Content-Type: application/json' \
  -H 'X-Customer-Tier: premium' \
  -d "{\"user_id\":\"$USER_ID\",\"payment_method\":\"credit_card\",\"items\":[{\"product_id\":\"prod-1\",\"qty\":2}]}"
```

Open <http://localhost:3000> → Explore → Tempo to inspect the trace.

## Layout

- `services/order/` — HTTP service
- `services/catalog/` — gRPC service
- `shared/telemetry/` — SDK setup, OTEP-4430 event helpers, Weaver-generated constants
- `proto/catalog/v1/` — gRPC contract
- `telemetry/` — Weaver registry and templates

## Spec / plan

- Spec: [`docs/superpowers/specs/2026-05-08-stage-2-microservices-design.md`](../docs/superpowers/specs/2026-05-08-stage-2-microservices-design.md)
- Plan: [`docs/superpowers/plans/2026-05-08-stage-2-microservices.md`](../docs/superpowers/plans/2026-05-08-stage-2-microservices.md)
````

- [ ] **Step 9: Final verification**

```bash
cd stage-2-microservices
go fmt ./...
go vet ./...
go build ./...
go test ./...
weaver registry check -r telemetry/registry/
```

Expected: all clean.

- [ ] **Step 10: Commit**

```bash
git add stage-2-microservices/docker-compose.yml stage-2-microservices/configs/ stage-2-microservices/README.md
git commit -m "feat(stage-2): wire compose stack and finalize README"
```

---

## Self-Review Checklist (engineer should run before declaring done)

- [ ] All 19 tasks committed in order, each with a passing `go build ./...` and `go test ./...` at the time of the commit
- [ ] `weaver registry check` passes against `telemetry/registry/`
- [ ] Generated `shared/telemetry/*_gen.go` is in sync with the registry (`weaver registry generate ...` produces no diff)
- [ ] Smoke test in Task 19 produced a 7-span trace in Grafana spanning both services
- [ ] No span name in the registry contains the dotted form in `name.note` (all use verb-object)
- [ ] No `db.*`, `http.*`, or `rpc.*` entries appear in the local registry
- [ ] `docker compose up -d --build` brings up the full stack from a clean checkout

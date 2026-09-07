# Stage 2: Microservices Split — Design

**Date:** 2026-05-08
**Status:** Approved
**Scope:** Reference implementation only (no exercises / lesson content)
****
## Goal

Extend the otel-in-practice course from a single instrumented monolith (stage-1) to a two-service distributed system that demonstrates cross-process OpenTelemetry patterns: HTTP↔gRPC propagation, baggage flow across services, and span ownership at service boundaries.

The output is a working codebase under `stage-2-microservices/` that mirrors stage-1's structural conventions, reuses its Weaver registry, and aligns with current OTel guidance (otelconf 1.0, OTEP 4430 events-as-logs, slog).

## Non-Goals

- Authentication and authorization
- Async messaging, queues, caching (stage-3 territory)
- React frontend with browser instrumentation
- Kubernetes manifests, service mesh (stage-4 territory)
- Retries, circuit breakers, rate limiting
- Telemetry-assertion tests or integration harnesses
- Exercise scenarios, lesson scripts, validation scripts

## Architecture

Two backend services in a single Go module, communicating over gRPC, both exporting OTLP/HTTP to a shared `grafana/otel-lgtm` container.

```
client ──HTTP──▶ order-service:8080 ──gRPC──▶ catalog-service:9090
                       │                              │
                       └──▶ postgres:5432/orders      └──▶ postgres:5432/catalog
                                          │
                       (both export OTLP/HTTP)
                                          ▼
                                  otel-lgtm:4318
```

- **order-service** — HTTP/Gin front door. Owns `users` and `orders` tables. Calls catalog-service via gRPC during order creation.
- **catalog-service** — gRPC server. Owns `products` table. Provides product lookup and inventory check.
- **One Postgres container** with two databases (`orders`, `catalog`) — each service connects to its own DSN.
- **One otel-lgtm container** receives traces, metrics, and logs from both services on port 4318 (OTLP/HTTP).

## Repository Layout

Single Go module, mirroring the stage-0 / stage-1 pattern:

```
stage-2-microservices/
├── go.mod
├── docker-compose.yml
├── Dockerfile.order
├── Dockerfile.catalog
├── README.md
├── configs/
│   ├── otel-order.yaml          # otelconf for order-service
│   └── otel-catalog.yaml        # otelconf for catalog-service
├── proto/
│   └── catalog/v1/
│       ├── catalog.proto
│       └── catalog.pb.go        # generated
├── shared/
│   └── telemetry/
│       ├── setup.go             # SDK initialization (otelconf-driven)
│       ├── events.go            # EmitEvent / EmitException helpers
│       ├── const.go             # cross-cutting non-generated constants
│       ├── attributes_gen.go    # weaver-generated
│       ├── spans_gen.go         # weaver-generated
│       └── metrics_gen.go       # weaver-generated
├── services/
│   ├── order/
│   │   ├── cmd/main.go
│   │   └── internal/
│   │       ├── config/
│   │       ├── handlers/        # Gin HTTP handlers
│   │       ├── services/        # business logic
│   │       ├── clients/         # typed catalog gRPC client wrapper
│   │       ├── repositories/    # GORM repos (orders, users)
│   │       └── models/
│   └── catalog/
│       ├── cmd/main.go
│       └── internal/
│           ├── config/
│           ├── grpc/            # gRPC server handlers
│           ├── services/        # business logic
│           ├── repositories/    # GORM repos (products)
│           └── models/
└── telemetry/
    ├── registry/                # Weaver registry (shared across both services)
    │   ├── manifest.yaml
    │   ├── attributes.yaml
    │   ├── spans.yaml
    │   └── metrics.yaml
    └── templates/               # Weaver Jinja2 templates → shared/telemetry/*_gen.go
```

**Why one module:** matches the existing stage-0 / stage-1 pattern, keeps build/test invocation uniform, avoids `go.work` complexity. The microservices boundary is shown by the gRPC wire and process separation, not by module separation.

**Why proto/ at the root:** generated proto produces its own Go package; both services import it. Top-level placement matches Buf/protoc convention and keeps `shared/` for hand-written Go.

**Why shared/ for the telemetry package only:** SDK setup, the EmitEvent/EmitException helpers, and the Weaver-generated constants are identical across both services. Service-specific span/metric usage stays inside each service's `internal/` tree.

## Endpoint Surface

**order-service (Gin/HTTP, port 8080)**
- `POST /orders` — create order; calls `catalog.GetProduct` per line item, then writes orders + order_items
- `GET /orders/:id`
- `POST /users`
- `GET /users/:id`

**catalog-service (gRPC, port 9090)** — `catalog.v1.CatalogService`
- `GetProduct(id) → Product`
- `ListProducts(filter) → stream Product`
- `CheckInventory(product_id, qty) → Availability`

## Data Model

**Catalog DB** (`postgres://.../catalog`):
- `products(id PK, name, category, price_cents, stock_qty, created_at, updated_at)`

**Orders DB** (`postgres://.../orders`):
- `users(id PK, email UNIQUE, tier, created_at)`
- `orders(id PK, user_id FK→users, status, total_cents, payment_method, created_at)`
- `order_items(id PK, order_id FK→orders, product_id, qty, unit_price_cents)`
  - `product_id` is a **string reference to catalog** — no FK across services; that is the microservices boundary.

GORM models with `gorm` and `json` tags. Schema bootstrapped via `AutoMigrate` on startup, matching stage-1.

## Telemetry Conventions

### Weaver Registry

The registry is **shared across both services** and lives at `stage-2-microservices/telemetry/registry/`. Stage-1's registry is the starting point; stage-2 reuses all existing entries and adds only what's new.

Reused from stage-1:
- Attributes: `ecommerce.user.id`, `ecommerce.order.id`, `ecommerce.order.total`, `ecommerce.product.id`, `ecommerce.product.category`, `ecommerce.payment.method`, `ecommerce.customer.tier`
- Spans: `ecommerce.user.register`, `ecommerce.user.get`, `ecommerce.order.process`, `ecommerce.order.get`, `ecommerce.order.list`, `ecommerce.product.lookup`, `ecommerce.product.list`, `ecommerce.inventory.check`
- Metrics: `ecommerce.orders.processing.duration`, `ecommerce.users.registrations`, `ecommerce.products.lookups`

The registry contains **only org-local `ecommerce.*` entries**. HTTP, gRPC, and DB attributes / spans live in upstream OTel semconv and are emitted via the language SDK's semconv package and instrumentation libraries. Do not duplicate them in the registry.

### Span Naming

| Span kind | Source | Naming rule | Example |
|---|---|---|---|
| HTTP server | otelgin (semconv) | `{method} {http.route}` | `POST /orders` |
| gRPC client / server | otelgrpc (semconv) | `{package.Service}/{Method}` | `catalog.v1.CatalogService/GetProduct` |
| DB client | manual + semconv | `{db.operation} {db.sql.table}` | `SELECT products`, `INSERT orders` |
| Internal business | our registry | **verb object** | `process order`, `lookup product` |

Internal business spans use the **verb-object** convention at runtime. The Weaver registry keeps a stable dotted id with the runtime name in `annotations.runtime_name`. (Weaver v0.22.1 does not surface `name.note` in the resolved schema, so the template reads from annotations instead.)

```yaml
spans:
  - type: ecommerce.order.process
    kind: internal
    stability: stable
    name:
      note: "ecommerce.order.process"   # decorative; matches the type
    brief: "Process a single customer order end-to-end."
    annotations:
      runtime_name: "process order"     # read by the Go template
    attributes: [...]
```

Generated constants:
```go
SpanEcommerceOrderProcessName = "process order"
SpanEcommerceProductLookupName = "lookup product"
SpanEcommerceUserRegisterName  = "register user"
SpanEcommerceInventoryCheckName = "check inventory"
// ... etc
```

This diverges from stage-1's runtime names (which used the dotted form). Stage-1 stays as-is; alignment is a separate task if desired.

### Tracer / Meter / Logger Scopes

Service-distinct, mirroring stage-1:
- order: `telemetrydrops.com/order-service`
- catalog: `telemetrydrops.com/catalog-service`

### Resource Attributes (per service, in otelconf YAML)
- `service.name` — `order-service` or `catalog-service`
- `service.version` — injected at build time
- `service.namespace` — `ecommerce`
- `deployment.environment.name` — `development`
- `service.instance.id` — UUID injected at runtime (matches stage-1's setup)

### Propagation
- W3C TraceContext + Baggage composite propagator, set globally in both services
- HTTP: otelgin middleware extracts inbound context (server side)
- gRPC: otelgrpc unary client interceptor injects, server interceptor extracts
- No manual header manipulation in business code

### Span Kinds
- HTTP server spans → `SpanKindServer` (otelgin)
- gRPC server spans → `SpanKindServer` (otelgrpc server interceptor)
- gRPC client spans → `SpanKindClient` (otelgrpc client interceptor)
- DB client spans → `SpanKindClient` (manually created in repos)
- Business logic spans → `SpanKindInternal` (default, not specified)

### DB Instrumentation Pattern

Repositories manually create client spans with semconv attributes — same pattern as stage-1, no GORM auto-instrumentation plugin:

```go
ctx, span := r.tracer.Start(ctx, "SELECT products",
    trace.WithSpanKind(trace.SpanKindClient),
    trace.WithAttributes(
        semconv.DBSystemPostgreSQL,
        semconv.DBOperation("SELECT"),
        semconv.DBSQLTable("products"),
    ))
defer span.End()
```

### Events and Errors (OTEP 4430)

`shared/telemetry` exposes the same helpers stage-1 uses:
- `telemetry.EmitEvent(ctx, name, attrs...)` — log-record-as-span-event
- `telemetry.EmitException(ctx, err)` — log-record-as-exception

Error pattern on every error path:
```go
telemetry.EmitException(ctx, err)
span.SetStatus(codes.Error, "<short reason>")
```

`EmitException` alone does **not** mark the span failed — `SetStatus` is still required. For expected business outcomes (e.g., `NotFound`), use `SetStatus` only when the service genuinely failed; do not mark handled outcomes as errors.

### Logging

`*slog.Logger` bridged via `otelslog`, identical to stage-1. On request paths, always use the `*Context` variants (`InfoContext`, `ErrorContext`, …) so the bridge attaches `trace_id` / `span_id` to each record.

### Metrics

Service-owned, registered in constructors. Reuse stage-1's where applicable:
- order: `ecommerce.orders.processing.duration` (histogram, `s`), `ecommerce.users.registrations` (counter)
- catalog: `ecommerce.products.lookups` (counter)
- HTTP request duration / RPC duration histograms come from otelhttp / otelgrpc — do not hand-roll

Cardinality discipline: metric attributes are limited to bounded enums (`payment.method`, `customer.tier`). No user IDs, order IDs, free-form messages, or full URLs in metric dimensions.

### Baggage Demonstration

The order-service handler reads `X-Customer-Tier` from the request header (default `standard`), validates against the bounded enum, and sets `ecommerce.customer.tier` in baggage. The baggage rides through gRPC metadata to catalog-service, which reads it and tags `lookup product` / `check inventory` spans with the value. This demonstrates real cross-process baggage flow rather than a contrived example. The constant for the baggage key lives in `shared/telemetry/const.go` (carried over from stage-1's pattern).

## Cross-Service Flow: `POST /orders`

Span tree per request (5 spans across 2 services):

1. `POST /orders` — kind=server, otelgin middleware (order-service)
2. `process order` — kind=internal (order-service)
3. `catalog.v1.CatalogService/GetProduct` — kind=client + kind=server pair (otelgrpc, both sides) — repeats per line item
4. `lookup product` — kind=internal (catalog-service) — repeats per line item
5. DB spans on each side — kind=client, semconv `db.system=postgresql`, manually created in repositories

The two `internal` spans are created in business code. All others come from middleware / interceptors.

**What is deliberately NOT instrumented:**
- Request body unmarshaling, response marshaling
- Handler-level input validation
- Pure computation (price totals, etc.)
- Repository constructors and dependency wiring

These are not meaningful runtime boundaries.

## Service Internals

Both services share the same internal layering, matching stage-1: `handlers / grpc → services → repositories`.

**`services/order/internal/`**
- `handlers/` — Gin handlers (`orders.go`, `users.go`)
- `services/` — `order_service.go`, `user_service.go`
- `clients/catalog.go` — typed wrapper around the generated gRPC client; otelgrpc client interceptor wired here
- `repositories/` — GORM repos for orders, users
- `models/`, `config/`

**`services/catalog/internal/`**
- `grpc/` — gRPC handlers (`server.go`)
- `services/` — `product_service.go`, `inventory_service.go`
- `repositories/` — GORM repo for products
- `models/`, `config/`

## Configuration

Each service is independent:
- Reads its own otelconf YAML (`configs/otel-order.yaml`, `configs/otel-catalog.yaml`) at startup
- Reads service config (HTTP/gRPC port, DB DSN, peer service address) from environment variables with sensible defaults — no shared service config file
- `service.name` set in the otelconf resource block, not at runtime

## Docker Compose

```yaml
services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_USER: ecommerce
      POSTGRES_PASSWORD: ecommerce
      POSTGRES_DB: postgres
    volumes:
      - postgres-data:/var/lib/postgresql/data
      - ./scripts/init-databases.sh:/docker-entrypoint-initdb.d/init-databases.sh
    # init script creates the `orders` and `catalog` databases on first boot

  otel-lgtm:
    image: grafana/otel-lgtm:latest
    ports: ["3000:3000", "4318:4318"]

  catalog-service:
    build: { context: ., dockerfile: Dockerfile.catalog }
    environment:
      - DATABASE_URL=postgres://.../catalog
      - GRPC_PORT=9090
    depends_on: [postgres, otel-lgtm]

  order-service:
    build: { context: ., dockerfile: Dockerfile.order }
    environment:
      - DATABASE_URL=postgres://.../orders
      - HTTP_PORT=8080
      - CATALOG_GRPC_ADDR=catalog-service:9090
    depends_on: [postgres, catalog-service, otel-lgtm]

volumes:
  postgres-data:
```

## Testing

**Unit tests only.** Services with mocked repositories and a mocked catalog client. Assert business logic (e.g., creating an order with an out-of-stock product returns the expected error). No telemetry-assertion tests, no integration harness, no end-to-end smoke script.

Verification of actual telemetry output happens by running the compose stack and inspecting traces in Grafana — not codified in CI.

## Verification Gates (CI)

- `go build ./...`
- `go test ./...`
- `go vet ./...`
- `gofmt -l .` (no diffs)
- `weaver registry check -r ./telemetry/registry/`
- `weaver registry generate ...` followed by `git diff --exit-code` on `shared/telemetry/*_gen.go`

## Decisions and Risks

- **Proto codegen toolchain:** Buf, for lockfile and reproducibility. `buf.yaml` and `buf.gen.yaml` at the repo root; generated `*.pb.go` checked in.
- **Postgres database bootstrap:** an init script (`scripts/init-databases.sh`) mounted into `docker-entrypoint-initdb.d/` creates the `orders` and `catalog` databases on first boot. Each service then runs `AutoMigrate` against its own DSN at startup, matching stage-1.
- **Order item price snapshotting:** `order_items.unit_price_cents` is captured at order creation from the catalog response. If a product's catalog price changes later, the order line item retains the original. This is intentional — orders are immutable snapshots.
- **Stage-1 runtime span name divergence:** stage-2 introduces verb-object runtime names while stage-1 keeps dotted runtime names. Acceptable; realigning stage-1 is a separate task.
- **Risk — gRPC name format from otelgrpc:** the exact runtime form (`catalog.v1.CatalogService/GetProduct` vs. with leading slash) is set by the otelgrpc version in use. Verify by inspecting traces during implementation; the design does not depend on the exact form.

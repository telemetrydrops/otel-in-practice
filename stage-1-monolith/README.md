# E-commerce Monolith (Stage 1 — Instrumented)

A sample e-commerce monolith built with Go, fully instrumented with OpenTelemetry. This is the reference implementation for the **Traces API** lesson at TelemetryDrops.

## Features

- REST API for users, products, and orders
- PostgreSQL database integration
- Configuration-driven OpenTelemetry setup using `otelconf`
- Docker containerization
- Multi-platform builds with GoReleaser

## Quick Start

### Start Infrastructure

```bash
docker-compose up -d
```

This starts the required infrastructure:
- PostgreSQL database on port 5432
- Grafana LGTM stack on port 3000

Once the infrastructure is running, you can run the application locally (see below).

### Local Development

1. Install dependencies:
```bash
go mod download
```

2. Start PostgreSQL and LGTM stack:
```bash
docker-compose up postgres otel-lgtm -d
```

3. Run the application:
```bash
go run main.go
```

## Configuration

Configuration is loaded from `configs/config.yaml` with environment variable expansion support.

### OpenTelemetry Configuration

OpenTelemetry is configured via `configs/otel.yaml` using the `otelconf` package for declarative setup.

## API Endpoints

- `POST /api/v1/users` - Create user
- `GET /api/v1/users` - List users
- `GET /api/v1/users/:user_id` - Get user
- `GET /api/v1/users/:user_id/orders` - Get user orders

- `POST /api/v1/products` - Create product
- `GET /api/v1/products` - List products
- `GET /api/v1/products/:id` - Get product
- `POST /api/v1/products/:id/stock` - Update stock

- `POST /api/v1/orders` - Create order
- `GET /api/v1/orders/:id` - Get order
- `PUT /api/v1/orders/:id/status` - Update order status

## Usage Examples

### 1. Create a User

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Jane Doe",
    "email": "jane@example.com",
    "tier": "premium"
  }'
```

### 2. List Products

```bash
curl http://localhost:8080/api/v1/products
```

### 3. Create an Order

Replace `<USER_ID>` and `<PRODUCT_ID>` with actual IDs from the previous steps.

```bash
curl -X POST http://localhost:8080/api/v1/orders \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "<USER_ID>",
    "payment_method": "credit_card",
    "items": [
      {
        "product_id": "<PRODUCT_ID>",
        "quantity": 1
      }
    ]
  }'
```

### 4. Get User Orders

```bash
curl http://localhost:8080/api/v1/users/<USER_ID>/orders
```

## Building

```bash
go build -o bin/ecommerce-monolith .
```

## Architecture

The application follows a layered architecture:

- **Handlers** (`internal/handlers/`) — HTTP request handling with Gin
- **Services** (`internal/services/`) — Business logic
- **Repositories** (`internal/repositories/`) — Data access layer with GORM
- **Models** (`internal/models/`) — Domain entities
- **Telemetry** (`internal/telemetry/`) — OpenTelemetry setup and constants

## OpenTelemetry Traces API Coverage

This codebase demonstrates every major feature of the OpenTelemetry Traces API. Each concept is shown in context across the three application layers.

### Tracer Initialization
Obtain a tracer from the global provider with an instrumentation scope.
- `services/order_service.go` — `otel.Tracer(telemetry.Scope)`

### Starting Spans
Create child spans with `tracer.Start(ctx, name)` and always `defer span.End()`.
- `services/order_service.go` — service-level span creation
- `repositories/order_repo.go` — repository-level span creation

### Recovering Spans from Context
Extract the current span (e.g., from middleware) to enrich it.
- `handlers/orders.go` — `trace.SpanFromContext(ctx)`

### Adding Attributes
Attach key-value metadata at creation time or after.
- `repositories/order_repo.go` — `trace.WithAttributes()` at span creation
- `handlers/orders.go` — `span.SetAttributes()` after request binding

### Span Events
Mark timestamped milestones within a span.
- `repositories/order_repo.go` — `span.AddEvent("order created successfully", ...)`

### Recording Errors and Span Status
Record error details and mark spans as failed.
- `handlers/orders.go` — `span.RecordError(err)` + `span.SetStatus(codes.Error, "...")`

### Span Kind
Specify the role of a span in the request flow.
- `repositories/order_repo.go` — `trace.WithSpanKind(trace.SpanKindClient)` for DB calls
- HTTP handler spans are `SpanKindServer` via `otelgin` middleware

### Context Propagation (In-Process)
Pass `ctx` through the call chain to build the trace tree automatically.
- handler → service → repository — context flows through all layers

### Cross-Process Propagation
W3C TraceContext and Baggage propagators for distributed traces.
- `telemetry/setup.go` — propagator configuration

### Baggage
Set and read cross-cutting context that travels with the request.
- `handlers/orders.go` — `baggage.ContextWithBaggage()` to set payment method
- `services/order_service.go` — `baggage.FromContext()` to read it downstream

### Span Links
Correlate spans across different traces (async processing pattern).
- `services/order_service.go` — `trace.WithLinks()` + `trace.WithNewRoot()` in background processing

### Trace ID Extraction
Extract the trace ID for response headers and log correlation.
- `handlers/orders.go` — `span.SpanContext().TraceID().String()` → `X-Trace-ID` header

### IsRecording Guard
Skip expensive attribute computation when the span is not being recorded.
- `services/product_service.go` — `span.IsRecording()` check

### Dynamic Span Names
Refine the span name after creation based on runtime information.
- `services/product_service.go` — `span.SetName("list products by category")`

## Observability Stack

Access Grafana at http://localhost:3000 to view metrics, logs, and traces.

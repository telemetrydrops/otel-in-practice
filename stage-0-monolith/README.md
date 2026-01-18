# E-commerce Monolith

A sample e-commerce monolith application built with Go, demonstrating OpenTelemetry observability patterns.

## Features

- REST API for users, products, and orders
- PostgreSQL database integration
- OpenTelemetry instrumentation with configuration-driven setup
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

- `GET /api/v1/users` - List users
- `POST /api/v1/users` - Create user
- `GET /api/v1/users/{user_id}` - Get user
- `GET /api/v1/users/{user_id}/orders` - Get user orders

- `GET /api/v1/products` - List products
- `POST /api/v1/products` - Create product
- `GET /api/v1/products/{id}` - Get product

- `GET /api/v1/orders` - List orders
- `POST /api/v1/orders` - Create order
- `GET /api/v1/orders/{id}` - Get order

## Usage Examples

Here are some `curl` commands to interact with the API:

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

### Local Build

```bash
go build -o bin/ecommerce-monolith .
```

## Architecture

The application follows a layered architecture:

- **Handlers** - HTTP request handling
- **Services** - Business logic
- **Repositories** - Data access layer
- **Models** - Domain entities

## Observability

The application includes comprehensive OpenTelemetry instrumentation:

- HTTP request tracing
- Database query tracing
- Custom business metrics
- Structured logging

Access Grafana at http://localhost:3000 to view metrics, logs, and traces.
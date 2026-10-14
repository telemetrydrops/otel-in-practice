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

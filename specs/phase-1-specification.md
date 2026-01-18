# Phase 1 Technical Specification: otel-in-practice
## Monolith Foundation & Microservices Split

**Version**: 1.0  
**Timeline**: Month 1 (4 weeks)  
**Target Audience**: Go developers learning OpenTelemetry

---

## Executive Summary

Phase 1 establishes the foundational educational content for the otel-in-practice course, covering the journey from instrumented monolith (Stage 1) to basic microservices architecture (Stage 2). This phase implements production-ready OpenTelemetry patterns using Go and React, with the grafana/otel-lgtm observability stack as the learning backend.

### Key Objectives
- **Progressive Learning**: Seamless transition from single service to distributed systems
- **Best Practices**: Implement boundary-based instrumentation following ai-context guidelines  
- **Configuration-Driven**: Use otelconf for flexible telemetry setup
- **Real-World Scenarios**: Hands-on debugging exercises with practical problems

---

## Stage 1: Monolith Foundation (Weeks 1-2)

### Architecture Overview

Single Go service implementing an e-commerce platform with complete observability:

```
stage-1-monolith/
├── cmd/
│   └── main.go                    # Application entry point
├── internal/
│   ├── telemetry/                 # OpenTelemetry setup
│   │   ├── const.go               # Span names, metrics, attributes
│   │   ├── setup.go               # SDK initialization with otelconf
│   │   └── providers.go           # Provider utilities
│   ├── handlers/                  # HTTP handlers (Gin-based)
│   │   ├── products.go            # Product catalog endpoints
│   │   ├── orders.go              # Order management endpoints
│   │   └── users.go               # User management endpoints
│   ├── services/                  # Business logic layer
│   │   ├── product_service.go     # Product domain logic
│   │   ├── order_service.go       # Order processing logic
│   │   └── user_service.go        # User management logic
│   ├── repositories/              # Data access layer
│   │   ├── product_repo.go        # Product data operations
│   │   ├── order_repo.go          # Order data operations
│   │   └── user_repo.go           # User data operations
│   └── models/                    # Domain models
│       ├── product.go             # Product entity
│       ├── order.go               # Order entity
│       └── user.go                # User entity
├── frontend/                      # React SPA
│   └── src/                       # React components with browser instrumentation
├── configs/
│   └── otel.yaml                  # OpenTelemetry configuration
├── docker-compose.yml             # Development environment
└── exercises/                     # Hands-on learning materials
    ├── 01-add-first-trace.md      # Basic tracing setup
    ├── 02-add-metrics.md          # Business metrics implementation
    ├── 03-structured-logging.md   # Correlated logging
    └── 04-connect-signals.md      # Trace-metrics-logs correlation
```

### Technical Implementation

#### 1. Service Constants (`internal/telemetry/const.go`)
Following ai-context naming conventions:

```go
package telemetry

import semconv "go.opentelemetry.io/otel/semconv/v1.36.0"

const (
    // Service scope for instrumentation
    Scope = "telemetrydrops.com/ecommerce-monolith"
    ServiceName = "ecommerce-monolith"
    ServiceVersion = "1.0.0"
)

// Business operation spans (hierarchical naming)
const (
    SPAN_USER_REGISTRATION   = "register user"
    SPAN_ORDER_PROCESSING    = "process order"  
    SPAN_PRODUCT_LOOKUP      = "lookup product"
    SPAN_INVENTORY_CHECK     = "check inventory"
    
    // Database operations
    SPAN_USER_SELECT    = "SELECT users"
    SPAN_ORDER_INSERT   = "INSERT orders"
    SPAN_PRODUCT_UPDATE = "UPDATE products"
)

// UCUM-compliant metrics
const (
    USER_REGISTRATIONS_TOTAL    = "users.registrations.total"
    ORDER_PROCESSING_DURATION   = "orders.processing.duration"
    PRODUCT_LOOKUPS_TOTAL       = "products.lookups.total"
    HTTP_REQUEST_DURATION       = "http.server.request.duration"
)

// Business attribute constants
const (
    ATTR_USER_ID          = "user.id"
    ATTR_ORDER_ID         = "order.id" 
    ATTR_PRODUCT_CATEGORY = "product.category"
    ATTR_CUSTOMER_TIER    = "customer.tier"
    ATTR_PAYMENT_METHOD   = "payment.method"
)
```

#### 2. Configuration-Driven Setup (`internal/telemetry/setup.go`)
Using otelconf for flexible configuration:

```go
package telemetry

import (
    "context"
    "errors"
    "fmt"
    "os"
    "time"

    "github.com/google/uuid"
    "go.opentelemetry.io/contrib/bridges/otelzap"
    otelconf "go.opentelemetry.io/contrib/otelconf/v0.3.0"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/log"
    "go.opentelemetry.io/otel/log/global"
    "go.opentelemetry.io/otel/metric"
    "go.opentelemetry.io/otel/propagation"
    semconv "go.opentelemetry.io/otel/semconv/v1.36.0"
    "go.opentelemetry.io/otel/trace"
    "go.uber.org/zap"
    "go.uber.org/zap/zapcore"
)

type Providers struct {
    TracerProvider trace.TracerProvider
    MeterProvider  metric.MeterProvider  
    LoggerProvider log.LoggerProvider
    Logger         *zap.Logger
    Closer         func(ctx context.Context) error
}

func SetupTelemetry(ctx context.Context, configFile string) (*Providers, error) {
    providers, err := providersFromConfig(ctx, configFile)
    if err != nil {
        return nil, err
    }

    // Set global providers
    otel.SetTracerProvider(providers.TracerProvider)
    otel.SetMeterProvider(providers.MeterProvider)
    global.SetLoggerProvider(providers.LoggerProvider)
    
    // Configure W3C TraceContext + Baggage propagation
    otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
        propagation.TraceContext{},
        propagation.Baggage{},
    ))

    return providers, nil
}

func providersFromConfig(ctx context.Context, cfgFile string) (*Providers, error) {
    b, err := os.ReadFile(cfgFile)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            // Fallback to no-op providers for development
            logger := zap.Must(zap.NewProduction())
            logger.Warn("OpenTelemetry config not found, using no-op providers", 
                zap.String("config_file", cfgFile))
            return &Providers{
                TracerProvider: trace.NewNoOpTracerProvider(),
                MeterProvider:  metric.NewNoOpMeterProvider(),
                LoggerProvider: log.NewNoOpLoggerProvider(),
                Logger:         logger,
                Closer:         func(ctx context.Context) error { return nil },
            }, nil
        }
        return nil, fmt.Errorf("reading config file %s: %w", cfgFile, err)
    }

    b = []byte(os.ExpandEnv(string(b)))

    conf, err := otelconf.ParseYAML(b)
    if err != nil {
        return nil, err
    }

    // Add service resource attributes
    if conf.Resource == nil {
        conf.Resource = &otelconf.Resource{}
    }
    if conf.Resource.Attributes == nil {
        conf.Resource.Attributes = []otelconf.AttributeNameValue{}
    }

    // Inject service metadata
    conf.Resource.Attributes = insertAttribute(conf.Resource.Attributes, 
        string(semconv.ServiceVersionKey), ServiceVersion)
    conf.Resource.Attributes = insertAttribute(conf.Resource.Attributes, 
        string(semconv.ServiceInstanceIDKey), uuid.New().String())

    sdk, err := otelconf.NewSDK(
        otelconf.WithContext(ctx), 
        otelconf.WithOpenTelemetryConfiguration(*conf),
    )
    if err != nil {
        return nil, err
    }

    // Structured logging with OpenTelemetry correlation
    core := zapcore.NewTee(
        zapcore.NewCore(
            zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()), 
            zapcore.AddSync(os.Stdout), 
            zapcore.InfoLevel,
        ),
        otelzap.NewCore(Scope, otelzap.WithLoggerProvider(global.GetLoggerProvider())),
    )

    return &Providers{
        TracerProvider: sdk.TracerProvider(),
        MeterProvider:  sdk.MeterProvider(),
        LoggerProvider: sdk.LoggerProvider(),
        Logger:         zap.New(core),
        Closer:         sdk.Shutdown,
    }, nil
}

func insertAttribute(attrs []otelconf.AttributeNameValue, name, value string) []otelconf.AttributeNameValue {
    for _, attr := range attrs {
        if attr.Name == name {
            return attrs
        }
    }
    return append(attrs, otelconf.AttributeNameValue{Name: name, Value: value})
}
```

#### 3. OpenTelemetry Configuration (`configs/otel.yaml`)

```yaml
file_format: "0.3"
resource:
  schema_url: https://opentelemetry.io/schemas/1.36.0
  attributes:
    - name: service.name
      value: "ecommerce-monolith"
    - name: deployment.environment.name  
      value: "development"

propagator:
  composite: [ tracecontext, baggage ]

tracer_provider:
  processors:
    - batch:
        timeout: 1s
        send_batch_size: 1024
        exporter:
          otlp:
            protocol: http/protobuf
            endpoint: "http://localhost:4318"

meter_provider:
  readers:
    - periodic:
        interval: 30s
        exporter:
          otlp:
            protocol: http/protobuf
            endpoint: "http://localhost:4318"

logger_provider:
  processors:
    - batch:
        exporter:
          otlp:
            protocol: http/protobuf
            endpoint: "http://localhost:4318"
```

#### 4. Docker Compose Environment

```yaml
version: '3.8'
services:
  app:
    build: .
    ports:
      - "8080:8080"
    environment:
      - OTEL_EXPORTER_OTLP_ENDPOINT=http://otel-lgtm:4318
      - ENVIRONMENT=development
    depends_on:
      - postgres
      - otel-lgtm

  otel-lgtm:
    image: grafana/otel-lgtm:latest
    ports:
      - "3000:3000"   # Grafana UI
      - "4318:4318"   # OTLP HTTP receiver
    environment:
      - OTEL_RESOURCE_ATTRIBUTES=service.name=otel-lgtm

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_DB=ecommerce
      - POSTGRES_USER=user
      - POSTGRES_PASSWORD=password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
```

### Exercise Scenarios (Stage 1)

#### Exercise 1: N+1 Query Problem
**Scenario**: Product list endpoint causing database performance issues
**Learning**: Using distributed tracing to identify inefficient database patterns
**Implementation**: Deliberate N+1 query in product repository, solve with joins

#### Exercise 2: Goroutine Leak Detection  
**Scenario**: Background processing goroutines not being cleaned up
**Learning**: Runtime metrics and custom spans for goroutine lifecycle tracking
**Implementation**: Leaking goroutines in order processing, fix with proper context cancellation

#### Exercise 3: Memory Growth Analysis
**Scenario**: Memory usage increasing over time without obvious cause
**Learning**: Go runtime metrics integration and memory profiling with telemetry
**Implementation**: Memory leak in cache implementation, identify and resolve

---

## Stage 2: Microservices Split (Weeks 3-4)

### Architecture Overview

Split monolith into distributed services with maintained observability:

```
stage-2-microservices-basic/
├── services/
│   ├── frontend/                  # React SPA
│   │   └── src/                   # Browser instrumentation
│   ├── api-gateway/               # Go reverse proxy
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   │   ├── handlers/          # Proxy handlers  
│   │   │   └── middleware/        # Auth, logging, tracing
│   │   └── configs/otel.yaml
│   ├── catalog-service/           # Product catalog (Gin)
│   │   ├── cmd/main.go
│   │   ├── internal/
│   │   │   ├── handlers/          # HTTP handlers
│   │   │   ├── services/          # Business logic
│   │   │   └── repositories/      # Data access
│   │   └── configs/otel.yaml
│   └── order-service/             # Order processing (gRPC)
│       ├── cmd/main.go
│       ├── internal/
│       │   ├── grpc/              # gRPC handlers
│       │   ├── services/          # Business logic  
│       │   └── repositories/      # Data access
│       └── configs/otel.yaml
├── shared/
│   ├── telemetry/                 # Common telemetry package
│   │   ├── const.go               # Service-agnostic constants
│   │   ├── setup.go               # Shared SDK setup
│   │   └── middleware.go          # HTTP/gRPC middleware
│   ├── proto/                     # gRPC definitions
│   │   └── orders/
│   │       ├── orders.proto
│   │       └── orders.pb.go
│   └── models/                    # Shared domain models
└── docker-compose.yml            # Multi-service environment
```

### Service Communication Patterns

#### HTTP to gRPC with Context Propagation
```go
// API Gateway to Order Service
func (h *Handler) CreateOrder(c *gin.Context) {
    ctx := c.Request.Context()
    
    // Extract existing span created by otelgin middleware
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(
        attribute.String("business.operation", "order_creation"),
        attribute.String(shared.ATTR_CUSTOMER_TIER, extractTier(c)))

    // Call order service with context propagation
    order, err := h.orderClient.CreateOrder(ctx, &orderRequest)
    if err != nil {
        span.SetStatus(codes.Error, "order creation failed")
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
        return
    }

    span.AddEvent("order created successfully", trace.WithAttributes(
        attribute.String(shared.ATTR_ORDER_ID, order.ID)))
    
    c.JSON(http.StatusCreated, order)
}
```

#### gRPC Server with Automatic Instrumentation
```go
// Order Service gRPC handler
func (s *OrderServer) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.Order, error) {
    // Context and span automatically provided by otelgrpc interceptor
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(
        attribute.String(shared.ATTR_USER_ID, req.UserId),
        attribute.String("business.operation", "order_processing"))

    order, err := s.service.ProcessOrder(ctx, req)
    if err != nil {
        // Return error - span will be marked as error by interceptor
        return nil, fmt.Errorf("processing order: %w", err)
    }

    span.AddEvent("order processed successfully")
    return order, nil
}
```

### Shared Telemetry Package

#### Service-Agnostic Constants (`shared/telemetry/const.go`)
```go
package telemetry

// Cross-service constants
const (
    // Business attributes used across services
    ATTR_USER_ID         = "user.id"
    ATTR_ORDER_ID        = "order.id"  
    ATTR_PRODUCT_ID      = "product.id"
    ATTR_CUSTOMER_TIER   = "customer.tier"
    ATTR_PAYMENT_METHOD  = "payment.method"
    ATTR_REQUEST_SOURCE  = "request.source"
    
    // Business operations (service-agnostic)
    SPAN_ORDER_VALIDATION = "validate order"
    SPAN_USER_LOOKUP     = "lookup user"
    SPAN_PRODUCT_LOOKUP  = "lookup product"
    SPAN_PAYMENT_PROCESS = "process payment"
)

// Service identification
const (
    SERVICE_API_GATEWAY   = "api-gateway"
    SERVICE_CATALOG       = "catalog-service"  
    SERVICE_ORDER         = "order-service"
    SERVICE_FRONTEND      = "frontend"
)

// Common metric names
const (
    HTTP_REQUEST_DURATION    = "http.server.request.duration"
    GRPC_REQUEST_DURATION    = "rpc.server.duration"
    DATABASE_QUERY_DURATION  = "db.client.operation.duration"
)
```

### Exercise Scenarios (Stage 2)

#### Exercise 1: Lost Context Debugging
**Scenario**: Trace context not propagating between services
**Learning**: Understanding W3C TraceContext headers and context propagation
**Implementation**: Missing context propagation in HTTP client, fix with otelhttp transport

#### Exercise 2: gRPC Timeout Chain Analysis  
**Scenario**: Cascading timeouts causing service failures
**Learning**: Distributed tracing for timeout analysis across service boundaries
**Implementation**: Chain of gRPC calls with different timeouts, visualize in Grafana

#### Exercise 3: Service Discovery Monitoring
**Scenario**: Services failing to discover each other properly  
**Learning**: Monitoring service registration/deregistration events
**Implementation**: Service registry with telemetry, track availability changes

#### Exercise 4: Concurrent Request Handling
**Scenario**: Goroutine pool exhaustion under load
**Learning**: Go runtime metrics and concurrency patterns with telemetry
**Implementation**: Limited goroutine pools, monitor queue depth and processing time

---

## Testing Strategy

### Performance Validation
- **Startup Time**: Services start with telemetry in <2 seconds
- **Overhead**: Telemetry adds <5% performance impact
- **Trace Completion**: >95% trace completion rate
- **Resource Usage**: Memory and CPU usage within acceptable bounds

### Exercise Validation Scripts
Each exercise includes validation scripts:

```bash
#!/bin/bash
# Exercise validation for N+1 queries

echo "🔍 Running N+1 query detection test..."

# Generate load
curl -s http://localhost:8080/products?limit=20

# Query Grafana for database spans
QUERY='rate(traces{span.name="SELECT products"}[1m])'
RESULT=$(curl -s "http://localhost:3000/api/datasources/proxy/1/api/v1/query?query=${QUERY}")

# Validate query count
QUERY_COUNT=$(echo $RESULT | jq '.data.result[0].value[1]' | sed 's/"//g')

if (( $(echo "$QUERY_COUNT > 1" | bc -l) )); then
    echo "❌ N+1 query detected: $QUERY_COUNT queries per request"
    exit 1
else
    echo "✅ Efficient querying: $QUERY_COUNT queries per request"
fi
```

### Integration Testing
- **Cross-Service Traces**: Validate end-to-end trace propagation
- **Metrics Collection**: Verify business metrics accuracy
- **Error Handling**: Confirm proper error span recording
- **Resource Correlation**: Check service.name and resource attributes

---

## Success Criteria

### Technical Metrics
- [ ] Go service startup time with telemetry: <2 seconds
- [ ] Telemetry overhead: <5% performance impact  
- [ ] Trace completion rate: >95%
- [ ] All exercise scenarios working correctly
- [ ] Configuration-driven telemetry setup functional
- [ ] Grafana dashboards showing expected data

### Learning Outcomes
- [ ] Students can instrument Go HTTP services  
- [ ] Students can configure OpenTelemetry with YAML
- [ ] Students can debug N+1 queries using traces
- [ ] Students can track goroutine leaks with metrics
- [ ] Students can propagate context between services
- [ ] Students can instrument gRPC services
- [ ] Students can create custom business spans and metrics

### Exercise Completion
- [ ] Stage 1: All 4 exercises completed and validated
- [ ] Stage 2: All 4 exercises completed and validated
- [ ] Real problems identified and solved using observability data
- [ ] Performance improvements measured and documented

---

## Deliverables

### Code Artifacts
1. **Complete Stage 1 Implementation**
   - Instrumented Go monolith with Gin, GORM, PostgreSQL
   - React frontend with browser instrumentation  
   - Docker Compose with grafana/otel-lgtm
   - OpenTelemetry configuration files

2. **Complete Stage 2 Implementation**
   - 3 Go microservices (api-gateway, catalog, order)
   - Shared telemetry package
   - gRPC service definitions and implementations
   - Service communication with context propagation

### Educational Materials  
1. **Exercise Guides**
   - Step-by-step problem setup instructions
   - Expected behavior and failure modes
   - Solution approaches and code samples
   - Validation scripts and success criteria

2. **Configuration Templates**
   - OpenTelemetry YAML configurations
   - Docker Compose environments
   - Grafana dashboard definitions
   - Alert rule templates

### Documentation
1. **Architecture Diagrams**
   - Service topology and communication patterns
   - Data flow through observability pipeline
   - Instrumentation boundary identification

2. **Troubleshooting Guides**
   - Common setup issues and resolutions
   - Performance tuning recommendations
   - Debugging context propagation problems

---

**Next Phase**: Phase 2 will expand to 8+ services with advanced patterns including messaging, caching, external APIs, and Kubernetes deployment.
# Agentic Coding Guidelines

This document provides essential information for AI agents operating within this repository.
It covers build instructions, testing procedures, and code style guidelines to ensure consistency and quality.

## Project Structure

The project is currently structured as a monorepo containing:
- `stage-0-monolith/`: The uninstrumented baseline Go e-commerce monolith.
- `stage-1-monolith/`: The fully instrumented version with comprehensive OpenTelemetry.
- `local/`: Local development artifacts (git-ignored).
- `specs/`: Documentation and specifications.

## 1. Build, Lint, and Test Commands

All commands should be executed from the `stage-1-monolith/` directory unless otherwise specified.

### Build
- **Build the application:**
  ```bash
  go build -o bin/ecommerce-monolith .
  ```
- **Run locally:**
  ```bash
  go run main.go
  ```
- **Multi-platform release (snapshot):**
  ```bash
  goreleaser release --snapshot --clean
  ```

### Test
- **Run all tests:**
  ```bash
  go test ./...
  ```
- **Run a single test:**
  To run a specific test function, use the `-run` flag with a regex matching the test name.
  ```bash
  # Syntax: go test -v -run <TestNameRegex> <PackagePath>
  # Example:
  go test -v -run TestUserCreation ./internal/services
  ```
- **Run tests with coverage:**
  ```bash
  go test -coverprofile=coverage.out ./...
  go tool cover -html=coverage.out
  ```

### Lint
- **Run linter:**
  Use `golangci-lint` if available.
  ```bash
  golangci-lint run
  ```
- **Verify formatting:**
  Ensure code is formatted with `gofmt` or `goimports`.
  ```bash
  gofmt -l -w .
  ```

## 2. Code Style Guidelines

Adhere strictly to standard Go idioms and the existing project style.

### Formatting & Layout
- **Tooling:** Always use `gofmt` (or `goimports`) to format code.
- **Line Length:** No specific hard limit, but keep lines readable (typically < 100-120 chars).
- **Indentation:** Use tabs for indentation, not spaces.

### Imports
- Group imports into three blocks separated by a blank line:
  1. Standard library imports (e.g., `"context"`, `"fmt"`).
  2. Third-party library imports (e.g., `"github.com/gin-gonic/gin"`).
  3. Internal project imports (e.g., `"github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/..."`).
- **Example:**
  ```go
  import (
      "context"
      "fmt"

      "github.com/gin-gonic/gin"
      "go.uber.org/zap"

      "github.com/telemetrydrops/otel-in-practice/stage-1-monolith/internal/models"
  )
  ```

### Naming Conventions
- **General:** Use `CamelCase` for exported identifiers and `mixedCase` for unexported ones.
- **Variables:** Use concise but descriptive names. `ctx` for Context, `err` for errors.
- **Interfaces:** Single-method interfaces should end in "-er" (e.g., `Reader`, `Writer`).
- **Packages:** Specific, short, lowercase, single-word names. Avoid underscores.

### Error Handling
- **Check Errors:** Always check returned errors immediately.
- **Wrapping:** Use `fmt.Errorf` with the `%w` verb to wrap errors when adding context.
  ```go
  if err != nil {
      return fmt.Errorf("failed to create user: %w", err)
  }
  ```
- **Logging:** Log errors using the configured `zap.Logger` rather than `log.Println` or `fmt.Printf`.
  ```go
  logger.Error("Failed to process order", zap.Error(err))
  ```

### Architecture & Patterns
- **Layered Architecture:** Respect the `handlers` -> `services` -> `repositories` -> `database` flow.
- **Dependency Injection:** Pass dependencies (like repositories, loggers, config) via constructor functions (e.g., `NewUserService`).
- **Context:** Always pass `context.Context` as the first argument to functions performing I/O or long-running operations.
- **Configuration:** Use the `config` package to load settings. Do not hardcode values.

### Types & Models
- Use the `models` package for domain entities.
- Use `gorm` tags for database mapping and `json` tags for API serialization.
- **Example:**
  ```go
  type User struct {
      ID        string `gorm:"primaryKey" json:"id"`
      Email     string `gorm:"uniqueIndex" json:"email"`
      CreatedAt time.Time `json:"created_at"`
  }
  ```

### Comments
- **Exported Code:** All exported functions, types, and constants must have a comment starting with the name of the identifier.
- **Complexity:** Explain *why* complex logic is implemented a certain way, not just *what* it does.

## 3. Observability Guidelines

This section defines the tracing, metrics, and logging patterns used across the codebase. Follow these consistently when adding new code.

### Tracer Initialization
- Obtain a tracer via `otel.Tracer(telemetry.Scope)` and store it as a struct field.
- The scope is defined in `telemetry/const.go` — use the same constant across all layers.

### Starting Spans
- Use `ctx, span := s.tracer.Start(ctx, "span name", ...)` and always `defer span.End()`.
- Pass the **returned** context (not the original) to downstream calls to maintain parent-child relationships.
- Provide known attributes at creation time via `trace.WithAttributes(...)` for efficiency.

### Span Kind
- **Repository/DB spans:** Always use `trace.WithSpanKind(trace.SpanKindClient)`.
- **HTTP handler spans:** Created by `otelgin` middleware as `SpanKindServer` — do not create duplicate spans.
- **Service spans:** Default to `SpanKindInternal` (no explicit kind needed).

### Recovering Spans from Context
- In handlers, use `span := trace.SpanFromContext(ctx)` to enrich the middleware-created span rather than creating a new one.

### Attributes
- Use constants from `telemetry/const.go` for attribute keys (e.g., `telemetry.ATTR_USER_ID`).
- Follow semantic conventions for database attributes: `db.system`, `db.operation`, `db.sql.table`.
- Add result-based attributes (like `result.count`) after the operation completes via `span.SetAttributes()`.

### Events
- Per [OTEP 4430](https://github.com/open-telemetry/opentelemetry-specification/blob/main/oteps/4430-span-event-api-deprecation-plan.md), the Span Event API (`span.AddEvent`) is deprecated. Emit events through the Logs API instead.
- Use `telemetry.EmitEvent(ctx, "event name", log.String("key", value), ...)` to mark milestones within the active span.
- The helper emits a log record correlated with the active span in `ctx`; event name and attributes are preserved.
- Events are for moments in time; attributes are for the span as a whole.

### Error Recording
- Per OTEP 4430, `span.RecordError` is deprecated. Emit exceptions through the Logs API instead.
- On error paths, always use **both**:
  ```go
  telemetry.EmitException(ctx, err)                // log-based exception event
  span.SetStatus(codes.Error, "short description") // marks span as failed
  ```
- `EmitException` alone does NOT mark the span as failed — `SetStatus` is still required.
- For expected conditions (e.g., not-found), `SetStatus` without `EmitException` is acceptable.

### Baggage
- Set baggage in handlers via `baggage.ContextWithBaggage(ctx, bag)`.
- Read baggage in services via `baggage.FromContext(ctx)`.
- Baggage key constants live in `telemetry/const.go` (e.g., `BAGGAGE_PAYMENT_METHOD`).
- Do not put sensitive data in baggage — it travels in HTTP headers.

### Span Links
- Use `trace.WithLinks(trace.Link{SpanContext: ..., Attributes: ...})` to correlate spans across traces.
- Combine with `trace.WithNewRoot()` for async operations that start a new trace but reference the trigger.

### Trace ID Extraction
- Use `span.SpanContext().TraceID().String()` when you need the trace ID for response headers or log correlation.

### IsRecording Guard
- Wrap expensive attribute computation in `if span.IsRecording() { ... }` to avoid wasted work when sampling is off.

### Dynamic Span Names
- Use `span.SetName()` when the final span name depends on runtime information (e.g., filtered vs. unfiltered query).

### Metrics
- Define metric instruments in constructor functions (e.g., `NewOrderService`).
- Use constants from `telemetry/const.go` for metric names with UCUM-compliant units.
- Record metrics with `metric.WithAttributes(...)` for dimensionality.

### Logging
- Use `zap.Logger` with structured fields.
- The logger is bridged to OpenTelemetry via `otelzap` for log-trace correlation.

### Telemetry Constants
- All span names, attribute keys, metric names, and baggage keys are centralized in `internal/telemetry/const.go`.
- Add new constants there rather than using inline strings.

## 4. Specific Libraries

- **Web Framework:** Gin (`github.com/gin-gonic/gin`)
- **ORM:** GORM (`gorm.io/gorm`)
- **Logging:** Zap (`go.uber.org/zap`)
- **Instrumentation:** OpenTelemetry (`go.opentelemetry.io/otel`)
- **HTTP Middleware:** otelgin (`go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin`)
- **Log Bridge:** otelzap (`go.opentelemetry.io/contrib/bridges/otelzap`)
- **Configuration:** otelconf (`go.opentelemetry.io/contrib/otelconf`)

## 5. Workflow Rules

- **Git:** Do not commit changes unless explicitly asked.
- **Verification:** Always run `go build ./...` and `go test ./...` after making changes to ensure no regressions.
- **Safety:** Verify file paths before reading/writing. Use `ls` to check directory existence.
- **Formatting:** Run `go fmt ./...` after modifying Go code.

End of Guidelines.

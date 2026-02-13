# OpenTelemetry in Practice

This repository contains practical examples and progressive stages of implementing OpenTelemetry in a Go-based e-commerce application. It is designed to demonstrate best practices for observability, including distributed tracing, metrics, and structured logging.

## Repository Structure

The project is organized into stages to show the evolution of the application:

### [stage-0-monolith](./stage-0-monolith)
The baseline e-commerce monolith application.
- **Status:** Uninstrumented (Plain Go/Gin/GORM application).
- **Use Case:** Start here if you want to practice adding instrumentation from scratch.

### [stage-1-monolith](./stage-1-monolith)
The instrumented version of the monolith.
- **Status:** Fully instrumented with OpenTelemetry.
- **Tracing API coverage:**
  - Starting spans and deferred `span.End()`
  - Recovering spans from context (`trace.SpanFromContext`)
  - Adding attributes (at creation and after)
  - Span events (`span.AddEvent`)
  - Error recording (`span.RecordError`) and span status (`span.SetStatus`)
  - Span kind (`trace.WithSpanKind`) — CLIENT for DB calls, SERVER via middleware
  - In-process context propagation via `ctx`
  - Cross-process propagation setup (W3C TraceContext + Baggage)
  - Baggage — setting and reading cross-cutting context
  - Span links (`trace.WithLinks`) for async processing correlation
  - Trace ID extraction for response headers and log correlation
  - `span.IsRecording()` guard for expensive attribute computation
  - Dynamic span names (`span.SetName()`)
- **Also includes:**
  - Configuration-driven setup using `otelconf`
  - Custom business metrics (counters, histograms)
  - Structured logging with `otelzap`
  - Semantic conventions for database operations
- **Use Case:** Reference implementation for the Traces API lesson. See each concept in context across handlers, services, and repositories.

## Prerequisites

- **Go:** 1.24 or higher
- **Docker:** Required for running infrastructure (PostgreSQL, Grafana, Otel Collector).
- **Docker Compose:** Required for orchestrating the environment.

## Getting Started

1.  Clone the repository.
2.  Navigate to the desired stage directory (e.g., `cd stage-0-monolith`).
3.  Follow the `README.md` within that directory for specific instructions on building and running the application.

## Development

See [AGENTS.md](./AGENTS.md) for guidelines on coding standards, testing, and agent-based workflows if you are an AI assistant or contributing code.

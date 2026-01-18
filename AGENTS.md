# Agentic Coding Guidelines

This document provides essential information for AI agents operating within this repository.
It covers build instructions, testing procedures, and code style guidelines to ensure consistency and quality.

## Project Structure

The project is currently structured as a monorepo containing:
- `stage-1-monolith/`: A Go-based e-commerce monolith application.
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
- **Line Length:** specific hard limit, but keep lines readable (typically < 100-120 chars).
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
- **Packages:** specific, short, lowercase, single-word names. Avoid underscores.

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

### Architecture & patterns
- **Layered Architecture:** Respect the `handlers` -> `services` -> `repositories` -> `database` flow.
- **Dependency Injection:** Pass dependencies (like repositories, loggers, config) via constructor functions (e.g., `NewUserService`).
- **Context:** Always pass `context.Context` as the first argument to functions performing I/O or long-running operations.
- **Configuration:** Use the `config` package to load settings. Do not hardcode values.

### Observability
- **OpenTelemetry:** Ensure new components are instrumented.
- **Tracing:** Propagate context to maintain trace continuity.
- **Logging:** Use structured logging with `zap`. key-value pairs for context (e.g., `zap.String("user_id", id)`).

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
- **Complexity:** specific explain *why* complex logic is implemented a certain way, not just *what* it does.

## 3. Workflow Rules

- **Git:** Do not commit changes unless explicitly asked.
- **Verification:** Always run `go build ./...` and `go test ./...` after making changes to ensure no regressions.
- **Safety:** Verify file paths before reading/writing. Use `ls` to check directory existence.

## 4. Specific Libraries

- **Web Framework:** Gin (`github.com/gin-gonic/gin`)
- **ORM:** GORM (`gorm.io/gorm`)
- **Logging:** Zap (`go.uber.org/zap`)
- **Instrumentation:** OpenTelemetry (`go.opentelemetry.io/otel`)

End of Guidelines.

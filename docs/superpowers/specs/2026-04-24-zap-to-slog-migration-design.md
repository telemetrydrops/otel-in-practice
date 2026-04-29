# zap → slog Migration Design

**Date:** 2026-04-24
**Scope:** `stage-0-monolith`, `stage-1-monolith`, `AGENTS.md`

## Goal

Replace `go.uber.org/zap` with the standard library's `log/slog` across both monolith stages. Preserve log-trace correlation in the instrumented stage. Pass `context.Context` to every logging call on a request path so the OpenTelemetry bridge can attach the active span.

## Dependencies

- Remove: `go.uber.org/zap`, `go.opentelemetry.io/contrib/bridges/otelzap`.
- Add (stage-1 only): `go.opentelemetry.io/contrib/bridges/otelslog`.
- stage-0 uses `log/slog` stdlib only.

## Logger Construction

**stage-0 `main.go`:**
```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
slog.SetDefault(logger)
```

**stage-1 `internal/telemetry/setup.go`:** fan out to stdout (JSON) and the otelslog bridge via a small hand-rolled `slog.Handler` that dispatches each record to both handlers. No third-party multi-handler dependency.

`Providers.Logger` changes from `*zap.Logger` to `*slog.Logger`.

## Call-Site Migration

| zap | slog |
|---|---|
| `logger.Info("msg", zap.String("k", v))` | `logger.InfoContext(ctx, "msg", "k", v)` |
| `zap.Error(err)` | `slog.Any("error", err)` |
| `logger.Fatal("msg", zap.Error(err))` | `logger.ErrorContext(ctx, "msg", "error", err); os.Exit(1)` |
| `*zap.Logger` field/param | `*slog.Logger` |

Request-path call sites (handlers, services, repositories) use the `…Context` variants so the otelslog bridge picks up the active span. Startup/shutdown logs in `main.go` (no request ctx) use plain `Info`/`Error` — don't manufacture `context.Background()` just to pass something.

Constructor signatures do not gain a `ctx` parameter; they already receive the logger, and request ctx flows through service method calls.

## `Fatal` Handling

slog has no `Fatal`. Replace with explicit `logger.ErrorContext(...); os.Exit(1)` at each site. Makes the exit visible rather than hidden in the logger call.

## AGENTS.md Updates

- Section 2 "Error Handling → Logging": swap zap example for slog.
- Section 2 import example: `"go.uber.org/zap"` → `"log/slog"`.
- Section 3 "Logging": replace otelzap description with otelslog.
- Section 4 "Specific Libraries": Zap → slog (stdlib); otelzap → otelslog.

## Verification

- `go build ./...` in both modules
- `go test ./...` in both modules
- `go fmt ./...`

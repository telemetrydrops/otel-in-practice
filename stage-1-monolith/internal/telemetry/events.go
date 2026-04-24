package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
)

// eventLogger returns the global OTel logger used for emitting log-based events.
// The logger is resolved lazily so it picks up the LoggerProvider set during
// SetupTelemetry, regardless of package initialization order.
func eventLogger() log.Logger {
	return global.GetLoggerProvider().Logger(Scope)
}

// EmitEvent emits a log-based event correlated with the active span in ctx.
// It replaces span.AddEvent per the OTEP 4430 deprecation of the Span Event API.
func EmitEvent(ctx context.Context, name string, attrs ...log.KeyValue) {
	var r log.Record
	r.SetEventName(name)
	r.SetBody(log.StringValue(name))
	if len(attrs) > 0 {
		r.AddAttributes(attrs...)
	}
	eventLogger().Emit(ctx, r)
}

// EmitException emits a log-based exception event correlated with the active
// span in ctx. It replaces span.RecordError per the OTEP 4430 deprecation.
// Callers should still call span.SetStatus(codes.Error, ...) separately when
// the error should mark the span as failed.
func EmitException(ctx context.Context, err error) {
	if err == nil {
		return
	}
	var r log.Record
	r.SetEventName("exception")
	r.SetSeverity(log.SeverityError)
	r.SetBody(log.StringValue(err.Error()))
	// exception.stacktrace is intentionally omitted: the Go error value does
	// not carry its origin stack, and capturing runtime.Stack here would point
	// at the emit site, not where the error arose. Per semconv, the attribute
	// is optional.
	r.AddAttributes(
		log.String("exception.type", fmt.Sprintf("%T", err)),
		log.String("exception.message", err.Error()),
	)
	eventLogger().Emit(ctx, r)
}

package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
)

const otlpHTTPEndpoint = "http://localhost:4318"

// Providers holds the OpenTelemetry providers and logger.
type Providers struct {
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	LoggerProvider log.LoggerProvider
	Propagator     propagation.TextMapPropagator
	Logger         *slog.Logger
	Closer         func(context.Context) error
}

// SetupTelemetry initializes the OpenTelemetry SDK programmatically.
func SetupTelemetry(ctx context.Context, serviceName, version string) (*Providers, error) {
	res, err := newResource(serviceName, version)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	tracerProvider, err := newTracerProvider(ctx, res)
	if err != nil {
		return nil, fmt.Errorf("creating tracer provider: %w", err)
	}

	meterProvider, err := newMeterProvider(ctx, res)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("creating meter provider: %w", err),
			tracerProvider.Shutdown(ctx),
		)
	}

	loggerProvider, err := newLoggerProvider(ctx, res)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("creating logger provider: %w", err),
			meterProvider.Shutdown(ctx),
			tracerProvider.Shutdown(ctx),
		)
	}

	propagator := propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	global.SetLoggerProvider(loggerProvider)
	otel.SetTextMapPropagator(propagator)

	stdoutHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	otelHandler := otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(loggerProvider))
	logger := slog.New(&multiHandler{handlers: []slog.Handler{stdoutHandler, otelHandler}})

	return &Providers{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
		LoggerProvider: loggerProvider,
		Propagator:     propagator,
		Logger:         logger,
		Closer: func(ctx context.Context) error {
			return errors.Join(
				loggerProvider.Shutdown(ctx),
				meterProvider.Shutdown(ctx),
				tracerProvider.Shutdown(ctx),
			)
		},
	}, nil
}

func newResource(serviceName, version string) (*resource.Resource, error) {
	serviceResource := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
		semconv.ServiceVersion(version),
		semconv.ServiceInstanceID(uuid.NewString()),
		semconv.DeploymentEnvironmentName("dev"),
	)

	return resource.Merge(resource.Default(), serviceResource)
}

func newTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL(otlpHTTPEndpoint+"/v1/traces"),
	)
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
		sdktrace.WithSpanLimits(sdktrace.NewSpanLimits()),
	), nil
}

func newMeterProvider(ctx context.Context, res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	exporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpointURL(otlpHTTPEndpoint+"/v1/metrics"),
	)
	if err != nil {
		return nil, err
	}

	reader := sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(15*time.Second))
	durationInstrument := sdkmetric.Instrument{
		Name: EcommerceOrdersProcessingDurationName,
		Kind: sdkmetric.InstrumentKindHistogram,
	}
	durationDistribution := sdkmetric.NewView(durationInstrument, sdkmetric.Stream{})
	durationSum := sdkmetric.NewView(durationInstrument, sdkmetric.Stream{
		Name:        EcommerceOrdersProcessingDurationName + ".sum",
		Aggregation: sdkmetric.AggregationSum{},
	})

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
		sdkmetric.WithView(durationDistribution, durationSum),
		sdkmetric.WithExemplarFilter(exemplar.TraceBasedFilter),
	), nil
}

func newLoggerProvider(ctx context.Context, res *resource.Resource) (*sdklog.LoggerProvider, error) {
	exporter, err := otlploghttp.New(ctx,
		otlploghttp.WithEndpointURL(otlpHTTPEndpoint+"/v1/logs"),
	)
	if err != nil {
		return nil, err
	}

	processor := sdklog.NewBatchProcessor(exporter)
	return sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(processor),
	), nil
}

// multiHandler dispatches each record to every configured slog.Handler.
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range m.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: next}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		next[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: next}
}

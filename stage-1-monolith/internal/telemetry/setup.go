package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"go.opentelemetry.io/contrib/bridges/otelzap"
	otelconf "go.opentelemetry.io/contrib/otelconf/v0.3.0"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Providers holds the OpenTelemetry providers and logger
type Providers struct {
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	LoggerProvider log.LoggerProvider
	Logger         *zap.Logger
	Closer         func(ctx context.Context) error
}

// SetupTelemetry initializes OpenTelemetry providers from configuration
func SetupTelemetry(ctx context.Context, serviceName, version, configFile string) (*Providers, error) {
	providers, err := providersFromConfig(ctx, serviceName, version, configFile)
	if err != nil {
		return nil, err
	}

	// Set global providers
	otel.SetTracerProvider(providers.TracerProvider)
	otel.SetMeterProvider(providers.MeterProvider)
	global.SetLoggerProvider(providers.LoggerProvider)

	// Set up context propagation, needed until this is fixed: https://github.com/open-telemetry/opentelemetry-go-contrib/issues/6712
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return providers, nil
}

// providersFromConfig creates providers from YAML configuration file
func providersFromConfig(ctx context.Context, scope, version, cfgFile string) (*Providers, error) {
	b, err := os.ReadFile(cfgFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Return default providers if config doesn't exist
			logger := zap.Must(zap.NewProduction())
			logger.Warn("OpenTelemetry config file not found, using no-op providers",
				zap.String("config_file", cfgFile))
			return &Providers{
				TracerProvider: tracenoop.NewTracerProvider(),
				MeterProvider:  metricnoop.NewMeterProvider(),
				LoggerProvider: noop.NewLoggerProvider(),
				Logger:         logger,
				Closer:         func(ctx context.Context) error { return nil },
			}, nil
		}
		return nil, fmt.Errorf("failed to read config file %s: %w", cfgFile, err)
	}

	// Expand environment variables in config
	b = []byte(os.ExpandEnv(string(b)))

	// Parse OpenTelemetry configuration
	conf, err := otelconf.ParseYAML(b)
	if err != nil {
		return nil, err
	}

	// Set resource attributes
	if conf.Resource == nil {
		conf.Resource = &otelconf.Resource{}
	}
	if conf.Resource.Attributes == nil {
		conf.Resource.Attributes = []otelconf.AttributeNameValue{}
	}

	// Add service metadata
	conf.Resource.Attributes = insertAttribute(conf.Resource.Attributes,
		string(semconv.ServiceVersionKey), version)
	conf.Resource.Attributes = insertAttribute(conf.Resource.Attributes,
		string(semconv.ServiceInstanceIDKey), uuid.New().String())

	// Create SDK
	sdk, err := otelconf.NewSDK(
		otelconf.WithContext(ctx),
		otelconf.WithOpenTelemetryConfiguration(*conf),
	)
	if err != nil {
		return nil, err
	}

	// Create zap logger with OpenTelemetry bridge
	core := zapcore.NewTee(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			zapcore.AddSync(os.Stdout),
			zapcore.InfoLevel,
		),
		otelzap.NewCore(scope, otelzap.WithLoggerProvider(global.GetLoggerProvider())),
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

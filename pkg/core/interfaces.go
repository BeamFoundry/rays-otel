package core

import (
	"go.opentelemetry.io/otel/attribute"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// OtelResourceCustomizer may be implemented by a Beam to append or override
// OpenTelemetry resource attributes during SDK initialization.
type OtelResourceCustomizer interface {
	CustomizeResource(attrs []attribute.KeyValue) []attribute.KeyValue
}

// OtelTracerProviderCustomizer may be implemented by a Beam to customize
// the [sdktrace.TracerProviderOption] slice before creating the TracerProvider.
type OtelTracerProviderCustomizer interface {
	CustomizeTracerProvider(opts []sdktrace.TracerProviderOption) []sdktrace.TracerProviderOption
}

// OtelMeterProviderCustomizer may be implemented by a Beam to customize
// the [sdkmetric.Option] slice before creating the MeterProvider.
type OtelMeterProviderCustomizer interface {
	CustomizeMeterProvider(opts []sdkmetric.Option) []sdkmetric.Option
}

// OtelLoggerProviderCustomizer may be implemented by a Beam to customize
// the [sdklog.LoggerProviderOption] slice before creating the LoggerProvider.
type OtelLoggerProviderCustomizer interface {
	CustomizeLoggerProvider(opts []sdklog.LoggerProviderOption) []sdklog.LoggerProviderOption
}

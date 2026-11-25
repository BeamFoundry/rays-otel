---
title: "2. Logging Bridge & Customizers"
weight: 2
---

# Logging Bridge & Customizers

## Connecting Rays `Logging` with OpenTelemetry

When `rays.otel.logs.enabled` is `true`, `rays-otel` uses `ContainerConfiguration.WrapLoggingHandler` to decorate Rays's
`slog.Handler` without replacing `rays.logging` formatting:

- **`rays.otel.logs.traceContext: true`**: Wraps the handler with `TraceContextHandler`, which resolves the active
`trace.SpanContext` from `ctx` — or falls back to `TransactionManager.GetContext()` when present — and injects
`trace_id` and `span_id` attributes into log records.
- **`rays.otel.logs.bridge: true`**: Fans out log records via `MultiHandler` to both Rays's configured handler and
`otelslog.NewHandler` backed by the configured `LoggerProvider`.

## Programmatic Customizers

Any singleton Beam implementing one of the interfaces in `github.com/BeamFoundry/rays-otel/pkg/core` is automatically
discovered and applied during initialization:

- `OtelResourceCustomizer`: `CustomizeResource(attrs []attribute.KeyValue) []attribute.KeyValue`
- `OtelTracerProviderCustomizer`: `CustomizeTracerProvider(opts []sdktrace.TracerProviderOption) []sdktrace.TracerProviderOption`
- `OtelMeterProviderCustomizer`: `CustomizeMeterProvider(opts []sdkmetric.Option) []sdkmetric.Option`
- `OtelLoggerProviderCustomizer`: `CustomizeLoggerProvider(opts []sdklog.LoggerProviderOption) []sdklog.LoggerProviderOption`

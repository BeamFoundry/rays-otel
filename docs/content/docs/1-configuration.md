---
title: "1. Configuration"
weight: 1
---

# Configuration (`rays.otel`)

Importing `github.com/BeamFoundry/rays-otel` registers default configuration under `rays.otel` and auto-configures:

- `oteltrace.TracerProvider` (also registered globally via `otel.SetTracerProvider`)
- `otelmetric.MeterProvider` (also registered globally via `otel.SetMeterProvider`)
- `otellog.LoggerProvider` (also registered globally via `otel.SetLoggerProvider`)
- `propagation.TextMapPropagator` (also registered globally via `otel.SetTextMapPropagator`)
- `*http.Client` using `otelhttp.NewTransport`

All SDK providers are flushed and shut down automatically on container `Shutdown()` / `Run()` exit via `BeamPreDestroy`.

## Default YAML Configuration

```yaml
rays:
  otel:
    enabled: true
    serviceName: "rays-app"
    serviceVersion: ""
    environment: ""
    resourceAttributes: {}
    propagators:
      - tracecontext
      - baggage
    traces:
      enabled: true
      exporter: none # none, stdout, otlp-grpc, otlp-http
      endpoint: ""
      insecure: false
      headers: {}
      sampleRatio: 1.0
    metrics:
      enabled: true
      exporter: none # none, stdout, otlp-grpc, otlp-http
      endpoint: ""
      insecure: false
      headers: {}
      interval: 60s
    logs:
      enabled: true
      exporter: none # none, stdout, otlp-grpc, otlp-http
      endpoint: ""
      insecure: false
      headers: {}
      bridge: true
      traceContext: true
    httpClient:
      enabled: true
```

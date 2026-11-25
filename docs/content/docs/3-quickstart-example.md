---
title: "3. Quickstart & Complete Example"
weight: 3
---

# Quickstart & Complete Example

## 1. Install `rays-otel`

```bash
go get github.com/BeamFoundry/rays-otel
```

Import `_ "github.com/BeamFoundry/rays-otel"` in your `main.go` alongside `rays`:

```go
package main

import (
    "os"

    "github.com/BeamFoundry/rays"
    _ "github.com/BeamFoundry/rays-otel"

    "myrepo/myapp/resources"
    _ "myrepo/myapp/pkg/configurations"
    _ "myrepo/myapp/pkg/services"
)

func main() {
    ctx := rays.ApplicationContext()
    ctx.AddConfigFS(resources.Config())

    if err := ctx.Initialize(); err != nil {
        panic(err)
    }

    // Automatically calls BeamPreDestroy on exit to flush traces, metrics, and logs.
    ctx.Run()
}
```

## 2. Configure `application.yaml`

```yaml
rays:
  logging:
    level: INFO
    handler: DEFAULT

  otel:
    enabled: true
    serviceName: "orders-api"
    serviceVersion: "1.0.0"
    environment: "production"
    resourceAttributes:
      team: "checkout"
    traces:
      enabled: true
      exporter: otlp-grpc
      endpoint: "localhost:4317"
      insecure: true
      sampleRatio: 1.0
    metrics:
      enabled: true
      exporter: otlp-grpc
      endpoint: "localhost:4317"
      insecure: true
      interval: 30s
    logs:
      enabled: true
      exporter: otlp-grpc
      endpoint: "localhost:4317"
      insecure: true
      bridge: true
      traceContext: true
    httpClient:
      enabled: true
```

## 3. Inject Providers & Log with Trace Correlation

All OTel providers and the `otelhttp`-wrapped `*http.Client` are registered as `ConditionalOnMissingBeam` singletons and can be injected directly into your services:

```go
package services

import (
    "context"
    "net/http"

    "go.opentelemetry.io/otel/attribute"
    otelmetric "go.opentelemetry.io/otel/metric"
    oteltrace "go.opentelemetry.io/otel/trace"

    . "github.com/BeamFoundry/rays/pkg/core"
    . "github.com/BeamFoundry/rays/pkg/lang"
)

type OrderService struct {
    Service
    logger     Logging                  `@:"Inject"`
    txManager  TransactionManager       `@:"Inject"`
    tracerProv oteltrace.TracerProvider `@:"Inject"`
    meterProv  otelmetric.MeterProvider `@:"Inject"`
    httpClient *http.Client             `@:"Inject"`

    ordersCounter otelmetric.Int64Counter
}

var _, _ = any(&OrderService{}).(Stereotype)

func (this *OrderService) PostConstruct() Error {
    counter, err := this.meterProv.Meter("orders").Int64Counter("orders.created")
    if err != nil {
        return NewWrappedError("MetricInitError", err)
    }
    this.ordersCounter = counter
    return nil
}

func (this *OrderService) CreateOrder(ctx context.Context, orderID string) Error {
    ctx, span := this.tracerProv.Tracer("orders").Start(ctx, "OrderService.CreateOrder",
        oteltrace.WithAttributes(attribute.String("order.id", orderID)),
    )
    defer span.End()

    // Optional: bind ctx to TransactionManager so context-free logger calls
    // (this.logger.Info(...)) and downstream repositories also inherit the span.
    this.txManager.SetContext(ctx)
    defer this.txManager.ClearContext()

    this.logger.Info("Creating order", "order_id", orderID)
    this.ordersCounter.Add(ctx, 1)

    req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://inventory.internal/check", http.NoBody)
    if err != nil {
        return NewWrappedError("RequestError", err)
    }
    resp, err := this.httpClient.Do(req)
    if err != nil {
        return NewWrappedError("UpstreamError", err)
    }
    _ = resp.Body.Close()

    return nil
}
```

## 4. Customize the SDK (Optional)

Implement any customizer interface in `github.com/BeamFoundry/rays-otel/pkg/core` on a `Component`:

```go
package configurations

import (
    "go.opentelemetry.io/otel/attribute"

    . "github.com/BeamFoundry/rays-otel/pkg/core"
    . "github.com/BeamFoundry/rays/pkg/core"
)

type OtelCustomizers struct {
    Component
}

var _ OtelResourceCustomizer = &OtelCustomizers{}

func (this *OtelCustomizers) CustomizeResource(attrs []attribute.KeyValue) []attribute.KeyValue {
    return append(attrs, attribute.String("deployment.region", "eu-west-1"))
}
```

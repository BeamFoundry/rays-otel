---
title: "BeamFoundry Rays OTel"
type: docs
---

# BeamFoundry Rays OTel

**Rays OTel** provides a configurable OpenTelemetry SDK environment for **BeamFoundry Rays** applications —
auto-configuring `TracerProvider`, `MeterProvider`, `LoggerProvider`, `TextMapPropagator`, an `otelhttp`-instrumented
`*http.Client`, and bridging Rays's `Logging` (`log/slog`) with OpenTelemetry logs and trace context.

### Installation

```go
package main

import ( "github.com/BeamFoundry/rays" 
    _ "github.com/BeamFoundry/rays-otel"
)
```

### Guide
1. [Configuration (`rays.otel`)](/rays-otel/docs/1-configuration/)
2. [Logging Bridge & Customizers](/rays-otel/docs/2-logging-and-customizers/)

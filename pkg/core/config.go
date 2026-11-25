// Package core provides OpenTelemetry configuration properties, customizer interfaces,
// and slog integration handlers for Rays applications.
package core

import (
	"time"

	. "github.com/BeamFoundry/rays/pkg/core"
)

// ExporterType represents the supported OpenTelemetry exporter backends.
const (
	ExporterNone     = "none"
	ExporterStdout   = "stdout"
	ExporterOTLPGRPC = "otlp-grpc"
	ExporterOTLPHTTP = "otlp-http"
)

// OtelSignalProperties holds exporter configuration shared by trace, metric, and log signals.
type OtelSignalProperties struct {
	headers     map[string]string
	exporter    string
	endpoint    string
	sampleRatio float64
	enabled     bool
	insecure    bool
}

// Enabled reports whether this signal is enabled.
func (this *OtelSignalProperties) Enabled() bool {
	return this.enabled
}

// Exporter returns the configured exporter backend name ("none", "stdout", "otlp-grpc", or "otlp-http").
func (this *OtelSignalProperties) Exporter() string {
	return this.exporter
}

// Endpoint returns the target collector endpoint for OTLP exporters.
func (this *OtelSignalProperties) Endpoint() string {
	return this.endpoint
}

// Insecure reports whether TLS verification/transport security should be disabled for OTLP exporters.
func (this *OtelSignalProperties) Insecure() bool {
	return this.insecure
}

// Headers returns custom headers attached to OTLP exporter requests.
func (this *OtelSignalProperties) Headers() map[string]string {
	return this.headers
}

// SampleRatio returns the trace sampling ratio in [0.0, 1.0].
func (this *OtelSignalProperties) SampleRatio() float64 {
	return this.sampleRatio
}

// OtelMetricProperties holds exporter and reader configuration for OpenTelemetry metrics.
type OtelMetricProperties struct {
	headers  map[string]string
	exporter string
	endpoint string
	interval time.Duration
	enabled  bool
	insecure bool
}

// Enabled reports whether metrics instrumentation is enabled.
func (this *OtelMetricProperties) Enabled() bool {
	return this.enabled
}

// Exporter returns the configured metric exporter backend name.
func (this *OtelMetricProperties) Exporter() string {
	return this.exporter
}

// Endpoint returns the target collector endpoint for OTLP metric exporters.
func (this *OtelMetricProperties) Endpoint() string {
	return this.endpoint
}

// Insecure reports whether TLS should be disabled for OTLP metric exporters.
func (this *OtelMetricProperties) Insecure() bool {
	return this.insecure
}

// Headers returns custom headers attached to OTLP metric exporter requests.
func (this *OtelMetricProperties) Headers() map[string]string {
	return this.headers
}

// Interval returns the periodic metric export interval.
func (this *OtelMetricProperties) Interval() time.Duration {
	return this.interval
}

// OtelLogProperties holds exporter and slog bridge configuration for OpenTelemetry logs.
type OtelLogProperties struct {
	headers      map[string]string
	exporter     string
	endpoint     string
	enabled      bool
	insecure     bool
	bridge       bool
	traceContext bool
}

// Enabled reports whether the OpenTelemetry LoggerProvider is enabled.
func (this *OtelLogProperties) Enabled() bool {
	return this.enabled
}

// Exporter returns the configured log exporter backend name.
func (this *OtelLogProperties) Exporter() string {
	return this.exporter
}

// Endpoint returns the target collector endpoint for OTLP log exporters.
func (this *OtelLogProperties) Endpoint() string {
	return this.endpoint
}

// Insecure reports whether TLS should be disabled for OTLP log exporters.
func (this *OtelLogProperties) Insecure() bool {
	return this.insecure
}

// Headers returns custom headers attached to OTLP log exporter requests.
func (this *OtelLogProperties) Headers() map[string]string {
	return this.headers
}

// Bridge reports whether Rays slog records should be bridged to the OpenTelemetry LoggerProvider.
func (this *OtelLogProperties) Bridge() bool {
	return this.bridge
}

// TraceContext reports whether active trace_id and span_id attributes should be injected into slog records.
func (this *OtelLogProperties) TraceContext() bool {
	return this.traceContext
}

// OtelHTTPClientProperties holds configuration for the auto-configured instrumented *http.Client Beam.
type OtelHTTPClientProperties struct {
	enabled bool
}

// Enabled reports whether the instrumented *http.Client Beam is enabled.
func (this *OtelHTTPClientProperties) Enabled() bool {
	return this.enabled
}

// OtelProperties binds the "rays.otel" YAML configuration namespace.
type OtelProperties struct {
	ConfigurationProperties `Name:"rays.otel"`
	resourceAttributes      map[string]string
	propagators             []string
	serviceName             string
	serviceVersion          string
	environment             string
	logs                    OtelLogProperties
	traces                  OtelSignalProperties
	metrics                 OtelMetricProperties
	httpClient              OtelHTTPClientProperties
	enabled                 bool
}

var _, _ = any(&OtelProperties{}).(Stereotype)

// Enabled reports whether OpenTelemetry auto-configuration is enabled.
func (this *OtelProperties) Enabled() bool {
	return this.enabled
}

// ServiceName returns the configured OpenTelemetry service.name resource attribute.
func (this *OtelProperties) ServiceName() string {
	return this.serviceName
}

// ServiceVersion returns the configured OpenTelemetry service.version resource attribute.
func (this *OtelProperties) ServiceVersion() string {
	return this.serviceVersion
}

// Environment returns the configured OpenTelemetry deployment.environment.name resource attribute.
func (this *OtelProperties) Environment() string {
	return this.environment
}

// ResourceAttributes returns additional static OpenTelemetry resource attributes.
func (this *OtelProperties) ResourceAttributes() map[string]string {
	return this.resourceAttributes
}

// Propagators returns the configured W3C/OTel propagator names (e.g. "tracecontext", "baggage").
func (this *OtelProperties) Propagators() []string {
	return this.propagators
}

// Traces returns the trace signal configuration properties.
func (this *OtelProperties) Traces() *OtelSignalProperties {
	return &this.traces
}

// Metrics returns the metric signal configuration properties.
func (this *OtelProperties) Metrics() *OtelMetricProperties {
	return &this.metrics
}

// Logs returns the log signal configuration properties.
func (this *OtelProperties) Logs() *OtelLogProperties {
	return &this.logs
}

// HTTPClient returns the HTTP client instrumentation properties.
func (this *OtelProperties) HTTPClient() *OtelHTTPClientProperties {
	return &this.httpClient
}

package autoconfig

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"net/http"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	. "github.com/BeamFoundry/rays-otel/pkg/core"
	. "github.com/BeamFoundry/rays/pkg/core"
	. "github.com/BeamFoundry/rays/pkg/testing"
)

type recordingLogProcessor struct {
	err     error
	records []sdklog.Record
	mu      sync.Mutex
}

func (this *recordingLogProcessor) OnEmit(_ context.Context, record *sdklog.Record) error {
	this.mu.Lock()
	this.records = append(this.records, *record)
	this.mu.Unlock()
	return nil
}

//nolint:gocritic // Respect sdklog.Processor interface
func (this *recordingLogProcessor) Enabled(_ context.Context, _ sdklog.EnabledParameters) bool {
	return true
}

func (this *recordingLogProcessor) Shutdown(_ context.Context) error {
	return this.err
}

func (this *recordingLogProcessor) ForceFlush(_ context.Context) error {
	return nil
}

type failingMetricReader struct {
	sdkmetric.Reader
}

func (this *failingMetricReader) Shutdown(_ context.Context) error {
	return errors.New("metric shutdown failed")
}

type failingSpanExporter struct{}

func (this *failingSpanExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error {
	return nil
}

func (this *failingSpanExporter) Shutdown(_ context.Context) error {
	return errors.New("trace shutdown failed")
}

type testCustomizers struct {
	Component
	IgnoreScanning
	spanRecorder *tracetest.SpanRecorder
	logProcessor *recordingLogProcessor
	failShutdown bool
}

func (this *testCustomizers) CustomizeResource(attrs []attribute.KeyValue) []attribute.KeyValue {
	return append(attrs, attribute.String("custom.key", "custom.val"))
}

func (this *testCustomizers) CustomizeTracerProvider(opts []sdktrace.TracerProviderOption) []sdktrace.TracerProviderOption {
	if this.spanRecorder != nil {
		opts = append(opts, sdktrace.WithSpanProcessor(this.spanRecorder))
	}
	if this.failShutdown {
		opts = append(opts, sdktrace.WithSyncer(&failingSpanExporter{}))
	}
	return opts
}

func (this *testCustomizers) CustomizeMeterProvider(opts []sdkmetric.Option) []sdkmetric.Option {
	if this.failShutdown {
		opts = append(opts, sdkmetric.WithReader(&failingMetricReader{
			Reader: sdkmetric.NewManualReader(),
		}))
	}
	return opts
}

func (this *testCustomizers) CustomizeLoggerProvider(opts []sdklog.LoggerProviderOption) []sdklog.LoggerProviderOption {
	if this.logProcessor != nil {
		opts = append(opts, sdklog.WithProcessor(this.logProcessor))
	}
	return opts
}

var (
	_ OtelResourceCustomizer       = &testCustomizers{}
	_ OtelTracerProviderCustomizer = &testCustomizers{}
	_ OtelMeterProviderCustomizer  = &testCustomizers{}
	_ OtelLoggerProviderCustomizer = &testCustomizers{}
)

func loadBaseOtelConfig(overrides map[string]any) {
	cfg := map[string]any{
		"enabled":            true,
		"serviceName":        "test-service",
		"serviceVersion":     "1.2.3",
		"environment":        "test",
		"resourceAttributes": map[string]any{"team": "platform"},
		"propagators":        []any{"tracecontext", "baggage"},
		"traces": map[string]any{
			"enabled":     true,
			"exporter":    "none",
			"sampleRatio": 1.0,
		},
		"metrics": map[string]any{
			"enabled":  true,
			"exporter": "none",
			"interval": "10s",
		},
		"logs": map[string]any{
			"enabled":      true,
			"exporter":     "none",
			"bridge":       true,
			"traceContext": true,
		},
		"httpClient": map[string]any{
			"enabled": true,
		},
	}
	maps.Copy(cfg, overrides)
	GetContainer().EnableScanning()
	Expect(GetContainer().LoadConfigAt(cfg, "rays.otel")).NotTo(HaveOccurred())
}

var _ = Describe("OtelConfiguration", func() {
	It("initializes providers, customizers, slog bridge, trace context, HTTPClient, and PreDestroy", func() {
		loadBaseOtelConfig(nil)
		spanRec := tracetest.NewSpanRecorder()
		logProc := &recordingLogProcessor{}
		customizers := &testCustomizers{
			spanRecorder: spanRec,
			logProcessor: logProc,
		}
		_, err := GetContainer().AddBeamFor(customizers)
		Expect(err).NotTo(HaveOccurred())
		Expect(GetContainer().Initialize()).NotTo(HaveOccurred())

		tp := GetBeamDefaultInstance[oteltrace.TracerProvider]()
		mp := GetBeamDefaultInstance[otelmetric.MeterProvider]()
		lp := GetBeamDefaultInstance[otellog.LoggerProvider]()
		prop := GetBeamDefaultInstance[propagation.TextMapPropagator]()
		client := GetBeamDefaultInstance[*http.Client]()

		Expect(tp).NotTo(BeNil())
		Expect(mp).NotTo(BeNil())
		Expect(lp).NotTo(BeNil())
		Expect(prop).NotTo(BeNil())
		Expect(client).NotTo(BeNil())

		var buf bytes.Buffer
		GetLogger().SetOutput(&buf)
		GetLogger().SetColor(false)

		txMgr := GetBeamDefaultInstance[TransactionManager]()
		ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
		txMgr.SetContext(ctx)
		GetLogger().Info("hello-otel", "k", "v")
		txMgr.ClearContext()
		span.End()

		Expect(buf.String()).To(ContainSubstring("trace_id=" + span.SpanContext().TraceID().String()))
		Expect(buf.String()).To(ContainSubstring("span_id=" + span.SpanContext().SpanID().String()))
		Expect(spanRec.Ended()).To(HaveLen(1))
		logProc.mu.Lock()
		var matched *sdklog.Record
		for i := range logProc.records {
			if logProc.records[i].Body().AsString() == "hello-otel" {
				matched = &logProc.records[i]
				break
			}
		}
		logProc.mu.Unlock()
		Expect(matched).NotTo(BeNil())
		Expect(matched.TraceID()).To(Equal(span.SpanContext().TraceID()))

		Expect(GetContainer().Shutdown()).NotTo(HaveOccurred())
	})

	It("returns noop providers and uninstrumented HTTPClient when disabled", func() {
		loadBaseOtelConfig(map[string]any{"enabled": false})
		Expect(GetContainer().Initialize()).NotTo(HaveOccurred())

		Expect(GetBeamDefaultInstance[oteltrace.TracerProvider]()).NotTo(BeNil())
		Expect(GetBeamDefaultInstance[otelmetric.MeterProvider]()).NotTo(BeNil())
		Expect(GetBeamDefaultInstance[otellog.LoggerProvider]()).NotTo(BeNil())
		Expect(GetBeamDefaultInstance[propagation.TextMapPropagator]()).NotTo(BeNil())
		Expect(GetBeamDefaultInstance[*http.Client]().Transport).To(BeNil())
		Expect(GetContainer().Shutdown()).NotTo(HaveOccurred())
	})

	It("supports traceContext-only and bridge-disabled logging modes", func() {
		loadBaseOtelConfig(map[string]any{
			"logs": map[string]any{
				"enabled":      true,
				"exporter":     "none",
				"bridge":       false,
				"traceContext": true,
			},
		})
		Expect(GetContainer().Initialize()).NotTo(HaveOccurred())

		var buf bytes.Buffer
		GetLogger().SetOutput(&buf)
		tp := GetBeamDefaultInstance[oteltrace.TracerProvider]()
		ctx, span := tp.Tracer("test").Start(context.Background(), "trace-only")
		GetLogger().InfoContext(ctx, "trace-ctx-only")
		span.End()
		Expect(buf.String()).To(ContainSubstring("trace_id=" + span.SpanContext().TraceID().String()))

		GetContainer().Reset()
		loadBaseOtelConfig(map[string]any{
			"logs": map[string]any{
				"enabled":      true,
				"exporter":     "none",
				"bridge":       false,
				"traceContext": false,
			},
		})
		Expect(GetContainer().Initialize()).NotTo(HaveOccurred())
	})

	It("supports stdout and otlp-grpc/otlp-http exporters and default empty propagators", func() {
		loadBaseOtelConfig(map[string]any{
			"propagators": []string{},
			"traces": map[string]any{
				"enabled":     true,
				"exporter":    "stdout",
				"sampleRatio": 1.0,
			},
			"metrics": map[string]any{
				"enabled":  true,
				"exporter": "stdout",
				"interval": "10s",
			},
			"logs": map[string]any{
				"enabled":  true,
				"exporter": "stdout",
			},
		})
		Expect(GetContainer().Initialize()).NotTo(HaveOccurred())
		Expect(GetContainer().Shutdown()).NotTo(HaveOccurred())

		GetContainer().Reset()
		loadBaseOtelConfig(map[string]any{
			"traces": map[string]any{
				"enabled":  true,
				"exporter": "otlp-grpc",
				"endpoint": "127.0.0.1:4317",
				"insecure": true,
				"headers":  map[string]any{"x-api-key": "secret"},
			},
			"metrics": map[string]any{
				"enabled":  true,
				"exporter": "otlp-grpc",
				"endpoint": "127.0.0.1:4317",
				"insecure": true,
				"headers":  map[string]any{"x-api-key": "secret"},
			},
			"logs": map[string]any{
				"enabled":  true,
				"exporter": "otlp-grpc",
				"endpoint": "127.0.0.1:4317",
				"insecure": true,
				"headers":  map[string]any{"x-api-key": "secret"},
			},
		})
		Expect(GetContainer().Initialize()).NotTo(HaveOccurred())

		GetContainer().Reset()
		loadBaseOtelConfig(map[string]any{
			"traces": map[string]any{
				"enabled":  true,
				"exporter": "otlp-http",
				"endpoint": "127.0.0.1:4318",
				"insecure": true,
				"headers":  map[string]any{"x-api-key": "secret"},
			},
			"metrics": map[string]any{
				"enabled":  true,
				"exporter": "otlp-http",
				"endpoint": "127.0.0.1:4318",
				"insecure": true,
				"headers":  map[string]any{"x-api-key": "secret"},
			},
			"logs": map[string]any{
				"enabled":  true,
				"exporter": "otlp-http",
				"endpoint": "127.0.0.1:4318",
				"insecure": true,
				"headers":  map[string]any{"x-api-key": "secret"},
			},
		})
		Expect(GetContainer().Initialize()).NotTo(HaveOccurred())
	})

	It("returns errors on unknown propagator or exporter names", func() {
		loadBaseOtelConfig(map[string]any{
			"propagators": []any{"unknown-prop"},
		})
		Expect(GetContainer().Initialize()).To(HaveOccurred())

		GetContainer().Reset()
		loadBaseOtelConfig(map[string]any{
			"traces": map[string]any{"enabled": true, "exporter": "unknown"},
		})
		Expect(GetContainer().Initialize()).To(HaveOccurred())

		GetContainer().Reset()
		loadBaseOtelConfig(map[string]any{
			"metrics": map[string]any{"enabled": true, "exporter": "unknown"},
		})
		Expect(GetContainer().Initialize()).To(HaveOccurred())

		GetContainer().Reset()
		loadBaseOtelConfig(map[string]any{
			"logs": map[string]any{"enabled": true, "exporter": "unknown"},
		})
		Expect(GetContainer().Initialize()).To(HaveOccurred())

		// Exercise exporter construction error path
		GetContainer().Reset()
		loadBaseOtelConfig(map[string]any{
			"metrics": map[string]any{"enabled": true, "exporter": "otlp-http", "endpoint": "bad\x7fhost"},
		})
		Expect(GetContainer().Initialize()).To(HaveOccurred())
	})

	It("records and returns shutdown errors from providers", func() {
		loadBaseOtelConfig(nil)
		customizers := &testCustomizers{
			logProcessor: &recordingLogProcessor{err: errors.New("log shutdown failed")},
			failShutdown: true,
		}
		_, err := GetContainer().AddBeamFor(customizers)
		Expect(err).NotTo(HaveOccurred())
		Expect(GetContainer().Initialize()).NotTo(HaveOccurred())

		shutdownErr := GetContainer().Shutdown()
		Expect(shutdownErr).To(HaveOccurred())
		Expect(shutdownErr.Name()).To(Equal("OtelShutdownError"))
	})
})

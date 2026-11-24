// Package autoconfig registers OpenTelemetry SDK providers, logging bridges,
// and instrumented HTTP client Beams from the "rays.otel" configuration namespace.
package autoconfig

import (
	"context"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/BeamFoundry/rays"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	nooplog "go.opentelemetry.io/otel/log/noop"
	otelmetric "go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"

	. "github.com/BeamFoundry/rays-otel/pkg/core"
	. "github.com/BeamFoundry/rays/pkg/core"
	. "github.com/BeamFoundry/rays/pkg/lang"
)

const shutdownTimeout = 5 * time.Second

// OtelConfiguration auto-configures OpenTelemetry providers, propagators, slog integration,
// and graceful shutdown based on [OtelProperties].
type OtelConfiguration struct {
	Configuration
	logger              Logging            `@:"Inject"`
	txManager           TransactionManager `@:"Inject{Optional=true}"`
	propagator          propagation.TextMapPropagator
	props               *OtelProperties `@:"Inject"`
	tracerProvider      *sdktrace.TracerProvider
	meterProvider       *sdkmetric.MeterProvider
	loggerProvider      *sdklog.LoggerProvider
	resourceCustomizers []OtelResourceCustomizer       `@:"Inject{Optional=true}"`
	tracerCustomizers   []OtelTracerProviderCustomizer `@:"Inject{Optional=true}"`
	meterCustomizers    []OtelMeterProviderCustomizer  `@:"Inject{Optional=true}"`
	loggerCustomizers   []OtelLoggerProviderCustomizer `@:"Inject{Optional=true}"`
}

var (
	_, _                   = any(&OtelConfiguration{}).(Stereotype)
	_    BeamPostConstruct = &OtelConfiguration{}
	_    BeamPreDestroy    = &OtelConfiguration{}
)

// PostConstruct initializes configured OpenTelemetry providers, registers global instances,
// and attaches the slog-to-OTel bridge / trace-context enricher when enabled.
func (this *OtelConfiguration) PostConstruct() Error {
	if !this.props.Enabled() {
		this.logger.Debug("OpenTelemetry auto-configuration is disabled")
		return nil
	}

	ctx := context.Background()
	res := this.buildResource()

	prop, err := buildPropagator(this.props.Propagators())
	if err != nil {
		return err
	}
	this.propagator = prop
	otel.SetTextMapPropagator(prop)

	if this.props.Traces().Enabled() {
		if err := this.initTracerProvider(ctx, res); err != nil {
			return err
		}
	}

	if this.props.Metrics().Enabled() {
		if err := this.initMeterProvider(ctx, res); err != nil {
			return err
		}
	}

	if this.props.Logs().Enabled() {
		if err := this.initLoggerProvider(ctx, res); err != nil {
			return err
		}
		this.wireLoggingBridge()
	}

	return nil
}

func (this *OtelConfiguration) buildResource() *resource.Resource {
	var attrs []attribute.KeyValue
	if this.props.ServiceName() != "" {
		attrs = append(attrs, semconv.ServiceName(this.props.ServiceName()))
	}
	if this.props.ServiceVersion() != "" {
		attrs = append(attrs, semconv.ServiceVersion(this.props.ServiceVersion()))
	}
	if this.props.Environment() != "" {
		attrs = append(attrs, semconv.DeploymentEnvironmentName(this.props.Environment()))
	}
	for _, k := range slices.Sorted(maps.Keys(this.props.ResourceAttributes())) {
		attrs = append(attrs, attribute.String(k, this.props.ResourceAttributes()[k]))
	}
	for _, customizer := range this.resourceCustomizers {
		attrs = customizer.CustomizeResource(attrs)
	}
	def := resource.Default()
	//nolint:errcheck // SchemaURLs are identical so Merge cannot fail
	res, _ := resource.Merge(
		def,
		resource.NewWithAttributes(def.SchemaURL(), attrs...),
	)
	return res
}

func (this *OtelConfiguration) initTracerProvider(ctx context.Context, res *resource.Resource) Error {
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(this.props.Traces().SampleRatio()))),
	}
	exp, err := buildTraceExporter(ctx, this.props.Traces())
	if err != nil {
		return err
	}
	if exp != nil {
		opts = append(opts, sdktrace.WithBatcher(exp))
	}
	for _, customizer := range this.tracerCustomizers {
		opts = customizer.CustomizeTracerProvider(opts)
	}
	this.tracerProvider = sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(this.tracerProvider)
	return nil
}

func (this *OtelConfiguration) initMeterProvider(ctx context.Context, res *resource.Resource) Error {
	opts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}
	exp, err := buildMetricExporter(ctx, this.props.Metrics())
	if err != nil {
		return err
	}
	if exp != nil {
		var readerOpts []sdkmetric.PeriodicReaderOption
		if interval := this.props.Metrics().Interval(); interval > 0 {
			readerOpts = append(readerOpts, sdkmetric.WithInterval(interval))
		}
		opts = append(opts, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp, readerOpts...)))
	}
	for _, customizer := range this.meterCustomizers {
		opts = customizer.CustomizeMeterProvider(opts)
	}
	this.meterProvider = sdkmetric.NewMeterProvider(opts...)
	otel.SetMeterProvider(this.meterProvider)
	return nil
}

func (this *OtelConfiguration) initLoggerProvider(ctx context.Context, res *resource.Resource) Error {
	opts := []sdklog.LoggerProviderOption{
		sdklog.WithResource(res),
	}
	exp, err := buildLogExporter(ctx, this.props.Logs())
	if err != nil {
		return err
	}
	if exp != nil {
		opts = append(opts, sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)))
	}
	for _, customizer := range this.loggerCustomizers {
		opts = customizer.CustomizeLoggerProvider(opts)
	}
	this.loggerProvider = sdklog.NewLoggerProvider(opts...)
	otel.SetLoggerProvider(this.loggerProvider)
	return nil
}

func (this *OtelConfiguration) wireLoggingBridge() {
	bridge := this.props.Logs().Bridge()
	traceCtx := this.props.Logs().TraceContext()
	if !bridge && !traceCtx {
		return
	}
	lp := this.loggerProvider
	txMgr := this.txManager
	serviceName := this.props.ServiceName()

	rays.ApplicationContext().WrapLoggingHandler(func(next slog.Handler) slog.Handler {
		base := next
		if traceCtx {
			base = NewTraceContextHandler(base, txMgr, true)
		}
		if bridge && lp != nil {
			otelH := otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(lp))
			return NewMultiHandler(base, NewTraceContextHandler(otelH, txMgr, false))
		}
		return base
	})
}

// PreDestroy flushes and shuts down all initialized OpenTelemetry SDK providers.
func (this *OtelConfiguration) PreDestroy() Error {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	var firstErr Error
	record := func(err error) {
		if err != nil {
			wrapped := NewWrappedError("OtelShutdownError", err)
			wrapped.LogError()
			if firstErr == nil {
				firstErr = wrapped
			}
		}
	}

	if this.tracerProvider != nil {
		record(this.tracerProvider.Shutdown(ctx))
	}
	if this.meterProvider != nil {
		record(this.meterProvider.Shutdown(ctx))
	}
	if this.loggerProvider != nil {
		record(this.loggerProvider.Shutdown(ctx))
	}
	return firstErr
}

type wrappedTracerProvider struct {
	oteltrace.TracerProvider
}

type wrappedMeterProvider struct {
	otelmetric.MeterProvider
}

type wrappedLoggerProvider struct {
	otellog.LoggerProvider
}

// TracerProvider provides the configured [oteltrace.TracerProvider] Beam.
func (this *OtelConfiguration) TracerProvider(
	_ struct {
		ConditionalOnMissingBeam
	},
) oteltrace.TracerProvider {
	if this.tracerProvider != nil {
		return this.tracerProvider
	}
	return &wrappedTracerProvider{TracerProvider: nooptrace.NewTracerProvider()}
}

// MeterProvider provides the configured [otelmetric.MeterProvider] Beam.
func (this *OtelConfiguration) MeterProvider(
	_ struct {
		ConditionalOnMissingBeam
	},
) otelmetric.MeterProvider {
	if this.meterProvider != nil {
		return this.meterProvider
	}
	return &wrappedMeterProvider{MeterProvider: noopmetric.NewMeterProvider()}
}

// LoggerProvider provides the configured [otellog.LoggerProvider] Beam.
func (this *OtelConfiguration) LoggerProvider(
	_ struct {
		ConditionalOnMissingBeam
	},
) otellog.LoggerProvider {
	if this.loggerProvider != nil {
		return this.loggerProvider
	}
	return &wrappedLoggerProvider{LoggerProvider: nooplog.NewLoggerProvider()}
}

// Propagator provides the configured [propagation.TextMapPropagator] Beam.
func (this *OtelConfiguration) Propagator(
	_ struct {
		ConditionalOnMissingBeam
	},
) propagation.TextMapPropagator {
	if this.propagator != nil {
		return this.propagator
	}
	return &wrappedPropagator{
		TextMapPropagator: propagation.NewCompositeTextMapPropagator(),
	}
}

// HTTPClient provides an [otelhttp]-instrumented [*http.Client] Beam when enabled.
func (this *OtelConfiguration) HTTPClient(
	_ struct {
		ConditionalOnMissingBeam
	},
) *http.Client {
	if !this.props.Enabled() || !this.props.HTTPClient().Enabled() {
		return &http.Client{}
	}
	return &http.Client{
		Transport: otelhttp.NewTransport(nil),
	}
}

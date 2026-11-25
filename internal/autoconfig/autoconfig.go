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
	otelmetric "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	oteltrace "go.opentelemetry.io/otel/trace"

	. "github.com/BeamFoundry/rays-otel/pkg/core"
	. "github.com/BeamFoundry/rays/pkg/core"
	. "github.com/BeamFoundry/rays/pkg/lang"
)

const shutdownTimeout = 5 * time.Second

// OtelConfiguration registers OpenTelemetry provider, propagator, and HTTP client Beams.
// Concrete SDK initialization is performed by [OtelLifecycle] during the Service initialization phase
// so all [Component] customizers have their dependencies injected and PostConstruct hooks completed first.
type OtelConfiguration struct {
	Configuration
	props          *OtelProperties `@:"Inject"`
	tracerDelegate *delegatingTracerProvider
	meterDelegate  *delegatingMeterProvider
	loggerDelegate *delegatingLoggerProvider
	propDelegate   *delegatingPropagator
	httpTransport  *delegatingTransport
}

var _, _ = any(&OtelConfiguration{}).(Stereotype)

// New initializes the delegating provider wrappers for [OtelConfiguration].
func (this OtelConfiguration) New() *OtelConfiguration {
	this.tracerDelegate = newDelegatingTracerProvider()
	this.meterDelegate = newDelegatingMeterProvider()
	this.loggerDelegate = newDelegatingLoggerProvider()
	this.propDelegate = newDelegatingPropagator()
	this.httpTransport = newDelegatingTransport()
	return &this
}

// TracerProvider provides the configured [oteltrace.TracerProvider] Beam.
func (this *OtelConfiguration) TracerProvider(
	_ struct {
		ConditionalOnMissingBeam
	},
) oteltrace.TracerProvider {
	return this.tracerDelegate
}

// MeterProvider provides the configured [otelmetric.MeterProvider] Beam.
func (this *OtelConfiguration) MeterProvider(
	_ struct {
		ConditionalOnMissingBeam
	},
) otelmetric.MeterProvider {
	return this.meterDelegate
}

// LoggerProvider provides the configured [otellog.LoggerProvider] Beam.
func (this *OtelConfiguration) LoggerProvider(
	_ struct {
		ConditionalOnMissingBeam
	},
) otellog.LoggerProvider {
	return this.loggerDelegate
}

// Propagator provides the configured [propagation.TextMapPropagator] Beam.
func (this *OtelConfiguration) Propagator(
	_ struct {
		ConditionalOnMissingBeam
	},
) propagation.TextMapPropagator {
	return this.propDelegate
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
		Transport: this.httpTransport,
	}
}

// OtelLifecycle initializes OpenTelemetry SDK providers, customizers, slog integration,
// and graceful shutdown during the Service lifecycle phase (after Component customizers initialize).
type OtelLifecycle struct {
	Service
	logger              Logging                      `@:"Inject"`
	txManager           Optional[TransactionManager] `@:"Inject"`
	props               *OtelProperties              `@:"Inject"`
	config              *OtelConfiguration           `@:"Inject"`
	tracerProvider      *sdktrace.TracerProvider
	meterProvider       *sdkmetric.MeterProvider
	loggerProvider      *sdklog.LoggerProvider
	resourceCustomizers []OtelResourceCustomizer       `@:"Inject"`
	tracerCustomizers   []OtelTracerProviderCustomizer `@:"Inject"`
	meterCustomizers    []OtelMeterProviderCustomizer  `@:"Inject"`
	loggerCustomizers   []OtelLoggerProviderCustomizer `@:"Inject"`
}

var (
	_, _                   = any(&OtelLifecycle{}).(Stereotype)
	_    BeamPostConstruct = &OtelLifecycle{}
	_    BeamPreDestroy    = &OtelLifecycle{}
)

// PostConstruct initializes configured OpenTelemetry providers, applies all initialized customizers,
// registers global instances, and attaches the slog-to-OTel bridge / trace-context enricher.
func (this *OtelLifecycle) PostConstruct() Error {
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
	this.config.propDelegate.set(prop)
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

	if this.props.HTTPClient().Enabled() {
		this.config.httpTransport.set(otelhttp.NewTransport(nil))
	}

	return nil
}

func (this *OtelLifecycle) buildResource() *resource.Resource {
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

func (this *OtelLifecycle) initTracerProvider(ctx context.Context, res *resource.Resource) Error {
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
	this.config.tracerDelegate.set(this.tracerProvider)
	otel.SetTracerProvider(this.tracerProvider)
	return nil
}

func (this *OtelLifecycle) initMeterProvider(ctx context.Context, res *resource.Resource) Error {
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
	this.config.meterDelegate.set(this.meterProvider)
	otel.SetMeterProvider(this.meterProvider)
	return nil
}

func (this *OtelLifecycle) initLoggerProvider(ctx context.Context, res *resource.Resource) Error {
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
	this.config.loggerDelegate.set(this.loggerProvider)
	otel.SetLoggerProvider(this.loggerProvider)
	return nil
}

func (this *OtelLifecycle) wireLoggingBridge() {
	bridge := this.props.Logs().Bridge()
	traceCtx := this.props.Logs().TraceContext()
	if !bridge && !traceCtx {
		return
	}
	lp := this.loggerProvider
	txMgr, _ := this.txManager.Get()
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
func (this *OtelLifecycle) PreDestroy() Error {
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

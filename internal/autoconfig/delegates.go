package autoconfig

import (
	"context"
	"net/http"
	"sync"

	otellog "go.opentelemetry.io/otel/log"
	logembedded "go.opentelemetry.io/otel/log/embedded"
	nooplog "go.opentelemetry.io/otel/log/noop"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
	traceembedded "go.opentelemetry.io/otel/trace/embedded"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

type versionProvider interface {
	currentVersion() uint64
}

type versionedHandle[T any] struct {
	provider      versionProvider
	create        func() T
	cached        T
	cachedVersion uint64
	mu            sync.Mutex
}

func (this *versionedHandle[T]) get() T {
	ver := this.provider.currentVersion()
	this.mu.Lock()
	defer this.mu.Unlock()
	if this.cachedVersion != ver {
		this.cached = this.create()
		this.cachedVersion = ver
	}
	return this.cached
}

type delegatingTracerProvider struct {
	traceembedded.TracerProvider
	inner   oteltrace.TracerProvider
	version uint64
	mu      sync.RWMutex
}

func newDelegatingTracerProvider() *delegatingTracerProvider {
	return &delegatingTracerProvider{
		inner:   nooptrace.NewTracerProvider(),
		version: 1,
	}
}

func (this *delegatingTracerProvider) set(tp oteltrace.TracerProvider) {
	if tp == nil {
		tp = nooptrace.NewTracerProvider()
	}
	this.mu.Lock()
	defer this.mu.Unlock()
	this.inner = tp
	this.version++
}

func (this *delegatingTracerProvider) currentVersion() uint64 {
	this.mu.RLock()
	defer this.mu.RUnlock()
	return this.version
}

func (this *delegatingTracerProvider) get() oteltrace.TracerProvider {
	this.mu.RLock()
	defer this.mu.RUnlock()
	return this.inner
}

func (this *delegatingTracerProvider) Tracer(name string, options ...oteltrace.TracerOption) oteltrace.Tracer {
	return &delegatingTracer{
		handle: versionedHandle[oteltrace.Tracer]{
			provider: this,
			create: func() oteltrace.Tracer {
				return this.get().Tracer(name, options...)
			},
		},
	}
}

type delegatingTracer struct {
	traceembedded.Tracer
	handle versionedHandle[oteltrace.Tracer]
}

func (this *delegatingTracer) Start(ctx context.Context, spanName string, opts ...oteltrace.SpanStartOption) (context.Context, oteltrace.Span) {
	//nolint:spancheck // Caller owns the returned span
	return this.handle.get().Start(ctx, spanName, opts...)
}

type delegatingLoggerProvider struct {
	logembedded.LoggerProvider
	inner   otellog.LoggerProvider
	version uint64
	mu      sync.RWMutex
}

func newDelegatingLoggerProvider() *delegatingLoggerProvider {
	return &delegatingLoggerProvider{
		inner:   nooplog.NewLoggerProvider(),
		version: 1,
	}
}

func (this *delegatingLoggerProvider) set(lp otellog.LoggerProvider) {
	if lp == nil {
		lp = nooplog.NewLoggerProvider()
	}
	this.mu.Lock()
	defer this.mu.Unlock()
	this.inner = lp
	this.version++
}

func (this *delegatingLoggerProvider) currentVersion() uint64 {
	this.mu.RLock()
	defer this.mu.RUnlock()
	return this.version
}

func (this *delegatingLoggerProvider) get() otellog.LoggerProvider {
	this.mu.RLock()
	defer this.mu.RUnlock()
	return this.inner
}

func (this *delegatingLoggerProvider) Logger(name string, options ...otellog.LoggerOption) otellog.Logger {
	return &delegatingLogger{
		handle: versionedHandle[otellog.Logger]{
			provider: this,
			create: func() otellog.Logger {
				return this.get().Logger(name, options...)
			},
		},
	}
}

type delegatingLogger struct {
	logembedded.Logger
	handle versionedHandle[otellog.Logger]
}

//nolint:gocritic // Respect otellog.Logger interface
func (this *delegatingLogger) Emit(ctx context.Context, record otellog.Record) {
	this.handle.get().Emit(ctx, record)
}

func (this *delegatingLogger) Enabled(ctx context.Context, param otellog.EnabledParameters) bool {
	return this.handle.get().Enabled(ctx, param)
}

type delegatingPropagator struct {
	inner propagation.TextMapPropagator
	mu    sync.RWMutex
}

func newDelegatingPropagator() *delegatingPropagator {
	return &delegatingPropagator{
		inner: propagation.NewCompositeTextMapPropagator(),
	}
}

func (this *delegatingPropagator) set(prop propagation.TextMapPropagator) {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.inner = prop
}

func (this *delegatingPropagator) get() propagation.TextMapPropagator {
	this.mu.RLock()
	defer this.mu.RUnlock()
	return this.inner
}

func (this *delegatingPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	this.get().Inject(ctx, carrier)
}

func (this *delegatingPropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return this.get().Extract(ctx, carrier)
}

func (this *delegatingPropagator) Fields() []string {
	return this.get().Fields()
}

type delegatingTransport struct {
	inner http.RoundTripper
	mu    sync.RWMutex
}

func newDelegatingTransport() *delegatingTransport {
	//nolint:forbidigo // Intentionally share http.DefaultTransport connection pool across pre-init and SDK-init phases
	return &delegatingTransport{
		inner: http.DefaultTransport,
	}
}

func (this *delegatingTransport) set(rt http.RoundTripper) {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.inner = rt
}

func (this *delegatingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	this.mu.RLock()
	rt := this.inner
	this.mu.RUnlock()
	//nolint:wrapcheck // Must preserve http.RoundTripper error semantics
	return rt.RoundTrip(req)
}

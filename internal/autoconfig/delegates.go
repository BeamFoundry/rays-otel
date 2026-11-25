package autoconfig

import (
	"context"
	"net/http"
	"sync"

	otellog "go.opentelemetry.io/otel/log"
	nooplog "go.opentelemetry.io/otel/log/noop"
	otelmetric "go.opentelemetry.io/otel/metric"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
	nooptrace "go.opentelemetry.io/otel/trace/noop"
)

type delegatingTracerProvider struct {
	oteltrace.TracerProvider
	inner oteltrace.TracerProvider
	mu    sync.RWMutex
}

func newDelegatingTracerProvider() *delegatingTracerProvider {
	noop := nooptrace.NewTracerProvider()
	return &delegatingTracerProvider{
		TracerProvider: noop,
		inner:          noop,
	}
}

func (this *delegatingTracerProvider) set(tp oteltrace.TracerProvider) {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.inner = tp
}

func (this *delegatingTracerProvider) get() oteltrace.TracerProvider {
	this.mu.RLock()
	defer this.mu.RUnlock()
	return this.inner
}

func (this *delegatingTracerProvider) Tracer(name string, options ...oteltrace.TracerOption) oteltrace.Tracer {
	return this.get().Tracer(name, options...)
}

type delegatingMeterProvider struct {
	otelmetric.MeterProvider
	inner otelmetric.MeterProvider
	mu    sync.RWMutex
}

func newDelegatingMeterProvider() *delegatingMeterProvider {
	noop := noopmetric.NewMeterProvider()
	return &delegatingMeterProvider{
		MeterProvider: noop,
		inner:         noop,
	}
}

func (this *delegatingMeterProvider) set(mp otelmetric.MeterProvider) {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.inner = mp
}

func (this *delegatingMeterProvider) get() otelmetric.MeterProvider {
	this.mu.RLock()
	defer this.mu.RUnlock()
	return this.inner
}

func (this *delegatingMeterProvider) Meter(name string, options ...otelmetric.MeterOption) otelmetric.Meter {
	return this.get().Meter(name, options...)
}

type delegatingLoggerProvider struct {
	otellog.LoggerProvider
	inner otellog.LoggerProvider
	mu    sync.RWMutex
}

func newDelegatingLoggerProvider() *delegatingLoggerProvider {
	noop := nooplog.NewLoggerProvider()
	return &delegatingLoggerProvider{
		LoggerProvider: noop,
		inner:          noop,
	}
}

func (this *delegatingLoggerProvider) set(lp otellog.LoggerProvider) {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.inner = lp
}

func (this *delegatingLoggerProvider) get() otellog.LoggerProvider {
	this.mu.RLock()
	defer this.mu.RUnlock()
	return this.inner
}

func (this *delegatingLoggerProvider) Logger(name string, options ...otellog.LoggerOption) otellog.Logger {
	return this.get().Logger(name, options...)
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
	return &delegatingTransport{
		inner: &http.Transport{Proxy: http.ProxyFromEnvironment},
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

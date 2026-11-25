package autoconfig

import (
	"context"
	"sync"

	otelmetric "go.opentelemetry.io/otel/metric"
	metricembedded "go.opentelemetry.io/otel/metric/embedded"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
)

type delegatingMeterProvider struct {
	metricembedded.MeterProvider
	inner     otelmetric.MeterProvider
	callbacks []*delegatingRegistration
	version   uint64
	mu        sync.RWMutex
}

func newDelegatingMeterProvider() *delegatingMeterProvider {
	return &delegatingMeterProvider{
		inner:   noopmetric.NewMeterProvider(),
		version: 1,
	}
}

func (this *delegatingMeterProvider) set(mp otelmetric.MeterProvider) {
	if mp == nil {
		mp = noopmetric.NewMeterProvider()
	}
	this.mu.Lock()
	this.inner = mp
	this.version++
	cbs := make([]*delegatingRegistration, len(this.callbacks))
	copy(cbs, this.callbacks)
	this.mu.Unlock()

	for _, reg := range cbs {
		reg.rebind()
	}
}

func (this *delegatingMeterProvider) currentVersion() uint64 {
	this.mu.RLock()
	defer this.mu.RUnlock()
	return this.version
}

func (this *delegatingMeterProvider) get() otelmetric.MeterProvider {
	this.mu.RLock()
	defer this.mu.RUnlock()
	return this.inner
}

func (this *delegatingMeterProvider) Meter(name string, options ...otelmetric.MeterOption) otelmetric.Meter {
	return &delegatingMeter{
		provider: this,
		handle: &versionedHandle[otelmetric.Meter]{
			provider: this,
			create: func() otelmetric.Meter {
				return this.get().Meter(name, options...)
			},
		},
	}
}

type delegatingMeter struct {
	metricembedded.Meter
	provider *delegatingMeterProvider
	handle   *versionedHandle[otelmetric.Meter]
}

func newMeterInstrument[I any](
	m *delegatingMeter,
	create func(otelmetric.Meter) (I, error),
	wrap func(*versionedHandle[I]) I,
) (I, error) {
	ver := m.provider.currentVersion()
	initial, err := create(m.handle.get())
	if err != nil {
		var zero I
		return zero, err
	}
	return wrap(&versionedHandle[I]{
		provider:      m.provider,
		cached:        initial,
		cachedVersion: ver,
		create: func() I {
			inst, instErr := create(m.handle.get())
			if instErr != nil {
				return initial
			}
			return inst
		},
	}), nil
}

func (this *delegatingMeter) Int64Counter(name string, options ...otelmetric.Int64CounterOption) (otelmetric.Int64Counter, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Int64Counter, error) {
			return m.Int64Counter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Int64Counter]) otelmetric.Int64Counter {
			return &delegatingInt64Counter{handle: h}
		},
	)
}

func (this *delegatingMeter) Int64UpDownCounter(name string, options ...otelmetric.Int64UpDownCounterOption) (otelmetric.Int64UpDownCounter, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Int64UpDownCounter, error) {
			return m.Int64UpDownCounter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Int64UpDownCounter]) otelmetric.Int64UpDownCounter {
			return &delegatingInt64UpDownCounter{handle: h}
		},
	)
}

func (this *delegatingMeter) Int64Histogram(name string, options ...otelmetric.Int64HistogramOption) (otelmetric.Int64Histogram, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Int64Histogram, error) {
			return m.Int64Histogram(name, options...)
		},
		func(h *versionedHandle[otelmetric.Int64Histogram]) otelmetric.Int64Histogram {
			return &delegatingInt64Histogram{handle: h}
		},
	)
}

func (this *delegatingMeter) Int64Gauge(name string, options ...otelmetric.Int64GaugeOption) (otelmetric.Int64Gauge, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Int64Gauge, error) {
			return m.Int64Gauge(name, options...)
		},
		func(h *versionedHandle[otelmetric.Int64Gauge]) otelmetric.Int64Gauge {
			return &delegatingInt64Gauge{handle: h}
		},
	)
}

func (this *delegatingMeter) Int64ObservableCounter(name string, options ...otelmetric.Int64ObservableCounterOption) (otelmetric.Int64ObservableCounter, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Int64ObservableCounter, error) {
			return m.Int64ObservableCounter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Int64ObservableCounter]) otelmetric.Int64ObservableCounter {
			return &delegatingInt64ObservableCounter{handle: h}
		},
	)
}

func (this *delegatingMeter) Int64ObservableUpDownCounter(name string, options ...otelmetric.Int64ObservableUpDownCounterOption) (otelmetric.Int64ObservableUpDownCounter, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Int64ObservableUpDownCounter, error) {
			return m.Int64ObservableUpDownCounter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Int64ObservableUpDownCounter]) otelmetric.Int64ObservableUpDownCounter {
			return &delegatingInt64ObservableUpDownCounter{handle: h}
		},
	)
}

func (this *delegatingMeter) Int64ObservableGauge(name string, options ...otelmetric.Int64ObservableGaugeOption) (otelmetric.Int64ObservableGauge, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Int64ObservableGauge, error) {
			return m.Int64ObservableGauge(name, options...)
		},
		func(h *versionedHandle[otelmetric.Int64ObservableGauge]) otelmetric.Int64ObservableGauge {
			return &delegatingInt64ObservableGauge{handle: h}
		},
	)
}

func (this *delegatingMeter) Float64Counter(name string, options ...otelmetric.Float64CounterOption) (otelmetric.Float64Counter, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Float64Counter, error) {
			return m.Float64Counter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Float64Counter]) otelmetric.Float64Counter {
			return &delegatingFloat64Counter{handle: h}
		},
	)
}

func (this *delegatingMeter) Float64UpDownCounter(name string, options ...otelmetric.Float64UpDownCounterOption) (otelmetric.Float64UpDownCounter, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Float64UpDownCounter, error) {
			return m.Float64UpDownCounter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Float64UpDownCounter]) otelmetric.Float64UpDownCounter {
			return &delegatingFloat64UpDownCounter{handle: h}
		},
	)
}

func (this *delegatingMeter) Float64Histogram(name string, options ...otelmetric.Float64HistogramOption) (otelmetric.Float64Histogram, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Float64Histogram, error) {
			return m.Float64Histogram(name, options...)
		},
		func(h *versionedHandle[otelmetric.Float64Histogram]) otelmetric.Float64Histogram {
			return &delegatingFloat64Histogram{handle: h}
		},
	)
}

func (this *delegatingMeter) Float64Gauge(name string, options ...otelmetric.Float64GaugeOption) (otelmetric.Float64Gauge, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Float64Gauge, error) {
			return m.Float64Gauge(name, options...)
		},
		func(h *versionedHandle[otelmetric.Float64Gauge]) otelmetric.Float64Gauge {
			return &delegatingFloat64Gauge{handle: h}
		},
	)
}

func (this *delegatingMeter) Float64ObservableCounter(name string, options ...otelmetric.Float64ObservableCounterOption) (otelmetric.Float64ObservableCounter, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Float64ObservableCounter, error) {
			return m.Float64ObservableCounter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Float64ObservableCounter]) otelmetric.Float64ObservableCounter {
			return &delegatingFloat64ObservableCounter{handle: h}
		},
	)
}

func (this *delegatingMeter) Float64ObservableUpDownCounter(name string, options ...otelmetric.Float64ObservableUpDownCounterOption) (otelmetric.Float64ObservableUpDownCounter, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Float64ObservableUpDownCounter, error) {
			return m.Float64ObservableUpDownCounter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Float64ObservableUpDownCounter]) otelmetric.Float64ObservableUpDownCounter {
			return &delegatingFloat64ObservableUpDownCounter{handle: h}
		},
	)
}

func (this *delegatingMeter) Float64ObservableGauge(name string, options ...otelmetric.Float64ObservableGaugeOption) (otelmetric.Float64ObservableGauge, error) {
	return newMeterInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Float64ObservableGauge, error) {
			return m.Float64ObservableGauge(name, options...)
		},
		func(h *versionedHandle[otelmetric.Float64ObservableGauge]) otelmetric.Float64ObservableGauge {
			return &delegatingFloat64ObservableGauge{handle: h}
		},
	)
}

type unwrappableObservable interface {
	unwrap() otelmetric.Observable
}

func (this *delegatingMeter) RegisterCallback(f otelmetric.Callback, instruments ...otelmetric.Observable) (otelmetric.Registration, error) {
	reg := &delegatingRegistration{
		meter:       this,
		fn:          f,
		instruments: instruments,
	}
	if err := reg.bind(); err != nil {
		return nil, err
	}
	this.provider.mu.Lock()
	this.provider.callbacks = append(this.provider.callbacks, reg)
	this.provider.mu.Unlock()
	return reg, nil
}

type delegatingRegistration struct {
	metricembedded.Registration
	inner        otelmetric.Registration
	meter        *delegatingMeter
	fn           otelmetric.Callback
	instruments  []otelmetric.Observable
	mu           sync.Mutex
	unregistered bool
}

func (this *delegatingRegistration) bind() error {
	resolved := make([]otelmetric.Observable, len(this.instruments))
	for i, inst := range this.instruments {
		if u, ok := inst.(unwrappableObservable); ok {
			resolved[i] = u.unwrap()
		} else {
			resolved[i] = inst
		}
	}
	wrappedFn := func(ctx context.Context, obs otelmetric.Observer) error {
		return this.fn(ctx, &unwrappingObserver{Observer: obs})
	}
	reg, err := this.meter.handle.get().RegisterCallback(wrappedFn, resolved...)
	if err != nil {
		//nolint:wrapcheck // Preserve otelmetric.Meter error semantics
		return err
	}
	this.mu.Lock()
	this.inner = reg
	this.mu.Unlock()
	return nil
}

func (this *delegatingRegistration) rebind() {
	this.mu.Lock()
	if this.unregistered {
		this.mu.Unlock()
		return
	}
	old := this.inner
	this.mu.Unlock()
	if old != nil {
		//nolint:errcheck // Best-effort unregister before rebinding to active MeterProvider
		_ = old.Unregister()
	}
	//nolint:errcheck // Best-effort rebind onto active MeterProvider
	_ = this.bind()
}

func (this *delegatingRegistration) Unregister() error {
	this.mu.Lock()
	this.unregistered = true
	reg := this.inner
	this.inner = nil
	this.mu.Unlock()
	if reg != nil {
		//nolint:wrapcheck // Preserve otelmetric.Registration error semantics
		return reg.Unregister()
	}
	return nil
}

type unwrappingObserver struct {
	otelmetric.Observer
}

func (this *unwrappingObserver) ObserveInt64(obsrv otelmetric.Int64Observable, value int64, options ...otelmetric.ObserveOption) {
	if u, ok := obsrv.(unwrappableObservable); ok {
		if unwrapped, ok := u.unwrap().(otelmetric.Int64Observable); ok {
			obsrv = unwrapped
		}
	}
	this.Observer.ObserveInt64(obsrv, value, options...)
}

func (this *unwrappingObserver) ObserveFloat64(obsrv otelmetric.Float64Observable, value float64, options ...otelmetric.ObserveOption) {
	if u, ok := obsrv.(unwrappableObservable); ok {
		if unwrapped, ok := u.unwrap().(otelmetric.Float64Observable); ok {
			obsrv = unwrapped
		}
	}
	this.Observer.ObserveFloat64(obsrv, value, options...)
}

type delegatingInt64Counter struct {
	metricembedded.Int64Counter
	handle *versionedHandle[otelmetric.Int64Counter]
}

func (this *delegatingInt64Counter) Add(ctx context.Context, incr int64, options ...otelmetric.AddOption) {
	this.handle.get().Add(ctx, incr, options...)
}

func (this *delegatingInt64Counter) Enabled(ctx context.Context) bool {
	return this.handle.get().Enabled(ctx)
}

type delegatingInt64UpDownCounter struct {
	metricembedded.Int64UpDownCounter
	handle *versionedHandle[otelmetric.Int64UpDownCounter]
}

func (this *delegatingInt64UpDownCounter) Add(ctx context.Context, incr int64, options ...otelmetric.AddOption) {
	this.handle.get().Add(ctx, incr, options...)
}

func (this *delegatingInt64UpDownCounter) Enabled(ctx context.Context) bool {
	return this.handle.get().Enabled(ctx)
}

type delegatingInt64Histogram struct {
	metricembedded.Int64Histogram
	handle *versionedHandle[otelmetric.Int64Histogram]
}

func (this *delegatingInt64Histogram) Record(ctx context.Context, incr int64, options ...otelmetric.RecordOption) {
	this.handle.get().Record(ctx, incr, options...)
}

func (this *delegatingInt64Histogram) Enabled(ctx context.Context) bool {
	return this.handle.get().Enabled(ctx)
}

type delegatingInt64Gauge struct {
	metricembedded.Int64Gauge
	handle *versionedHandle[otelmetric.Int64Gauge]
}

func (this *delegatingInt64Gauge) Record(ctx context.Context, value int64, options ...otelmetric.RecordOption) {
	this.handle.get().Record(ctx, value, options...)
}

func (this *delegatingInt64Gauge) Enabled(ctx context.Context) bool {
	return this.handle.get().Enabled(ctx)
}

type delegatingInt64ObservableCounter struct {
	otelmetric.Int64ObservableCounter
	handle *versionedHandle[otelmetric.Int64ObservableCounter]
}

func (this *delegatingInt64ObservableCounter) unwrap() otelmetric.Observable {
	return this.handle.get()
}

type delegatingInt64ObservableUpDownCounter struct {
	otelmetric.Int64ObservableUpDownCounter
	handle *versionedHandle[otelmetric.Int64ObservableUpDownCounter]
}

func (this *delegatingInt64ObservableUpDownCounter) unwrap() otelmetric.Observable {
	return this.handle.get()
}

type delegatingInt64ObservableGauge struct {
	otelmetric.Int64ObservableGauge
	handle *versionedHandle[otelmetric.Int64ObservableGauge]
}

func (this *delegatingInt64ObservableGauge) unwrap() otelmetric.Observable {
	return this.handle.get()
}

type delegatingFloat64Counter struct {
	metricembedded.Float64Counter
	handle *versionedHandle[otelmetric.Float64Counter]
}

func (this *delegatingFloat64Counter) Add(ctx context.Context, incr float64, options ...otelmetric.AddOption) {
	this.handle.get().Add(ctx, incr, options...)
}

func (this *delegatingFloat64Counter) Enabled(ctx context.Context) bool {
	return this.handle.get().Enabled(ctx)
}

type delegatingFloat64UpDownCounter struct {
	metricembedded.Float64UpDownCounter
	handle *versionedHandle[otelmetric.Float64UpDownCounter]
}

func (this *delegatingFloat64UpDownCounter) Add(ctx context.Context, incr float64, options ...otelmetric.AddOption) {
	this.handle.get().Add(ctx, incr, options...)
}

func (this *delegatingFloat64UpDownCounter) Enabled(ctx context.Context) bool {
	return this.handle.get().Enabled(ctx)
}

type delegatingFloat64Histogram struct {
	metricembedded.Float64Histogram
	handle *versionedHandle[otelmetric.Float64Histogram]
}

func (this *delegatingFloat64Histogram) Record(ctx context.Context, incr float64, options ...otelmetric.RecordOption) {
	this.handle.get().Record(ctx, incr, options...)
}

func (this *delegatingFloat64Histogram) Enabled(ctx context.Context) bool {
	return this.handle.get().Enabled(ctx)
}

type delegatingFloat64Gauge struct {
	metricembedded.Float64Gauge
	handle *versionedHandle[otelmetric.Float64Gauge]
}

func (this *delegatingFloat64Gauge) Record(ctx context.Context, value float64, options ...otelmetric.RecordOption) {
	this.handle.get().Record(ctx, value, options...)
}

func (this *delegatingFloat64Gauge) Enabled(ctx context.Context) bool {
	return this.handle.get().Enabled(ctx)
}

type delegatingFloat64ObservableCounter struct {
	otelmetric.Float64ObservableCounter
	handle *versionedHandle[otelmetric.Float64ObservableCounter]
}

func (this *delegatingFloat64ObservableCounter) unwrap() otelmetric.Observable {
	return this.handle.get()
}

type delegatingFloat64ObservableUpDownCounter struct {
	otelmetric.Float64ObservableUpDownCounter
	handle *versionedHandle[otelmetric.Float64ObservableUpDownCounter]
}

func (this *delegatingFloat64ObservableUpDownCounter) unwrap() otelmetric.Observable {
	return this.handle.get()
}

type delegatingFloat64ObservableGauge struct {
	otelmetric.Float64ObservableGauge
	handle *versionedHandle[otelmetric.Float64ObservableGauge]
}

func (this *delegatingFloat64ObservableGauge) unwrap() otelmetric.Observable {
	return this.handle.get()
}

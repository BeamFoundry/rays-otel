package autoconfig

import (
	"context"
	"reflect"
	"slices"
	"sync"

	"go.opentelemetry.io/otel"
	otelmetric "go.opentelemetry.io/otel/metric"
	metricembedded "go.opentelemetry.io/otel/metric/embedded"
	noopmetric "go.opentelemetry.io/otel/metric/noop"
)

type delegatingMeterProvider struct {
	metricembedded.MeterProvider
	inner       otelmetric.MeterProvider
	callbacks   []*delegatingRegistration
	observables []func()
	version     uint64
	mu          sync.RWMutex
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
	obsRebinds := slices.Clone(this.observables)
	cbs := slices.Clone(this.callbacks)
	this.mu.Unlock()

	for _, rebindObs := range obsRebinds {
		rebindObs()
	}
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

func (this *delegatingMeterProvider) snapshot() (mp otelmetric.MeterProvider, version uint64) {
	this.mu.RLock()
	defer this.mu.RUnlock()
	return this.inner, this.version
}

func (this *delegatingMeterProvider) Meter(name string, options ...otelmetric.MeterOption) otelmetric.Meter {
	return &delegatingMeter{
		provider: this,
		name:     name,
		options:  options,
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
	name     string
	options  []otelmetric.MeterOption
}

func (this *delegatingMeter) meterSnapshot() (meter otelmetric.Meter, version uint64) {
	mp, ver := this.provider.snapshot()
	return mp.Meter(this.name, this.options...), ver
}

func isUsableInstrument[I any](inst I) bool {
	v := reflect.ValueOf(&inst).Elem()
	return !v.IsNil()
}

func newMeterInstrumentHandle[I any](
	m *delegatingMeter,
	create func(otelmetric.Meter) (I, error),
) (*versionedHandle[I], error) {
	ver := m.provider.currentVersion()
	initial, err := create(m.handle.get())
	if err != nil {
		return nil, err
	}
	return &versionedHandle[I]{
		provider:      m.provider,
		cached:        initial,
		cachedVersion: ver,
		create: func() I {
			inst, instErr := create(m.handle.get())
			if instErr != nil {
				otel.Handle(instErr)
				if !isUsableInstrument(inst) {
					return initial
				}
			}
			return inst
		},
	}, nil
}

func newMeterInstrument[I any](
	m *delegatingMeter,
	create func(otelmetric.Meter) (I, error),
	wrap func(*versionedHandle[I]) I,
) (I, error) {
	h, err := newMeterInstrumentHandle(m, create)
	if err != nil {
		var zero I
		return zero, err
	}
	return wrap(h), nil
}

func newObservableInstrument[I any](
	m *delegatingMeter,
	create func(otelmetric.Meter) (I, error),
	wrap func(*versionedHandle[I]) I,
) (I, error) {
	h, err := newMeterInstrumentHandle(m, create)
	if err != nil {
		var zero I
		return zero, err
	}
	m.provider.mu.Lock()
	m.provider.observables = append(m.provider.observables, func() {
		_ = h.get()
	})
	m.provider.mu.Unlock()
	return wrap(h), nil
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
	return newObservableInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Int64ObservableCounter, error) {
			return m.Int64ObservableCounter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Int64ObservableCounter]) otelmetric.Int64ObservableCounter {
			return &delegatingInt64ObservableCounter{handle: h}
		},
	)
}

func (this *delegatingMeter) Int64ObservableUpDownCounter(name string, options ...otelmetric.Int64ObservableUpDownCounterOption) (otelmetric.Int64ObservableUpDownCounter, error) {
	return newObservableInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Int64ObservableUpDownCounter, error) {
			return m.Int64ObservableUpDownCounter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Int64ObservableUpDownCounter]) otelmetric.Int64ObservableUpDownCounter {
			return &delegatingInt64ObservableUpDownCounter{handle: h}
		},
	)
}

func (this *delegatingMeter) Int64ObservableGauge(name string, options ...otelmetric.Int64ObservableGaugeOption) (otelmetric.Int64ObservableGauge, error) {
	return newObservableInstrument(this,
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
	return newObservableInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Float64ObservableCounter, error) {
			return m.Float64ObservableCounter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Float64ObservableCounter]) otelmetric.Float64ObservableCounter {
			return &delegatingFloat64ObservableCounter{handle: h}
		},
	)
}

func (this *delegatingMeter) Float64ObservableUpDownCounter(name string, options ...otelmetric.Float64ObservableUpDownCounterOption) (otelmetric.Float64ObservableUpDownCounter, error) {
	return newObservableInstrument(this,
		func(m otelmetric.Meter) (otelmetric.Float64ObservableUpDownCounter, error) {
			return m.Float64ObservableUpDownCounter(name, options...)
		},
		func(h *versionedHandle[otelmetric.Float64ObservableUpDownCounter]) otelmetric.Float64ObservableUpDownCounter {
			return &delegatingFloat64ObservableUpDownCounter{handle: h}
		},
	)
}

func (this *delegatingMeter) Float64ObservableGauge(name string, options ...otelmetric.Float64ObservableGaugeOption) (otelmetric.Float64ObservableGauge, error) {
	return newObservableInstrument(this,
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
		instruments: slices.Clone(instruments),
	}
	for {
		snapMeter, ver := this.meterSnapshot()
		innerReg, err := reg.createBinding(snapMeter)
		if err != nil {
			return nil, err
		}

		this.provider.mu.Lock()
		if this.provider.version == ver {
			reg.mu.Lock()
			reg.inner = innerReg
			reg.mu.Unlock()
			this.provider.callbacks = append(this.provider.callbacks, reg)
			this.provider.mu.Unlock()
			return reg, nil
		}
		this.provider.mu.Unlock()

		// Provider generation changed concurrently; release stale binding and retry.
		//nolint:errcheck // Best-effort cleanup of stale registration before retry
		_ = innerReg.Unregister()
	}
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

func (this *delegatingRegistration) createBinding(meter otelmetric.Meter) (otelmetric.Registration, error) {
	this.mu.Lock()
	fn := this.fn
	instruments := slices.Clone(this.instruments)
	this.mu.Unlock()

	obsMap := make(map[otelmetric.Observable]otelmetric.Observable, len(instruments))
	resolved := make([]otelmetric.Observable, len(instruments))
	for i, inst := range instruments {
		if u, ok := inst.(unwrappableObservable); ok {
			unwrapped := u.unwrap()
			resolved[i] = unwrapped
			obsMap[inst] = unwrapped
		} else {
			resolved[i] = inst
			obsMap[inst] = inst
		}
	}
	wrappedFn := func(ctx context.Context, obs otelmetric.Observer) error {
		return fn(ctx, &unwrappingObserver{Observer: obs, resolved: obsMap})
	}
	reg, err := meter.RegisterCallback(wrappedFn, resolved...)
	if err != nil {
		if reg != nil {
			//nolint:errcheck // Best-effort cleanup of partial registration on error
			_ = reg.Unregister()
		}
		//nolint:wrapcheck // Preserve otelmetric.Meter error semantics
		return nil, err
	}
	return reg, nil
}

func (this *delegatingRegistration) rebind() {
	this.mu.Lock()
	if this.unregistered {
		this.mu.Unlock()
		return
	}
	old := this.inner
	this.inner = nil
	this.mu.Unlock()

	if old != nil {
		//nolint:errcheck // Best-effort unregister before rebinding to active MeterProvider
		_ = old.Unregister()
	}

	newReg, err := this.createBinding(this.meter.handle.get())
	if err != nil {
		otel.Handle(err)
		return
	}

	this.mu.Lock()
	if this.unregistered {
		this.mu.Unlock()
		//nolint:errcheck // Cancellation won concurrently; discard replacement
		_ = newReg.Unregister()
		return
	}
	this.inner = newReg
	this.mu.Unlock()
}

func (this *delegatingRegistration) Unregister() error {
	this.mu.Lock()
	this.unregistered = true
	reg := this.inner
	this.inner = nil
	this.fn = nil
	this.instruments = nil
	meter := this.meter
	this.mu.Unlock()

	if meter != nil && meter.provider != nil {
		meter.provider.mu.Lock()
		meter.provider.callbacks = slices.DeleteFunc(meter.provider.callbacks, func(item *delegatingRegistration) bool {
			return item == this
		})
		meter.provider.mu.Unlock()
	}

	if reg != nil {
		//nolint:wrapcheck // Preserve otelmetric.Registration error semantics
		return reg.Unregister()
	}
	return nil
}

type unwrappingObserver struct {
	otelmetric.Observer
	resolved map[otelmetric.Observable]otelmetric.Observable
}

func (this *unwrappingObserver) ObserveInt64(obsrv otelmetric.Int64Observable, value int64, options ...otelmetric.ObserveOption) {
	if mapped, ok := this.resolved[obsrv]; ok {
		if typed, ok := mapped.(otelmetric.Int64Observable); ok {
			obsrv = typed
		}
	}
	this.Observer.ObserveInt64(obsrv, value, options...)
}

func (this *unwrappingObserver) ObserveFloat64(obsrv otelmetric.Float64Observable, value float64, options ...otelmetric.ObserveOption) {
	if mapped, ok := this.resolved[obsrv]; ok {
		if typed, ok := mapped.(otelmetric.Float64Observable); ok {
			obsrv = typed
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

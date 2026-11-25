package autoconfig

import (
	"context"
	"errors"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	otellog "go.opentelemetry.io/otel/log"
	otelmetric "go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	. "github.com/BeamFoundry/rays-otel/pkg/core"
	. "github.com/BeamFoundry/rays/pkg/core"
	. "github.com/BeamFoundry/rays/pkg/lang"
	. "github.com/BeamFoundry/rays/pkg/testing"
)

var _ = Describe("Delegating Providers and Rollback", func() {
	It("upgrades pre-init Tracer, Logger, and Meter handles/instruments/callbacks when SDK providers are set", func() {
		tpDel := newDelegatingTracerProvider()
		lpDel := newDelegatingLoggerProvider()
		mpDel := newDelegatingMeterProvider()

		// Create handles BEFORE SDK providers are set
		preTracer := tpDel.Tracer("pre")
		preLogger := lpDel.Logger("pre")
		preMeter := mpDel.Meter("pre")

		i64c, err := preMeter.Int64Counter("i64.c")
		Expect(err).NotTo(HaveOccurred())
		i64u, err := preMeter.Int64UpDownCounter("i64.u")
		Expect(err).NotTo(HaveOccurred())
		i64h, err := preMeter.Int64Histogram("i64.h")
		Expect(err).NotTo(HaveOccurred())
		i64g, err := preMeter.Int64Gauge("i64.g")
		Expect(err).NotTo(HaveOccurred())
		i64oc, err := preMeter.Int64ObservableCounter("i64.oc")
		Expect(err).NotTo(HaveOccurred())
		i64ou, err := preMeter.Int64ObservableUpDownCounter("i64.ou")
		Expect(err).NotTo(HaveOccurred())
		i64og, err := preMeter.Int64ObservableGauge("i64.og")
		Expect(err).NotTo(HaveOccurred())

		// Observable with inline callback option created before SDK is active
		i64ogOpt, err := preMeter.Int64ObservableGauge("i64.og.opt",
			otelmetric.WithInt64Callback(func(_ context.Context, obs otelmetric.Int64Observer) error {
				obs.Observe(42)
				return nil
			}),
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(i64ogOpt).NotTo(BeNil())

		f64c, err := preMeter.Float64Counter("f64.c")
		Expect(err).NotTo(HaveOccurred())
		f64u, err := preMeter.Float64UpDownCounter("f64.u")
		Expect(err).NotTo(HaveOccurred())
		f64h, err := preMeter.Float64Histogram("f64.h")
		Expect(err).NotTo(HaveOccurred())
		f64g, err := preMeter.Float64Gauge("f64.g")
		Expect(err).NotTo(HaveOccurred())
		f64oc, err := preMeter.Float64ObservableCounter("f64.oc")
		Expect(err).NotTo(HaveOccurred())
		f64ou, err := preMeter.Float64ObservableUpDownCounter("f64.ou")
		Expect(err).NotTo(HaveOccurred())
		f64og, err := preMeter.Float64ObservableGauge("f64.og")
		Expect(err).NotTo(HaveOccurred())

		reg1, err := preMeter.RegisterCallback(func(_ context.Context, obs otelmetric.Observer) error {
			obs.ObserveInt64(i64oc, 1)
			obs.ObserveInt64(i64ou, 2)
			obs.ObserveInt64(i64og, 3)
			obs.ObserveFloat64(f64oc, 1.5)
			obs.ObserveFloat64(f64ou, 2.5)
			obs.ObserveFloat64(f64og, 3.5)
			return nil
		}, i64oc, i64ou, i64og, f64oc, f64ou, f64og)
		Expect(err).NotTo(HaveOccurred())

		reg2, err := preMeter.RegisterCallback(func(_ context.Context, _ otelmetric.Observer) error {
			return nil
		}, i64oc)
		Expect(err).NotTo(HaveOccurred())
		Expect(reg2.Unregister()).NotTo(HaveOccurred())

		// Exercise noop pre-init state
		ctx := context.Background()
		_, noopSpan := preTracer.Start(ctx, "noop-span")
		noopSpan.End()
		Expect(preLogger.Enabled(ctx, otellog.EnabledParameters{})).To(BeFalse())
		preLogger.Emit(ctx, otellog.Record{})
		Expect(i64c.Enabled(ctx)).To(BeFalse())

		// Now attach real SDK providers
		spanRec := tracetest.NewSpanRecorder()
		sdkTP := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRec))
		DeferCleanup(func() { Expect(sdkTP.Shutdown(ctx)).To(Succeed()) })
		tpDel.set(sdkTP)

		logProc := &recordingLogProcessor{}
		sdkLP := sdklog.NewLoggerProvider(sdklog.WithProcessor(logProc))
		DeferCleanup(func() { Expect(sdkLP.Shutdown(ctx)).To(Succeed()) })
		lpDel.set(sdkLP)

		reader := sdkmetric.NewManualReader()
		sdkMP := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		DeferCleanup(func() { Expect(sdkMP.Shutdown(ctx)).To(Succeed()) })
		mpDel.set(sdkMP)

		// Previously cached handles must now emit to real SDKs
		_, activeSpan := preTracer.Start(ctx, "active-span")
		activeSpan.End()
		Expect(spanRec.Ended()).To(HaveLen(1))

		Expect(preLogger.Enabled(ctx, otellog.EnabledParameters{})).To(BeTrue())
		var rec otellog.Record
		rec.SetSeverityText("INFO")
		preLogger.Emit(ctx, rec)
		Expect(logProc.records).To(HaveLen(1))

		Expect(i64c.Enabled(ctx)).To(BeTrue())
		Expect(i64u.Enabled(ctx)).To(BeTrue())
		Expect(i64h.Enabled(ctx)).To(BeTrue())
		Expect(i64g.Enabled(ctx)).To(BeTrue())
		Expect(f64c.Enabled(ctx)).To(BeTrue())
		Expect(f64u.Enabled(ctx)).To(BeTrue())
		Expect(f64h.Enabled(ctx)).To(BeTrue())
		Expect(f64g.Enabled(ctx)).To(BeTrue())

		i64c.Add(ctx, 1)
		i64u.Add(ctx, 2)
		i64h.Record(ctx, 3)
		i64g.Record(ctx, 4)
		f64c.Add(ctx, 1.5)
		f64u.Add(ctx, 2.5)
		f64h.Record(ctx, 3.5)
		f64g.Record(ctx, 4.5)

		var rm metricdata.ResourceMetrics
		Expect(reader.Collect(ctx, &rm)).To(Succeed())
		Expect(rm.ScopeMetrics).NotTo(BeEmpty())
		Expect(rm.ScopeMetrics[0].Metrics).To(HaveLen(15))

		Expect(reg1.Unregister()).NotTo(HaveOccurred())
		Expect((&delegatingRegistration{}).Unregister()).To(Succeed())

		// Test error branch of newMeterInstrument and newObservableInstrument (empty instrument name)
		_, badErr := preMeter.Int64Counter("")
		Expect(badErr).To(HaveOccurred())
		_, badObsErr := preMeter.Int64ObservableGauge("")
		Expect(badObsErr).To(HaveOccurred())

		// Test non-unwrappable observable + RegisterCallback error + instErr fallback via failingMeterProvider
		noopObs, err := sdkmetric.NewMeterProvider().Meter("raw").Int64ObservableCounter("raw.oc")
		Expect(err).NotTo(HaveOccurred())
		rawReg, err := preMeter.RegisterCallback(func(_ context.Context, _ otelmetric.Observer) error { return nil }, noopObs)
		Expect(err).NotTo(HaveOccurred())
		Expect(rawReg.Unregister()).To(Succeed())

		// 1. Failing meter with nil instrument falls back to initial
		mpDel.set(&failingMeterProvider{MeterProvider: sdkMP})
		i64c.Add(ctx, 1)
		_, cbErr := preMeter.RegisterCallback(func(_ context.Context, _ otelmetric.Observer) error { return nil }, noopObs)
		Expect(cbErr).To(HaveOccurred())

		// 2. Partial-error meter returning usable instrument alongside error caches and uses it
		usableCounter, err := sdkMP.Meter("usable").Int64Counter("usable.c")
		Expect(err).NotTo(HaveOccurred())
		mpDel.set(&partialErrorMeterProvider{MeterProvider: sdkMP, usable: usableCounter})
		i64c.Add(ctx, 7)
		var usableRM metricdata.ResourceMetrics
		Expect(reader.Collect(ctx, &usableRM)).To(Succeed())
		var foundUsable bool
		for _, sm := range usableRM.ScopeMetrics {
			for _, m := range sm.Metrics {
				if m.Name == "usable.c" {
					sum, ok := m.Data.(metricdata.Sum[int64])
					Expect(ok).To(BeTrue())
					Expect(sum.DataPoints).To(HaveLen(1))
					Expect(sum.DataPoints[0].Value).To(Equal(int64(7)))
					foundUsable = true
				}
			}
		}
		Expect(foundUsable).To(BeTrue())

		// 3. RegisterCallback returns error with non-nil registration -> unregisters partial registration
		partialReg := &spyRegistration{}
		mpDel.set(&partialRegErrorMeterProvider{MeterProvider: sdkMP, reg: partialReg})
		_, partialCBErr := preMeter.RegisterCallback(func(_ context.Context, _ otelmetric.Observer) error { return nil }, noopObs)
		Expect(partialCBErr).To(HaveOccurred())
		Expect(partialReg.unregistered).To(BeTrue())

		// 4. Concurrent set during RegisterCallback triggers stale cleanup and retry
		mpDel.set(&concurrentSetMeterProvider{
			MeterProvider: sdkMP,
			onRegister: func() {
				mpDel.set(sdkMP)
			},
		})
		retryReg, retryErr := preMeter.RegisterCallback(func(_ context.Context, _ otelmetric.Observer) error { return nil }, i64oc)
		Expect(retryErr).NotTo(HaveOccurred())
		Expect(retryReg.Unregister()).To(Succeed())

		// 5. Rebind error path + concurrent Unregister winning over rebind commit
		mpDel.set(&failingMeterProvider{MeterProvider: sdkMP})
		rebindReg, err := sdkMP.Meter("rebind").Int64ObservableCounter("rebind.oc")
		Expect(err).NotTo(HaveOccurred())
		_ = rebindReg
		dReg := &delegatingRegistration{
			meter: preMeter.(*delegatingMeter),
			fn:    func(_ context.Context, _ otelmetric.Observer) error { return nil },
		}
		dReg.rebind() // createBinding fails on failingMeterProvider -> otel.Handle(err)

		mpDel.set(sdkMP)
		dReg2 := &delegatingRegistration{
			meter: preMeter.(*delegatingMeter),
			fn:    func(_ context.Context, _ otelmetric.Observer) error { return nil },
		}
		dReg2.unregistered = true
		dReg2.rebind() // early return when already unregistered

		// Simulate Unregister winning during createBinding inside rebind
		racingReg := &spyRegistration{}
		var raceTarget *delegatingRegistration
		mpDel.set(&concurrentSetMeterProvider{
			MeterProvider: sdkMP,
			reg:           racingReg,
			onRegister: func() {
				raceTarget.mu.Lock()
				raceTarget.unregistered = true
				raceTarget.mu.Unlock()
			},
		})
		raceTarget = &delegatingRegistration{
			meter: preMeter.(*delegatingMeter),
			fn:    func(_ context.Context, _ otelmetric.Observer) error { return nil },
		}
		raceTarget.rebind()
		Expect(racingReg.unregistered).To(BeTrue())

		// Verify set(nil) resets back to noop
		tpDel.set(nil)
		lpDel.set(nil)
		mpDel.set(nil)
		Expect(preLogger.Enabled(ctx, otellog.EnabledParameters{})).To(BeFalse())

		// Verify pre-init delegatingTransport uses http.DefaultTransport
		dt := newDelegatingTransport()
		//nolint:forbidigo // Verify delegate shares http.DefaultTransport
		Expect(dt.inner).To(Equal(http.DefaultTransport))
	})

	It("cleans up and clears created providers when a later PostConstruct step fails", func() {
		loadBaseOtelConfig(map[string]any{
			"traces":  map[string]any{"enabled": true, "exporter": "none"},
			"metrics": map[string]any{"enabled": true, "exporter": "none"},
			"logs":    map[string]any{"enabled": true, "exporter": "unknown"},
		})
		spanRec := tracetest.NewSpanRecorder()
		traceProbe := &shutdownProbeSpanProcessor{SpanProcessor: spanRec}
		metricProbe := &shutdownProbeMetricReader{Reader: sdkmetric.NewManualReader()}
		probeCustomizers := &rollbackProbeCustomizers{
			traceProbe:  traceProbe,
			metricProbe: metricProbe,
		}
		handleCache := &preInitHandleCache{}

		_, err := GetContainer().AddBeamFor(probeCustomizers)
		Expect(err).NotTo(HaveOccurred())
		_, err = GetContainer().AddBeamFor(handleCache)
		Expect(err).NotTo(HaveOccurred())

		Expect(GetContainer().Initialize()).To(HaveOccurred())
		Expect(traceProbe.shutDown).To(BeTrue())
		Expect(metricProbe.shutDown).To(BeTrue())

		// Cached handles from before failed Initialize must remain noop and not record
		_, sp := handleCache.tracer.Start(context.Background(), "after-rollback")
		sp.End()
		Expect(spanRec.Ended()).To(BeEmpty())

		handleCache.counter.Add(context.Background(), 1)
		var rm metricdata.ResourceMetrics
		Expect(metricProbe.Reader.Collect(context.Background(), &rm)).To(MatchError(sdkmetric.ErrReaderShutdown))
		Expect(rm.ScopeMetrics).To(BeEmpty())
	})
})

type preInitHandleCache struct {
	Component
	IgnoreScanning
	tp      oteltrace.TracerProvider `@:"Inject"`
	mp      otelmetric.MeterProvider `@:"Inject"`
	tracer  oteltrace.Tracer
	counter otelmetric.Int64Counter
}

func (this *preInitHandleCache) PostConstruct() Error {
	this.tracer = this.tp.Tracer("rollback-probe")
	c, err := this.mp.Meter("rollback-probe").Int64Counter("rollback.counter")
	if err != nil {
		return NewWrappedError("MetricInitError", err)
	}
	this.counter = c
	return nil
}

type shutdownProbeSpanProcessor struct {
	sdktrace.SpanProcessor
	shutDown bool
}

func (this *shutdownProbeSpanProcessor) Shutdown(ctx context.Context) error {
	this.shutDown = true
	//nolint:errcheck // Probe intentionally returns custom shutdown error
	_ = this.SpanProcessor.Shutdown(ctx)
	return errors.New("trace probe shutdown")
}

type shutdownProbeMetricReader struct {
	sdkmetric.Reader
	shutDown bool
}

func (this *shutdownProbeMetricReader) Shutdown(ctx context.Context) error {
	this.shutDown = true
	//nolint:errcheck // Probe intentionally returns custom shutdown error
	_ = this.Reader.Shutdown(ctx)
	return errors.New("metric probe shutdown")
}

type rollbackProbeCustomizers struct {
	Component
	IgnoreScanning
	traceProbe  *shutdownProbeSpanProcessor
	metricProbe *shutdownProbeMetricReader
}

func (this *rollbackProbeCustomizers) CustomizeTracerProvider(opts []sdktrace.TracerProviderOption) []sdktrace.TracerProviderOption {
	return append(opts, sdktrace.WithSpanProcessor(this.traceProbe))
}

func (this *rollbackProbeCustomizers) CustomizeMeterProvider(opts []sdkmetric.Option) []sdkmetric.Option {
	return append(opts, sdkmetric.WithReader(this.metricProbe))
}

var (
	_ OtelTracerProviderCustomizer = &rollbackProbeCustomizers{}
	_ OtelMeterProviderCustomizer  = &rollbackProbeCustomizers{}
)

type failingMeterProvider struct {
	otelmetric.MeterProvider
}

func (this *failingMeterProvider) Meter(name string, opts ...otelmetric.MeterOption) otelmetric.Meter {
	return &failingMeter{Meter: this.MeterProvider.Meter(name, opts...)}
}

type failingMeter struct {
	otelmetric.Meter
}

func (this *failingMeter) Int64Counter(_ string, _ ...otelmetric.Int64CounterOption) (otelmetric.Int64Counter, error) {
	return nil, context.Canceled
}

func (this *failingMeter) RegisterCallback(_ otelmetric.Callback, _ ...otelmetric.Observable) (otelmetric.Registration, error) {
	return nil, context.Canceled
}

type partialErrorMeterProvider struct {
	otelmetric.MeterProvider
	usable otelmetric.Int64Counter
}

func (this *partialErrorMeterProvider) Meter(name string, opts ...otelmetric.MeterOption) otelmetric.Meter {
	return &partialErrorMeter{Meter: this.MeterProvider.Meter(name, opts...), usable: this.usable}
}

type partialErrorMeter struct {
	otelmetric.Meter
	usable otelmetric.Int64Counter
}

func (this *partialErrorMeter) Int64Counter(_ string, _ ...otelmetric.Int64CounterOption) (otelmetric.Int64Counter, error) {
	return this.usable, context.Canceled
}

type spyRegistration struct {
	otelmetric.Registration
	unregistered bool
}

func (this *spyRegistration) Unregister() error {
	this.unregistered = true
	return nil
}

type partialRegErrorMeterProvider struct {
	otelmetric.MeterProvider
	reg *spyRegistration
}

func (this *partialRegErrorMeterProvider) Meter(name string, opts ...otelmetric.MeterOption) otelmetric.Meter {
	return &partialRegErrorMeter{Meter: this.MeterProvider.Meter(name, opts...), reg: this.reg}
}

type partialRegErrorMeter struct {
	otelmetric.Meter
	reg *spyRegistration
}

func (this *partialRegErrorMeter) RegisterCallback(_ otelmetric.Callback, _ ...otelmetric.Observable) (otelmetric.Registration, error) {
	return this.reg, context.Canceled
}

type concurrentSetMeterProvider struct {
	otelmetric.MeterProvider
	reg        *spyRegistration
	onRegister func()
	triggered  bool
}

func (this *concurrentSetMeterProvider) Meter(name string, opts ...otelmetric.MeterOption) otelmetric.Meter {
	return &concurrentSetMeter{Meter: this.MeterProvider.Meter(name, opts...), parent: this}
}

type concurrentSetMeter struct {
	otelmetric.Meter
	parent *concurrentSetMeterProvider
}

func (this *concurrentSetMeter) RegisterCallback(f otelmetric.Callback, instruments ...otelmetric.Observable) (otelmetric.Registration, error) {
	if !this.parent.triggered && this.parent.onRegister != nil {
		this.parent.triggered = true
		this.parent.onRegister()
	}
	if this.parent.reg != nil {
		return this.parent.reg, nil
	}
	//nolint:wrapcheck // Preserve Meter RegisterCallback error semantics
	return this.Meter.RegisterCallback(f, instruments...)
}

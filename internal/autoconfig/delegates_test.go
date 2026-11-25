package autoconfig

import (
	"context"
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
		Expect(rm.ScopeMetrics[0].Metrics).To(HaveLen(14))

		Expect(reg1.Unregister()).NotTo(HaveOccurred())
		Expect((&delegatingRegistration{}).Unregister()).To(Succeed())

		// Test error branch of newMeterInstrument (empty instrument name)
		_, badErr := preMeter.Int64Counter("")
		Expect(badErr).To(HaveOccurred())

		// Test non-unwrappable observable + RegisterCallback error + instErr fallback via failingMeterProvider
		noopObs, err := sdkmetric.NewMeterProvider().Meter("raw").Int64ObservableCounter("raw.oc")
		Expect(err).NotTo(HaveOccurred())
		rawReg, err := preMeter.RegisterCallback(func(_ context.Context, _ otelmetric.Observer) error { return nil }, noopObs)
		Expect(err).NotTo(HaveOccurred())
		Expect(rawReg.Unregister()).To(Succeed())

		mpDel.set(&failingMeterProvider{MeterProvider: sdkMP})
		i64c.Add(ctx, 1) // triggers instErr != nil fallback
		_, cbErr := preMeter.RegisterCallback(func(_ context.Context, _ otelmetric.Observer) error { return nil }, noopObs)
		Expect(cbErr).To(HaveOccurred())

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
		customizers := &testCustomizers{failShutdown: true}
		_, err := GetContainer().AddBeamFor(customizers)
		Expect(err).NotTo(HaveOccurred())
		Expect(GetContainer().Initialize()).To(HaveOccurred())
	})
})

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

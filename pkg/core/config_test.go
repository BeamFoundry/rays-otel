package core_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/BeamFoundry/rays-otel/pkg/core"
	. "github.com/BeamFoundry/rays/pkg/testing"
)

var _ = Describe("OtelProperties", func() {
	It("unmarshals rays.otel configuration and exposes all getters", func() {
		GetContainer().EnableScanning()
		Expect(GetContainer().LoadConfigAt(map[string]any{
			"enabled":            true,
			"serviceName":        "orders-api",
			"serviceVersion":     "2.0.1",
			"environment":        "staging",
			"resourceAttributes": map[string]any{"region": "eu-west-1"},
			"propagators":        []any{"tracecontext", "baggage"},
			"traces": map[string]any{
				"enabled":     true,
				"exporter":    ExporterOTLPGRPC,
				"endpoint":    "otel:4317",
				"insecure":    true,
				"headers":     map[string]any{"auth": "tok"},
				"sampleRatio": 0.5,
			},
			"metrics": map[string]any{
				"enabled":  true,
				"exporter": ExporterOTLPHTTP,
				"endpoint": "otel:4318",
				"insecure": false,
				"headers":  map[string]any{"m": "1"},
				"interval": "15s",
			},
			"logs": map[string]any{
				"enabled":      true,
				"exporter":     ExporterStdout,
				"endpoint":     "otel:4319",
				"insecure":     true,
				"headers":      map[string]any{"l": "2"},
				"bridge":       true,
				"traceContext": true,
			},
			"httpClient": map[string]any{
				"enabled": true,
			},
		}, "rays.otel")).NotTo(HaveOccurred())

		Expect(GetContainer().Initialize()).NotTo(HaveOccurred())

		props := GetBeamDefaultInstance[*OtelProperties]()
		Expect(props.Enabled()).To(BeTrue())
		Expect(props.ServiceName()).To(Equal("orders-api"))
		Expect(props.ServiceVersion()).To(Equal("2.0.1"))
		Expect(props.Environment()).To(Equal("staging"))
		Expect(props.ResourceAttributes()).To(Equal(map[string]string{"region": "eu-west-1"}))
		Expect(props.Propagators()).To(Equal([]string{"tracecontext", "baggage"}))

		Expect(props.Traces().Enabled()).To(BeTrue())
		Expect(props.Traces().Exporter()).To(Equal(ExporterOTLPGRPC))
		Expect(props.Traces().Endpoint()).To(Equal("otel:4317"))
		Expect(props.Traces().Insecure()).To(BeTrue())
		Expect(props.Traces().Headers()).To(Equal(map[string]string{"auth": "tok"}))
		Expect(props.Traces().SampleRatio()).To(Equal(0.5))

		Expect(props.Metrics().Enabled()).To(BeTrue())
		Expect(props.Metrics().Exporter()).To(Equal(ExporterOTLPHTTP))
		Expect(props.Metrics().Endpoint()).To(Equal("otel:4318"))
		Expect(props.Metrics().Insecure()).To(BeFalse())
		Expect(props.Metrics().Headers()).To(Equal(map[string]string{"m": "1"}))
		Expect(props.Metrics().Interval()).To(Equal(15 * time.Second))

		Expect(props.Logs().Enabled()).To(BeTrue())
		Expect(props.Logs().Exporter()).To(Equal(ExporterStdout))
		Expect(props.Logs().Endpoint()).To(Equal("otel:4319"))
		Expect(props.Logs().Insecure()).To(BeTrue())
		Expect(props.Logs().Headers()).To(Equal(map[string]string{"l": "2"}))
		Expect(props.Logs().Bridge()).To(BeTrue())
		Expect(props.Logs().TraceContext()).To(BeTrue())

		Expect(props.HTTPClient().Enabled()).To(BeTrue())
	})
})

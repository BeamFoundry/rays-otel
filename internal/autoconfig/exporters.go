package autoconfig

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	. "github.com/BeamFoundry/rays-otel/pkg/core"
	. "github.com/BeamFoundry/rays/pkg/lang"
)

func buildExporter[T any](fn func() (T, error)) (T, Error) {
	exp, err := fn()
	if err != nil {
		var zero T
		return zero, NewWrappedError("OtelExporterError", err)
	}
	return exp, nil
}

func buildTraceExporter(ctx context.Context, props *OtelSignalProperties) (sdktrace.SpanExporter, Error) {
	switch props.Exporter() {
	case "", ExporterNone:
		//nolint:nilnil // ExporterNone intentionally produces no exporter
		return nil, nil
	case ExporterStdout:
		return buildExporter(func() (sdktrace.SpanExporter, error) {
			return stdouttrace.New()
		})
	case ExporterOTLPGRPC:
		var expOpts []otlptracegrpc.Option
		if props.Endpoint() != "" {
			expOpts = append(expOpts, otlptracegrpc.WithEndpoint(props.Endpoint()))
		}
		if props.Insecure() {
			expOpts = append(expOpts, otlptracegrpc.WithInsecure())
		}
		if len(props.Headers()) > 0 {
			expOpts = append(expOpts, otlptracegrpc.WithHeaders(maps.Clone(props.Headers())))
		}
		return buildExporter(func() (sdktrace.SpanExporter, error) {
			return otlptracegrpc.New(ctx, expOpts...)
		})
	case ExporterOTLPHTTP:
		var expOpts []otlptracehttp.Option
		if props.Endpoint() != "" {
			expOpts = append(expOpts, otlptracehttp.WithEndpoint(props.Endpoint()))
		}
		if props.Insecure() {
			expOpts = append(expOpts, otlptracehttp.WithInsecure())
		}
		if len(props.Headers()) > 0 {
			expOpts = append(expOpts, otlptracehttp.WithHeaders(maps.Clone(props.Headers())))
		}
		return buildExporter(func() (sdktrace.SpanExporter, error) {
			return otlptracehttp.New(ctx, expOpts...)
		})
	default:
		return nil, NewError("OtelConfigError", fmt.Sprintf("unknown trace exporter: %q", props.Exporter()))
	}
}

func buildMetricExporter(ctx context.Context, props *OtelMetricProperties) (sdkmetric.Exporter, Error) {
	switch props.Exporter() {
	case "", ExporterNone:
		//nolint:nilnil // ExporterNone intentionally produces no exporter
		return nil, nil
	case ExporterStdout:
		return buildExporter(func() (sdkmetric.Exporter, error) {
			return stdoutmetric.New()
		})
	case ExporterOTLPGRPC:
		var expOpts []otlpmetricgrpc.Option
		if props.Endpoint() != "" {
			expOpts = append(expOpts, otlpmetricgrpc.WithEndpoint(props.Endpoint()))
		}
		if props.Insecure() {
			expOpts = append(expOpts, otlpmetricgrpc.WithInsecure())
		}
		if len(props.Headers()) > 0 {
			expOpts = append(expOpts, otlpmetricgrpc.WithHeaders(maps.Clone(props.Headers())))
		}
		return buildExporter(func() (sdkmetric.Exporter, error) {
			return otlpmetricgrpc.New(ctx, expOpts...)
		})
	case ExporterOTLPHTTP:
		var expOpts []otlpmetrichttp.Option
		if props.Endpoint() != "" {
			expOpts = append(expOpts, otlpmetrichttp.WithEndpoint(props.Endpoint()))
		}
		if props.Insecure() {
			expOpts = append(expOpts, otlpmetrichttp.WithInsecure())
		}
		if len(props.Headers()) > 0 {
			expOpts = append(expOpts, otlpmetrichttp.WithHeaders(maps.Clone(props.Headers())))
		}
		return buildExporter(func() (sdkmetric.Exporter, error) {
			return otlpmetrichttp.New(ctx, expOpts...)
		})
	default:
		return nil, NewError("OtelConfigError", fmt.Sprintf("unknown metric exporter: %q", props.Exporter()))
	}
}

func buildLogExporter(ctx context.Context, props *OtelLogProperties) (sdklog.Exporter, Error) {
	switch props.Exporter() {
	case "", ExporterNone:
		//nolint:nilnil // ExporterNone intentionally produces no exporter
		return nil, nil
	case ExporterStdout:
		return buildExporter(func() (sdklog.Exporter, error) {
			return stdoutlog.New()
		})
	case ExporterOTLPGRPC:
		var expOpts []otlploggrpc.Option
		if props.Endpoint() != "" {
			expOpts = append(expOpts, otlploggrpc.WithEndpoint(props.Endpoint()))
		}
		if props.Insecure() {
			expOpts = append(expOpts, otlploggrpc.WithInsecure())
		}
		if len(props.Headers()) > 0 {
			expOpts = append(expOpts, otlploggrpc.WithHeaders(maps.Clone(props.Headers())))
		}
		return buildExporter(func() (sdklog.Exporter, error) {
			return otlploggrpc.New(ctx, expOpts...)
		})
	case ExporterOTLPHTTP:
		var expOpts []otlploghttp.Option
		if props.Endpoint() != "" {
			expOpts = append(expOpts, otlploghttp.WithEndpoint(props.Endpoint()))
		}
		if props.Insecure() {
			expOpts = append(expOpts, otlploghttp.WithInsecure())
		}
		if len(props.Headers()) > 0 {
			expOpts = append(expOpts, otlploghttp.WithHeaders(maps.Clone(props.Headers())))
		}
		return buildExporter(func() (sdklog.Exporter, error) {
			return otlploghttp.New(ctx, expOpts...)
		})
	default:
		return nil, NewError("OtelConfigError", fmt.Sprintf("unknown log exporter: %q", props.Exporter()))
	}
}

type wrappedPropagator struct {
	propagation.TextMapPropagator
}

func buildPropagator(names []string) (propagation.TextMapPropagator, Error) {
	if len(names) == 0 {
		return &wrappedPropagator{
			TextMapPropagator: propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			),
		}, nil
	}
	list := make([]propagation.TextMapPropagator, 0, len(names))
	for _, name := range slices.Compact(slices.Clone(names)) {
		switch name {
		case "tracecontext":
			list = append(list, propagation.TraceContext{})
		case "baggage":
			list = append(list, propagation.Baggage{})
		default:
			return nil, NewError("OtelConfigError", fmt.Sprintf("unknown propagator: %q", name))
		}
	}
	return &wrappedPropagator{
		TextMapPropagator: propagation.NewCompositeTextMapPropagator(list...),
	}, nil
}

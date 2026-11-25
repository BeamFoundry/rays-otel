package core

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"go.opentelemetry.io/otel/trace"

	. "github.com/BeamFoundry/rays/pkg/core"
)

// TraceContextHandler decorates an [slog.Handler] by resolving an active OpenTelemetry
// [trace.SpanContext] from ctx (or falling back to [TransactionManager.GetContext] when
// available) and optionally injecting "trace_id" and "span_id" attributes.
type TraceContextHandler struct {
	next        slog.Handler
	txManager   TransactionManager
	injectAttrs bool
}

// NewTraceContextHandler creates a [TraceContextHandler] wrapping next.
// When txManager is non-nil and ctx carries no valid span, txManager.GetContext() is checked.
// When injectAttrs is true, valid trace_id and span_id attributes are appended to each record.
func NewTraceContextHandler(next slog.Handler, txManager TransactionManager, injectAttrs bool) *TraceContextHandler {
	return &TraceContextHandler{
		next:        next,
		txManager:   txManager,
		injectAttrs: injectAttrs,
	}
}

// Enabled reports whether the underlying handler emits records at the given level.
func (this *TraceContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return this.next.Enabled(ctx, level)
}

// Handle resolves the active span context, optionally adds trace_id and span_id attributes,
// and forwards the record to the underlying handler with the resolved context.
//
//nolint:contextcheck,gocritic,wrapcheck // Falls back to TransactionManager context and respects slog.Handler interface
func (this *TraceContextHandler) Handle(ctx context.Context, r slog.Record) error {
	resolvedCtx := ctx
	sc := trace.SpanContextFromContext(resolvedCtx)
	if !sc.IsValid() && this.txManager != nil {
		if txCtx := this.txManager.GetContext(); txCtx != nil {
			if txSC := trace.SpanContextFromContext(txCtx); txSC.IsValid() {
				resolvedCtx = txCtx
				sc = txSC
			}
		}
	}

	if this.injectAttrs && sc.IsValid() {
		r = r.Clone()
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}

	return this.next.Handle(resolvedCtx, r)
}

// WithAttrs returns a new [TraceContextHandler] with the given attributes added to the underlying handler.
func (this *TraceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceContextHandler{
		next:        this.next.WithAttrs(attrs),
		txManager:   this.txManager,
		injectAttrs: this.injectAttrs,
	}
}

// WithGroup returns a new [TraceContextHandler] with the given group added to the underlying handler.
func (this *TraceContextHandler) WithGroup(name string) slog.Handler {
	return &TraceContextHandler{
		next:        this.next.WithGroup(name),
		txManager:   this.txManager,
		injectAttrs: this.injectAttrs,
	}
}

// MultiHandler fans out log records across multiple [slog.Handler] instances.
type MultiHandler struct {
	handlers []slog.Handler
}

// NewMultiHandler creates a [MultiHandler] delegating to all non-nil handlers.
func NewMultiHandler(handlers ...slog.Handler) *MultiHandler {
	filtered := make([]slog.Handler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			filtered = append(filtered, h)
		}
	}
	return &MultiHandler{handlers: filtered}
}

// Enabled reports true if any child handler is enabled for the given level.
func (this *MultiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range this.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle forwards the log record to every child handler that is enabled for r.Level.
//
//nolint:gocritic // Have to respect slog.Handler interface
func (this *MultiHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range this.handlers {
		if h.Enabled(ctx, r.Level) {
			if err := h.Handle(ctx, r.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// WithAttrs returns a new [MultiHandler] with attrs applied to every child handler.
func (this *MultiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(this.handlers))
	for i, h := range this.handlers {
		next[i] = h.WithAttrs(slices.Clone(attrs))
	}
	return &MultiHandler{handlers: next}
}

// WithGroup returns a new [MultiHandler] with name applied to every child handler.
func (this *MultiHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(this.handlers))
	for i, h := range this.handlers {
		next[i] = h.WithGroup(name)
	}
	return &MultiHandler{handlers: next}
}

package core_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	. "github.com/BeamFoundry/rays-otel/pkg/core"
	"github.com/BeamFoundry/rays/pkg/tx"
)

type failingHandler struct {
	err     error
	enabled bool
}

func (this *failingHandler) Enabled(_ context.Context, _ slog.Level) bool {
	return this.enabled
}

//nolint:gocritic // Respect slog.Handler interface
func (this *failingHandler) Handle(_ context.Context, _ slog.Record) error {
	return this.err
}

func (this *failingHandler) WithAttrs(_ []slog.Attr) slog.Handler {
	return this
}

func (this *failingHandler) WithGroup(_ string) slog.Handler {
	return this
}

type ctxCapturingHandler struct {
	inner       slog.Handler
	capturedCtx context.Context
}

func (this *ctxCapturingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return this.inner.Enabled(ctx, level)
}

//nolint:gocritic,wrapcheck // Respect slog.Handler interface
func (this *ctxCapturingHandler) Handle(ctx context.Context, r slog.Record) error {
	this.capturedCtx = ctx
	return this.inner.Handle(ctx, r)
}

func (this *ctxCapturingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &ctxCapturingHandler{inner: this.inner.WithAttrs(attrs)}
}

func (this *ctxCapturingHandler) WithGroup(name string) slog.Handler {
	return &ctxCapturingHandler{inner: this.inner.WithGroup(name)}
}

var _ = Describe("Logging Handlers", func() {
	Describe("TraceContextHandler", func() {
		It("injects trace_id and span_id from ctx and falls back to TransactionManager", func() {
			var buf bytes.Buffer
			spy := &ctxCapturingHandler{inner: slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})}
			txMgr := tx.DefaultTransactionManager{}.New()

			h := NewTraceContextHandler(spy, txMgr, true)
			Expect(h.Enabled(context.Background(), slog.LevelInfo)).To(BeTrue())
			Expect(h.Enabled(context.Background(), slog.LevelDebug-1)).To(BeFalse())

			// 1. No span anywhere
			rec := slog.NewRecord(time.Now(), slog.LevelInfo, "no-span", 0)
			Expect(h.Handle(context.Background(), rec)).NotTo(HaveOccurred())
			Expect(buf.String()).NotTo(ContainSubstring("trace_id="))

			// 2. Span in explicit ctx
			tp := sdktrace.NewTracerProvider()
			defer func() { Expect(tp.Shutdown(context.Background())).NotTo(HaveOccurred()) }()
			spanCtx, span := tp.Tracer("test").Start(context.Background(), "op")
			defer span.End()

			buf.Reset()
			Expect(h.Handle(spanCtx, slog.NewRecord(time.Now(), slog.LevelInfo, "with-ctx", 0))).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("trace_id=" + span.SpanContext().TraceID().String()))
			Expect(buf.String()).To(ContainSubstring("span_id=" + span.SpanContext().SpanID().String()))

			// 3. Fallback to TransactionManager.GetContext()
			txMgr.SetContext(spanCtx)
			defer txMgr.ClearContext()

			buf.Reset()
			Expect(h.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "from-tx", 0))).NotTo(HaveOccurred())
			Expect(buf.String()).To(ContainSubstring("trace_id=" + span.SpanContext().TraceID().String()))
			Expect(spy.capturedCtx).To(Equal(spanCtx))

			// 4. WithAttrs and WithGroup without injecting attrs
			noInject := NewTraceContextHandler(spy, txMgr, false).
				WithAttrs([]slog.Attr{slog.String("k", "v")}).
				WithGroup("grp")
			buf.Reset()
			Expect(noInject.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "no-inject", 0))).NotTo(HaveOccurred())
			Expect(buf.String()).NotTo(ContainSubstring("trace_id="))
			Expect(buf.String()).To(ContainSubstring("k=v"))
		})
	})

	Describe("MultiHandler", func() {
		It("fans out to enabled non-nil handlers, preserves WithAttrs/WithGroup, and joins errors", func() {
			var buf1, buf2 bytes.Buffer
			h1 := slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelInfo})
			h2 := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelWarn})
			disabled := &failingHandler{enabled: false, err: errors.New("should-not-run")}
			failing := &failingHandler{enabled: true, err: errors.New("boom")}

			mh := NewMultiHandler(nil, h1, h2, disabled)
			Expect(mh.Enabled(context.Background(), slog.LevelInfo)).To(BeTrue())
			Expect(mh.Enabled(context.Background(), slog.LevelDebug)).To(BeFalse())

			grouped := mh.WithAttrs([]slog.Attr{slog.String("a", "1")}).WithGroup("g")
			Expect(grouped.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "info-only", 0))).NotTo(HaveOccurred())
			Expect(buf1.String()).To(ContainSubstring("info-only"))
			Expect(buf2.String()).To(BeEmpty())

			withErr := NewMultiHandler(h1, failing)
			err := withErr.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelError, "err-msg", 0))
			Expect(err).To(MatchError("boom"))
		})

		It("clones attrs per child handler so mutations do not affect other children", func() {
			var buf1, buf2 bytes.Buffer
			mutating := &mutatingAttrsHandler{
				inner: slog.NewTextHandler(&buf1, &slog.HandlerOptions{Level: slog.LevelInfo}),
			}
			observer := slog.NewTextHandler(&buf2, &slog.HandlerOptions{Level: slog.LevelInfo})

			attrs := []slog.Attr{slog.String("k", "original")}
			mh := NewMultiHandler(mutating, observer).WithAttrs(attrs)

			Expect(attrs[0].Value.String()).To(Equal("original"))
			Expect(mh.Handle(context.Background(), slog.NewRecord(time.Now(), slog.LevelInfo, "msg", 0))).NotTo(HaveOccurred())
			Expect(buf1.String()).To(ContainSubstring("k=mutated"))
			Expect(buf2.String()).To(ContainSubstring("k=original"))
		})
	})
})

type mutatingAttrsHandler struct {
	inner slog.Handler
}

func (this *mutatingAttrsHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return this.inner.Enabled(ctx, level)
}

//nolint:gocritic,wrapcheck // Respect slog.Handler interface
func (this *mutatingAttrsHandler) Handle(ctx context.Context, r slog.Record) error {
	return this.inner.Handle(ctx, r)
}

func (this *mutatingAttrsHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) > 0 {
		attrs[0] = slog.String(attrs[0].Key, "mutated")
	}
	return &mutatingAttrsHandler{inner: this.inner.WithAttrs(attrs)}
}

func (this *mutatingAttrsHandler) WithGroup(name string) slog.Handler {
	return &mutatingAttrsHandler{inner: this.inner.WithGroup(name)}
}

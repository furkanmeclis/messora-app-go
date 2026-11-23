// Package tracing sets up OpenTelemetry tracing. It is off unless an OTLP
// endpoint is configured with the standard environment variables
// (OTEL_EXPORTER_OTLP_ENDPOINT or OTEL_EXPORTER_OTLP_TRACES_ENDPOINT, plus
// OTEL_EXPORTER_OTLP_HEADERS, OTEL_TRACES_SAMPLER, OTEL_TRACES_SAMPLER_ARG …).
package tracing

import (
	"context"
	"log/slog"
	"os"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Name is the instrumentation scope.
const Name = "github.com/furkanmeclis/messora-app-go"

// Enabled reports whether an OTLP endpoint is configured.
func Enabled() bool {
	return os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
}

// Setup installs the global tracer provider for a process role and returns
// its shutdown function (flushes pending spans). Without an endpoint it
// installs nothing and tracing calls are no-ops.
func Setup(ctx context.Context, service, role, version string) (func(context.Context) error, error) {
	if !Enabled() {
		return func(context.Context) error { return nil }, nil
	}
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName(service),
		semconv.ServiceVersion(version),
		attribute.String("messora.role", role),
	))
	if err != nil {
		return nil, err
	}
	// The sampler honours OTEL_TRACES_SAMPLER / OTEL_TRACES_SAMPLER_ARG.
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp.Shutdown, nil
}

// Tracer returns Messora's tracer.
func Tracer() trace.Tracer { return otel.Tracer(Name) }

// Start begins a span (a no-op without a provider).
func Start(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return Tracer().Start(ctx, name, trace.WithAttributes(attrs...))
}

// End records err on the span and ends it.
func End(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.End()
}

// PgxTracer records one span per query. Statements are not recorded: they
// may contain personal data in literals, and the operation name says enough.
type PgxTracer struct{}

type spanKey struct{}

// TraceQueryStart implements pgx.QueryTracer.
func (PgxTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	if !trace.SpanFromContext(ctx).SpanContext().IsValid() {
		return ctx // only as a child of a request or task span
	}
	name := "db.query"
	if op := operation(d.SQL); op != "" {
		name = "db." + op
	}
	ctx, span := Tracer().Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("db.system", "postgresql")))
	return context.WithValue(ctx, spanKey{}, span)
}

// TraceQueryEnd implements pgx.QueryTracer.
func (PgxTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryEndData) {
	span, ok := ctx.Value(spanKey{}).(trace.Span)
	if !ok {
		return
	}
	span.SetAttributes(attribute.Int64("db.rows_affected", d.CommandTag.RowsAffected()))
	End(span, d.Err)
}

// operation extracts sqlc's query name ("-- name: GetContact :one") or
// the leading SQL keyword.
func operation(sql string) string {
	const marker = "-- name: "
	for i := 0; i+len(marker) < len(sql) && i < 200; i++ {
		if sql[i:i+len(marker)] == marker {
			rest := sql[i+len(marker):]
			for j := 0; j < len(rest); j++ {
				if rest[j] == ' ' || rest[j] == '\n' {
					return rest[:j]
				}
			}
		}
	}
	for i := 0; i < len(sql); i++ {
		if c := sql[i]; c != ' ' && c != '\n' && c != '\t' {
			j := i
			for j < len(sql) && sql[j] >= 'A' && sql[j] <= 'z' && sql[j] != '_' {
				j++
			}
			if j > i {
				return sql[i:j]
			}
			return ""
		}
	}
	return ""
}

// LogHandler adds trace_id and span_id to log records of traced contexts.
type LogHandler struct{ slog.Handler }

// Handle implements slog.Handler.
func (h LogHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanFromContext(ctx).SpanContext(); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs implements slog.Handler.
func (h LogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return LogHandler{h.Handler.WithAttrs(attrs)}
}

// WithGroup implements slog.Handler.
func (h LogHandler) WithGroup(name string) slog.Handler { return LogHandler{h.Handler.WithGroup(name)} }

// TaskMiddleware records one span per queue task.
func TaskMiddleware(next asynq.Handler) asynq.Handler {
	return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
		if !Enabled() {
			return next.ProcessTask(ctx, t)
		}
		ctx, span := Tracer().Start(ctx, "task "+t.Type(), trace.WithSpanKind(trace.SpanKindConsumer),
			trace.WithAttributes(attribute.String("messaging.system", "asynq"), attribute.String("messora.task_type", t.Type())))
		err := next.ProcessTask(ctx, t)
		End(span, err)
		return err
	})
}

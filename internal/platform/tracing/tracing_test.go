package tracing

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func recorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	rec := tracetest.NewSpanRecorder()
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return rec
}

func TestDisabledWithoutEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "")
	if Enabled() {
		t.Fatal("enabled without endpoint")
	}
	shutdown, err := Setup(context.Background(), "messora", "api", "test")
	if err != nil || shutdown(context.Background()) != nil {
		t.Fatal("no-op setup failed")
	}
}

func TestTaskAndQuerySpans(t *testing.T) {
	rec := recorder(t)
	h := TaskMiddleware(asynq.HandlerFunc(func(ctx context.Context, _ *asynq.Task) error {
		var tr PgxTracer
		qctx := tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "-- name: GetContact :one\nSELECT * FROM contacts WHERE id = $1"})
		tr.TraceQueryEnd(qctx, nil, pgx.TraceQueryEndData{CommandTag: pgconn.NewCommandTag("SELECT 1")})
		qctx = tr.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: "UPDATE messages SET status = 'x'"})
		tr.TraceQueryEnd(qctx, nil, pgx.TraceQueryEndData{Err: errors.New("boom")})
		return errors.New("task failed")
	}))
	if err := h.ProcessTask(context.Background(), asynq.NewTask("whatsapp:send", nil)); err == nil {
		t.Fatal("error swallowed")
	}
	spans := rec.Ended()
	if len(spans) != 3 {
		t.Fatalf("spans: %d", len(spans))
	}
	names := map[string]sdktrace.ReadOnlySpan{}
	for _, s := range spans {
		names[s.Name()] = s
	}
	task, ok := names["task whatsapp:send"]
	if !ok || task.Status().Code != codes.Error {
		t.Fatalf("task span: %v", names)
	}
	q, ok := names["db.GetContact"]
	if !ok || q.Parent().SpanID() != task.SpanContext().SpanID() {
		t.Fatal("query span not a child of the task")
	}
	for _, a := range q.Attributes() {
		if strings.Contains(string(a.Key), "statement") {
			t.Fatal("SQL statement recorded")
		}
	}
	if u := names["db.UPDATE"]; u == nil || u.Status().Code != codes.Error {
		t.Fatal("failed query not marked")
	}
}

func TestQueriesOutsideSpansAreNotTraced(t *testing.T) {
	rec := recorder(t)
	var tr PgxTracer
	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	if len(rec.Ended()) != 0 {
		t.Fatal("background query traced")
	}
}

func TestLogHandlerAddsTraceID(t *testing.T) {
	recorder(t)
	var buf bytes.Buffer
	log := slog.New(LogHandler{Handler: slog.NewJSONHandler(&buf, nil)})
	ctx, span := Start(context.Background(), "op")
	log.InfoContext(ctx, "hello")
	span.End()
	if !strings.Contains(buf.String(), `"trace_id":"`+span.SpanContext().TraceID().String()+`"`) {
		t.Fatalf("log: %s", buf.String())
	}
	buf.Reset()
	log.Info("no span")
	if strings.Contains(buf.String(), "trace_id") {
		t.Fatal("trace id without a span")
	}
}

func TestOperation(t *testing.T) {
	cases := map[string]string{
		"-- name: ListContacts :many\nSELECT 1":  "ListContacts",
		"  SELECT c.* FROM contacts c":           "SELECT",
		"\nINSERT INTO outbox_events VALUES ($1)": "INSERT",
		"":                                        "",
	}
	for sql, want := range cases {
		if got := operation(sql); got != want {
			t.Errorf("%q → %q, want %q", sql, got, want)
		}
	}
}

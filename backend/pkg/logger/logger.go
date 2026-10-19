// Package logger wires slog and tracing to OpenTelemetry. Traces and logs are
// exported over OTLP/HTTP to a collector, which writes them to ClickHouse.
package logger

import (
	"context"
	"errors"
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Init configures the global tracer provider and the default slog logger.
// endpoint is host:port of the OTLP/HTTP receiver (e.g. localhost:4318).
// The returned function flushes and shuts down the exporters.
func Init(ctx context.Context, serviceName, endpoint string) (func(context.Context) error, error) {
	return InitWith(ctx, serviceName, Options{Endpoint: endpoint})
}

// Options configures InitWith. The zero value (plus an Endpoint) is the local
// collector: plain HTTP, no extra headers.
type Options struct {
	Endpoint string            // host:port of the OTLP/HTTP receiver
	Secure   bool              // use TLS (a hosted backend such as Axiom)
	Headers  map[string]string // sent on every export, e.g. Authorization
}

// InitWith is Init with transport options, for exporting to a hosted backend.
func InitWith(ctx context.Context, serviceName string, o Options) (func(context.Context) error, error) {
	traceOpts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(o.Endpoint)}
	logOpts := []otlploghttp.Option{otlploghttp.WithEndpoint(o.Endpoint)}
	if !o.Secure {
		traceOpts = append(traceOpts, otlptracehttp.WithInsecure())
		logOpts = append(logOpts, otlploghttp.WithInsecure())
	}
	if len(o.Headers) > 0 {
		traceOpts = append(traceOpts, otlptracehttp.WithHeaders(o.Headers))
		logOpts = append(logOpts, otlploghttp.WithHeaders(o.Headers))
	}

	// Schemaless on purpose: a resource carrying its own schema URL fails to
	// merge with resource.Default() whenever the SDK moves to a newer semconv
	// version ("conflicting Schema URL"), which would stop every service at boot.
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(semconv.ServiceName(serviceName)))
	if err != nil {
		return nil, err
	}

	traceExp, err := otlptracehttp.New(ctx, traceOpts...)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(traceExp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)

	logExp, err := otlploghttp.New(ctx, logOpts...)
	if err != nil {
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)), sdklog.WithResource(res))

	// Fan out: console output plus OTLP export.
	slog.SetDefault(slog.New(fanout{
		slog.NewTextHandler(os.Stdout, nil),
		otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(lp)),
	}))

	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), lp.Shutdown(ctx))
	}, nil
}

type fanout []slog.Handler

func (f fanout) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f {
		if h.Enabled(ctx, r.Level) {
			errs = append(errs, h.Handle(ctx, r.Clone()))
		}
	}
	return errors.Join(errs...)
}

func (f fanout) WithAttrs(a []slog.Attr) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithAttrs(a)
	}
	return out
}

func (f fanout) WithGroup(n string) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithGroup(n)
	}
	return out
}

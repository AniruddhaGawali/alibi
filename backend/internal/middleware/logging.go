package middleware

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Logging wraps a handler, opening a span per request and logging the
// request (method, path, IP) and response (status, latency).
func Logging(next http.Handler) http.Handler {
	tracer := otel.Tracer("alibi/http")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
		defer span.End()

		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		span.SetAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.path", r.URL.Path),
			attribute.String("client.ip", ip),
		)
		slog.InfoContext(ctx, "request received", "method", r.Method, "path", r.URL.Path, "ip", ip)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r.WithContext(ctx))
		latency := time.Since(start)

		span.SetAttributes(
			attribute.Int("http.status_code", rec.status),
			attribute.Int64("http.latency_ms", latency.Milliseconds()),
		)
		if rec.status >= 500 {
			span.SetStatus(codes.Error, http.StatusText(rec.status))
		}
		slog.InfoContext(ctx, "request completed", "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "latency_ms", latency.Milliseconds())
	})
}

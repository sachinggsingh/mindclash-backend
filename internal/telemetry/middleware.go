package telemetry

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/trace"

	"go.uber.org/zap"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func RequestLogger(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()

		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(rw, r)

		latency := time.Since(start)

		span := trace.SpanFromContext(r.Context())

		traceID := span.SpanContext().TraceID().String()

		Logger.Info(
			"HTTP Request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", rw.statusCode),
			zap.Duration("latency", latency),
			zap.String("client_ip", r.RemoteAddr),
			zap.String("trace_id", traceID),
			zap.String("user_agent", r.UserAgent()),
		)
	})
}

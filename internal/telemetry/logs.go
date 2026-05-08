package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var Logger *zap.Logger

func InitLogger() {
	logger, err := zap.NewProduction()

	if err != nil {
		panic(err)
	}

	Logger = logger
}

func SyncLogger() error {
	err := Logger.Sync()
	if err != nil {
		return err
	}
	return nil
}

// Logger with Trace ID
func LogWithTrace(ctx context.Context) *zap.Logger {

	span := trace.SpanFromContext(ctx)

	traceID := span.SpanContext().TraceID().String()

	return Logger.With(
		zap.String("trace_id", traceID),
	)
}

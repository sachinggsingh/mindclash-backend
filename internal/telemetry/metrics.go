package telemetry

import (
	"context"
	"log"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"

	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"go.opentelemetry.io/otel/sdk/resource"

	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

var Meter = otel.Meter("quiz-backend")

var (
	UserRegistrationCounter, _ = Meter.Int64Counter("user_registrations_total", metric.WithDescription("Total number of user registrations"))
	UserLoginCounter, _        = Meter.Int64Counter("user_logins_total", metric.WithDescription("Total number of user logins"))
)

func InitMetrics() func(context.Context) error {

	ctx := context.Background()

	exporter, err := otlpmetrichttp.New(
		ctx,
		otlpmetrichttp.WithEndpoint("localhost:4318"),
		otlpmetrichttp.WithInsecure(),
	)

	if err != nil {
		log.Fatal(err)
	}

	resource, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceName("quiz-backend"),
		),
	)

	if err != nil {
		log.Fatal(err)
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(resource),
		sdkmetric.WithReader(
			sdkmetric.NewPeriodicReader(exporter),
		),
	)

	otel.SetMeterProvider(provider)

	return provider.Shutdown
}

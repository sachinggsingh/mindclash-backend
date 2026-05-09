package telemetry

import (
	"context"
	"log"
	"os"

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
	UserLogoutCounter, _       = Meter.Int64Counter("user_logouts_total", metric.WithDescription("Total number of user logouts"))
	TokenRefreshCounter, _     = Meter.Int64Counter("token_refreshes_total", metric.WithDescription("Total number of token refreshes"))
	UserProfileFetchCounter, _ = Meter.Int64Counter("user_profile_fetches_total", metric.WithDescription("Total number of user profile fetches"))
	QuizGeneratedCounter, _    = Meter.Int64Counter("quizzes_generated_total", metric.WithDescription("Total number of quizzes generated"))
	QuizCreatedCounter, _      = Meter.Int64Counter("quizzes_created_total", metric.WithDescription("Total number of quizzes created manually"))
	QuizSubmittedCounter, _    = Meter.Int64Counter("quiz_submissions_total", metric.WithDescription("Total number of quiz submissions"))
	CommentCreatedCounter, _   = Meter.Int64Counter("comments_created_total", metric.WithDescription("Total number of comments created"))
)

func InitMetrics() func(context.Context) error {

	ctx := context.Background()

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4318"
	}

	exporter, err := otlpmetrichttp.New(
		ctx,
		otlpmetrichttp.WithEndpoint(endpoint),
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

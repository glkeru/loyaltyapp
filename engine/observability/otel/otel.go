package engine

import (
	"context"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func InitTracer(ctx context.Context) func() {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		panic("env OTEL_EXPORTER_OTLP_ENDPOINT is not set")
	}

	//	dctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	//	defer cancel()

	// экспортер
	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("create OTLP exporter error: %v", err)
	}
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(e error) {
		log.Printf("OTel error: %v", e)
	}))
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(("engine"))),
	)
	if err != nil {
		log.Fatalf("create resourse error: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithMaxQueueSize(2048),
			sdktrace.WithBatchTimeout(200*time.Millisecond),
		),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// тестовый спан при старте
	tr := otel.Tracer("engine")
	_, span := tr.Start(ctx, "startup-test")
	span.End()

	return func() {
		_ = tp.Shutdown(context.Background())
	}
}

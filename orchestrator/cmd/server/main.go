package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"

	httphandler "github.com/henriqueMontalione/zipweathertracing/orchestrator/internal/adapters/http"
	"github.com/henriqueMontalione/zipweathertracing/orchestrator/internal/adapters/viacep"
	"github.com/henriqueMontalione/zipweathertracing/orchestrator/internal/adapters/weatherapi"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdown, err := initTracer(ctx, "orchestrator")
	if err != nil {
		slog.Error("failed to initialize tracer", "err", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			slog.Error("tracer shutdown error", "err", err)
		}
	}()

	apiKey := os.Getenv("WEATHERAPI_KEY")
	if apiKey == "" {
		slog.Error("WEATHERAPI_KEY is required")
		os.Exit(1)
	}

	viacepBaseURL := os.Getenv("VIACEP_BASE_URL")
	if viacepBaseURL == "" {
		viacepBaseURL = "https://viacep.com.br"
	}

	weatherAPIBaseURL := os.Getenv("WEATHERAPI_BASE_URL")
	if weatherAPIBaseURL == "" {
		weatherAPIBaseURL = "https://api.weatherapi.com"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	tracer := otel.Tracer("orchestrator")
	httpClient := &http.Client{Timeout: 10 * time.Second}

	locAdapter := viacep.NewClient(viacepBaseURL, httpClient, tracer)
	wthrAdapter := weatherapi.NewClient(weatherAPIBaseURL, apiKey, httpClient, tracer)
	h := httphandler.NewHandler(locAdapter, wthrAdapter)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /", h.Handle)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      otelhttp.NewHandler(mux, "orchestrator"),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown error", "err", err)
		}
	}()

	slog.Info("server started", "port", port)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}

func initTracer(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4317"
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(serviceName),
		)),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp.Shutdown, nil
}

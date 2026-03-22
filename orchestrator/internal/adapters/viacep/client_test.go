package viacep_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/henriqueMontalione/zipweathertracing/orchestrator/internal/adapters/viacep"
	"github.com/henriqueMontalione/zipweathertracing/orchestrator/internal/domain"
)

func newTestClient(handler http.Handler) (*viacep.Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	httpClient := &http.Client{Timeout: 5 * time.Second}
	tracer := noop.NewTracerProvider().Tracer("")
	return viacep.NewClient(srv.URL, httpClient, tracer), srv
}

func TestGetLocation_Success(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"localidade":"São Paulo","erro":""}`))
	})

	client, srv := newTestClient(h)
	defer srv.Close()

	city, err := client.GetLocation(context.Background(), "01001000")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if city != "São Paulo" {
		t.Errorf("city = %q, want %q", city, "São Paulo")
	}
}

func TestGetLocation_ErroField(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"erro":"true"}`))
	})

	client, srv := newTestClient(h)
	defer srv.Close()

	_, err := client.GetLocation(context.Background(), "99999999")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetLocation_NonOKStatus(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	client, srv := newTestClient(h)
	defer srv.Close()

	_, err := client.GetLocation(context.Background(), "99999999")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetLocation_ContextCanceled(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	client, srv := newTestClient(h)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.GetLocation(ctx, "01001000")
	if err == nil {
		t.Fatal("expected error for canceled context, got nil")
	}
}

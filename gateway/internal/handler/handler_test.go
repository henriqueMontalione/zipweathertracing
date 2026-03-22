package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/henriqueMontalione/zipweathertracing/gateway/internal/handler"
)

// newGatewayHandler creates a gateway Handler pointed at the given fake orchestrator URL.
func newGatewayHandler(orchestratorURL string) *handler.Handler {
	httpClient := &http.Client{Timeout: 5 * time.Second}
	return handler.NewHandler(orchestratorURL, httpClient)
}

func TestHandle_InvalidCEP_TooShort(t *testing.T) {
	h := newGatewayHandler("http://unused")

	body := strings.NewReader(`{"cep":"1234"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid zipcode") {
		t.Errorf("body = %q, want to contain 'invalid zipcode'", w.Body.String())
	}
}

func TestHandle_InvalidCEP_TooLong(t *testing.T) {
	h := newGatewayHandler("http://unused")

	body := strings.NewReader(`{"cep":"123456789"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestHandle_InvalidCEP_NonNumeric(t *testing.T) {
	h := newGatewayHandler("http://unused")

	body := strings.NewReader(`{"cep":"0131010A"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestHandle_InvalidCEP_BadJSON(t *testing.T) {
	h := newGatewayHandler("http://unused")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`not-json`))
	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestHandle_ProxiesResponseFromOrchestrator(t *testing.T) {
	orchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"city":"São Paulo","temp_C":28.5,"temp_F":83.3,"temp_K":301.5}`))
	}))
	defer orchestrator.Close()

	h := newGatewayHandler(orchestrator.URL)

	body := strings.NewReader(`{"cep":"01310100"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "São Paulo") {
		t.Errorf("body = %q, expected to contain city name", w.Body.String())
	}
}

func TestHandle_ProxiesOrchestratorNotFound(t *testing.T) {
	orchestrator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "can not find zipcode", http.StatusNotFound)
	}))
	defer orchestrator.Close()

	h := newGatewayHandler(orchestrator.URL)

	body := strings.NewReader(`{"cep":"99999999"}`)
	req := httptest.NewRequest(http.MethodPost, "/", body)
	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

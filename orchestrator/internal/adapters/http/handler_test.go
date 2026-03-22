package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	handler "github.com/henriqueMontalione/zipweathertracing/orchestrator/internal/adapters/http"
	"github.com/henriqueMontalione/zipweathertracing/orchestrator/internal/domain"
)

// --- mocks ---

type mockLocation struct {
	city string
	err  error
}

func (m *mockLocation) GetLocation(_ context.Context, _ string) (string, error) {
	return m.city, m.err
}

type mockWeather struct {
	celsius float64
	err     error
}

func (m *mockWeather) GetTemperature(_ context.Context, _ string) (float64, error) {
	return m.celsius, m.err
}

// --- helper ---

func newRequest(cep string) *http.Request {
	body := strings.NewReader(`{"cep":"` + cep + `"}`)
	return httptest.NewRequest(http.MethodPost, "/", body)
}

// --- tests ---

func TestHandle_Success(t *testing.T) {
	h := handler.NewHandler(
		&mockLocation{city: "São Paulo"},
		&mockWeather{celsius: 28.5},
	)

	w := httptest.NewRecorder()
	h.Handle(w, newRequest("01001000"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var result domain.WeatherResult
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.City != "São Paulo" {
		t.Errorf("city = %q, want %q", result.City, "São Paulo")
	}
	if result.TempC != 28.5 {
		t.Errorf("temp_C = %v, want 28.5", result.TempC)
	}
	if result.TempF != 83.3 {
		t.Errorf("temp_F = %v, want 83.3", result.TempF)
	}
	if result.TempK != 301.5 {
		t.Errorf("temp_K = %v, want 301.5", result.TempK)
	}
}

func TestHandle_InvalidCEP_TooShort(t *testing.T) {
	h := handler.NewHandler(&mockLocation{}, &mockWeather{})

	w := httptest.NewRecorder()
	h.Handle(w, newRequest("0100100"))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid zipcode") {
		t.Errorf("body = %q, want to contain 'invalid zipcode'", w.Body.String())
	}
}

func TestHandle_InvalidCEP_NonNumeric(t *testing.T) {
	h := handler.NewHandler(&mockLocation{}, &mockWeather{})

	w := httptest.NewRecorder()
	h.Handle(w, newRequest("0100100a"))

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestHandle_InvalidCEP_BadJSON(t *testing.T) {
	h := handler.NewHandler(&mockLocation{}, &mockWeather{})

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`not-json`))
	w := httptest.NewRecorder()
	h.Handle(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", w.Code)
	}
}

func TestHandle_NotFound(t *testing.T) {
	h := handler.NewHandler(
		&mockLocation{err: domain.ErrNotFound},
		&mockWeather{},
	)

	w := httptest.NewRecorder()
	h.Handle(w, newRequest("99999999"))

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if !strings.Contains(w.Body.String(), "can not find zipcode") {
		t.Errorf("body = %q, want to contain 'can not find zipcode'", w.Body.String())
	}
}

func TestHandle_WeatherError(t *testing.T) {
	h := handler.NewHandler(
		&mockLocation{city: "Recife"},
		&mockWeather{err: errors.New("upstream unavailable")},
	)

	w := httptest.NewRecorder()
	h.Handle(w, newRequest("50010000"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

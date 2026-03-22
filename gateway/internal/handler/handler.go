package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

type request struct {
	CEP string `json:"cep"`
}

// Handler validates the CEP and proxies the request to the orchestrator.
type Handler struct {
	orchestratorURL string
	httpClient      *http.Client
}

// NewHandler creates a new Handler.
func NewHandler(orchestratorURL string, httpClient *http.Client) *Handler {
	return &Handler{orchestratorURL: orchestratorURL, httpClient: httpClient}
}

// Handle handles POST / — validates the CEP and forwards to the orchestrator.
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || !isValidCEP(req.CEP) {
		http.Error(w, "invalid zipcode", http.StatusUnprocessableEntity)
		return
	}

	body, err := json.Marshal(req)
	if err != nil {
		slog.ErrorContext(ctx, "marshal request", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	outReq, err := http.NewRequestWithContext(ctx, http.MethodPost, h.orchestratorURL, bytes.NewReader(body))
	if err != nil {
		slog.ErrorContext(ctx, "build orchestrator request", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	outReq.Header.Set("Content-Type", "application/json")

	resp, err := h.httpClient.Do(outReq)
	if err != nil {
		slog.ErrorContext(ctx, "call orchestrator", "err", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		slog.ErrorContext(ctx, "copy orchestrator response", "err", err)
	}
}

func isValidCEP(cep string) bool {
	if len(cep) != 8 {
		return false
	}
	for _, c := range cep {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

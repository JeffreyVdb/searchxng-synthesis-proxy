// Package api provides the HTTP surface for the search synthesis proxy.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/example/search-synthesis-proxy/internal/proxy"
)

// Service is the interface for the proxy service.
type Service interface {
	SearchAndSynthesize(ctx context.Context, query string) (proxy.Response, error)
}

// NewHandler returns an http.Handler with all routes registered.
func NewHandler(logger *slog.Logger, svc Service) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	mux := http.NewServeMux()
	h := &handler{logger: logger, svc: svc}
	mux.HandleFunc("GET /healthz", h.handleHealthz)
	mux.HandleFunc("GET /v1/search", h.handleSearch)
	return mux
}

type handler struct {
	logger *slog.Logger
	svc    Service
}

func (h *handler) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	resp, err := h.svc.SearchAndSynthesize(r.Context(), q)
	if err != nil {
		status, code, message := proxy.StatusOf(err)
		h.logger.Error("search failed",
			slog.String("code", code),
			slog.Int("status", status),
			slog.String("error", err.Error()),
		)
		writeError(w, status, code, message)
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/search-synthesis-proxy/internal/proxy"
)

// fakeService implements api.Service for tests.
type fakeService struct {
	resp proxy.Response
	err  error
}

func (f *fakeService) SearchAndSynthesize(_ context.Context, query string) (proxy.Response, error) {
	return f.resp, f.err
}

func TestHealthz(t *testing.T) {
	handler := NewHandler(nil, &fakeService{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want %q", body["status"], "ok")
	}
}

func TestSearch_Success(t *testing.T) {
	svc := &fakeService{
		resp: proxy.Response{
			Query:  "golang",
			Answer: "Go is great",
			Sources: []proxy.Source{
				{Index: 1, Title: "Go", URL: "https://go.dev"},
			},
			Meta: proxy.Meta{Model: "test", ResultsConsidered: 1, TookMS: 100},
		},
	}
	handler := NewHandler(nil, svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=golang", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	var body proxy.Response
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Answer != "Go is great" {
		t.Errorf("Answer = %q", body.Answer)
	}
}

func TestSearch_MissingQuery(t *testing.T) {
	svc := &fakeService{
		err: &proxy.AppError{Status: 400, Code: "invalid_query", Message: "query parameter q is required"},
	}
	handler := NewHandler(nil, svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/search", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	errObj := body["error"].(map[string]interface{})
	if errObj["code"] != "invalid_query" {
		t.Errorf("code = %q", errObj["code"])
	}
}

func TestSearch_WrongMethod(t *testing.T) {
	handler := NewHandler(nil, &fakeService{})
	req := httptest.NewRequest(http.MethodPost, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}

func TestSearch_ServiceError(t *testing.T) {
	svc := &fakeService{
		err: &proxy.AppError{Status: 502, Code: "search_upstream_error", Message: "search backend request failed"},
	}
	handler := NewHandler(nil, svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 502 {
		t.Fatalf("status = %d, want 502", w.Code)
	}
}

func TestSearch_ContentType(t *testing.T) {
	svc := &fakeService{
		resp: proxy.Response{Query: "test", Answer: "ok"},
	}
	handler := NewHandler(nil, svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestHealthz_WrongMethod(t *testing.T) {
	handler := NewHandler(nil, &fakeService{})
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Fatalf("status = %d, want 405", w.Code)
	}
}

func TestSearch_ErrorJSONFormat(t *testing.T) {
	svc := &fakeService{
		err: &proxy.AppError{Status: 502, Code: "llm_upstream_error", Message: "language model request failed"},
	}
	handler := NewHandler(nil, svc)
	req := httptest.NewRequest(http.MethodGet, "/v1/search?q=test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify the error JSON has the right structure
	var body map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatal("response missing 'error' object")
	}
	if errObj["code"] != "llm_upstream_error" {
		t.Errorf("code = %v", errObj["code"])
	}
	if errObj["message"] != "language model request failed" {
		t.Errorf("message = %v", errObj["message"])
	}
}



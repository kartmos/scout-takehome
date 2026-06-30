package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"scout/internal/httpapi"
)

func TestHealthz_Success(t *testing.T) {
	handler := httpapi.NewRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	handler.ServeHTTP(w, r)

	if got, want := w.Code, http.StatusOK; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if got, want := w.Header().Get("Content-Type"), "application/json"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	if got, want := w.Body.String(), "{\"status\":\"ok\"}\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestHealthz_MethodNotAllowed(t *testing.T) {
	handler := httpapi.NewRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/healthz", nil)

	handler.ServeHTTP(w, r)

	if got, want := w.Code, http.StatusMethodNotAllowed; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if allow := w.Header().Get("Allow"); allow == "" {
		t.Error("Allow header missing on 405 response")
	}
}

func TestHealthz_NotFound(t *testing.T) {
	handler := httpapi.NewRouter()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)

	handler.ServeHTTP(w, r)

	if got, want := w.Code, http.StatusNotFound; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
}

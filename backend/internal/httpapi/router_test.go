package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"scout/internal/httpapi"
)

func productionRouter(t *testing.T) http.Handler {
	t.Helper()
	return httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           noopRepo{},
		Storage:        noopStorage{},
	})
}

func TestHealthz_Success(t *testing.T) {
	handler := productionRouter(t)
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
	if got := w.Header().Get("X-Request-ID"); got == "" {
		t.Error("X-Request-ID header must be set on /healthz response")
	}
}

func TestHealthz_MethodNotAllowed(t *testing.T) {
	handler := productionRouter(t)
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
	handler := productionRouter(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)

	handler.ServeHTTP(w, r)

	if got, want := w.Code, http.StatusNotFound; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
}

func TestNewRouter_PanicsOnMissingLogger(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewRouter must panic when Logger is nil")
		}
	}()
	httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         nil,
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
	})
}

func TestNewRouter_PanicsOnEmptyOrigins(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewRouter must panic when AllowedOrigins is empty")
		}
	}()
	httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: nil,
		APIKey:         testAPIKey,
	})
}

func TestNewRouter_PanicsOnEmptyAPIKey(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewRouter must panic when APIKey is empty")
		}
	}()
	httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         "",
	})
}

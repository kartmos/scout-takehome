package httpapi_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"scout/internal/apperror"
	"scout/internal/httpapi"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func bufLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, nil))
}

// callWriteError runs WriteError through the full middleware stack so the
// request context contains a valid request ID from the requestID middleware.
// errLogger receives only the logs emitted by WriteError itself.
func callWriteError(t *testing.T, err error, errLogger *slog.Logger) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /test-err", func(w http.ResponseWriter, r *http.Request) {
		httpapi.WriteError(w, r, errLogger, err)
	})
	h := httpapi.NewRouterWithMux(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
	}, mux)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test-err", nil)
	r.Header.Set("X-Request-ID", "test-req-id-001")
	h.ServeHTTP(w, r)
	return w
}

func TestWriteError_ValidationError(t *testing.T) {
	appErr := apperror.NewValidation("bad input", []apperror.FieldViolation{
		{Field: "name", Issue: "required"},
		{Field: "age", Issue: "must be positive"},
	})
	w := callWriteError(t, appErr, discardLogger())

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}

	var body struct {
		RequestID string `json:"request_id"`
		Message   string `json:"message"`
		Code      string `json:"code"`
		Details   []struct {
			Field string `json:"field"`
			Issue string `json:"issue"`
		} `json:"details"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != "ValidationError" {
		t.Errorf("code = %q, want ValidationError", body.Code)
	}
	if body.RequestID == "" {
		t.Error("request_id must not be empty")
	}
	if body.Message == "" {
		t.Error("message must not be empty")
	}
	if len(body.Details) != 2 {
		t.Fatalf("details len = %d, want 2", len(body.Details))
	}
	if body.Details[0].Field != "name" || body.Details[0].Issue != "required" {
		t.Errorf("details[0] = %+v, want {name required}", body.Details[0])
	}
	if body.Details[1].Field != "age" || body.Details[1].Issue != "must be positive" {
		t.Errorf("details[1] = %+v, want {age must be positive}", body.Details[1])
	}
}

func TestWriteError_AuthError(t *testing.T) {
	appErr := apperror.NewAuth("")
	w := callWriteError(t, appErr, discardLogger())

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}

	var body struct {
		RequestID string `json:"request_id"`
		Code      string `json:"code"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != "AuthenticationRequired" {
		t.Errorf("code = %q, want AuthenticationRequired", body.Code)
	}
	if body.RequestID == "" {
		t.Error("request_id must not be empty")
	}
	if body.Message == "" {
		t.Error("message must not be empty")
	}
}

func TestWriteError_NotFoundError(t *testing.T) {
	appErr := apperror.NewNotFound("photo not found", "abc-123")
	w := callWriteError(t, appErr, discardLogger())

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}

	var body struct {
		RequestID  string `json:"request_id"`
		Code       string `json:"code"`
		Message    string `json:"message"`
		ResourceID string `json:"resource_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != "NotFound" {
		t.Errorf("code = %q, want NotFound", body.Code)
	}
	if body.ResourceID != "abc-123" {
		t.Errorf("resource_id = %q, want abc-123", body.ResourceID)
	}
	if body.RequestID == "" {
		t.Error("request_id must not be empty")
	}
}

func TestWriteError_InternalError_CauseLogged_NotExposed(t *testing.T) {
	var logBuf bytes.Buffer
	cause := errors.New("database connection refused")
	appErr := apperror.NewInternal(cause)
	w := callWriteError(t, appErr, bufLogger(&logBuf))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}

	var body struct {
		RequestID string `json:"request_id"`
		Code      string `json:"code"`
		Message   string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Code != "InternalServerError" {
		t.Errorf("code = %q, want InternalServerError", body.Code)
	}
	if strings.Contains(w.Body.String(), "database") {
		t.Error("response body must not expose the internal cause")
	}

	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "database connection refused") {
		t.Error("internal cause must appear in log output")
	}
	if !strings.Contains(logOutput, "request_id") {
		t.Error("request_id must appear in internal error log")
	}
}

func TestWriteError_UnknownError_Safe500(t *testing.T) {
	var logBuf bytes.Buffer
	unknown := errors.New("some unexpected sentinel error")
	w := callWriteError(t, unknown, bufLogger(&logBuf))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["code"] != "InternalServerError" {
		t.Errorf("code = %v, want InternalServerError", body["code"])
	}
	if strings.Contains(w.Body.String(), "sentinel") {
		t.Error("unknown error message must not appear in response body")
	}
	if !strings.Contains(logBuf.String(), "some unexpected sentinel error") {
		t.Error("unknown error must be logged")
	}
}

func TestWriteError_NoCauseLeakage(t *testing.T) {
	secretCause := errors.New("xyzSecretInternalDetail")
	appErr := apperror.NewInternal(secretCause)
	w := callWriteError(t, appErr, discardLogger())

	if strings.Contains(w.Body.String(), "xyzSecretInternalDetail") {
		t.Error("internal cause must not appear in response body")
	}
}

func TestWriteError_FallbackRequestID_WhenNoMiddleware(t *testing.T) {
	// Call WriteError directly without any middleware — it must generate its own ID.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	httpapi.WriteError(w, r, discardLogger(), apperror.NewAuth(""))

	var body struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.RequestID == "" {
		t.Error("fallback request_id must not be empty")
	}
}

func TestWriteError_RequestIDInBodyAndHeader(t *testing.T) {
	appErr := apperror.NewAuth("")
	w := callWriteError(t, appErr, discardLogger())

	headerID := w.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Error("X-Request-ID header must be set")
	}

	var body struct {
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.RequestID == "" {
		t.Error("request_id in body must not be empty")
	}
	// The body ID should match the header ID that was set by the middleware.
	if body.RequestID != headerID {
		t.Errorf("body request_id = %q, header X-Request-ID = %q, want same", body.RequestID, headerID)
	}
}

func TestWriteError_ContentTypeAndCacheControl(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"validation", apperror.NewValidation("", nil)},
		{"auth", apperror.NewAuth("")},
		{"not_found", apperror.NewNotFound("", "rid")},
		{"internal", apperror.NewInternal(nil)},
		{"unknown", errors.New("unknown")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := callWriteError(t, tt.err, discardLogger())
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", cc)
			}
		})
	}
}

package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"scout/internal/httpapi"
)

// capturedLog holds one structured log record for inspection in tests.
type capturedLog struct {
	level   slog.Level
	message string
	attrs   map[string]string
}

// memHandler is an in-memory slog.Handler that captures records for counting
// and attribute inspection. WithAttrs/WithGroup are no-ops because the
// middleware loggers do not use them.
type memHandler struct {
	mu   sync.Mutex
	logs []capturedLog
}

func newMemLogger() (*memHandler, *slog.Logger) {
	h := &memHandler{}
	return h, slog.New(h)
}

func (h *memHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *memHandler) Handle(_ context.Context, r slog.Record) error {
	entry := capturedLog{
		level:   r.Level,
		message: r.Message,
		attrs:   make(map[string]string),
	}
	r.Attrs(func(a slog.Attr) bool {
		entry.attrs[a.Key] = a.Value.String()
		return true
	})
	h.mu.Lock()
	h.logs = append(h.logs, entry)
	h.mu.Unlock()
	return nil
}

func (h *memHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *memHandler) WithGroup(string) slog.Handler      { return h }

// findAll returns all captured records whose Message equals msg.
func (h *memHandler) findAll(msg string) []capturedLog {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []capturedLog
	for _, l := range h.logs {
		if l.message == msg {
			out = append(out, l)
		}
	}
	return out
}

const testAPIKey = "test-api-key-xyzlocal"

// newStackWith builds the full middleware stack around an arbitrary handler for
// targeted middleware tests. Uses a path-registered mux so pattern matching works.
func newStackWith(logger *slog.Logger, path string, inner http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(path, inner)
	return httpapi.NewRouterWithMux(httpapi.RouterConfig{
		Logger:         logger,
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
	}, mux)
}

// -- Request ID tests --------------------------------------------------------

func TestRequestID_Generated_WhenMissingOrInvalid(t *testing.T) {
	tests := []struct {
		name     string
		incoming string
	}{
		{name: "missing", incoming: ""},
		{name: "too_long", incoming: strings.Repeat("a", 129)},
		{name: "space", incoming: "bad id with space"},
		{name: "slash", incoming: "a/b"},
		{name: "at_sign", incoming: "req@1"},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := httpapi.NewRouter(httpapi.RouterConfig{
				Logger:         discardLogger(),
				AllowedOrigins: []string{"http://localhost:5173"},
				APIKey:         testAPIKey,
				Repo:           noopRepo{},
				Storage:        noopStorage{},
			})
			_ = handler // using the production router for the healthz endpoint
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			if tt.incoming != "" {
				r.Header.Set("X-Request-ID", tt.incoming)
			}
			h.ServeHTTP(w, r)

			got := w.Header().Get("X-Request-ID")
			if got == "" {
				t.Fatal("X-Request-ID must be set in response")
			}
			if tt.incoming != "" && got == tt.incoming {
				t.Errorf("invalid incoming ID was echoed: %q", got)
			}
		})
	}
}

func TestRequestID_AcceptedAndEchoed_WhenValid(t *testing.T) {
	cases := []string{
		"a",
		"abc-123",
		"my.request_id-001",
		strings.Repeat("x", 128),
	}
	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           noopRepo{},
		Storage:        noopStorage{},
	})
	for _, incoming := range cases {
		truncated := incoming
		if len(truncated) > 20 {
			truncated = truncated[:20]
		}
		t.Run(truncated, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			r.Header.Set("X-Request-ID", incoming)
			h.ServeHTTP(w, r)

			if got := w.Header().Get("X-Request-ID"); got != incoming {
				t.Errorf("X-Request-ID = %q, want %q", got, incoming)
			}
		})
	}
}

// -- Auth middleware tests ----------------------------------------------------

func authOnlyHandler(key string, next http.Handler) http.Handler {
	return httpapi.Authenticate(key, discardLogger())(next)
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
}

func TestAuthenticate_Missing(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	h := httpapi.Authenticate(testAPIKey, logger)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if strings.Contains(w.Body.String(), testAPIKey) {
		t.Error("response must not contain the API key")
	}
	if strings.Contains(logBuf.String(), testAPIKey) {
		t.Error("logs must not contain the API key")
	}
}

func TestAuthenticate_Wrong(t *testing.T) {
	const wrongKey = "wrong-key-value"
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	h := httpapi.Authenticate(testAPIKey, logger)(okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", wrongKey)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
	if strings.Contains(w.Body.String(), wrongKey) {
		t.Error("response must not contain the provided wrong key")
	}
	if strings.Contains(logBuf.String(), testAPIKey) {
		t.Error("logs must not contain the expected API key")
	}
	if strings.Contains(logBuf.String(), wrongKey) {
		t.Error("logs must not contain the provided wrong key")
	}
}

func TestAuthenticate_Correct(t *testing.T) {
	h := authOnlyHandler(testAPIKey, okHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", testAPIKey)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestAuthenticate_MissingAndWrong_SameStatus(t *testing.T) {
	h := authOnlyHandler(testAPIKey, okHandler())

	wMissing := httptest.NewRecorder()
	rMissing := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(wMissing, rMissing)

	wWrong := httptest.NewRecorder()
	rWrong := httptest.NewRequest(http.MethodGet, "/", nil)
	rWrong.Header.Set("X-API-Key", "not-correct")
	h.ServeHTTP(wWrong, rWrong)

	if wMissing.Code != wWrong.Code {
		t.Errorf("missing=%d wrong=%d, must not distinguish key presence from invalidity", wMissing.Code, wWrong.Code)
	}
}

// -- Panic recovery tests ----------------------------------------------------

func panicHandler(val any) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(val)
	})
}

func panicAfterWriteHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		panic("panic after write")
	})
}

func TestPanicRecovery_BeforeWrite(t *testing.T) {
	mem, logger := newMemLogger()
	h := newStackWith(logger, "/", panicHandler("boom"))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", "panic-test-id")

	h.ServeHTTP(w, r)

	// Response: safe 500 with no panic value exposed.
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if !strings.Contains(w.Body.String(), "InternalServerError") {
		t.Errorf("body = %q, want InternalServerError code", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "boom") {
		t.Error("response body must not contain the panic value")
	}
	for name, vals := range w.Header() {
		for _, v := range vals {
			if strings.Contains(v, "boom") {
				t.Errorf("response header %q must not contain panic value, got %q", name, v)
			}
		}
	}

	// Exactly one diagnostic failure record containing request ID and panic value.
	panics := mem.findAll("panic recovered")
	if len(panics) != 1 {
		t.Errorf("want 1 'panic recovered' diagnostic, got %d", len(panics))
	}
	if len(panics) == 1 {
		if panics[0].level != slog.LevelError {
			t.Errorf("diagnostic level = %v, want ERROR", panics[0].level)
		}
		if panics[0].attrs["request_id"] != "panic-test-id" {
			t.Errorf("request_id = %q, want panic-test-id", panics[0].attrs["request_id"])
		}
		if !strings.Contains(panics[0].attrs["panic"], "boom") {
			t.Errorf("panic attr = %q, want to contain boom", panics[0].attrs["panic"])
		}
	}

	// No duplicate internal-error diagnostic for the same panic.
	if dups := mem.findAll("internal error"); len(dups) > 0 {
		t.Errorf("must not emit a second 'internal error' diagnostic, got %d", len(dups))
	}

	// Exactly one access-completion record with status 500.
	access := mem.findAll("request completed")
	if len(access) != 1 {
		t.Errorf("want 1 access log, got %d", len(access))
	}
	if len(access) == 1 && access[0].attrs["status"] != "500" {
		t.Errorf("access log status = %q, want 500", access[0].attrs["status"])
	}
}

func TestPanicRecovery_AfterWrite(t *testing.T) {
	mem, logger := newMemLogger()
	h := newStackWith(logger, "/", panicAfterWriteHandler())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Request-ID", "panic-after-write")

	// Must not re-panic; recovery detects committed headers and aborts gracefully.
	h.ServeHTTP(w, r)

	// Exactly one diagnostic failure record containing request ID and panic value.
	panics := mem.findAll("panic recovered")
	if len(panics) != 1 {
		t.Errorf("want 1 'panic recovered' diagnostic, got %d", len(panics))
	}
	if len(panics) == 1 {
		if panics[0].level != slog.LevelError {
			t.Errorf("diagnostic level = %v, want ERROR", panics[0].level)
		}
		if panics[0].attrs["request_id"] != "panic-after-write" {
			t.Errorf("request_id = %q, want panic-after-write", panics[0].attrs["request_id"])
		}
		if !strings.Contains(panics[0].attrs["panic"], "panic after write") {
			t.Errorf("panic attr = %q, want to contain panic value", panics[0].attrs["panic"])
		}
	}

	// No JSON error must be appended after the partial body.
	body := w.Body.String()
	if !strings.HasPrefix(body, "partial") {
		t.Errorf("body = %q, want to start with partial", body)
	}
	if strings.Contains(body, "InternalServerError") {
		t.Error("partial-write body must not have a JSON error appended")
	}

	// Access-completion event reflects the committed status and bytes.
	access := mem.findAll("request completed")
	if len(access) != 1 {
		t.Errorf("want 1 access log, got %d", len(access))
	}
	if len(access) == 1 {
		if access[0].attrs["status"] != "200" {
			t.Errorf("access log status = %q, want 200", access[0].attrs["status"])
		}
		if access[0].attrs["bytes"] != "7" {
			t.Errorf("access log bytes = %q, want 7", access[0].attrs["bytes"])
		}
	}
}

func TestPanicRecovery_ErrAbortHandler_Repanics(t *testing.T) {
	mem, logger := newMemLogger()
	h := newStackWith(logger, "/", panicHandler(http.ErrAbortHandler))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)

	defer func() {
		p := recover()
		if p != http.ErrAbortHandler {
			t.Errorf("expected ErrAbortHandler re-panic, got %v", p)
		}
		// No recovery diagnostic must be emitted for http.ErrAbortHandler.
		if panics := mem.findAll("panic recovered"); len(panics) != 0 {
			t.Errorf("must not log 'panic recovered' for ErrAbortHandler, got %d records", len(panics))
		}
	}()
	h.ServeHTTP(w, r)
	t.Error("expected re-panic but ServeHTTP returned normally")
}

// -- Access log tests --------------------------------------------------------

func TestAccessLog_RecordsCorrectLevel(t *testing.T) {
	tests := []struct {
		status    int
		wantLevel string
	}{
		{200, "INFO"},
		{201, "INFO"},
		{304, "INFO"},
		{400, "WARN"},
		{404, "WARN"},
		{500, "ERROR"},
		{503, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			var logBuf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			h := newStackWith(logger, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			h.ServeHTTP(w, r)

			logOut := logBuf.String()
			if !strings.Contains(logOut, "request completed") {
				t.Fatalf("'request completed' not found in log: %s", logOut)
			}
			if !strings.Contains(logOut, tt.wantLevel) {
				t.Errorf("want log level %s in: %s", tt.wantLevel, logOut)
			}
		})
	}
}

func TestAccessLog_NoQueryString(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	h := newStackWith(logger, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/?secret=supersecretvalue", nil)
	h.ServeHTTP(w, r)

	if strings.Contains(logBuf.String(), "supersecretvalue") {
		t.Error("query string value must not appear in access log")
	}
}

func TestAccessLog_NoAPIKey(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	h := newStackWith(logger, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-API-Key", "secretkeyvalue12345")
	h.ServeHTTP(w, r)

	if strings.Contains(logBuf.String(), "secretkeyvalue12345") {
		t.Error("API key must not appear in access log")
	}
}

func TestAccessLog_Flusher_ViaResponseController(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	flushed := false
	h := newStackWith(logger, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		// httptest.ResponseRecorder implements Flusher; reaching it via Unwrap must work.
		if err := rc.Flush(); err != nil {
			t.Errorf("Flush failed: %v (should reach underlying recorder via Unwrap)", err)
		} else {
			flushed = true
		}
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(w, r)

	if !flushed {
		t.Error("Flush must succeed through the responseRecorder via Unwrap")
	}
}

// -- CORS tests --------------------------------------------------------------

const (
	corsAllowedOrigin    = "http://localhost:5173"
	corsDisallowedOrigin = "http://attacker.example.com"
)

func corsTestRouter(t *testing.T) http.Handler {
	t.Helper()
	return httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{corsAllowedOrigin},
		APIKey:         testAPIKey,
		Repo:           noopRepo{},
		Storage:        noopStorage{},
	})
}

func TestCORS_AllowedOrigin_SimpleRequest(t *testing.T) {
	h := corsTestRouter(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.Header.Set("Origin", corsAllowedOrigin)
	h.ServeHTTP(w, r)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != corsAllowedOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, corsAllowedOrigin)
	}
	if !containsVaryValue(w.Header(), "Origin") {
		t.Error("Vary must include Origin")
	}
	if got := w.Header().Get("Access-Control-Expose-Headers"); got != "X-Request-ID" {
		t.Errorf("Access-Control-Expose-Headers = %q, want X-Request-ID", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Access-Control-Allow-Credentials must not be set, got %q", got)
	}
}

func TestCORS_AllowedOrigin_Preflight(t *testing.T) {
	h := corsTestRouter(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/healthz", nil)
	r.Header.Set("Origin", corsAllowedOrigin)
	r.Header.Set("Access-Control-Request-Method", "GET")
	r.Header.Set("Access-Control-Request-Headers", "X-API-Key")
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != corsAllowedOrigin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, corsAllowedOrigin)
	}
	if !containsVaryValue(w.Header(), "Origin") {
		t.Error("Vary must include Origin")
	}
	if !containsVaryValue(w.Header(), "Access-Control-Request-Method") {
		t.Error("Vary must include Access-Control-Request-Method")
	}
	if !containsVaryValue(w.Header(), "Access-Control-Request-Headers") {
		t.Error("Vary must include Access-Control-Request-Headers")
	}

	allowedMethods := w.Header().Get("Access-Control-Allow-Methods")
	for _, m := range []string{"GET", "POST", "OPTIONS"} {
		if !strings.Contains(allowedMethods, m) {
			t.Errorf("Access-Control-Allow-Methods missing %q, got %q", m, allowedMethods)
		}
	}
	allowedHeaders := w.Header().Get("Access-Control-Allow-Headers")
	for _, h := range []string{"Content-Type", "X-API-Key", "X-Request-ID"} {
		if !strings.Contains(allowedHeaders, h) {
			t.Errorf("Access-Control-Allow-Headers missing %q, got %q", h, allowedHeaders)
		}
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Access-Control-Allow-Credentials must not be set, got %q", got)
	}
}

func TestCORS_Preflight_NoAuthRequired(t *testing.T) {
	h := corsTestRouter(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/healthz", nil)
	r.Header.Set("Origin", corsAllowedOrigin)
	r.Header.Set("Access-Control-Request-Method", "GET")
	// No X-API-Key header.
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNoContent {
		t.Errorf("preflight must not require auth, got status %d", w.Code)
	}
}

func TestCORS_DisallowedOrigin_NoAllowHeaders(t *testing.T) {
	h := corsTestRouter(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.Header.Set("Origin", corsDisallowedOrigin)
	h.ServeHTTP(w, r)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty for disallowed origin", got)
	}
}

func TestCORS_DisallowedOrigin_Preflight_NoAllowHeaders(t *testing.T) {
	h := corsTestRouter(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodOptions, "/healthz", nil)
	r.Header.Set("Origin", corsDisallowedOrigin)
	r.Header.Set("Access-Control-Request-Method", "GET")
	h.ServeHTTP(w, r)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty for disallowed origin", got)
	}
	if w.Code == http.StatusNoContent {
		t.Error("disallowed origin preflight must not return 204")
	}
}

func TestCORS_NoReflectArbitraryOrigin(t *testing.T) {
	h := corsTestRouter(t)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.Header.Set("Origin", "http://evil.com")
	h.ServeHTTP(w, r)

	acao := w.Header().Get("Access-Control-Allow-Origin")
	if acao == "http://evil.com" || acao == "*" {
		t.Errorf("arbitrary origin must not be reflected, got %q", acao)
	}
}

// containsVaryValue checks whether the Vary header contains the given token.
func containsVaryValue(h http.Header, target string) bool {
	for _, v := range h[http.CanonicalHeaderKey("Vary")] {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), target) {
				return true
			}
		}
	}
	return false
}

// -- /healthz public route tests ---------------------------------------------

func TestHealthz_PublicWithRequestID(t *testing.T) {
	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{corsAllowedOrigin},
		APIKey:         testAPIKey,
		Repo:           noopRepo{},
		Storage:        noopStorage{},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.Header.Set("X-Request-ID", "smoke-test-1")
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if got := w.Header().Get("X-Request-ID"); got != "smoke-test-1" {
		t.Errorf("X-Request-ID = %q, want smoke-test-1", got)
	}
}

func TestHealthz_PublicWithoutAPIKey(t *testing.T) {
	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{corsAllowedOrigin},
		APIKey:         testAPIKey,
		Repo:           noopRepo{},
		Storage:        noopStorage{},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	// No X-API-Key header.
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("/healthz must be public (no auth), got status %d", w.Code)
	}
}

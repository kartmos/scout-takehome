package httpapi

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"scout/internal/apperror"
	"scout/internal/observability"
)

// responseRecorder captures status and bytes written for access logging.
// Unwrap exposes the underlying ResponseWriter so http.NewResponseController
// can reach Flusher and other optional interfaces on the real writer.
type responseRecorder struct {
	http.ResponseWriter
	statusCode  int
	written     int64
	wroteHeader bool
}

func (rw *responseRecorder) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.statusCode = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseRecorder) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// Unwrap lets http.NewResponseController reach optional interfaces (e.g. Flusher)
// on the underlying ResponseWriter.
func (rw *responseRecorder) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// accessLogMiddleware logs one completion event per request and optionally records
// Prometheus HTTP metrics when m is non-nil.
// It wraps panic recovery so recovered failures are recorded with their status.
// Never logs query strings, bodies, API keys, or auth headers.
func accessLogMiddleware(logger *slog.Logger, m *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rw, r)

			dur := time.Since(start)
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}

			if m != nil {
				statusClass := fmt.Sprintf("%dxx", rw.statusCode/100)
				m.HTTPRequestsTotal.WithLabelValues(r.Method, route, statusClass).Inc()
				m.HTTPDurationSeconds.WithLabelValues(r.Method, route, statusClass).Observe(dur.Seconds())
			}

			attrs := []any{
				"request_id", RequestIDFromContext(r.Context()),
				"method", r.Method,
				"route", route,
				"status", rw.statusCode,
				"bytes", rw.written,
				"duration_ms", dur.Milliseconds(),
			}

			switch {
			case rw.statusCode >= 500:
				logger.Error("request completed", attrs...)
			case rw.statusCode >= 400:
				logger.Warn("request completed", attrs...)
			default:
				logger.Info("request completed", attrs...)
			}
		})
	}
}

// panicRecoveryMiddleware catches downstream panics and emits a typed safe 500
// if headers have not yet been committed. It re-panics http.ErrAbortHandler so
// net/http can perform its intended connection-close behavior.
func panicRecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				p := recover()
				if p == nil {
					return
				}
				if p == http.ErrAbortHandler {
					panic(p)
				}
				reqID := RequestIDFromContext(r.Context())
				// Emit exactly one diagnostic containing the panic value.
				// The panic value never appears in the client response.
				logger.Error("panic recovered", "request_id", reqID, "panic", p)

				if rw, ok := w.(*responseRecorder); ok && rw.wroteHeader {
					// Output already started; do not append JSON to a partial body.
					return
				}
				// Use a discard logger so WriteError does not emit a second diagnostic.
				WriteError(w, r, slog.New(slog.DiscardHandler), apperror.NewInternal(nil))
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// corsMiddleware enforces an exact origin allowlist, handles valid preflight
// requests with 204 No Content, and sets appropriate headers for allowed origins.
// Disallowed origins receive no CORS headers and continue through normal routing.
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			_, originAllowed := allowed[origin]

			if originAllowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
			}

			// Valid preflight: OPTIONS + Origin (allowed) + Access-Control-Request-Method.
			if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
				if originAllowed {
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Key, X-Request-ID")
					w.Header().Add("Vary", "Access-Control-Request-Method")
					w.Header().Add("Vary", "Access-Control-Request-Headers")
					w.WriteHeader(http.StatusNoContent)
					return
				}
				// Disallowed origin preflight: no CORS headers, fall through.
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Authenticate returns middleware that requires a valid X-API-Key header.
// Keys are compared with constant-time comparison after SHA-256 hashing for
// length-safe preparation. Missing and wrong keys return the same typed 401.
// Neither the provided nor the expected key is ever logged.
func Authenticate(apiKey string, logger *slog.Logger) func(http.Handler) http.Handler {
	wantSum := sha256.Sum256([]byte(apiKey))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotSum := sha256.Sum256([]byte(r.Header.Get("X-API-Key")))
			if subtle.ConstantTimeCompare(wantSum[:], gotSum[:]) != 1 {
				WriteError(w, r, logger, apperror.NewAuth(""))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

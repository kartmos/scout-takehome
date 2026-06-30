package httpapi

import (
	"log/slog"
	"net/http"
)

// RouterConfig holds the dependencies required to build the HTTP handler tree.
type RouterConfig struct {
	Logger         *slog.Logger
	AllowedOrigins []string
	// APIKey is used only inside middleware closures and is never logged or returned.
	APIKey string
}

// NewRouter builds the HTTP handler with the production route set
// (currently only GET /healthz) and the full middleware stack:
//
//	RequestID → AccessLogging → PanicRecovery → CORS → mux
//
// Auth middleware is not applied globally; it will be wired per-handler for
// the three Data API operations. Panics if any required field in cfg is zero.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	return NewRouterWithMux(cfg, mux)
}

// NewRouterWithMux applies the full middleware stack around the provided mux.
// It is intended for use in tests that need to inject custom handlers while
// still exercising the request-ID, access-log, recovery, and CORS layers.
// Panics if any required field in cfg is zero.
func NewRouterWithMux(cfg RouterConfig, mux *http.ServeMux) http.Handler {
	if cfg.Logger == nil {
		panic("httpapi: Logger is required")
	}
	if len(cfg.AllowedOrigins) == 0 {
		panic("httpapi: AllowedOrigins must not be empty")
	}
	if cfg.APIKey == "" {
		panic("httpapi: APIKey is required")
	}

	return requestIDMiddleware(
		accessLogMiddleware(cfg.Logger)(
			panicRecoveryMiddleware(cfg.Logger)(
				corsMiddleware(cfg.AllowedOrigins)(
					mux,
				),
			),
		),
	)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("{\"status\":\"ok\"}\n"))
}

package httpapi

import (
	"log/slog"
	"net/http"

	"scout/internal/apperror"
	"scout/internal/observability"
)

// RouterConfig holds the dependencies required to build the HTTP handler tree.
type RouterConfig struct {
	Logger         *slog.Logger
	AllowedOrigins []string
	// APIKey is used only inside middleware closures and is never logged or returned.
	APIKey       string
	Repo         photoRepository
	Storage      photoStorage
	ThumbnailSvc thumbnailService
	// Metrics is optional. When non-nil, HTTP metrics (count, duration) are recorded.
	Metrics *observability.Metrics
	// MetricsHandler is optional. When non-nil, GET /metrics is registered as a
	// public (unauthenticated) endpoint. Must not expose secrets or signed URLs.
	MetricsHandler http.Handler
}

// NewRouter builds the HTTP handler with the production route set and the full
// middleware stack:
//
//	RequestID → AccessLogging → PanicRecovery → CORS → mux
//
// The three Data API routes are registered with Authenticate per-route.
// /healthz, /metrics (when MetricsHandler is non-nil), and valid CORS preflights
// remain public. Panics if any required field in cfg is zero/nil.
func NewRouter(cfg RouterConfig) http.Handler {
	if cfg.Repo == nil {
		panic("httpapi: Repo is required")
	}
	if cfg.Storage == nil {
		panic("httpapi: Storage is required")
	}
	if cfg.ThumbnailSvc == nil {
		panic("httpapi: ThumbnailSvc is required")
	}
	auth := Authenticate(cfg.APIKey, cfg.Logger)
	notAllowed := notAllowedJSON(cfg.Logger)
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("/healthz", notAllowed("GET, HEAD"))

	if cfg.MetricsHandler != nil {
		mux.Handle("GET /metrics", cfg.MetricsHandler)
		mux.HandleFunc("/metrics", notAllowed("GET, HEAD"))
	}

	// Thumbnail endpoint is public (no auth); registered before the /photos/{photoId}
	// pattern so the more-specific path wins in Go 1.22+ ServeMux matching.
	mux.HandleFunc("GET /photos/{photoId}/thumbnail",
		handleGetThumbnail(cfg.Repo, cfg.ThumbnailSvc, cfg.Logger))
	mux.HandleFunc("/photos/{photoId}/thumbnail", notAllowed("GET, HEAD"))

	mux.Handle("POST /photos/{photoId}/upload-link",
		auth(handleUploadLink(cfg.Repo, cfg.Storage, cfg.Logger)))
	mux.HandleFunc("/photos/{photoId}/upload-link", notAllowed("POST"))

	mux.Handle("GET /photos/{photoId}",
		auth(handleGetPhoto(cfg.Repo, cfg.Storage, cfg.Logger)))
	mux.HandleFunc("/photos/{photoId}", notAllowed("GET, HEAD"))

	mux.Handle("GET /photos",
		auth(handleListPhotos(cfg.Repo, cfg.Storage, cfg.Logger)))
	mux.HandleFunc("/photos", notAllowed("GET, HEAD"))

	// Catch-all: typed JSON 404 for any path not matched above.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		WriteError(w, r, cfg.Logger, apperror.NewNotFound("not found", r.URL.Path))
	})

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
		accessLogMiddleware(cfg.Logger, cfg.Metrics)(
			panicRecoveryMiddleware(cfg.Logger)(
				corsMiddleware(cfg.AllowedOrigins)(
					mux,
				),
			),
		),
	)
}

func notAllowedJSON(logger *slog.Logger) func(allowed string) http.HandlerFunc {
	return func(allowed string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			WriteError(w, r, logger, apperror.NewMethodNotAllowed(allowed))
		}
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("{\"status\":\"ok\"}\n"))
}

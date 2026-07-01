package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"scout/internal/config"
	"scout/internal/httpapi"
	"scout/internal/objectstorage"
	"scout/internal/observability"
	"scout/internal/repository/sqlite"
	"scout/internal/thumbnail"
)

func main() {
	os.Exit(run())
}

func run() (exitCode int) {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "error", err)
		return 1
	}

	s3cfg, err := config.LoadS3Config()
	if err != nil {
		logger.Error("storage configuration error", "error", err)
		return 1
	}

	repo, err := sqlite.Open(cfg.DatabasePath)
	if err != nil {
		logger.Error("database open failed", "error", err)
		return 1
	}
	defer func() {
		if cerr := repo.Close(); cerr != nil {
			logger.Error("database close failed", "error", cerr)
			if exitCode == 0 {
				exitCode = 1
			}
		}
	}()

	storage, err := objectstorage.New(*s3cfg)
	if err != nil {
		logger.Error("storage construction failed", "error", err)
		return 1
	}

	checkCtx, checkCancel := context.WithTimeout(context.Background(), 10*time.Second)
	bucketErr := storage.CheckBucket(checkCtx)
	checkCancel()
	if bucketErr != nil {
		logger.Error("bucket check failed", "error", bucketErr)
		return 1
	}

	thumbCfg, err := config.LoadThumbnailConfig()
	if err != nil {
		logger.Error("thumbnail configuration error", "error", err)
		return 1
	}

	cacheCfg, err := config.LoadThumbnailCacheConfig()
	if err != nil {
		logger.Error("thumbnail cache configuration error", "error", err)
		return 1
	}

	thumbCache, err := thumbnail.NewCache(cacheCfg.Dir, cacheCfg.MaxBytes)
	if err != nil {
		logger.Error("thumbnail cache initialization failed", "error", err)
		return 1
	}

	// Build a fresh Prometheus registry so this binary cannot conflict with any
	// process-global default registry (and tests stay panic-free too).
	reg := prometheus.NewRegistry()
	metrics, err := observability.NewMetrics(reg, func() (int64, int, int64) {
		s := thumbCache.Stats()
		return s.Bytes, s.Entries, s.Evictions
	})
	if err != nil {
		logger.Error("metrics registration failed", "error", err)
		return 1
	}

	// Register signal handling before constructing the service so the signal context
	// can be used as the generation lifetime context. Shared generation survives an
	// individual client disconnect but stops cleanly on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	thumbGen := thumbnail.NewGenerator(storage, thumbCfg.GenerationConcurrency)
	thumbSvc := thumbnail.NewServiceWithHooks(ctx, thumbGen, thumbCache, thumbnail.ServiceHooks{
		OnCacheHit:  metrics.ThumbCacheHitsTotal.Inc,
		OnCacheMiss: metrics.ThumbCacheMissesTotal.Inc,
		OnGenDone: func(dur time.Duration, err error) {
			metrics.ThumbGenTotal.Inc()
			metrics.ThumbGenDuration.Observe(dur.Seconds())
			if err != nil {
				metrics.ThumbGenErrorsTotal.Inc()
			}
		},
	})

	srv := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewRouter(httpapi.RouterConfig{
			Logger:         logger,
			AllowedOrigins: cfg.CORSAllowedOrigins,
			APIKey:         cfg.APIKey,
			Repo:           repo,
			Storage:        storage,
			ThumbnailSvc:   thumbSvc,
			Metrics:        metrics,
			MetricsHandler: promhttp.HandlerFor(reg, promhttp.HandlerOpts{
				EnableOpenMetrics: false,
			}),
		}),
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		MaxHeaderBytes:    cfg.HTTPMaxHeaderBytes,
	}

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("server starting", "addr", cfg.HTTPAddr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			return 1
		}
		return 0
	case <-ctx.Done():
	}

	logger.Info("shutdown starting")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
		return 1
	}

	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server error after shutdown", "error", err)
		return 1
	}

	logger.Info("shutdown complete")
	return 0
}

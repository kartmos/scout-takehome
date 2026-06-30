package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"scout/internal/seed"
)

func main() {
	apiURL := flag.String("api-url", envOr("SCOUT_SEED_API_URL", "http://localhost:8080"), "Base URL of the Scout Data API")
	apiKey := flag.String("api-key", os.Getenv("SCOUT_API_KEY"), "API key (X-API-Key header)")
	imagesDir := flag.String("images-dir", envOr("SCOUT_SEED_IMAGES_DIR", "dataset/images"), "Directory containing JPEG photos to upload")
	concurrency := flag.Int("concurrency", envInt("SCOUT_SEED_CONCURRENCY", 2), "Upload concurrency (1–4)")
	timeout := flag.Duration("timeout", envDuration("SCOUT_SEED_TIMEOUT", 60*time.Second), "Per-request timeout")
	flag.Parse()

	cfg := seed.Config{
		APIURL:      *apiURL,
		APIKey:      *apiKey,
		ImagesDir:   *imagesDir,
		Concurrency: *concurrency,
		Timeout:     *timeout,
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	result, err := seed.Run(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	for _, msg := range result.Errors {
		fmt.Fprintf(os.Stderr, "failed: %s\n", msg)
	}

	fmt.Printf("discovered=%d succeeded=%d failed=%d\n", result.Discovered, result.Succeeded, result.Failed)

	if ctx.Err() != nil || result.Failed > 0 {
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

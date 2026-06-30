package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultHTTPAddr              = ":8080"
	DefaultHTTPReadHeaderTimeout = 5 * time.Second
	DefaultHTTPReadTimeout       = 15 * time.Second
	DefaultHTTPWriteTimeout      = 30 * time.Second
	DefaultHTTPIdleTimeout       = 60 * time.Second
	DefaultShutdownTimeout       = 10 * time.Second
	DefaultCORSAllowedOrigins    = "http://localhost:5173"
	DefaultHTTPMaxHeaderBytes    = 65536
	MaxHTTPMaxHeaderBytes        = 1048576
)

type Config struct {
	HTTPAddr              string
	HTTPReadHeaderTimeout time.Duration
	HTTPReadTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	ShutdownTimeout       time.Duration
	// APIKey must never be logged, printed, or included in error messages.
	APIKey             string
	CORSAllowedOrigins []string
	HTTPMaxHeaderBytes int
}

func Load() (*Config, error) {
	addr, ok := os.LookupEnv("SCOUT_HTTP_ADDR")
	if !ok {
		addr = DefaultHTTPAddr
	}
	if addr == "" {
		return nil, fmt.Errorf("SCOUT_HTTP_ADDR must not be empty")
	}

	readHeaderTimeout, err := loadDuration("SCOUT_HTTP_READ_HEADER_TIMEOUT", DefaultHTTPReadHeaderTimeout)
	if err != nil {
		return nil, err
	}
	readTimeout, err := loadDuration("SCOUT_HTTP_READ_TIMEOUT", DefaultHTTPReadTimeout)
	if err != nil {
		return nil, err
	}
	writeTimeout, err := loadDuration("SCOUT_HTTP_WRITE_TIMEOUT", DefaultHTTPWriteTimeout)
	if err != nil {
		return nil, err
	}
	idleTimeout, err := loadDuration("SCOUT_HTTP_IDLE_TIMEOUT", DefaultHTTPIdleTimeout)
	if err != nil {
		return nil, err
	}
	shutdownTimeout, err := loadDuration("SCOUT_SHUTDOWN_TIMEOUT", DefaultShutdownTimeout)
	if err != nil {
		return nil, err
	}

	// API key is required; whitespace-only values are rejected.
	// The value is never included in any error message.
	apiKey := os.Getenv("SCOUT_API_KEY")
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("SCOUT_API_KEY is required and must not be empty or whitespace-only")
	}

	originsRaw, ok := os.LookupEnv("SCOUT_CORS_ALLOWED_ORIGINS")
	if !ok || originsRaw == "" {
		originsRaw = DefaultCORSAllowedOrigins
	}
	origins, err := parseCORSOrigins(originsRaw)
	if err != nil {
		return nil, err
	}

	maxHeaderBytes, err := loadMaxHeaderBytes()
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTPAddr:              addr,
		HTTPReadHeaderTimeout: readHeaderTimeout,
		HTTPReadTimeout:       readTimeout,
		HTTPWriteTimeout:      writeTimeout,
		HTTPIdleTimeout:       idleTimeout,
		ShutdownTimeout:       shutdownTimeout,
		APIKey:                apiKey,
		CORSAllowedOrigins:    origins,
		HTTPMaxHeaderBytes:    maxHeaderBytes,
	}, nil
}

func loadDuration(name string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", name, v, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s: duration must be positive, got %q", name, v)
	}
	return d, nil
}

func loadMaxHeaderBytes() (int, error) {
	v := os.Getenv("SCOUT_HTTP_MAX_HEADER_BYTES")
	if v == "" {
		return DefaultHTTPMaxHeaderBytes, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("SCOUT_HTTP_MAX_HEADER_BYTES: invalid integer %q: %w", v, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("SCOUT_HTTP_MAX_HEADER_BYTES: must be positive, got %d", n)
	}
	if n > MaxHTTPMaxHeaderBytes {
		return 0, fmt.Errorf("SCOUT_HTTP_MAX_HEADER_BYTES: must not exceed %d, got %d", MaxHTTPMaxHeaderBytes, n)
	}
	return n, nil
}

// parseCORSOrigins splits raw by comma, trims entries, validates each as an
// exact http/https origin, deduplicates, and returns a fresh slice.
func parseCORSOrigins(raw string) ([]string, error) {
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{})
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		origin := strings.TrimSpace(p)
		if origin == "" {
			continue
		}
		if err := validateOrigin(origin); err != nil {
			return nil, fmt.Errorf("SCOUT_CORS_ALLOWED_ORIGINS: %w", err)
		}
		if _, dup := seen[origin]; !dup {
			seen[origin] = struct{}{}
			out = append(out, origin)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("SCOUT_CORS_ALLOWED_ORIGINS: at least one valid origin is required")
	}
	return out, nil
}

// validateOrigin checks that origin is an exact http or https origin:
// scheme + host only, no credentials, no path beyond "/", no query, no fragment.
func validateOrigin(origin string) error {
	u, err := url.Parse(origin)
	if err != nil {
		return fmt.Errorf("malformed origin %q: %w", origin, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("origin %q must use http or https scheme", origin)
	}
	if u.Host == "" {
		return fmt.Errorf("origin %q must have a host", origin)
	}
	if u.User != nil {
		return fmt.Errorf("origin %q must not contain credentials", origin)
	}
	if u.Path != "" && u.Path != "/" {
		return fmt.Errorf("origin %q must not have a path", origin)
	}
	if u.RawQuery != "" {
		return fmt.Errorf("origin %q must not have a query string", origin)
	}
	if u.Fragment != "" {
		return fmt.Errorf("origin %q must not have a fragment", origin)
	}
	return nil
}

package config

import (
	"fmt"
	"os"
	"time"
)

const (
	DefaultHTTPAddr              = ":8080"
	DefaultHTTPReadHeaderTimeout = 5 * time.Second
	DefaultHTTPReadTimeout       = 15 * time.Second
	DefaultHTTPWriteTimeout      = 30 * time.Second
	DefaultHTTPIdleTimeout       = 60 * time.Second
	DefaultShutdownTimeout       = 10 * time.Second
)

type Config struct {
	HTTPAddr              string
	HTTPReadHeaderTimeout time.Duration
	HTTPReadTimeout       time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration
	ShutdownTimeout       time.Duration
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

	return &Config{
		HTTPAddr:              addr,
		HTTPReadHeaderTimeout: readHeaderTimeout,
		HTTPReadTimeout:       readTimeout,
		HTTPWriteTimeout:      writeTimeout,
		HTTPIdleTimeout:       idleTimeout,
		ShutdownTimeout:       shutdownTimeout,
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

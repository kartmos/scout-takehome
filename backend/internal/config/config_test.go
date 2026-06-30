package config_test

import (
	"os"
	"testing"
	"time"

	"scout/internal/config"
)

// unsetForTest unsets key for the test and restores its previous state on cleanup.
func unsetForTest(t *testing.T, key string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	_ = os.Unsetenv(key)
	t.Cleanup(func() {
		if had {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	})
}

func TestLoad_Defaults(t *testing.T) {
	unsetForTest(t, "SCOUT_HTTP_ADDR")
	t.Setenv("SCOUT_HTTP_READ_HEADER_TIMEOUT", "")
	t.Setenv("SCOUT_HTTP_READ_TIMEOUT", "")
	t.Setenv("SCOUT_HTTP_WRITE_TIMEOUT", "")
	t.Setenv("SCOUT_HTTP_IDLE_TIMEOUT", "")
	t.Setenv("SCOUT_SHUTDOWN_TIMEOUT", "")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.HTTPAddr, config.DefaultHTTPAddr; got != want {
		t.Errorf("HTTPAddr = %q, want %q", got, want)
	}
	if got, want := cfg.HTTPReadHeaderTimeout, config.DefaultHTTPReadHeaderTimeout; got != want {
		t.Errorf("HTTPReadHeaderTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.HTTPReadTimeout, config.DefaultHTTPReadTimeout; got != want {
		t.Errorf("HTTPReadTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.HTTPWriteTimeout, config.DefaultHTTPWriteTimeout; got != want {
		t.Errorf("HTTPWriteTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.HTTPIdleTimeout, config.DefaultHTTPIdleTimeout; got != want {
		t.Errorf("HTTPIdleTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.ShutdownTimeout, config.DefaultShutdownTimeout; got != want {
		t.Errorf("ShutdownTimeout = %v, want %v", got, want)
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("SCOUT_HTTP_ADDR", ":9090")
	t.Setenv("SCOUT_HTTP_READ_HEADER_TIMEOUT", "3s")
	t.Setenv("SCOUT_HTTP_READ_TIMEOUT", "10s")
	t.Setenv("SCOUT_HTTP_WRITE_TIMEOUT", "20s")
	t.Setenv("SCOUT_HTTP_IDLE_TIMEOUT", "45s")
	t.Setenv("SCOUT_SHUTDOWN_TIMEOUT", "7s")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.HTTPAddr, ":9090"; got != want {
		t.Errorf("HTTPAddr = %q, want %q", got, want)
	}
	if got, want := cfg.HTTPReadHeaderTimeout, 3*time.Second; got != want {
		t.Errorf("HTTPReadHeaderTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.HTTPReadTimeout, 10*time.Second; got != want {
		t.Errorf("HTTPReadTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.HTTPWriteTimeout, 20*time.Second; got != want {
		t.Errorf("HTTPWriteTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.HTTPIdleTimeout, 45*time.Second; got != want {
		t.Errorf("HTTPIdleTimeout = %v, want %v", got, want)
	}
	if got, want := cfg.ShutdownTimeout, 7*time.Second; got != want {
		t.Errorf("ShutdownTimeout = %v, want %v", got, want)
	}
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name  string
		env   string
		value string
	}{
		{name: "empty_addr", env: "SCOUT_HTTP_ADDR", value: ""},
		{name: "malformed_read_header_timeout", env: "SCOUT_HTTP_READ_HEADER_TIMEOUT", value: "abc"},
		{name: "malformed_read_timeout", env: "SCOUT_HTTP_READ_TIMEOUT", value: "not-a-duration"},
		{name: "zero_write_timeout", env: "SCOUT_HTTP_WRITE_TIMEOUT", value: "0s"},
		{name: "negative_idle_timeout", env: "SCOUT_HTTP_IDLE_TIMEOUT", value: "-1s"},
		{name: "zero_shutdown_timeout", env: "SCOUT_SHUTDOWN_TIMEOUT", value: "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.env, tt.value)
			_, err := config.Load()
			if err == nil {
				t.Errorf("Load() error = nil, want non-nil for %s=%q", tt.env, tt.value)
			}
		})
	}
}

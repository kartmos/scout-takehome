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
	t.Setenv("SCOUT_API_KEY", "test-api-key-for-defaults")
	unsetForTest(t, "SCOUT_CORS_ALLOWED_ORIGINS")
	unsetForTest(t, "SCOUT_HTTP_MAX_HEADER_BYTES")

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
	if got := cfg.CORSAllowedOrigins; len(got) != 1 || got[0] != "http://localhost:5173" {
		t.Errorf("CORSAllowedOrigins = %v, want [http://localhost:5173]", got)
	}
	if got, want := cfg.HTTPMaxHeaderBytes, config.DefaultHTTPMaxHeaderBytes; got != want {
		t.Errorf("HTTPMaxHeaderBytes = %d, want %d", got, want)
	}
}

func TestLoad_Overrides(t *testing.T) {
	t.Setenv("SCOUT_HTTP_ADDR", ":9090")
	t.Setenv("SCOUT_HTTP_READ_HEADER_TIMEOUT", "3s")
	t.Setenv("SCOUT_HTTP_READ_TIMEOUT", "10s")
	t.Setenv("SCOUT_HTTP_WRITE_TIMEOUT", "20s")
	t.Setenv("SCOUT_HTTP_IDLE_TIMEOUT", "45s")
	t.Setenv("SCOUT_SHUTDOWN_TIMEOUT", "7s")
	t.Setenv("SCOUT_API_KEY", "test-override-key")
	t.Setenv("SCOUT_CORS_ALLOWED_ORIGINS", "https://example.com, http://app.local:3000")
	t.Setenv("SCOUT_HTTP_MAX_HEADER_BYTES", "131072")

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
	wantOrigins := []string{"https://example.com", "http://app.local:3000"}
	if got := cfg.CORSAllowedOrigins; len(got) != len(wantOrigins) ||
		got[0] != wantOrigins[0] || got[1] != wantOrigins[1] {
		t.Errorf("CORSAllowedOrigins = %v, want %v", got, wantOrigins)
	}
	if got, want := cfg.HTTPMaxHeaderBytes, 131072; got != want {
		t.Errorf("HTTPMaxHeaderBytes = %d, want %d", got, want)
	}
}

func TestLoad_Errors(t *testing.T) {
	// Set a valid API key so each subtest fails only for its intended reason.
	t.Setenv("SCOUT_API_KEY", "test-key-for-error-tests")

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
		{name: "zero_max_header_bytes", env: "SCOUT_HTTP_MAX_HEADER_BYTES", value: "0"},
		{name: "negative_max_header_bytes", env: "SCOUT_HTTP_MAX_HEADER_BYTES", value: "-1"},
		{name: "malformed_max_header_bytes", env: "SCOUT_HTTP_MAX_HEADER_BYTES", value: "abc"},
		{name: "too_large_max_header_bytes", env: "SCOUT_HTTP_MAX_HEADER_BYTES", value: "1048577"},
		{name: "malformed_cors_origin", env: "SCOUT_CORS_ALLOWED_ORIGINS", value: "not-a-url"},
		{name: "empty_cors_origins", env: "SCOUT_CORS_ALLOWED_ORIGINS", value: "   "},
		{name: "cors_with_path", env: "SCOUT_CORS_ALLOWED_ORIGINS", value: "http://example.com/path"},
		{name: "cors_with_query", env: "SCOUT_CORS_ALLOWED_ORIGINS", value: "http://example.com?q=1"},
		{name: "cors_non_http_scheme", env: "SCOUT_CORS_ALLOWED_ORIGINS", value: "ftp://example.com"},
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

func TestLoad_APIKey(t *testing.T) {
	t.Setenv("SCOUT_CORS_ALLOWED_ORIGINS", "http://localhost:5173")
	t.Setenv("SCOUT_HTTP_MAX_HEADER_BYTES", "")

	tests := []struct {
		name    string
		value   string
		unset   bool
		wantErr bool
	}{
		{name: "missing", unset: true, wantErr: true},
		{name: "empty", value: "", wantErr: true},
		{name: "whitespace_only", value: "   ", wantErr: true},
		{name: "tab_only", value: "\t", wantErr: true},
		{name: "valid", value: "valid-api-key-abc123", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unset {
				unsetForTest(t, "SCOUT_API_KEY")
			} else {
				t.Setenv("SCOUT_API_KEY", tt.value)
			}
			_, err := config.Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoad_CORSAllowedOrigins(t *testing.T) {
	t.Setenv("SCOUT_API_KEY", "test-cors-key")
	t.Setenv("SCOUT_HTTP_MAX_HEADER_BYTES", "")

	tests := []struct {
		name        string
		value       string
		wantOrigins []string
		wantErr     bool
	}{
		{
			name:        "single_valid",
			value:       "http://localhost:3000",
			wantOrigins: []string{"http://localhost:3000"},
		},
		{
			name:        "multiple_valid",
			value:       "http://localhost:3000,https://app.example.com",
			wantOrigins: []string{"http://localhost:3000", "https://app.example.com"},
		},
		{
			name:        "trims_whitespace",
			value:       " http://localhost:3000 , https://app.example.com ",
			wantOrigins: []string{"http://localhost:3000", "https://app.example.com"},
		},
		{
			name:        "deduplicates",
			value:       "http://localhost:3000,http://localhost:3000,https://example.com",
			wantOrigins: []string{"http://localhost:3000", "https://example.com"},
		},
		{
			name:        "with_port",
			value:       "http://localhost:5173",
			wantOrigins: []string{"http://localhost:5173"},
		},
		{name: "no_scheme", value: "example.com", wantErr: true},
		{name: "ftp_scheme", value: "ftp://example.com", wantErr: true},
		{name: "with_path", value: "http://example.com/api", wantErr: true},
		{name: "with_query", value: "http://example.com?x=1", wantErr: true},
		{name: "with_fragment", value: "http://example.com#section", wantErr: true},
		{name: "with_credentials", value: "http://user:pass@example.com", wantErr: true},
		{name: "empty_after_trim", value: "  ,  ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SCOUT_CORS_ALLOWED_ORIGINS", tt.value)
			cfg, err := config.Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(cfg.CORSAllowedOrigins) != len(tt.wantOrigins) {
				t.Fatalf("CORSAllowedOrigins len = %d, want %d", len(cfg.CORSAllowedOrigins), len(tt.wantOrigins))
			}
			for i, want := range tt.wantOrigins {
				if cfg.CORSAllowedOrigins[i] != want {
					t.Errorf("CORSAllowedOrigins[%d] = %q, want %q", i, cfg.CORSAllowedOrigins[i], want)
				}
			}
		})
	}
}

func TestLoad_MaxHeaderBytes(t *testing.T) {
	t.Setenv("SCOUT_API_KEY", "test-maxheader-key")
	t.Setenv("SCOUT_CORS_ALLOWED_ORIGINS", "http://localhost:5173")

	tests := []struct {
		name    string
		value   string
		want    int
		wantErr bool
	}{
		{name: "default_when_empty", value: "", want: config.DefaultHTTPMaxHeaderBytes},
		{name: "valid_small", value: "4096", want: 4096},
		{name: "valid_max", value: "1048576", want: config.MaxHTTPMaxHeaderBytes},
		{name: "zero", value: "0", wantErr: true},
		{name: "negative", value: "-1", wantErr: true},
		{name: "above_max", value: "1048577", wantErr: true},
		{name: "malformed", value: "not-a-number", wantErr: true},
		{name: "float", value: "65536.5", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SCOUT_HTTP_MAX_HEADER_BYTES", tt.value)
			cfg, err := config.Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if cfg.HTTPMaxHeaderBytes != tt.want {
				t.Errorf("HTTPMaxHeaderBytes = %d, want %d", cfg.HTTPMaxHeaderBytes, tt.want)
			}
		})
	}
}

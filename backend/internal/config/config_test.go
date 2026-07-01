package config_test

import (
	"os"
	"strings"
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
	t.Setenv("SCOUT_DATABASE_PATH", "dataset/predictions.db")

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
	if got, want := cfg.DatabasePath, "dataset/predictions.db"; got != want {
		t.Errorf("DatabasePath = %q, want %q", got, want)
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
	t.Setenv("SCOUT_DATABASE_PATH", "/data/scout.db")

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
	if got, want := cfg.DatabasePath, "/data/scout.db"; got != want {
		t.Errorf("DatabasePath = %q, want %q", got, want)
	}
}

func TestLoad_Errors(t *testing.T) {
	// Set valid base env so each subtest fails only for its intended reason.
	t.Setenv("SCOUT_API_KEY", "test-key-for-error-tests")
	t.Setenv("SCOUT_DATABASE_PATH", "dataset/predictions.db")

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
	t.Setenv("SCOUT_DATABASE_PATH", "dataset/predictions.db")

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
	t.Setenv("SCOUT_DATABASE_PATH", "dataset/predictions.db")

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
	t.Setenv("SCOUT_DATABASE_PATH", "dataset/predictions.db")

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

func TestLoad_DatabasePath(t *testing.T) {
	t.Setenv("SCOUT_API_KEY", "test-db-key")
	t.Setenv("SCOUT_CORS_ALLOWED_ORIGINS", "http://localhost:5173")

	tests := []struct {
		name    string
		value   string
		unset   bool
		wantErr bool
	}{
		{name: "valid_relative", value: "dataset/predictions.db", wantErr: false},
		{name: "valid_absolute", value: "/var/data/scout.db", wantErr: false},
		{name: "missing", unset: true, wantErr: true},
		{name: "empty", value: "", wantErr: true},
		{name: "whitespace_only", value: "   ", wantErr: true},
		{name: "tab_only", value: "\t", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unset {
				unsetForTest(t, "SCOUT_DATABASE_PATH")
			} else {
				t.Setenv("SCOUT_DATABASE_PATH", tt.value)
			}
			cfg, err := config.Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr || cfg == nil {
				return
			}
			want := strings.TrimSpace(tt.value)
			if cfg.DatabasePath != want {
				t.Errorf("DatabasePath = %q, want %q", cfg.DatabasePath, want)
			}
		})
	}
}

// setValidS3Env sets all required S3 env vars to valid development values.
func setValidS3Env(t *testing.T) {
	t.Helper()
	t.Setenv("SCOUT_S3_ENDPOINT", "localhost:9000")
	t.Setenv("SCOUT_S3_ACCESS_KEY", "test-access-key")
	t.Setenv("SCOUT_S3_SECRET_KEY", "test-secret-key-value")
	t.Setenv("SCOUT_S3_BUCKET", "scout-photos")
	t.Setenv("SCOUT_S3_SECURE", "false")
	unsetForTest(t, "SCOUT_S3_REGION")
	unsetForTest(t, "SCOUT_S3_UPLOAD_TTL")
	unsetForTest(t, "SCOUT_S3_DOWNLOAD_TTL")
}

func TestLoadS3Config_Defaults(t *testing.T) {
	setValidS3Env(t)
	cfg, err := config.LoadS3Config()
	if err != nil {
		t.Fatalf("LoadS3Config() error = %v", err)
	}
	if cfg.Region != config.DefaultS3Region {
		t.Errorf("Region = %q, want %q", cfg.Region, config.DefaultS3Region)
	}
	if cfg.UploadTTL != config.DefaultS3UploadTTL {
		t.Errorf("UploadTTL = %v, want %v", cfg.UploadTTL, config.DefaultS3UploadTTL)
	}
	if cfg.DownloadTTL != config.DefaultS3DownloadTTL {
		t.Errorf("DownloadTTL = %v, want %v", cfg.DownloadTTL, config.DefaultS3DownloadTTL)
	}
	if cfg.Secure {
		t.Error("Secure = true, want false")
	}
}

func TestLoadS3Config_Overrides(t *testing.T) {
	setValidS3Env(t)
	t.Setenv("SCOUT_S3_ENDPOINT", "s3.example.com:443")
	t.Setenv("SCOUT_S3_BUCKET", "my-bucket-01")
	t.Setenv("SCOUT_S3_SECURE", "true")
	t.Setenv("SCOUT_S3_REGION", "eu-west-1")
	t.Setenv("SCOUT_S3_UPLOAD_TTL", "30m")
	t.Setenv("SCOUT_S3_DOWNLOAD_TTL", "5m")

	cfg, err := config.LoadS3Config()
	if err != nil {
		t.Fatalf("LoadS3Config() error = %v", err)
	}
	if cfg.Endpoint != "s3.example.com:443" {
		t.Errorf("Endpoint = %q", cfg.Endpoint)
	}
	if cfg.Bucket != "my-bucket-01" {
		t.Errorf("Bucket = %q", cfg.Bucket)
	}
	if !cfg.Secure {
		t.Error("Secure = false, want true")
	}
	if cfg.Region != "eu-west-1" {
		t.Errorf("Region = %q", cfg.Region)
	}
	if cfg.UploadTTL != 30*time.Minute {
		t.Errorf("UploadTTL = %v, want 30m", cfg.UploadTTL)
	}
	if cfg.DownloadTTL != 5*time.Minute {
		t.Errorf("DownloadTTL = %v, want 5m", cfg.DownloadTTL)
	}
}

func TestLoadS3Config_Endpoint(t *testing.T) {
	setValidS3Env(t)
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "bare_host", value: "localhost", wantErr: false},
		{name: "host_port", value: "localhost:9000", wantErr: false},
		{name: "hostname_port", value: "minio.internal:9000", wantErr: false},
		{name: "empty", value: "", wantErr: true},
		{name: "whitespace_only", value: "   ", wantErr: true},
		{name: "with_scheme_http", value: "http://localhost:9000", wantErr: true},
		{name: "with_scheme_https", value: "https://s3.example.com", wantErr: true},
		{name: "with_credentials", value: "user@localhost:9000", wantErr: true},
		{name: "trailing_slash", value: "localhost:9000/", wantErr: true},
		{name: "with_path", value: "localhost:9000/bucket", wantErr: true},
		{name: "with_query", value: "localhost:9000?region=us", wantErr: true},
		{name: "with_fragment", value: "localhost:9000#section", wantErr: true},
		{name: "trailing_colon", value: "localhost:", wantErr: true},
		{name: "invalid_port_alpha", value: "localhost:abc", wantErr: true},
		{name: "invalid_port_zero", value: "localhost:0", wantErr: true},
		{name: "invalid_port_large", value: "localhost:99999", wantErr: true},
		{name: "embedded_space", value: "local host:9000", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SCOUT_S3_ENDPOINT", tt.value)
			_, err := config.LoadS3Config()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadS3Config() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadS3Config_Bucket(t *testing.T) {
	setValidS3Env(t)
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid_simple", value: "mybucket", wantErr: false},
		{name: "valid_with_hyphen", value: "scout-photos", wantErr: false},
		{name: "valid_with_dot", value: "my.bucket", wantErr: false},
		{name: "valid_min_length", value: "abc", wantErr: false},
		{name: "valid_digits", value: "bucket01", wantErr: false},
		{name: "too_short", value: "ab", wantErr: true},
		{name: "empty", value: "", wantErr: true},
		{name: "uppercase", value: "MyBucket", wantErr: true},
		{name: "starts_with_hyphen", value: "-bucket", wantErr: true},
		{name: "ends_with_hyphen", value: "bucket-", wantErr: true},
		{name: "starts_with_dot", value: ".bucket", wantErr: true},
		{name: "ends_with_dot", value: "bucket.", wantErr: true},
		{name: "adjacent_dots", value: "my..bucket", wantErr: true},
		{name: "ip_address", value: "192.168.1.1", wantErr: true},
		{name: "contains_underscore", value: "my_bucket", wantErr: true},
		{name: "contains_space", value: "my bucket", wantErr: true},
		{name: "too_long", value: strings.Repeat("a", 64), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SCOUT_S3_BUCKET", tt.value)
			_, err := config.LoadS3Config()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadS3Config() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadS3Config_Secure(t *testing.T) {
	setValidS3Env(t)
	tests := []struct {
		name    string
		value   string
		want    bool
		wantErr bool
	}{
		{name: "true", value: "true", want: true},
		{name: "false", value: "false", want: false},
		{name: "one", value: "1", want: true},
		{name: "zero", value: "0", want: false},
		{name: "TRUE", value: "TRUE", want: true},
		{name: "FALSE", value: "FALSE", want: false},
		{name: "empty", value: "", wantErr: true},
		{name: "malformed_yes", value: "yes", wantErr: true},
		{name: "malformed_no", value: "no", wantErr: true},
		{name: "malformed_on", value: "on", wantErr: true},
		{name: "malformed_number", value: "2", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SCOUT_S3_SECURE", tt.value)
			cfg, err := config.LoadS3Config()
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadS3Config() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if cfg.Secure != tt.want {
				t.Errorf("Secure = %v, want %v", cfg.Secure, tt.want)
			}
		})
	}
}

func TestLoadS3Config_TTL(t *testing.T) {
	setValidS3Env(t)
	tests := []struct {
		name    string
		env     string
		value   string
		want    time.Duration
		wantErr bool
	}{
		{name: "upload_default", env: "SCOUT_S3_UPLOAD_TTL", value: "", want: config.DefaultS3UploadTTL},
		{name: "upload_30m", env: "SCOUT_S3_UPLOAD_TTL", value: "30m", want: 30 * time.Minute},
		{name: "upload_1s_min", env: "SCOUT_S3_UPLOAD_TTL", value: "1s", want: time.Second},
		{name: "upload_7d_max", env: "SCOUT_S3_UPLOAD_TTL", value: "168h", want: 168 * time.Hour},
		{name: "download_default", env: "SCOUT_S3_DOWNLOAD_TTL", value: "", want: config.DefaultS3DownloadTTL},
		{name: "download_5m", env: "SCOUT_S3_DOWNLOAD_TTL", value: "5m", want: 5 * time.Minute},
		{name: "upload_too_short", env: "SCOUT_S3_UPLOAD_TTL", value: "500ms", wantErr: true},
		{name: "upload_too_long", env: "SCOUT_S3_UPLOAD_TTL", value: "169h", wantErr: true},
		{name: "upload_zero", env: "SCOUT_S3_UPLOAD_TTL", value: "0s", wantErr: true},
		{name: "upload_negative", env: "SCOUT_S3_UPLOAD_TTL", value: "-1m", wantErr: true},
		{name: "upload_malformed", env: "SCOUT_S3_UPLOAD_TTL", value: "abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.env, tt.value)
			cfg, err := config.LoadS3Config()
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadS3Config() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			var got time.Duration
			if tt.env == "SCOUT_S3_UPLOAD_TTL" {
				got = cfg.UploadTTL
			} else {
				got = cfg.DownloadTTL
			}
			if got != tt.want {
				t.Errorf("%s = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestLoadS3Config_Credentials(t *testing.T) {
	setValidS3Env(t)
	tests := []struct {
		name    string
		env     string
		value   string
		wantErr bool
	}{
		{name: "access_key_missing", env: "SCOUT_S3_ACCESS_KEY", value: "", wantErr: true},
		{name: "access_key_whitespace", env: "SCOUT_S3_ACCESS_KEY", value: "   ", wantErr: true},
		{name: "secret_key_missing", env: "SCOUT_S3_SECRET_KEY", value: "", wantErr: true},
		{name: "secret_key_whitespace", env: "SCOUT_S3_SECRET_KEY", value: "\t", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.env, tt.value)
			_, err := config.LoadS3Config()
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadS3Config() error = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadS3Config_PublicEndpointDefaults(t *testing.T) {
	setValidS3Env(t)
	unsetForTest(t, "SCOUT_S3_PUBLIC_ENDPOINT")
	unsetForTest(t, "SCOUT_S3_PUBLIC_SECURE")

	cfg, err := config.LoadS3Config()
	if err != nil {
		t.Fatalf("LoadS3Config() error = %v", err)
	}
	if cfg.PublicEndpoint != cfg.Endpoint {
		t.Errorf("PublicEndpoint = %q, want %q (same as Endpoint)", cfg.PublicEndpoint, cfg.Endpoint)
	}
	if cfg.PublicSecure != cfg.Secure {
		t.Errorf("PublicSecure = %v, want %v (same as Secure)", cfg.PublicSecure, cfg.Secure)
	}
}

func TestLoadS3Config_PublicEndpoint(t *testing.T) {
	setValidS3Env(t)

	tests := []struct {
		name         string
		endpoint     string
		secure       string
		wantEndpoint string
		wantSecure   bool
		wantErr      bool
	}{
		{
			name:         "valid_host_port",
			endpoint:     "minio.localhost:9000",
			secure:       "false",
			wantEndpoint: "minio.localhost:9000",
			wantSecure:   false,
		},
		{
			name:         "secure_public",
			endpoint:     "s3.example.com:443",
			secure:       "true",
			wantEndpoint: "s3.example.com:443",
			wantSecure:   true,
		},
		{
			name:         "inherits_internal_secure_when_public_secure_unset",
			endpoint:     "minio.localhost:9000",
			wantEndpoint: "minio.localhost:9000",
			wantSecure:   false, // internal SCOUT_S3_SECURE is "false" from setValidS3Env
		},
		{name: "with_scheme", endpoint: "http://minio.localhost:9000", wantErr: true},
		{name: "with_path", endpoint: "minio.localhost:9000/bucket", wantErr: true},
		{name: "with_query", endpoint: "minio.localhost:9000?x=1", wantErr: true},
		{name: "with_credentials", endpoint: "user@minio.localhost:9000", wantErr: true},
		{name: "invalid_port", endpoint: "minio.localhost:99999", wantErr: true},
		{name: "whitespace", endpoint: "  minio.localhost:9000  ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SCOUT_S3_PUBLIC_ENDPOINT", tt.endpoint)
			if tt.secure != "" {
				t.Setenv("SCOUT_S3_PUBLIC_SECURE", tt.secure)
			} else {
				unsetForTest(t, "SCOUT_S3_PUBLIC_SECURE")
			}

			cfg, err := config.LoadS3Config()
			if (err != nil) != tt.wantErr {
				t.Fatalf("LoadS3Config() error = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if cfg.PublicEndpoint != tt.wantEndpoint {
				t.Errorf("PublicEndpoint = %q, want %q", cfg.PublicEndpoint, tt.wantEndpoint)
			}
			if cfg.PublicSecure != tt.wantSecure {
				t.Errorf("PublicSecure = %v, want %v", cfg.PublicSecure, tt.wantSecure)
			}
		})
	}
}

func TestLoadS3Config_PublicSecureIgnoredWithoutPublicEndpoint(t *testing.T) {
	setValidS3Env(t)
	unsetForTest(t, "SCOUT_S3_PUBLIC_ENDPOINT")
	// SCOUT_S3_PUBLIC_SECURE set but SCOUT_S3_PUBLIC_ENDPOINT absent — public values must equal internal.
	t.Setenv("SCOUT_S3_PUBLIC_SECURE", "true")

	cfg, err := config.LoadS3Config()
	if err != nil {
		t.Fatalf("LoadS3Config() error = %v", err)
	}
	if cfg.PublicEndpoint != cfg.Endpoint {
		t.Errorf("PublicEndpoint = %q, want %q", cfg.PublicEndpoint, cfg.Endpoint)
	}
	// PublicSecure must track internal Secure (false), not the ignored SCOUT_S3_PUBLIC_SECURE.
	if cfg.PublicSecure != cfg.Secure {
		t.Errorf("PublicSecure = %v, want %v (must mirror internal Secure when PUBLIC_ENDPOINT absent)", cfg.PublicSecure, cfg.Secure)
	}
}

func TestLoadS3Config_SecretNotLeaked(t *testing.T) {
	setValidS3Env(t)
	secretValue := "super-secret-credential-abc123"
	t.Setenv("SCOUT_S3_SECRET_KEY", secretValue)
	t.Setenv("SCOUT_S3_ACCESS_KEY", "") // force an error so we get an error message to inspect

	_, err := config.LoadS3Config()
	if err == nil {
		t.Fatal("expected error when access key is empty")
	}
	if strings.Contains(err.Error(), secretValue) {
		t.Errorf("error message must not contain the secret key value: %q", err.Error())
	}
}

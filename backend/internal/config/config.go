package config

import (
	"fmt"
	"net"
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
	DatabasePath       string
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

	dbPath := strings.TrimSpace(os.Getenv("SCOUT_DATABASE_PATH"))
	if dbPath == "" {
		return nil, fmt.Errorf("SCOUT_DATABASE_PATH is required and must not be empty or whitespace-only")
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
		DatabasePath:          dbPath,
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

// S3Config holds the object storage configuration.
type S3Config struct {
	Endpoint    string
	AccessKey   string // never log or include in error messages
	SecretKey   string // never log or include in error messages
	Bucket      string
	Secure      bool
	Region      string
	UploadTTL   time.Duration
	DownloadTTL time.Duration
	// PublicEndpoint is the host[:port] embedded in presigned URLs so browsers and
	// the seed container can reach them. Defaults to Endpoint when unset.
	// Never used for bucket checks or object reads.
	PublicEndpoint string
	// PublicSecure enables TLS in presigned URLs. Defaults to Secure when unset.
	PublicSecure bool
}

const (
	DefaultS3Region      = "us-east-1"
	DefaultS3UploadTTL   = 15 * time.Minute
	DefaultS3DownloadTTL = 15 * time.Minute
	minS3TTL             = time.Second
	maxS3TTL             = 7 * 24 * time.Hour
)

// LoadS3Config loads and validates S3/MinIO configuration from environment variables.
// Credential values are never included in returned error messages.
func LoadS3Config() (*S3Config, error) {
	endpoint := os.Getenv("SCOUT_S3_ENDPOINT")
	if err := validateBareHostPort("SCOUT_S3_ENDPOINT", endpoint); err != nil {
		return nil, err
	}

	accessKey := os.Getenv("SCOUT_S3_ACCESS_KEY")
	if strings.TrimSpace(accessKey) == "" {
		return nil, fmt.Errorf("SCOUT_S3_ACCESS_KEY is required and must not be empty")
	}

	secretKey := os.Getenv("SCOUT_S3_SECRET_KEY")
	if strings.TrimSpace(secretKey) == "" {
		return nil, fmt.Errorf("SCOUT_S3_SECRET_KEY is required and must not be empty")
	}

	bucket := os.Getenv("SCOUT_S3_BUCKET")
	if err := validateS3BucketName(bucket); err != nil {
		return nil, err
	}

	secure, err := loadS3Bool("SCOUT_S3_SECURE")
	if err != nil {
		return nil, err
	}

	region := os.Getenv("SCOUT_S3_REGION")
	if region == "" {
		region = DefaultS3Region
	}

	uploadTTL, err := loadS3TTL("SCOUT_S3_UPLOAD_TTL", DefaultS3UploadTTL)
	if err != nil {
		return nil, err
	}

	downloadTTL, err := loadS3TTL("SCOUT_S3_DOWNLOAD_TTL", DefaultS3DownloadTTL)
	if err != nil {
		return nil, err
	}

	// Optional public endpoint for presigned URLs. When absent, defaults to internal.
	publicEndpoint, publicSecure, err := loadPublicS3Endpoint(endpoint, secure)
	if err != nil {
		return nil, err
	}

	return &S3Config{
		Endpoint:       endpoint,
		AccessKey:      accessKey,
		SecretKey:      secretKey,
		Bucket:         bucket,
		Secure:         secure,
		Region:         region,
		UploadTTL:      uploadTTL,
		DownloadTTL:    downloadTTL,
		PublicEndpoint: publicEndpoint,
		PublicSecure:   publicSecure,
	}, nil
}

// loadPublicS3Endpoint reads SCOUT_S3_PUBLIC_ENDPOINT and SCOUT_S3_PUBLIC_SECURE.
// When SCOUT_S3_PUBLIC_ENDPOINT is absent both default to the internal values.
func loadPublicS3Endpoint(internalEP string, internalSecure bool) (string, bool, error) {
	ep := os.Getenv("SCOUT_S3_PUBLIC_ENDPOINT")
	if ep == "" {
		return internalEP, internalSecure, nil
	}
	if err := validateBareHostPort("SCOUT_S3_PUBLIC_ENDPOINT", ep); err != nil {
		return "", false, err
	}
	secure := internalSecure
	if sv := os.Getenv("SCOUT_S3_PUBLIC_SECURE"); sv != "" {
		b, err := strconv.ParseBool(sv)
		if err != nil {
			return "", false, fmt.Errorf("SCOUT_S3_PUBLIC_SECURE: must be a boolean (true/false/1/0), got %q", sv)
		}
		secure = b
	}
	return ep, secure, nil
}

// validateBareHostPort ensures varName's value is a bare host[:port] with no scheme,
// credentials, path, query, fragment, or whitespace.
func validateBareHostPort(varName, ep string) error {
	if strings.TrimSpace(ep) == "" {
		return fmt.Errorf("%s must not be empty or whitespace-only", varName)
	}
	if strings.ContainsAny(ep, " \t\n\r") {
		return fmt.Errorf("%s must not contain whitespace", varName)
	}
	if strings.Contains(ep, "://") {
		return fmt.Errorf("%s must not include a scheme (use host:port form)", varName)
	}
	u, err := url.Parse("dummy://" + ep)
	if err != nil {
		return fmt.Errorf("%s is malformed: %w", varName, err)
	}
	if u.User != nil {
		return fmt.Errorf("%s must not include credentials", varName)
	}
	if u.Path != "" {
		return fmt.Errorf("%s must not include a path", varName)
	}
	if u.RawQuery != "" {
		return fmt.Errorf("%s must not include a query string", varName)
	}
	if u.Fragment != "" {
		return fmt.Errorf("%s must not include a fragment", varName)
	}
	if u.Hostname() == "" {
		return fmt.Errorf("%s must specify a host", varName)
	}
	if strings.HasSuffix(u.Host, ":") {
		return fmt.Errorf("%s has a trailing colon with no port number", varName)
	}
	if port := u.Port(); port != "" {
		n, nerr := strconv.Atoi(port)
		if nerr != nil || n < 1 || n > 65535 {
			return fmt.Errorf("%s has invalid port %q", varName, port)
		}
	}
	return nil
}

// validateS3BucketName enforces the DNS-compatible S3 bucket naming rules.
func validateS3BucketName(name string) error {
	if len(name) < 3 || len(name) > 63 {
		return fmt.Errorf("SCOUT_S3_BUCKET must be 3–63 characters, got %d", len(name))
	}
	if !isS3AlphaNum(name[0]) || !isS3AlphaNum(name[len(name)-1]) {
		return fmt.Errorf("SCOUT_S3_BUCKET must start and end with a lowercase letter or digit")
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c == '.' || c == '-' {
			continue
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			continue
		}
		return fmt.Errorf("SCOUT_S3_BUCKET must contain only lowercase letters, digits, dots, and hyphens")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("SCOUT_S3_BUCKET must not contain adjacent dots")
	}
	if net.ParseIP(name) != nil {
		return fmt.Errorf("SCOUT_S3_BUCKET must not be an IP address")
	}
	return nil
}

func isS3AlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
}

func loadS3Bool(name string) (bool, error) {
	v, ok := os.LookupEnv(name)
	if !ok || v == "" {
		return false, fmt.Errorf("%s is required (set to true or false)", name)
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: must be a boolean (true/false/1/0), got %q", name, v)
	}
	return b, nil
}

func loadS3TTL(name string, def time.Duration) (time.Duration, error) {
	v := os.Getenv(name)
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", name, v, err)
	}
	if d < minS3TTL || d > maxS3TTL {
		return 0, fmt.Errorf("%s: duration must be between 1s and 7 days, got %q", name, v)
	}
	return d, nil
}

// ThumbnailConfig holds configuration for the thumbnail generator.
type ThumbnailConfig struct {
	// GenerationConcurrency is the maximum number of simultaneous thumbnail
	// generations. Kept at 1..2 to bound peak decode/encode memory.
	GenerationConcurrency int
}

const (
	DefaultThumbnailGenerationConcurrency = 1
	MaxThumbnailGenerationConcurrency     = 2
)

// ThumbnailCacheConfig holds configuration for the on-disk thumbnail cache.
type ThumbnailCacheConfig struct {
	// Dir is the root directory for the thumbnail cache.
	// In production this must be a persistent writable volume outside the container image.
	Dir string
	// MaxBytes is the maximum total disk usage in bytes.
	MaxBytes int64
}

const (
	// DefaultThumbnailCacheDir is a safe transient default for local development only.
	// Production deployments must set SCOUT_THUMBNAIL_CACHE_DIR to a persistent volume.
	DefaultThumbnailCacheDir = "/tmp/scout-thumb-cache"
	// DefaultThumbnailCacheMaxBytes is 512 MiB.
	DefaultThumbnailCacheMaxBytes int64 = 512 * 1024 * 1024
	// MinThumbnailCacheMaxBytes requires at least 1 MiB to ensure one small thumbnail fits.
	MinThumbnailCacheMaxBytes int64 = 1024 * 1024
)

// LoadThumbnailCacheConfig loads thumbnail cache configuration from environment variables.
func LoadThumbnailCacheConfig() (*ThumbnailCacheConfig, error) {
	dir := os.Getenv("SCOUT_THUMBNAIL_CACHE_DIR")
	if dir == "" {
		dir = DefaultThumbnailCacheDir
	}

	maxBytesStr := os.Getenv("SCOUT_THUMBNAIL_CACHE_MAX_BYTES")
	var maxBytes int64
	if maxBytesStr == "" {
		maxBytes = DefaultThumbnailCacheMaxBytes
	} else {
		var err error
		maxBytes, err = strconv.ParseInt(maxBytesStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("SCOUT_THUMBNAIL_CACHE_MAX_BYTES: invalid integer %q: %w", maxBytesStr, err)
		}
		if maxBytes < MinThumbnailCacheMaxBytes {
			return nil, fmt.Errorf("SCOUT_THUMBNAIL_CACHE_MAX_BYTES: must be at least %d bytes, got %d",
				MinThumbnailCacheMaxBytes, maxBytes)
		}
	}

	return &ThumbnailCacheConfig{
		Dir:      dir,
		MaxBytes: maxBytes,
	}, nil
}

// LoadThumbnailConfig loads thumbnail configuration from environment variables.
func LoadThumbnailConfig() (*ThumbnailConfig, error) {
	concurrency, err := loadThumbnailConcurrency()
	if err != nil {
		return nil, err
	}
	return &ThumbnailConfig{GenerationConcurrency: concurrency}, nil
}

func loadThumbnailConcurrency() (int, error) {
	v := os.Getenv("SCOUT_THUMBNAIL_GENERATION_CONCURRENCY")
	if v == "" {
		return DefaultThumbnailGenerationConcurrency, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("SCOUT_THUMBNAIL_GENERATION_CONCURRENCY: invalid integer %q: %w", v, err)
	}
	if n < 1 || n > MaxThumbnailGenerationConcurrency {
		return 0, fmt.Errorf("SCOUT_THUMBNAIL_GENERATION_CONCURRENCY: must be in [1, %d], got %d",
			MaxThumbnailGenerationConcurrency, n)
	}
	return n, nil
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

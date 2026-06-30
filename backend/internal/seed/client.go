package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Config holds all seed configuration.
type Config struct {
	APIURL      string
	APIKey      string
	ImagesDir   string
	Concurrency int
	Timeout     time.Duration
}

// Result summarizes a seed run.
type Result struct {
	Discovered int
	Succeeded  int
	Failed     int
	Errors     []string
}

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// headerNameRe accepts only RFC 7230 token characters.
var headerNameRe = regexp.MustCompile("^[A-Za-z0-9!#$%&'*+.^_`|~-]+$")

type imageFile struct {
	ID   string
	Path string
	Size int64
}

type uploadLinkResponse struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

// Validate checks the Config for correctness, normalizing APIURL in place.
func (c *Config) Validate() error {
	c.APIURL = strings.TrimRight(c.APIURL, "/")
	if c.APIKey == "" {
		return errors.New("API key is required (set SCOUT_API_KEY or --api-key)")
	}
	if c.APIURL == "" {
		return errors.New("API URL is required")
	}
	u, err := url.Parse(c.APIURL)
	if err != nil {
		return fmt.Errorf("invalid API URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("API URL must use http or https, got %q", u.Scheme)
	}
	if !u.IsAbs() || u.Host == "" {
		return errors.New("API URL must be absolute with a host")
	}
	if u.User != nil {
		return errors.New("API URL must not contain userinfo")
	}
	if u.RawQuery != "" {
		return errors.New("API URL must not contain a query string")
	}
	if u.Fragment != "" {
		return errors.New("API URL must not contain a fragment")
	}
	if c.Concurrency < 1 || c.Concurrency > 4 {
		return fmt.Errorf("concurrency must be between 1 and 4, got %d", c.Concurrency)
	}
	if c.Timeout <= 0 {
		return errors.New("timeout must be positive")
	}
	if c.ImagesDir == "" {
		return errors.New("images directory is required")
	}
	return nil
}

// Run discovers images and uploads them through the configured API.
func Run(ctx context.Context, cfg Config) (Result, error) {
	files, err := discoverImages(cfg.ImagesDir)
	if err != nil {
		return Result{}, err
	}

	noRedirect := func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	apiClient := &http.Client{
		Timeout:       cfg.Timeout,
		CheckRedirect: noRedirect,
	}
	putClient := &http.Client{
		Timeout:       cfg.Timeout,
		CheckRedirect: noRedirect,
	}

	succeeded, failed, errs := runPool(ctx, cfg, files, apiClient, putClient)
	return Result{
		Discovered: len(files),
		Succeeded:  succeeded,
		Failed:     failed,
		Errors:     errs,
	}, nil
}

func discoverImages(dir string) ([]imageFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read images directory %s: %w", dir, err)
	}

	seen := make(map[string]struct{})
	var files []imageFile

	for _, e := range entries {
		name := e.Name()

		if !e.Type().IsRegular() {
			continue
		}
		if !strings.HasSuffix(name, ".jpg") {
			continue
		}

		id := strings.TrimSuffix(name, ".jpg")
		if !uuidRe.MatchString(id) {
			return nil, fmt.Errorf("file %q has a non-canonical UUID name", name)
		}
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("duplicate UUID %s in images directory", id)
		}
		seen[id] = struct{}{}

		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", name, err)
		}
		if info.Size() == 0 {
			return nil, fmt.Errorf("file %s is empty", name)
		}

		files = append(files, imageFile{
			ID:   id,
			Path: filepath.Join(dir, name),
			Size: info.Size(),
		})
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no valid JPEG files found in %s", dir)
	}
	return files, nil
}

func runPool(ctx context.Context, cfg Config, files []imageFile, apiClient, putClient *http.Client) (succeeded, failed int, errs []string) {
	jobs := make(chan imageFile, len(files))
	for _, f := range files {
		jobs <- f
	}
	close(jobs)

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	for range cfg.Concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for f := range jobs {
				if ctx.Err() != nil {
					return
				}
				err := uploadOne(ctx, cfg, apiClient, putClient, f)
				mu.Lock()
				if err != nil {
					failed++
					errs = append(errs, fmt.Sprintf("%s: %v", f.ID, err))
				} else {
					succeeded++
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	return succeeded, failed, errs
}

func uploadOne(ctx context.Context, cfg Config, apiClient, putClient *http.Client, f imageFile) error {
	link, err := getUploadLink(ctx, cfg, apiClient, f.ID)
	if err != nil {
		return err
	}
	return putFile(ctx, putClient, f, link)
}

func getUploadLink(ctx context.Context, cfg Config, apiClient *http.Client, id string) (*uploadLinkResponse, error) {
	const body = `{"contentType":"image/jpeg"}`
	reqURL := cfg.APIURL + "/photos/" + id + "/upload-link"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build upload-link request: %w", err)
	}
	req.Header.Set("X-API-Key", cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))

	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upload-link request: %w", sanitizeErr(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024))
		return nil, fmt.Errorf("upload-link returned %d", resp.StatusCode)
	}

	var link uploadLinkResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&link); err != nil {
		return nil, fmt.Errorf("decode upload-link response: %w", err)
	}

	if link.Method != "PUT" {
		return nil, fmt.Errorf("unexpected method %q", link.Method)
	}
	if link.ExpiresAt.IsZero() || !time.Now().Before(link.ExpiresAt) {
		return nil, errors.New("upload-link is expired or missing expiry")
	}
	if err := validateSignedURL(link.URL); err != nil {
		return nil, fmt.Errorf("invalid signed URL: %w", err)
	}
	if err := validateSignedHeaders(link.Headers); err != nil {
		return nil, fmt.Errorf("invalid signed headers: %w", err)
	}

	return &link, nil
}

func validateSignedURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
	if !u.IsAbs() || u.Host == "" {
		return errors.New("must be an absolute URL with a host")
	}
	if u.User != nil {
		return errors.New("must not contain userinfo")
	}
	return nil
}

func validateSignedHeaders(headers map[string]string) error {
	for name, value := range headers {
		if !headerNameRe.MatchString(name) {
			return fmt.Errorf("invalid header name %q", name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("header %q value contains CRLF", name)
		}
	}
	return nil
}

func putFile(ctx context.Context, putClient *http.Client, f imageFile, link *uploadLinkResponse) error {
	file, err := os.Open(f.Path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, link.URL, file)
	if err != nil {
		return fmt.Errorf("build PUT request: %w", err)
	}
	req.ContentLength = f.Size
	for name, value := range link.Headers {
		req.Header.Set(name, value)
	}
	// Safety: never forward the API key to object storage.
	req.Header.Del("X-API-Key")

	resp, err := putClient.Do(req)
	if err != nil {
		return fmt.Errorf("PUT upload: %w", sanitizeErr(err))
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4*1024))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("PUT returned %d", resp.StatusCode)
	}
	return nil
}

// sanitizeErr strips URLs from http transport errors to avoid leaking signed URLs in logs.
func sanitizeErr(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%s [url]: %w", ue.Op, ue.Err)
	}
	return err
}

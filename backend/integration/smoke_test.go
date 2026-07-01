//go:build integration

package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// envOrDefault returns the env var or the provided default.
func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func apiURL() string { return envOrDefault("SCOUT_API_URL", "http://localhost:8080") }
func apiKey() string { return envOrDefault("SCOUT_API_KEY", "") }

// datasetDir returns the path to the dataset images directory.
// Relative path from the integration package directory: ../../dataset/images.
func datasetDir() string {
	return envOrDefault("SCOUT_DATASET_DIR", "../../dataset/images")
}

// boundedClient creates an HTTP client that never follows redirects and
// logs no secrets. Body reads are always bounded by callers.
func boundedClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// selectPhoto returns the lexicographically first JPEG in the dataset directory.
// Deterministic across reruns.
func selectPhoto(t *testing.T) (id string, path string, data []byte) {
	t.Helper()
	dir := datasetDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dataset dir %s: %v", dir, err)
	}
	var jpgs []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), ".jpg") {
			jpgs = append(jpgs, e.Name())
		}
	}
	if len(jpgs) == 0 {
		t.Fatalf("no JPEG files found in %s", dir)
	}
	sort.Strings(jpgs)
	name := jpgs[0]
	id = strings.TrimSuffix(name, ".jpg")
	path = filepath.Join(dir, name)
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return id, path, data
}

// uploadLinkResponse is the API response shape for POST /photos/{id}/upload-link.
type uploadLinkResponse struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

// photoResponse is a partial API response shape for GET /photos/{id}.
type photoResponse struct {
	ID          string `json:"id"`
	OriginalURL string `json:"originalUrl"`
	Predictions []any  `json:"predictions"`
}

// TestSmoke_IngestReadThumbnail exercises the complete ingest → read → thumbnail flow.
func TestSmoke_IngestReadThumbnail(t *testing.T) {
	key := apiKey()
	if key == "" {
		t.Fatal("SCOUT_API_KEY must be set for integration smoke")
	}
	base := apiURL()
	client := boundedClient()

	photoID, _, originalData := selectPhoto(t)
	t.Logf("smoke photo ID: %s (%d bytes)", photoID, len(originalData))

	// ── 1. Request upload link ────────────────────────────────────────────────
	uploadLinkURL := base + "/photos/" + photoID + "/upload-link"
	body := `{"contentType":"image/jpeg"}`
	req, _ := http.NewRequest(http.MethodPost, uploadLinkURL, strings.NewReader(body))
	req.Header.Set("X-API-Key", key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("upload-link request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		t.Fatalf("upload-link returned %d: %s", resp.StatusCode, body)
	}

	var link uploadLinkResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&link); err != nil {
		t.Fatalf("decode upload-link: %v", err)
	}
	if link.Method != "PUT" {
		t.Fatalf("unexpected method %q", link.Method)
	}

	// ── 2. PUT original to MinIO via signed URL ───────────────────────────────
	putReq, _ := http.NewRequest(http.MethodPut, link.URL, bytes.NewReader(originalData))
	putReq.ContentLength = int64(len(originalData))
	for name, value := range link.Headers {
		putReq.Header.Set(name, value)
	}
	// Safety: never forward the API key to object storage.
	putReq.Header.Del("X-API-Key")

	putResp, err := client.Do(putReq)
	if err != nil {
		t.Fatalf("PUT original: %v", err)
	}
	io.Copy(io.Discard, io.LimitReader(putResp.Body, 4096))
	putResp.Body.Close()
	if putResp.StatusCode < 200 || putResp.StatusCode >= 300 {
		t.Fatalf("PUT returned %d", putResp.StatusCode)
	}

	// ── 3. GET photo and verify metadata ─────────────────────────────────────
	photoReq, _ := http.NewRequest(http.MethodGet, base+"/photos/"+photoID, nil)
	photoReq.Header.Set("X-API-Key", key)

	photoResp, err := client.Do(photoReq)
	if err != nil {
		t.Fatalf("GET photo: %v", err)
	}
	defer photoResp.Body.Close()
	if photoResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(photoResp.Body, 4096))
		t.Fatalf("GET photo returned %d: %s", photoResp.StatusCode, body)
	}

	var photo photoResponse
	if err := json.NewDecoder(io.LimitReader(photoResp.Body, 64*1024)).Decode(&photo); err != nil {
		t.Fatalf("decode photo: %v", err)
	}
	if photo.ID != photoID {
		t.Errorf("photo.id = %q, want %q", photo.ID, photoID)
	}
	if photo.OriginalURL == "" {
		t.Error("photo.originalUrl must not be empty")
	}

	// ── 4. Fetch original via signed URL and compare bytes ───────────────────
	origResp, err := client.Get(photo.OriginalURL)
	if err != nil {
		t.Fatalf("fetch original: %v", err)
	}
	defer origResp.Body.Close()
	if origResp.StatusCode != http.StatusOK {
		t.Fatalf("original URL returned %d", origResp.StatusCode)
	}
	fetched, err := io.ReadAll(io.LimitReader(origResp.Body, 100*1024*1024))
	if err != nil {
		t.Fatalf("read original: %v", err)
	}
	if !bytes.Equal(fetched, originalData) {
		t.Errorf("fetched %d bytes, original %d bytes; content differs",
			len(fetched), len(originalData))
	}

	// ── 5. Request thumbnail; verify JPEG, ETag, dimensions ──────────────────
	thumbURL := fmt.Sprintf("%s/photos/%s/thumbnail?width=200", base, photoID)

	thumbResp, err := client.Get(thumbURL)
	if err != nil {
		t.Fatalf("GET thumbnail: %v", err)
	}
	defer thumbResp.Body.Close()
	if thumbResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(thumbResp.Body, 4096))
		t.Fatalf("thumbnail returned %d: %s", thumbResp.StatusCode, body)
	}
	if ct := thumbResp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("thumbnail Content-Type = %q, want image/jpeg", ct)
	}
	etag := thumbResp.Header.Get("ETag")
	if etag == "" {
		t.Error("thumbnail response must have ETag")
	}

	thumbBytes, err := io.ReadAll(io.LimitReader(thumbResp.Body, 10*1024*1024))
	if err != nil {
		t.Fatalf("read thumbnail: %v", err)
	}
	img, err := jpeg.Decode(bytes.NewReader(thumbBytes))
	if err != nil {
		t.Fatalf("decode thumbnail JPEG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() > 200 {
		t.Errorf("thumbnail width = %d, must not exceed 200", b.Dx())
	}

	// ── 6. Repeat thumbnail; verify identical bytes and ETag ─────────────────
	thumbResp2, err := client.Get(thumbURL)
	if err != nil {
		t.Fatalf("second GET thumbnail: %v", err)
	}
	defer thumbResp2.Body.Close()
	thumbBytes2, _ := io.ReadAll(io.LimitReader(thumbResp2.Body, 10*1024*1024))

	if etag2 := thumbResp2.Header.Get("ETag"); etag2 != etag {
		t.Errorf("second thumbnail ETag = %q, want %q", etag2, etag)
	}
	if !bytes.Equal(thumbBytes2, thumbBytes) {
		t.Errorf("second thumbnail differs: first=%d second=%d bytes", len(thumbBytes), len(thumbBytes2))
	}

	// ── 7. Conditional GET; verify 304 ───────────────────────────────────────
	condReq, _ := http.NewRequest(http.MethodGet, thumbURL, nil)
	condReq.Header.Set("If-None-Match", etag)

	condResp, err := client.Do(condReq)
	if err != nil {
		t.Fatalf("conditional GET: %v", err)
	}
	io.Copy(io.Discard, io.LimitReader(condResp.Body, 4096))
	condResp.Body.Close()
	if condResp.StatusCode != http.StatusNotModified {
		t.Errorf("conditional GET status = %d, want 304", condResp.StatusCode)
	}

	// ── 8. Scrape /metrics and verify required families ───────────────────────
	metricsResp, err := client.Get(base + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer metricsResp.Body.Close()
	if metricsResp.StatusCode != http.StatusOK {
		t.Fatalf("/metrics returned %d", metricsResp.StatusCode)
	}
	metricsBody, err := io.ReadAll(io.LimitReader(metricsResp.Body, 1024*1024))
	if err != nil {
		t.Fatalf("read /metrics: %v", err)
	}
	metricsText := string(metricsBody)

	requiredFamilies := []string{
		"scout_http_requests_total",
		"scout_http_request_duration_seconds",
		"scout_thumbnail_cache_hits_total",
		"scout_thumbnail_cache_misses_total",
		"scout_thumbnail_generations_total",
	}
	for _, name := range requiredFamilies {
		if !strings.Contains(metricsText, name) {
			t.Errorf("/metrics missing required family %q", name)
		}
	}

	// Verify no API key or signed URL fragments in metrics output.
	if strings.Contains(metricsText, key) {
		t.Error("/metrics must not contain API key")
	}
	if strings.Contains(metricsText, "X-Amz-Signature") || strings.Contains(metricsText, "Signature") {
		t.Error("/metrics must not contain signed URL parameters")
	}
}

// TestSmoke_ConcurrentThumbnails verifies duplicate-suppression under concurrent load.
func TestSmoke_ConcurrentThumbnails(t *testing.T) {
	key := apiKey()
	if key == "" {
		t.Skip("SCOUT_API_KEY not set")
	}
	base := apiURL()
	photoID, _, _ := selectPhoto(t)
	thumbURL := fmt.Sprintf("%s/photos/%s/thumbnail?width=150&dpr=2", base, photoID)

	const burst = 10
	type result struct {
		etag string
		body []byte
		code int
	}
	results := make([]result, burst)
	done := make(chan int, burst)

	for i := range burst {
		go func(idx int) {
			client := boundedClient()
			resp, err := client.Get(thumbURL)
			if err != nil {
				done <- idx
				return
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
			results[idx] = result{
				etag: resp.Header.Get("ETag"),
				body: body,
				code: resp.StatusCode,
			}
			done <- idx
		}(i)
	}

	timeout := time.After(30 * time.Second)
	for range burst {
		select {
		case <-done:
		case <-timeout:
			t.Fatal("concurrent thumbnail requests timed out")
		}
	}

	// All successful responses must return the same ETag and body.
	var refEtag string
	var refBody []byte
	for i, r := range results {
		if r.code == 0 {
			t.Errorf("result %d: request error", i)
			continue
		}
		if r.code != http.StatusOK {
			t.Errorf("result %d: status %d", i, r.code)
			continue
		}
		if refEtag == "" {
			refEtag = r.etag
			refBody = r.body
			continue
		}
		if r.etag != refEtag {
			t.Errorf("result %d: ETag %q != first ETag %q", i, r.etag, refEtag)
		}
		if !bytes.Equal(r.body, refBody) {
			t.Errorf("result %d: body differs (%d vs %d bytes)", i, len(r.body), len(refBody))
		}
	}
}

package seed_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"scout/internal/seed"
)

// makeJPEG returns a minimal valid JPEG (1x1 white pixel).
func makeJPEG() []byte {
	return []byte{
		0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xff, 0xdb, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0a, 0x0c, 0x14, 0x0d, 0x0c, 0x0b, 0x0b, 0x0c, 0x19, 0x12,
		0x13, 0x0f, 0x14, 0x1d, 0x1a, 0x1f, 0x1e, 0x1d, 0x1a, 0x1c, 0x1c, 0x20,
		0x24, 0x2e, 0x27, 0x20, 0x22, 0x2c, 0x23, 0x1c, 0x1c, 0x28, 0x37, 0x29,
		0x2c, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1f, 0x27, 0x39, 0x3d, 0x38, 0x32,
		0x3c, 0x2e, 0x33, 0x34, 0x32, 0xff, 0xc0, 0x00, 0x0b, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xff, 0xc4, 0x00, 0x1f, 0x00, 0x00,
		0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0xff, 0xc4, 0x00, 0xb5, 0x10, 0x00, 0x02, 0x01, 0x03,
		0x03, 0x02, 0x04, 0x03, 0x05, 0x05, 0x04, 0x04, 0x00, 0x00, 0x01, 0x7d,
		0x01, 0x02, 0x03, 0x00, 0x04, 0x11, 0x05, 0x12, 0x21, 0x31, 0x41, 0x06,
		0x13, 0x51, 0x61, 0x07, 0x22, 0x71, 0x14, 0x32, 0x81, 0x91, 0xa1, 0x08,
		0x23, 0x42, 0xb1, 0xc1, 0x15, 0x52, 0xd1, 0xf0, 0x24, 0x33, 0x62, 0x72,
		0x82, 0x09, 0x0a, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x25, 0x26, 0x27, 0x28,
		0x29, 0x2a, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x43, 0x44, 0x45,
		0x46, 0x47, 0x48, 0x49, 0x4a, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59,
		0x5a, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x73, 0x74, 0x75,
		0x76, 0x77, 0x78, 0x79, 0x7a, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89,
		0x8a, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0xa2, 0xa3,
		0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6,
		0xb7, 0xb8, 0xb9, 0xba, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9,
		0xca, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xe1, 0xe2,
		0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xf1, 0xf2, 0xf3, 0xf4,
		0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0xff, 0xda, 0x00, 0x08, 0x01, 0x01,
		0x00, 0x00, 0x3f, 0x00, 0xfb, 0xd2, 0x8a, 0x28, 0x03, 0xff, 0xd9,
	}
}

// uploadLinkResp is a minimal upload-link API response.
type uploadLinkResp struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

const testUUID = "550e8400-e29b-41d4-a716-446655440001"

// setupImagesDir creates a temp dir with one JPEG file named after testUUID.
func setupImagesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	data := makeJPEG()
	path := filepath.Join(dir, testUUID+".jpg")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write jpeg: %v", err)
	}
	return dir
}

// TestValidate covers Config.Validate error paths.
func TestValidate(t *testing.T) {
	valid := seed.Config{
		APIURL:      "http://localhost:8080",
		APIKey:      "key",
		ImagesDir:   "/tmp",
		Concurrency: 1,
		Timeout:     5 * time.Second,
	}

	cases := []struct {
		name   string
		mutate func(*seed.Config)
	}{
		{"no key", func(c *seed.Config) { c.APIKey = "" }},
		{"no url", func(c *seed.Config) { c.APIURL = "" }},
		{"bad url scheme", func(c *seed.Config) { c.APIURL = "ftp://x" }},
		{"url with userinfo", func(c *seed.Config) { c.APIURL = "http://u:p@host" }},
		{"url with query", func(c *seed.Config) { c.APIURL = "http://host?q=1" }},
		{"url with fragment", func(c *seed.Config) { c.APIURL = "http://host#frag" }},
		{"concurrency zero", func(c *seed.Config) { c.Concurrency = 0 }},
		{"concurrency too high", func(c *seed.Config) { c.Concurrency = 5 }},
		{"zero timeout", func(c *seed.Config) { c.Timeout = 0 }},
		{"no images dir", func(c *seed.Config) { c.ImagesDir = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Errorf("Validate() = nil; want error for %q", tc.name)
			}
		})
	}
}

// TestRun_UploadLinkRequest verifies the exact API request: method, URL, headers, body.
func TestRun_UploadLinkRequest(t *testing.T) {
	var gotReq *http.Request
	var gotBody []byte

	putSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer putSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotReq = r
		gotBody = body
		resp := uploadLinkResp{
			URL:       putSrv.URL + "/object/" + testUUID,
			Method:    "PUT",
			Headers:   map[string]string{"Content-Type": "image/jpeg"},
			ExpiresAt: time.Now().Add(15 * time.Minute),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer apiSrv.Close()

	dir := setupImagesDir(t)
	cfg := seed.Config{
		APIURL:      apiSrv.URL,
		APIKey:      "test-key-abc",
		ImagesDir:   dir,
		Concurrency: 1,
		Timeout:     10 * time.Second,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	result, err := seed.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Succeeded != 1 {
		t.Errorf("Succeeded = %d, want 1; errors: %v", result.Succeeded, result.Errors)
	}

	if gotReq == nil {
		t.Fatal("upload-link request was not sent")
	}
	if gotReq.Method != http.MethodPost {
		t.Errorf("method = %q, want POST", gotReq.Method)
	}
	if gotReq.Header.Get("X-API-Key") != "test-key-abc" {
		t.Errorf("X-API-Key = %q, want test-key-abc", gotReq.Header.Get("X-API-Key"))
	}
	if ct := gotReq.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if string(gotBody) != `{"contentType":"image/jpeg"}` {
		t.Errorf("body = %q, want JSON object", string(gotBody))
	}
}

// TestRun_APIKeyNeverReachesPUT proves X-API-Key is stripped before object storage PUT.
func TestRun_APIKeyNeverReachesPUT(t *testing.T) {
	const apiKey = "secret-api-key-xyz"
	var putReq *http.Request

	putSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		putReq = r
		w.WriteHeader(http.StatusOK)
	}))
	defer putSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := uploadLinkResp{
			URL:    putSrv.URL + "/object/" + testUUID,
			Method: "PUT",
			// Include X-API-Key in the signed headers to test that it's stripped.
			Headers:   map[string]string{"X-API-Key": apiKey, "Content-Type": "image/jpeg"},
			ExpiresAt: time.Now().Add(15 * time.Minute),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer apiSrv.Close()

	dir := setupImagesDir(t)
	cfg := seed.Config{
		APIURL:      apiSrv.URL,
		APIKey:      apiKey,
		ImagesDir:   dir,
		Concurrency: 1,
		Timeout:     10 * time.Second,
	}
	_ = cfg.Validate()
	seed.Run(context.Background(), cfg)

	if putReq == nil {
		t.Fatal("PUT request was not sent")
	}
	if putReq.Header.Get("X-API-Key") != "" {
		t.Errorf("X-API-Key must not reach object storage PUT, got %q", putReq.Header.Get("X-API-Key"))
	}
}

// TestRun_Rerun verifies that uploading the same key twice succeeds both times.
func TestRun_Rerun(t *testing.T) {
	putSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer putSrv.Close()

	var calls int
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		resp := uploadLinkResp{
			URL:       putSrv.URL + "/object/" + testUUID,
			Method:    "PUT",
			Headers:   map[string]string{"Content-Type": "image/jpeg"},
			ExpiresAt: time.Now().Add(15 * time.Minute),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer apiSrv.Close()

	dir := setupImagesDir(t)
	cfg := seed.Config{
		APIURL: apiSrv.URL, APIKey: "key", ImagesDir: dir,
		Concurrency: 1, Timeout: 10 * time.Second,
	}
	_ = cfg.Validate()

	for i := range 2 {
		r, err := seed.Run(context.Background(), cfg)
		if err != nil || r.Succeeded != 1 {
			t.Errorf("run %d: succeeded=%d err=%v errors=%v", i+1, r.Succeeded, err, r.Errors)
		}
	}
	if calls != 2 {
		t.Errorf("want 2 upload-link calls, got %d", calls)
	}
}

// TestRun_RedirectRejected verifies that redirects from object storage are rejected.
func TestRun_RedirectRejected(t *testing.T) {
	redirectSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			http.Redirect(w, r, "http://evil.example.com/", http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer redirectSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := uploadLinkResp{
			URL:       redirectSrv.URL + "/object/" + testUUID,
			Method:    "PUT",
			Headers:   map[string]string{"Content-Type": "image/jpeg"},
			ExpiresAt: time.Now().Add(15 * time.Minute),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer apiSrv.Close()

	dir := setupImagesDir(t)
	cfg := seed.Config{
		APIURL: apiSrv.URL, APIKey: "key", ImagesDir: dir,
		Concurrency: 1, Timeout: 10 * time.Second,
	}
	_ = cfg.Validate()

	r, err := seed.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run returned unexpected error: %v", err)
	}
	// Redirect must be treated as a failure (PUT returned non-2xx redirect response).
	if r.Failed == 0 {
		t.Error("redirect must cause a failure")
	}
}

// TestRun_InvalidImagesDir covers pre-network validation errors on images directory.
func TestRun_InvalidImagesDir(t *testing.T) {
	cfg := seed.Config{
		APIURL:      "http://localhost:1", // unreachable, should never be hit
		APIKey:      "key",
		ImagesDir:   "/no-such-directory-xyz-abc",
		Concurrency: 1,
		Timeout:     time.Second,
	}
	_ = cfg.Validate()
	_, err := seed.Run(context.Background(), cfg)
	if err == nil {
		t.Error("Run must return an error for non-existent images directory")
	}
}

// TestRun_EmptyFile covers rejection of zero-byte image files before network calls.
func TestRun_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	// Create an empty file with a valid UUID name.
	if err := os.WriteFile(filepath.Join(dir, testUUID+".jpg"), []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}

	var apiCalled bool
	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		apiCalled = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer apiSrv.Close()

	cfg := seed.Config{
		APIURL: apiSrv.URL, APIKey: "key", ImagesDir: dir,
		Concurrency: 1, Timeout: time.Second,
	}
	_ = cfg.Validate()
	_, err := seed.Run(context.Background(), cfg)
	if err == nil {
		t.Error("Run must return an error for empty image file")
	}
	if apiCalled {
		t.Error("network must not be called for empty file")
	}
}

// TestRun_NonJPEGFile covers rejection of non-.jpg files (they should be ignored).
func TestRun_NonJPEGFile(t *testing.T) {
	dir := t.TempDir()
	// Create a non-JPEG file - should be ignored (no UUID match).
	if err := os.WriteFile(filepath.Join(dir, "image.png"), []byte("png"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := seed.Config{
		APIURL: "http://localhost:1", APIKey: "key", ImagesDir: dir,
		Concurrency: 1, Timeout: time.Second,
	}
	_ = cfg.Validate()
	_, err := seed.Run(context.Background(), cfg)
	// no .jpg files → error (no files found)
	if err == nil {
		t.Error("Run must return an error when no valid JPEG files exist")
	}
}

// TestRun_NonUUIDFile covers rejection of files with non-UUID names.
func TestRun_NonUUIDFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "not-a-uuid.jpg"), makeJPEG(), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := seed.Config{
		APIURL: "http://localhost:1", APIKey: "key", ImagesDir: dir,
		Concurrency: 1, Timeout: time.Second,
	}
	_ = cfg.Validate()
	_, err := seed.Run(context.Background(), cfg)
	if err == nil {
		t.Error("Run must return an error for non-UUID file name")
	}
}

// TestRun_SymlinkRejected covers rejection of symlinks in the images directory.
func TestRun_SymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.jpg")
	if err := os.WriteFile(target, makeJPEG(), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, testUUID+".jpg")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	cfg := seed.Config{
		APIURL: "http://localhost:1", APIKey: "key", ImagesDir: dir,
		Concurrency: 1, Timeout: time.Second,
	}
	_ = cfg.Validate()
	_, err := seed.Run(context.Background(), cfg)
	// Symlinks are non-regular files and should be skipped. Only the symlink is present,
	// so Run should fail because no valid regular files were found.
	if err == nil {
		t.Error("Run must return an error when only symlinks are present (no regular files)")
	}
}

// TestRun_Cancellation verifies that context cancellation stops the run.
func TestRun_Cancellation(t *testing.T) {
	started := make(chan struct{})
	block := make(chan struct{})

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-block // hang until test unblocks
		w.WriteHeader(http.StatusOK)
	}))
	defer func() {
		close(block)
		apiSrv.Close()
	}()

	dir := setupImagesDir(t)
	cfg := seed.Config{
		APIURL: apiSrv.URL, APIKey: "key", ImagesDir: dir,
		Concurrency: 1, Timeout: 30 * time.Second,
	}
	_ = cfg.Validate()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := seed.Run(ctx, cfg)
		done <- err
	}()

	// Wait for the request to start, then cancel.
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for request to start")
	}
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
}

// TestRun_ConcurrencyNeverExceedsConfig verifies worker count stays at or below config.
func TestRun_ConcurrencyNeverExceedsConfig(t *testing.T) {
	const maxConcurrency = 2
	const fileCount = 8

	var active atomic.Int32
	var peak atomic.Int32

	release := make(chan struct{}, fileCount)

	putSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	defer putSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := active.Add(1)
		defer active.Add(-1)
		// Track peak concurrency.
		for {
			cur := peak.Load()
			if n <= cur || peak.CompareAndSwap(cur, n) {
				break
			}
		}
		resp := uploadLinkResp{
			URL:       putSrv.URL + "/obj",
			Method:    "PUT",
			Headers:   map[string]string{"Content-Type": "image/jpeg"},
			ExpiresAt: time.Now().Add(15 * time.Minute),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer apiSrv.Close()

	// Unblock all PUT requests upfront.
	for range fileCount {
		release <- struct{}{}
	}

	dir := t.TempDir()
	uuids := []string{
		"550e8400-e29b-41d4-a716-446655440001",
		"550e8400-e29b-41d4-a716-446655440002",
		"550e8400-e29b-41d4-a716-446655440003",
		"550e8400-e29b-41d4-a716-446655440004",
		"550e8400-e29b-41d4-a716-446655440005",
		"550e8400-e29b-41d4-a716-446655440006",
		"550e8400-e29b-41d4-a716-446655440007",
		"550e8400-e29b-41d4-a716-446655440008",
	}
	data := makeJPEG()
	for _, u := range uuids {
		if err := os.WriteFile(filepath.Join(dir, u+".jpg"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	cfg := seed.Config{
		APIURL: apiSrv.URL, APIKey: "key", ImagesDir: dir,
		Concurrency: maxConcurrency, Timeout: 10 * time.Second,
	}
	_ = cfg.Validate()

	result, err := seed.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Succeeded != fileCount {
		t.Errorf("Succeeded = %d, want %d; errors: %v", result.Succeeded, fileCount, result.Errors)
	}
	if got := peak.Load(); got > maxConcurrency {
		t.Errorf("peak concurrency = %d, must not exceed %d", got, maxConcurrency)
	}
}

// TestRun_SanitizedFailureOutput verifies that error messages don't expose signed URLs or API keys.
func TestRun_SanitizedFailureOutput(t *testing.T) {
	const signedURL = "http://minio.example.com/bucket/obj?X-Amz-Signature=supersecret"
	const apiKey = "very-secret-api-key"

	putSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer putSrv.Close()

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := uploadLinkResp{
			URL:       signedURL,
			Method:    "PUT",
			Headers:   map[string]string{"Content-Type": "image/jpeg"},
			ExpiresAt: time.Now().Add(15 * time.Minute),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer apiSrv.Close()

	dir := setupImagesDir(t)
	cfg := seed.Config{
		APIURL: apiSrv.URL, APIKey: apiKey, ImagesDir: dir,
		Concurrency: 1, Timeout: 10 * time.Second,
	}
	_ = cfg.Validate()

	result, _ := seed.Run(context.Background(), cfg)
	for _, errMsg := range result.Errors {
		var buf bytes.Buffer
		fmt.Fprint(&buf, errMsg)
		out := buf.String()
		if bytes.Contains([]byte(out), []byte("supersecret")) {
			t.Errorf("error message must not expose signed URL details: %q", out)
		}
		if bytes.Contains([]byte(out), []byte(apiKey)) {
			t.Errorf("error message must not expose API key: %q", out)
		}
	}
}

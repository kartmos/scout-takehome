//go:build integration

package objectstorage_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"scout/internal/config"
	"scout/internal/objectstorage"
)

// newTestUUID generates a random UUID v4 using the standard library.
func newTestUUID(t *testing.T) string {
	t.Helper()
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatalf("generate test UUID: %v", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant bits
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// TestIntegration_ObjectStorageAdapter exercises the adapter against a live MinIO instance.
// Requires environment variables from SCOUT_S3_* (see .env.example).
func TestIntegration_ObjectStorageAdapter(t *testing.T) {
	cfg, err := config.LoadS3Config()
	if err != nil {
		t.Skipf("S3 config not available, skipping integration test: %v", err)
	}

	adapter, err := objectstorage.New(*cfg)
	if err != nil {
		t.Fatalf("objectstorage.New: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create a direct minio client for cleanup only.
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
		Region: cfg.Region,
	})
	if err != nil {
		t.Fatalf("minio.New for cleanup client: %v", err)
	}

	photoID := newTestUUID(t)
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanCancel()
		if err := mc.RemoveObject(cleanCtx, cfg.Bucket, photoID, minio.RemoveObjectOptions{}); err != nil {
			t.Errorf("cleanup RemoveObject %s: %v", photoID, err)
		}
	})

	testBytes := bytes.Repeat([]byte{0xFF, 0xD8, 0xFF}, 17) // JPEG-like bytes

	// 1. Verify bucket is accessible.
	if err := adapter.CheckBucket(ctx); err != nil {
		t.Fatalf("CheckBucket: %v", err)
	}

	// 2. Presign an upload URL and PUT the test bytes to MinIO.
	uploadResult, err := adapter.PresignUpload(ctx, photoID, "image/jpeg")
	if err != nil {
		t.Fatalf("PresignUpload: %v", err)
	}
	if uploadResult.URL == "" {
		t.Fatal("PresignUpload returned empty URL")
	}
	if uploadResult.ExpiresAt.Before(time.Now()) {
		t.Errorf("ExpiresAt %v is in the past", uploadResult.ExpiresAt)
	}
	if ct := uploadResult.Headers["Content-Type"]; ct != "image/jpeg" {
		t.Errorf("upload Headers[Content-Type] = %q, want image/jpeg", ct)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, uploadResult.URL, bytes.NewReader(testBytes))
	if err != nil {
		t.Fatalf("build PUT request: %v", err)
	}
	for k, v := range uploadResult.Headers {
		req.Header.Set(k, v)
	}
	req.ContentLength = int64(len(testBytes))

	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT to presigned URL: %v", err)
	}
	putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status %d, want 200", putResp.StatusCode)
	}

	// 3. Presign a download URL and GET the object, verify bytes.
	downloadResult, err := adapter.PresignDownload(ctx, photoID)
	if err != nil {
		t.Fatalf("PresignDownload: %v", err)
	}
	if downloadResult.URL == "" {
		t.Fatal("PresignDownload returned empty URL")
	}

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadResult.URL, nil)
	if err != nil {
		t.Fatalf("build GET request: %v", err)
	}
	getResp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("GET presigned URL: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("GET status %d, want 200", getResp.StatusCode)
	}
	gotBytes, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("read GET body: %v", err)
	}
	if !bytes.Equal(gotBytes, testBytes) {
		t.Errorf("GET bytes mismatch: got len=%d, want len=%d", len(gotBytes), len(testBytes))
	}

	// 4. OpenOriginal and verify bytes.
	rc, err := adapter.OpenOriginal(ctx, photoID)
	if err != nil {
		t.Fatalf("OpenOriginal: %v", err)
	}
	defer rc.Close()
	streamBytes, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read OpenOriginal stream: %v", err)
	}
	if !bytes.Equal(streamBytes, testBytes) {
		t.Errorf("OpenOriginal bytes mismatch: got len=%d, want len=%d", len(streamBytes), len(testBytes))
	}

	// 5. OpenOriginal on a non-existent object returns not-found.
	missingID := newTestUUID(t)
	_, err = adapter.OpenOriginal(ctx, missingID)
	if !objectstorage.IsNotFound(err) {
		t.Errorf("OpenOriginal(missing): want IsNotFound, got %v", err)
	}
}

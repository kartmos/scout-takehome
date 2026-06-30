package objectstorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"scout/internal/config"
	"scout/internal/domain"
)

// sdkObject abstracts *minio.Object so unit tests need no network.
type sdkObject interface {
	io.ReadCloser
	Stat() (minio.ObjectInfo, error)
}

// sdkClient abstracts the minio.Client methods used by MinIOAdapter.
type sdkClient interface {
	PresignHeader(ctx context.Context, method, bucket, object string, expires time.Duration, reqParams url.Values, extraHeaders http.Header) (*url.URL, error)
	GetObject(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (sdkObject, error)
	BucketExists(ctx context.Context, bucket string) (bool, error)
}

// realSDKClient wraps *minio.Client to satisfy sdkClient.
type realSDKClient struct {
	c *minio.Client
}

func (r *realSDKClient) PresignHeader(ctx context.Context, method, bucket, object string, expires time.Duration, reqParams url.Values, extraHeaders http.Header) (*url.URL, error) {
	return r.c.PresignHeader(ctx, method, bucket, object, expires, reqParams, extraHeaders)
}

func (r *realSDKClient) GetObject(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (sdkObject, error) {
	obj, err := r.c.GetObject(ctx, bucket, object, opts)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

func (r *realSDKClient) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return r.c.BucketExists(ctx, bucket)
}

// clock abstracts time.Now for deterministic tests.
type clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// MinIOAdapter implements OriginalStorage backed by a MinIO/S3-compatible store.
type MinIOAdapter struct {
	client      sdkClient
	bucket      string
	uploadTTL   time.Duration
	downloadTTL time.Duration
	clk         clock
}

// New constructs a MinIOAdapter from the provided S3Config.
// No network calls, bucket creation, or background goroutines are performed.
func New(cfg config.S3Config) (*MinIOAdapter, error) {
	opts := &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.Secure,
		Region: cfg.Region,
	}
	c, err := minio.New(cfg.Endpoint, opts)
	if err != nil {
		return nil, fmt.Errorf("objectstorage: construct client: %w", err)
	}
	return &MinIOAdapter{
		client:      &realSDKClient{c: c},
		bucket:      cfg.Bucket,
		uploadTTL:   cfg.UploadTTL,
		downloadTTL: cfg.DownloadTTL,
		clk:         realClock{},
	}, nil
}

// newWithClient is the internal constructor for unit tests.
func newWithClient(client sdkClient, bucket string, uploadTTL, downloadTTL time.Duration, clk clock) *MinIOAdapter {
	return &MinIOAdapter{
		client:      client,
		bucket:      bucket,
		uploadTTL:   uploadTTL,
		downloadTTL: downloadTTL,
		clk:         clk,
	}
}

func (a *MinIOAdapter) PresignUpload(ctx context.Context, photoID, contentType string) (UploadResult, error) {
	if err := validatePhotoID(photoID); err != nil {
		return UploadResult{}, newStorageError(OpPresignUpload, CategoryInvalidInput, err.Error(), err)
	}
	if err := validateContentType(contentType); err != nil {
		return UploadResult{}, newStorageError(OpPresignUpload, CategoryInvalidInput, err.Error(), err)
	}

	extraHeaders := http.Header{}
	extraHeaders.Set("Content-Type", contentType)

	u, err := a.client.PresignHeader(ctx, "PUT", a.bucket, photoID, a.uploadTTL, nil, extraHeaders)
	if err != nil {
		return UploadResult{}, newStorageError(OpPresignUpload, CategoryInternal, "presign failed", err)
	}

	return UploadResult{
		URL:       u.String(),
		Headers:   map[string]string{"Content-Type": contentType},
		ExpiresAt: a.clk.Now().Add(a.uploadTTL),
	}, nil
}

func (a *MinIOAdapter) PresignDownload(ctx context.Context, photoID string) (DownloadResult, error) {
	if err := validatePhotoID(photoID); err != nil {
		return DownloadResult{}, newStorageError(OpPresignDownload, CategoryInvalidInput, err.Error(), err)
	}

	u, err := a.client.PresignHeader(ctx, "GET", a.bucket, photoID, a.downloadTTL, nil, nil)
	if err != nil {
		return DownloadResult{}, newStorageError(OpPresignDownload, CategoryInternal, "presign failed", err)
	}

	return DownloadResult{
		URL:       u.String(),
		ExpiresAt: a.clk.Now().Add(a.downloadTTL),
	}, nil
}

func (a *MinIOAdapter) OpenOriginal(ctx context.Context, photoID string) (io.ReadCloser, error) {
	if err := validatePhotoID(photoID); err != nil {
		return nil, newStorageError(OpOpenOriginal, CategoryInvalidInput, err.Error(), err)
	}

	obj, err := a.client.GetObject(ctx, a.bucket, photoID, minio.GetObjectOptions{})
	if err != nil {
		return nil, classifyError(OpOpenOriginal, "get failed", err)
	}

	// GetObject is lazy; Stat() forces the actual network request so we can
	// detect a missing or inaccessible object before handing the stream to the caller.
	if _, statErr := obj.Stat(); statErr != nil {
		obj.Close()
		return nil, classifyError(OpOpenOriginal, "stat failed", statErr)
	}

	return &translatingReader{rc: obj, op: OpOpenOriginal}, nil
}

func (a *MinIOAdapter) CheckBucket(ctx context.Context) error {
	exists, err := a.client.BucketExists(ctx, a.bucket)
	if err != nil {
		return classifyError(OpCheckBucket, "bucket check failed", err)
	}
	if !exists {
		return newStorageError(OpCheckBucket, CategoryBucketUnavailable, "bucket not found", nil)
	}
	return nil
}

// translatingReader wraps an sdkObject and maps read-time errors to StorageError.
type translatingReader struct {
	rc sdkObject
	op Op
}

func (r *translatingReader) Read(p []byte) (int, error) {
	n, err := r.rc.Read(p)
	if err == nil || err == io.EOF {
		return n, err
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return n, err
	}
	return n, newStorageError(r.op, CategoryInternal, "read failed", err)
}

func (r *translatingReader) Close() error {
	return r.rc.Close()
}

// classifyError maps a minio SDK error to a typed StorageError.
// It inspects the S3 error code without string-matching message bodies.
func classifyError(op Op, safeMsg string, err error) *StorageError {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return newStorageError(op, CategoryInternal, "context cancelled", err)
	}
	er := minio.ToErrorResponse(err)
	switch er.Code {
	case "NoSuchKey":
		return newStorageError(op, CategoryNotFound, "object not found", err)
	case "NoSuchBucket", "AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch":
		return newStorageError(op, CategoryBucketUnavailable, "bucket not found or inaccessible", err)
	}
	return newStorageError(op, CategoryInternal, safeMsg, err)
}

// validatePhotoID ensures the ID is a canonical UUID.
// A valid UUID cannot contain path separators or other unsafe characters.
func validatePhotoID(id string) error {
	if !domain.IsValidUUID(id) {
		return fmt.Errorf("photo ID must be a canonical UUID")
	}
	return nil
}

// validateContentType ensures the content type is safe for use as an HTTP header value.
func validateContentType(ct string) error {
	if strings.TrimSpace(ct) == "" {
		return fmt.Errorf("content type must not be empty")
	}
	if strings.ContainsAny(ct, "\r\n") {
		return fmt.Errorf("content type contains unsafe characters (CR/LF)")
	}
	return nil
}

package objectstorage

import (
	"context"
	"io"
	"time"
)

// UploadResult carries the presigned PUT URL and required signed headers.
type UploadResult struct {
	URL       string            // presigned PUT URL
	Headers   map[string]string // headers the client must include (includes at minimum Content-Type)
	ExpiresAt time.Time
}

// DownloadResult carries the presigned GET URL.
type DownloadResult struct {
	URL       string
	ExpiresAt time.Time
}

// OriginalStorage is the minimal interface for photo original object storage.
type OriginalStorage interface {
	// PresignUpload returns a short-lived presigned PUT URL for uploading a photo original.
	// photoID must be a canonical UUID; contentType must be a safe, non-empty MIME type.
	// Content-Type is part of the PUT signature; the client must send it with the upload.
	PresignUpload(ctx context.Context, photoID string, contentType string) (UploadResult, error)

	// PresignDownload returns a short-lived presigned GET URL for downloading a photo original.
	// photoID must be a canonical UUID. No public bucket policy is required.
	PresignDownload(ctx context.Context, photoID string) (DownloadResult, error)

	// OpenOriginal opens the photo original as a cancellable read stream.
	// The caller owns the returned ReadCloser and must close it.
	// Returns StorageError with CategoryNotFound when the object does not exist.
	OpenOriginal(ctx context.Context, photoID string) (io.ReadCloser, error)

	// CheckBucket verifies the configured bucket is accessible without modifying it.
	// Returns StorageError with CategoryBucketUnavailable when the bucket is missing.
	CheckBucket(ctx context.Context) error
}

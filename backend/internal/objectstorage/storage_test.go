package objectstorage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

// fixedClock returns a deterministic time for tests.
type fixedClock struct{ t time.Time }

func (f fixedClock) Now() time.Time { return f.t }

// fakeSdkClient implements sdkClient for unit tests without network access.
type fakeSdkClient struct {
	presignFn      func(ctx context.Context, method, bucket, object string, expires time.Duration, reqParams url.Values, extraHeaders http.Header) (*url.URL, error)
	getObjectFn    func(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (sdkObject, error)
	bucketExistsFn func(ctx context.Context, bucket string) (bool, error)
}

func (f *fakeSdkClient) PresignHeader(ctx context.Context, method, bucket, object string, expires time.Duration, reqParams url.Values, extraHeaders http.Header) (*url.URL, error) {
	return f.presignFn(ctx, method, bucket, object, expires, reqParams, extraHeaders)
}

func (f *fakeSdkClient) GetObject(ctx context.Context, bucket, object string, opts minio.GetObjectOptions) (sdkObject, error) {
	return f.getObjectFn(ctx, bucket, object, opts)
}

func (f *fakeSdkClient) BucketExists(ctx context.Context, bucket string) (bool, error) {
	return f.bucketExistsFn(ctx, bucket)
}

// fakeSDKObject implements sdkObject for unit tests.
type fakeSDKObject struct {
	r      io.Reader
	statFn func() (minio.ObjectInfo, error)
	closed bool
}

func (o *fakeSDKObject) Read(p []byte) (int, error) { return o.r.Read(p) }
func (o *fakeSDKObject) Close() error               { o.closed = true; return nil }
func (o *fakeSDKObject) Stat() (minio.ObjectInfo, error) {
	if o.statFn != nil {
		return o.statFn()
	}
	return minio.ObjectInfo{}, nil
}

var (
	validUUID   = "01234567-89ab-cdef-0123-456789abcdef"
	fixedNow    = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	testUpTTL   = 15 * time.Minute
	testDownTTL = 10 * time.Minute
)

func newTestAdapter(fake *fakeSdkClient) *MinIOAdapter {
	return newWithClient(fake, fake, "test-bucket", testUpTTL, testDownTTL, fixedClock{t: fixedNow})
}

func mustParseURL(raw string) *url.URL {
	u, _ := url.Parse(raw)
	return u
}

// ---- PresignUpload tests ----

func TestPresignUpload_ValidInput(t *testing.T) {
	fake := &fakeSdkClient{
		presignFn: func(_ context.Context, method, bucket, object string, expires time.Duration, _ url.Values, extra http.Header) (*url.URL, error) {
			if method != "PUT" {
				t.Errorf("method = %q, want PUT", method)
			}
			if object != validUUID {
				t.Errorf("object = %q, want %q", object, validUUID)
			}
			if bucket != "test-bucket" {
				t.Errorf("bucket = %q, want test-bucket", bucket)
			}
			if expires != testUpTTL {
				t.Errorf("expires = %v, want %v", expires, testUpTTL)
			}
			if ct := extra.Get("Content-Type"); ct != "image/jpeg" {
				t.Errorf("signed Content-Type = %q, want image/jpeg", ct)
			}
			return mustParseURL("http://minio:9000/test-bucket/" + object + "?X-Amz-Signature=abc"), nil
		},
	}
	a := newTestAdapter(fake)
	res, err := a.PresignUpload(context.Background(), validUUID, "image/jpeg")
	if err != nil {
		t.Fatalf("PresignUpload error: %v", err)
	}
	if !strings.Contains(res.URL, validUUID) {
		t.Errorf("URL %q does not contain photo ID", res.URL)
	}
	if res.Headers["Content-Type"] != "image/jpeg" {
		t.Errorf("Headers[Content-Type] = %q, want image/jpeg", res.Headers["Content-Type"])
	}
	wantExpiry := fixedNow.Add(testUpTTL)
	if !res.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", res.ExpiresAt, wantExpiry)
	}
}

func TestPresignUpload_InvalidUUID(t *testing.T) {
	called := false
	fake := &fakeSdkClient{
		presignFn: func(_ context.Context, _, _, _ string, _ time.Duration, _ url.Values, _ http.Header) (*url.URL, error) {
			called = true
			return nil, nil
		},
	}
	a := newTestAdapter(fake)
	for _, id := range []string{"", "not-a-uuid", "../../etc/passwd", "test/path", validUUID + "x"} {
		_, err := a.PresignUpload(context.Background(), id, "image/jpeg")
		if err == nil {
			t.Errorf("PresignUpload(%q): want error, got nil", id)
		}
		var se *StorageError
		if !errors.As(err, &se) || se.Cat != CategoryInvalidInput {
			t.Errorf("PresignUpload(%q): want CategoryInvalidInput, got %v", id, err)
		}
	}
	if called {
		t.Error("SDK was called despite invalid photo ID")
	}
}

func TestPresignUpload_InvalidContentType(t *testing.T) {
	called := false
	fake := &fakeSdkClient{
		presignFn: func(_ context.Context, _, _, _ string, _ time.Duration, _ url.Values, _ http.Header) (*url.URL, error) {
			called = true
			return nil, nil
		},
	}
	a := newTestAdapter(fake)
	for _, ct := range []string{"", "   ", "image/jpeg\r\n", "text/html\r", "img\nX-Injected: val"} {
		_, err := a.PresignUpload(context.Background(), validUUID, ct)
		if err == nil {
			t.Errorf("PresignUpload with ct=%q: want error, got nil", ct)
		}
		var se *StorageError
		if !errors.As(err, &se) || se.Cat != CategoryInvalidInput {
			t.Errorf("expected CategoryInvalidInput for ct=%q, got %v", ct, err)
		}
	}
	if called {
		t.Error("SDK was called despite invalid content type")
	}
}

func TestPresignUpload_SDKError(t *testing.T) {
	sdkErr := errors.New("sdk internal error")
	fake := &fakeSdkClient{
		presignFn: func(_ context.Context, _, _, _ string, _ time.Duration, _ url.Values, _ http.Header) (*url.URL, error) {
			return nil, sdkErr
		},
	}
	a := newTestAdapter(fake)
	_, err := a.PresignUpload(context.Background(), validUUID, "image/jpeg")
	var se *StorageError
	if !errors.As(err, &se) || se.Cat != CategoryInternal {
		t.Fatalf("want CategoryInternal, got %v", err)
	}
	if !errors.Is(err, sdkErr) {
		t.Error("cause should be unwrappable")
	}
}

// ---- PresignDownload tests ----

func TestPresignDownload_ValidInput(t *testing.T) {
	fake := &fakeSdkClient{
		presignFn: func(_ context.Context, method, _, object string, expires time.Duration, _ url.Values, extra http.Header) (*url.URL, error) {
			if method != "GET" {
				t.Errorf("method = %q, want GET", method)
			}
			if expires != testDownTTL {
				t.Errorf("expires = %v, want %v", expires, testDownTTL)
			}
			return mustParseURL("http://minio:9000/test-bucket/" + object + "?X-Amz-Signature=xyz"), nil
		},
	}
	a := newTestAdapter(fake)
	res, err := a.PresignDownload(context.Background(), validUUID)
	if err != nil {
		t.Fatalf("PresignDownload error: %v", err)
	}
	if !strings.Contains(res.URL, validUUID) {
		t.Errorf("URL %q does not contain photo ID", res.URL)
	}
	wantExpiry := fixedNow.Add(testDownTTL)
	if !res.ExpiresAt.Equal(wantExpiry) {
		t.Errorf("ExpiresAt = %v, want %v", res.ExpiresAt, wantExpiry)
	}
}

func TestPresignDownload_InvalidUUID(t *testing.T) {
	called := false
	fake := &fakeSdkClient{
		presignFn: func(_ context.Context, _, _, _ string, _ time.Duration, _ url.Values, _ http.Header) (*url.URL, error) {
			called = true
			return nil, nil
		},
	}
	a := newTestAdapter(fake)
	_, err := a.PresignDownload(context.Background(), "bad-id")
	if err == nil {
		t.Fatal("want error, got nil")
	}
	var se *StorageError
	if !errors.As(err, &se) || se.Cat != CategoryInvalidInput {
		t.Errorf("want CategoryInvalidInput, got %v", err)
	}
	if called {
		t.Error("SDK was called despite invalid photo ID")
	}
}

// ---- OpenOriginal tests ----

func TestOpenOriginal_Success(t *testing.T) {
	data := []byte("fake-jpeg-bytes")
	fake := &fakeSdkClient{
		getObjectFn: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (sdkObject, error) {
			return &fakeSDKObject{r: bytes.NewReader(data)}, nil
		},
	}
	a := newTestAdapter(fake)
	rc, err := a.OpenOriginal(context.Background(), validUUID)
	if err != nil {
		t.Fatalf("OpenOriginal error: %v", err)
	}
	defer rc.Close()
	got, _ := io.ReadAll(rc)
	if !bytes.Equal(got, data) {
		t.Errorf("read %q, want %q", got, data)
	}
}

func TestOpenOriginal_NotFound(t *testing.T) {
	notFoundErr := minio.ErrorResponse{Code: "NoSuchKey"}
	fake := &fakeSdkClient{
		getObjectFn: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (sdkObject, error) {
			return &fakeSDKObject{
				r:      bytes.NewReader(nil),
				statFn: func() (minio.ObjectInfo, error) { return minio.ObjectInfo{}, notFoundErr },
			}, nil
		},
	}
	a := newTestAdapter(fake)
	_, err := a.OpenOriginal(context.Background(), validUUID)
	if !IsNotFound(err) {
		t.Fatalf("want IsNotFound, got %v", err)
	}
}

func TestOpenOriginal_BucketUnavailable(t *testing.T) {
	bucketErr := minio.ErrorResponse{Code: "NoSuchBucket"}
	fake := &fakeSdkClient{
		getObjectFn: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (sdkObject, error) {
			return &fakeSDKObject{
				r:      bytes.NewReader(nil),
				statFn: func() (minio.ObjectInfo, error) { return minio.ObjectInfo{}, bucketErr },
			}, nil
		},
	}
	a := newTestAdapter(fake)
	_, err := a.OpenOriginal(context.Background(), validUUID)
	if !IsBucketUnavailable(err) {
		t.Fatalf("want IsBucketUnavailable, got %v", err)
	}
}

func TestOpenOriginal_ClosesObjectOnStatFailure(t *testing.T) {
	obj := &fakeSDKObject{
		r:      bytes.NewReader(nil),
		statFn: func() (minio.ObjectInfo, error) { return minio.ObjectInfo{}, errors.New("stat error") },
	}
	fake := &fakeSdkClient{
		getObjectFn: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (sdkObject, error) {
			return obj, nil
		},
	}
	a := newTestAdapter(fake)
	_, err := a.OpenOriginal(context.Background(), validUUID)
	if err == nil {
		t.Fatal("want error")
	}
	if !obj.closed {
		t.Error("object was not closed after stat failure")
	}
}

func TestOpenOriginal_InvalidUUID(t *testing.T) {
	called := false
	fake := &fakeSdkClient{
		getObjectFn: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (sdkObject, error) {
			called = true
			return nil, nil
		},
	}
	a := newTestAdapter(fake)
	_, err := a.OpenOriginal(context.Background(), "not-a-uuid")
	if err == nil {
		t.Fatal("want error")
	}
	var se *StorageError
	if !errors.As(err, &se) || se.Cat != CategoryInvalidInput {
		t.Errorf("want CategoryInvalidInput, got %v", err)
	}
	if called {
		t.Error("SDK was called despite invalid photo ID")
	}
}

func TestOpenOriginal_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fake := &fakeSdkClient{
		getObjectFn: func(ctx context.Context, _, _ string, _ minio.GetObjectOptions) (sdkObject, error) {
			return nil, ctx.Err()
		},
	}
	a := newTestAdapter(fake)
	_, err := a.OpenOriginal(ctx, validUUID)
	if err == nil {
		t.Fatal("want error")
	}
	// The SDK error from context cancellation is classified as internal.
	var se *StorageError
	if !errors.As(err, &se) {
		t.Errorf("want StorageError, got %T: %v", err, err)
	}
}

func TestOpenOriginal_ReadTimeErrorTranslated(t *testing.T) {
	readErr := errors.New("connection reset")
	obj := &fakeSDKObject{
		r: &errorReader{err: readErr},
	}
	fake := &fakeSdkClient{
		getObjectFn: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (sdkObject, error) {
			return obj, nil
		},
	}
	a := newTestAdapter(fake)
	rc, err := a.OpenOriginal(context.Background(), validUUID)
	if err != nil {
		t.Fatalf("OpenOriginal error: %v", err)
	}
	defer rc.Close()
	_, readErr2 := io.ReadAll(rc)
	var se *StorageError
	if !errors.As(readErr2, &se) || se.Cat != CategoryInternal {
		t.Errorf("want CategoryInternal read error, got %v", readErr2)
	}
	if !errors.Is(readErr2, readErr) {
		t.Error("original cause should be unwrappable")
	}
}

func TestOpenOriginal_ReadTimeContextPreserved(t *testing.T) {
	obj := &fakeSDKObject{
		r: &errorReader{err: context.Canceled},
	}
	fake := &fakeSdkClient{
		getObjectFn: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (sdkObject, error) {
			return obj, nil
		},
	}
	a := newTestAdapter(fake)
	rc, err := a.OpenOriginal(context.Background(), validUUID)
	if err != nil {
		t.Fatalf("OpenOriginal error: %v", err)
	}
	defer rc.Close()
	_, readErr := io.ReadAll(rc)
	if !errors.Is(readErr, context.Canceled) {
		t.Errorf("want context.Canceled to pass through, got %v", readErr)
	}
}

func TestOpenOriginal_EOFPreserved(t *testing.T) {
	obj := &fakeSDKObject{r: bytes.NewReader([]byte("x"))}
	fake := &fakeSdkClient{
		getObjectFn: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (sdkObject, error) {
			return obj, nil
		},
	}
	a := newTestAdapter(fake)
	rc, err := a.OpenOriginal(context.Background(), validUUID)
	if err != nil {
		t.Fatalf("OpenOriginal error: %v", err)
	}
	defer rc.Close()
	buf := make([]byte, 10)
	rc.Read(buf) //nolint:errcheck
	_, err2 := rc.Read(buf)
	if err2 != io.EOF {
		t.Errorf("want io.EOF, got %v", err2)
	}
}

// ---- CheckBucket tests ----

func TestCheckBucket_Exists(t *testing.T) {
	fake := &fakeSdkClient{
		bucketExistsFn: func(_ context.Context, bucket string) (bool, error) {
			if bucket != "test-bucket" {
				t.Errorf("bucket = %q, want test-bucket", bucket)
			}
			return true, nil
		},
	}
	a := newTestAdapter(fake)
	if err := a.CheckBucket(context.Background()); err != nil {
		t.Fatalf("CheckBucket error: %v", err)
	}
}

func TestCheckBucket_NotFound(t *testing.T) {
	fake := &fakeSdkClient{
		bucketExistsFn: func(_ context.Context, _ string) (bool, error) {
			return false, nil
		},
	}
	a := newTestAdapter(fake)
	err := a.CheckBucket(context.Background())
	if !IsBucketUnavailable(err) {
		t.Fatalf("want IsBucketUnavailable, got %v", err)
	}
}

func TestCheckBucket_SDKError(t *testing.T) {
	sdkErr := errors.New("network error")
	fake := &fakeSdkClient{
		bucketExistsFn: func(_ context.Context, _ string) (bool, error) {
			return false, sdkErr
		},
	}
	a := newTestAdapter(fake)
	err := a.CheckBucket(context.Background())
	var se *StorageError
	if !errors.As(err, &se) || se.Cat != CategoryInternal {
		t.Fatalf("want CategoryInternal, got %v", err)
	}
}

func TestCheckBucket_S3Errors(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantCat Category
	}{
		{name: "NoSuchBucket", code: "NoSuchBucket", wantCat: CategoryBucketUnavailable},
		{name: "AccessDenied", code: "AccessDenied", wantCat: CategoryBucketUnavailable},
		{name: "InvalidAccessKeyId", code: "InvalidAccessKeyId", wantCat: CategoryBucketUnavailable},
		{name: "SignatureDoesNotMatch", code: "SignatureDoesNotMatch", wantCat: CategoryBucketUnavailable},
		{name: "UnknownCode", code: "InternalError", wantCat: CategoryInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s3Err := minio.ErrorResponse{Code: tt.code}
			fake := &fakeSdkClient{
				bucketExistsFn: func(_ context.Context, _ string) (bool, error) {
					return false, s3Err
				},
			}
			a := newTestAdapter(fake)
			err := a.CheckBucket(context.Background())
			var se *StorageError
			if !errors.As(err, &se) {
				t.Fatalf("want StorageError, got %T: %v", err, err)
			}
			if se.Cat != tt.wantCat {
				t.Errorf("Cat = %v, want %v", se.Cat, tt.wantCat)
			}
			if !errors.Is(err, s3Err) {
				t.Error("cause should be unwrappable")
			}
		})
	}
}

// ---- Error safety tests ----

func TestStorageError_SafeText(t *testing.T) {
	sensitiveURL := "http://key:secret@minio:9000/bucket/obj?X-Amz-Signature=TOPSECRET&X-Amz-Credential=ACCESSKEY"
	se := newStorageError(OpPresignUpload, CategoryInternal, "presign failed", errors.New(sensitiveURL))
	msg := se.Error()
	for _, sensitive := range []string{"key", "secret", "TOPSECRET", "ACCESSKEY", sensitiveURL} {
		if strings.Contains(msg, sensitive) {
			t.Errorf("error message contains sensitive value %q: %q", sensitive, msg)
		}
	}
}

func TestStorageError_Unwrap(t *testing.T) {
	cause := errors.New("root cause")
	se := newStorageError(OpOpenOriginal, CategoryInternal, "internal", cause)
	if !errors.Is(se, cause) {
		t.Error("errors.Is should find the cause via Unwrap")
	}
}

func TestIsNotFound(t *testing.T) {
	se := newStorageError(OpOpenOriginal, CategoryNotFound, "not found", nil)
	if !IsNotFound(se) {
		t.Error("IsNotFound should be true")
	}
	if IsNotFound(newStorageError(OpOpenOriginal, CategoryInternal, "other", nil)) {
		t.Error("IsNotFound should be false for CategoryInternal")
	}
	if IsNotFound(errors.New("plain error")) {
		t.Error("IsNotFound should be false for non-StorageError")
	}
}

func TestIsBucketUnavailable(t *testing.T) {
	se := newStorageError(OpCheckBucket, CategoryBucketUnavailable, "unavailable", nil)
	if !IsBucketUnavailable(se) {
		t.Error("IsBucketUnavailable should be true")
	}
}

// ---- Key derivation tests ----

func TestValidatePhotoID(t *testing.T) {
	valid := []string{
		"01234567-89ab-cdef-0123-456789abcdef",
		"AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",
	}
	for _, id := range valid {
		if err := validatePhotoID(id); err != nil {
			t.Errorf("validatePhotoID(%q) = %v, want nil", id, err)
		}
	}

	invalid := []string{
		"",
		"not-a-uuid",
		"../../etc",
		"01234567-89ab-cdef-0123-456789abcde",   // too short
		"01234567-89ab-cdef-0123-456789abcdeff", // too long
		"01234567/89ab/cdef/0123/456789abcdef",  // slashes
		"01234567-89ab-cdef-0123-456789abcde!",  // invalid char
		"01234567-89ab-cdef-0123-456789abcde ",  // space
		"..",
	}
	for _, id := range invalid {
		if err := validatePhotoID(id); err == nil {
			t.Errorf("validatePhotoID(%q) = nil, want error", id)
		}
	}
}

// ---- Split-client (public endpoint) tests ----

// TestPresignUsesPublicClient verifies that PresignUpload and PresignDownload use
// the presign client (public endpoint) and not the internal client.
func TestPresignUsesPublicClient(t *testing.T) {
	internalFake := &fakeSdkClient{
		presignFn: func(_ context.Context, _, _, _ string, _ time.Duration, _ url.Values, _ http.Header) (*url.URL, error) {
			t.Error("internal client must not be used for presigning when a public client is configured")
			return nil, nil
		},
	}
	publicFake := &fakeSdkClient{
		presignFn: func(_ context.Context, _, _, object string, _ time.Duration, _ url.Values, _ http.Header) (*url.URL, error) {
			return mustParseURL("http://minio.localhost:9000/test-bucket/" + object + "?sig=abc"), nil
		},
	}
	a := newWithClient(internalFake, publicFake, "test-bucket", testUpTTL, testDownTTL, fixedClock{t: fixedNow})

	upRes, err := a.PresignUpload(context.Background(), validUUID, "image/jpeg")
	if err != nil {
		t.Fatalf("PresignUpload error: %v", err)
	}
	if !strings.Contains(upRes.URL, "minio.localhost") {
		t.Errorf("PresignUpload URL %q must contain the public hostname", upRes.URL)
	}

	downRes, err := a.PresignDownload(context.Background(), validUUID)
	if err != nil {
		t.Fatalf("PresignDownload error: %v", err)
	}
	if !strings.Contains(downRes.URL, "minio.localhost") {
		t.Errorf("PresignDownload URL %q must contain the public hostname", downRes.URL)
	}
}

// TestInternalOperationsUseInternalClient verifies that CheckBucket and OpenOriginal
// never use the public presign client.
func TestInternalOperationsUseInternalClient(t *testing.T) {
	publicCalled := false
	publicFake := &fakeSdkClient{
		bucketExistsFn: func(_ context.Context, _ string) (bool, error) {
			publicCalled = true
			return true, nil
		},
		getObjectFn: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (sdkObject, error) {
			publicCalled = true
			return nil, nil
		},
	}
	internalFake := &fakeSdkClient{
		bucketExistsFn: func(_ context.Context, _ string) (bool, error) {
			return true, nil
		},
		getObjectFn: func(_ context.Context, _, _ string, _ minio.GetObjectOptions) (sdkObject, error) {
			return &fakeSDKObject{r: bytes.NewReader([]byte("data"))}, nil
		},
	}
	a := newWithClient(internalFake, publicFake, "test-bucket", testUpTTL, testDownTTL, fixedClock{t: fixedNow})

	if err := a.CheckBucket(context.Background()); err != nil {
		t.Fatalf("CheckBucket error: %v", err)
	}
	rc, err := a.OpenOriginal(context.Background(), validUUID)
	if err != nil {
		t.Fatalf("OpenOriginal error: %v", err)
	}
	rc.Close()

	if publicCalled {
		t.Error("public (presign) client was used for internal bucket/object operations")
	}
}

// errorReader is an io.Reader that always returns an error.
type errorReader struct{ err error }

func (e *errorReader) Read(_ []byte) (int, error) { return 0, e.err }

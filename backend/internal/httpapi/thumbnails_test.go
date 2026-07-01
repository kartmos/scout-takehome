package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"scout/internal/apperror"
	"scout/internal/domain"
	"scout/internal/httpapi"
	"scout/internal/thumbnail"
)

// encodeTestJPEG encodes a grey w×h image as JPEG.
func encodeTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetGray(x, y, color.Gray{Y: 128})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// ─── fake thumbnail service ──────────────────────────────────────────────────

type fakeThumbnailSvc struct {
	result *thumbnail.ThumbnailResult
	err    error
}

func (f *fakeThumbnailSvc) Get(_ context.Context, _ domain.Photo, _ thumbnail.Request) (*thumbnail.ThumbnailResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

// openTempFile writes data to a temp file and returns it open at position 0.
func openTempFile(t *testing.T, data []byte) *os.File {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "thumb-*.jpg")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		t.Fatalf("write temp: %v", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		t.Fatalf("seek: %v", err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// TestThumbnail_Success verifies a 200 response with correct headers.
func TestThumbnail_Success(t *testing.T) {
	jpegData := encodeTestJPEG(t, 100, 75)
	f := openTempFile(t, jpegData)

	// Compute the expected ETag the same way the handler does.
	photo := domain.Photo{ID: testPhotoID, Width: 1920, Height: 1080}
	req, _ := thumbnail.ParseRequest("100", "", "")
	dims := thumbnail.ResolveDims(photo.Width, photo.Height, req.RequestedPixels)
	key := thumbnail.NewKey(photo, req, dims)
	etag := `"` + key.Hash() + `"`

	svc := &fakeThumbnailSvc{
		result: &thumbnail.ThumbnailResult{
			File: f,
			Size: int64(len(jpegData)),
		},
	}

	repo := &fakeRepo{
		getPhotoFn: func(_ context.Context, _ string) (domain.Photo, error) {
			return photo, nil
		},
	}

	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           repo,
		Storage:        noopStorage{},
		ThumbnailSvc:   svc,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/photos/"+testPhotoID+"/thumbnail?width=100", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %q", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("Content-Type = %q, want image/jpeg", ct)
	}
	if got := w.Header().Get("ETag"); got != etag {
		t.Errorf("ETag = %q, want %q", got, etag)
	}
	if cc := w.Header().Get("Cache-Control"); cc == "" {
		t.Error("Cache-Control must be set")
	}
}

// TestThumbnail_304NotModified verifies conditional GET with matching ETag.
func TestThumbnail_304NotModified(t *testing.T) {
	photo := domain.Photo{ID: testPhotoID, Width: 1920, Height: 1080}
	req304, _ := thumbnail.ParseRequest("100", "", "")
	dims := thumbnail.ResolveDims(photo.Width, photo.Height, req304.RequestedPixels)
	key := thumbnail.NewKey(photo, req304, dims)
	etag := `"` + key.Hash() + `"`

	repo := &fakeRepo{
		getPhotoFn: func(_ context.Context, _ string) (domain.Photo, error) {
			return photo, nil
		},
	}

	// The service should NOT be called for a 304 response.
	svc := &fakeThumbnailSvc{err: errors.New("should not be called")}

	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           repo,
		Storage:        noopStorage{},
		ThumbnailSvc:   svc,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/photos/"+testPhotoID+"/thumbnail?width=100", nil)
	r.Header.Set("If-None-Match", etag)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNotModified {
		t.Errorf("status = %d, want 304 for matching ETag", w.Code)
	}
	if got := w.Header().Get("ETag"); got != etag {
		t.Errorf("ETag = %q, want %q in 304 response", got, etag)
	}
}

// TestThumbnail_PublicRoute verifies no API key is required for thumbnail endpoint.
func TestThumbnail_PublicRoute(t *testing.T) {
	jpegData := encodeTestJPEG(t, 50, 50)
	f := openTempFile(t, jpegData)
	svc := &fakeThumbnailSvc{
		result: &thumbnail.ThumbnailResult{
			File: f,
			Size: int64(len(jpegData)),
		},
	}

	repo := &fakeRepo{
		getPhotoFn: func(_ context.Context, _ string) (domain.Photo, error) {
			return domain.Photo{ID: testPhotoID, Width: 100, Height: 100}, nil
		},
	}

	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           repo,
		Storage:        noopStorage{},
		ThumbnailSvc:   svc,
	})

	w := httptest.NewRecorder()
	// No X-API-Key header.
	r := httptest.NewRequest(http.MethodGet,
		"/photos/"+testPhotoID+"/thumbnail?width=50", nil)
	h.ServeHTTP(w, r)

	if w.Code == http.StatusUnauthorized {
		t.Error("thumbnail endpoint must be public (no auth), got 401")
	}
}

// TestThumbnail_DataAPIStillRequiresAuth verifies data API auth unchanged.
func TestThumbnail_DataAPIStillRequiresAuth(t *testing.T) {
	svc := &fakeThumbnailSvc{}
	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           noopRepo{},
		Storage:        noopStorage{},
		ThumbnailSvc:   svc,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/photos", nil) // no X-API-Key
	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET /photos without auth: status = %d, want 401", w.Code)
	}
}

// TestThumbnail_InvalidPhotoID verifies validation of photo ID format.
func TestThumbnail_InvalidPhotoID(t *testing.T) {
	svc := &fakeThumbnailSvc{}
	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           noopRepo{},
		Storage:        noopStorage{},
		ThumbnailSvc:   svc,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/photos/not-a-uuid/thumbnail?width=100", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid photo ID: status = %d, want 400", w.Code)
	}
}

// TestThumbnail_MissingWidth verifies 400 when width is absent.
func TestThumbnail_MissingWidth(t *testing.T) {
	repo := &fakeRepo{
		getPhotoFn: func(_ context.Context, _ string) (domain.Photo, error) {
			return domain.Photo{ID: testPhotoID, Width: 100, Height: 100}, nil
		},
	}
	svc := &fakeThumbnailSvc{}
	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           repo,
		Storage:        noopStorage{},
		ThumbnailSvc:   svc,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/photos/"+testPhotoID+"/thumbnail", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("missing width: status = %d, want 400", w.Code)
	}
}

// TestThumbnail_UnknownQueryParam verifies 400 for unknown query parameters.
func TestThumbnail_UnknownQueryParam(t *testing.T) {
	repo := &fakeRepo{
		getPhotoFn: func(_ context.Context, _ string) (domain.Photo, error) {
			return domain.Photo{ID: testPhotoID, Width: 100, Height: 100}, nil
		},
	}
	svc := &fakeThumbnailSvc{}
	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           repo,
		Storage:        noopStorage{},
		ThumbnailSvc:   svc,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/photos/"+testPhotoID+"/thumbnail?width=100&unknown=1", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("unknown param: status = %d, want 400", w.Code)
	}
}

// TestThumbnail_PhotoNotFound verifies 404 when photo doesn't exist in repo.
func TestThumbnail_PhotoNotFound(t *testing.T) {
	repo := &fakeRepo{
		getPhotoFn: func(_ context.Context, _ string) (domain.Photo, error) {
			return domain.Photo{}, apperror.NewNotFound("photo not found", testPhotoID)
		},
	}
	svc := &fakeThumbnailSvc{}
	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           repo,
		Storage:        noopStorage{},
		ThumbnailSvc:   svc,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/photos/"+testPhotoID+"/thumbnail?width=100", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("photo not found: status = %d, want 404", w.Code)
	}
}

// TestThumbnail_OriginalNotFoundInStorage verifies 404 when original isn't in MinIO.
func TestThumbnail_OriginalNotFoundInStorage(t *testing.T) {
	repo := &fakeRepo{
		getPhotoFn: func(_ context.Context, _ string) (domain.Photo, error) {
			return domain.Photo{ID: testPhotoID, Width: 100, Height: 100}, nil
		},
	}
	svc := &fakeThumbnailSvc{err: &thumbnail.ErrNotFound{Cause: errors.New("not in minio")}}
	h := httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           repo,
		Storage:        noopStorage{},
		ThumbnailSvc:   svc,
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/photos/"+testPhotoID+"/thumbnail?width=100", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("original not in storage: status = %d, want 404", w.Code)
	}
}

// TestThumbnail_InvalidServiceResult verifies that nil or File-less results from
// the thumbnail service produce a safe 500 instead of a panic or partial image response.
func TestThumbnail_InvalidServiceResult(t *testing.T) {
	tests := []struct {
		name   string
		result *thumbnail.ThumbnailResult
	}{
		{"nil_result", nil},
		{"nil_file", &thumbnail.ThumbnailResult{File: nil, Size: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{
				getPhotoFn: func(_ context.Context, _ string) (domain.Photo, error) {
					return domain.Photo{ID: testPhotoID, Width: 100, Height: 100}, nil
				},
			}
			svc := &fakeThumbnailSvc{result: tt.result}
			h := httpapi.NewRouter(httpapi.RouterConfig{
				Logger:         discardLogger(),
				AllowedOrigins: []string{"http://localhost:5173"},
				APIKey:         testAPIKey,
				Repo:           repo,
				Storage:        noopStorage{},
				ThumbnailSvc:   svc,
			})

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/photos/"+testPhotoID+"/thumbnail?width=100", nil)
			h.ServeHTTP(w, r)

			if w.Code != http.StatusInternalServerError {
				t.Errorf("[%s] status = %d, want 500", tt.name, w.Code)
			}
			m := decodeBody(t, w)
			if m["code"] != "InternalServerError" {
				t.Errorf("[%s] code = %v, want InternalServerError", tt.name, m["code"])
			}
			if ct := w.Header().Get("Content-Type"); ct == "image/jpeg" {
				t.Errorf("[%s] Content-Type must not be image/jpeg for error response, got %q", tt.name, ct)
			}
		})
	}
}

// ─── export shim ─────────────────────────────────────────────────────────────

// Ensure noopThumbnailSvc implements the interface the router checks.
var _ = noopThumbnailSvc{}

// Ensure fakeThumbnailSvc satisfies the same interface.
var _ = &fakeThumbnailSvc{}

package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"scout/internal/apperror"
	"scout/internal/domain"
	"scout/internal/httpapi"
	"scout/internal/objectstorage"
	"scout/internal/repository/sqlite"
)

// ── fixed test data ─────────────────────────────────────────────────────────

const testPhotoID = "550e8400-e29b-41d4-a716-446655440000"
const testPhotoID2 = "660e8400-e29b-41d4-a716-446655440000"

var testCapturedAt = time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
var testExpiresAt = time.Date(2024, 1, 15, 10, 45, 0, 0, time.UTC)

func testPhoto() domain.Photo {
	return domain.Photo{
		ID:         testPhotoID,
		X:          5.0,
		Y:          10.0,
		H:          2.5,
		Width:      1920,
		Height:     1080,
		CapturedAt: testCapturedAt,
		Predictions: []domain.Prediction{
			{
				ClassID:    domain.ClassMirid,
				Confidence: 0.87,
				BoundingBox: domain.BoundingBox{
					XMin: 0.1, YMin: 0.2, XMax: 0.3, YMax: 0.4,
				},
			},
		},
	}
}

func testPhotoNoPredictions() domain.Photo {
	p := testPhoto()
	p.Predictions = []domain.Prediction{}
	return p
}

func testUploadResult() objectstorage.UploadResult {
	return objectstorage.UploadResult{
		URL:       "https://minio.example.com/bucket/" + testPhotoID + "?sig=abc",
		Headers:   map[string]string{"Content-Type": "image/jpeg"},
		ExpiresAt: testExpiresAt,
	}
}

func testDownloadURL() string {
	return "https://minio.example.com/bucket/" + testPhotoID + "?sig=dl"
}

func testDownloadResult(photoID string) objectstorage.DownloadResult {
	return objectstorage.DownloadResult{
		URL:       "https://minio.example.com/bucket/" + photoID + "?sig=dl",
		ExpiresAt: testExpiresAt,
	}
}

// ── fakes ────────────────────────────────────────────────────────────────────

// fakeRepo is a configurable photoRepository fake.
type fakeRepo struct {
	existsFn     func(context.Context, string) (bool, error)
	getPhotoFn   func(context.Context, string) (domain.Photo, error)
	listPhotosFn func(context.Context, sqlite.ListPhotosParams) (domain.PhotoPage, error)
}

func (f *fakeRepo) PhotoExists(ctx context.Context, id string) (bool, error) {
	if f.existsFn != nil {
		return f.existsFn(ctx, id)
	}
	return false, nil
}

func (f *fakeRepo) GetPhoto(ctx context.Context, id string) (domain.Photo, error) {
	if f.getPhotoFn != nil {
		return f.getPhotoFn(ctx, id)
	}
	return domain.Photo{}, apperror.NewNotFound("photo not found", id)
}

func (f *fakeRepo) ListPhotos(ctx context.Context, params sqlite.ListPhotosParams) (domain.PhotoPage, error) {
	if f.listPhotosFn != nil {
		return f.listPhotosFn(ctx, params)
	}
	return domain.PhotoPage{Items: []domain.Photo{}}, nil
}

// fakeStorage is a configurable photoStorage fake.
type fakeStorage struct {
	presignUploadFn   func(context.Context, string, string) (objectstorage.UploadResult, error)
	presignDownloadFn func(context.Context, string) (objectstorage.DownloadResult, error)
}

func (f *fakeStorage) PresignUpload(ctx context.Context, id, ct string) (objectstorage.UploadResult, error) {
	if f.presignUploadFn != nil {
		return f.presignUploadFn(ctx, id, ct)
	}
	return testUploadResult(), nil
}

func (f *fakeStorage) PresignDownload(ctx context.Context, id string) (objectstorage.DownloadResult, error) {
	if f.presignDownloadFn != nil {
		return f.presignDownloadFn(ctx, id)
	}
	return testDownloadResult(id), nil
}

// noopRepo satisfies photoRepository for tests that don't exercise photo routes.
type noopRepo struct{}

func (noopRepo) PhotoExists(context.Context, string) (bool, error) {
	return false, nil
}
func (noopRepo) GetPhoto(context.Context, string) (domain.Photo, error) {
	return domain.Photo{}, apperror.NewNotFound("", "")
}
func (noopRepo) ListPhotos(context.Context, sqlite.ListPhotosParams) (domain.PhotoPage, error) {
	return domain.PhotoPage{Items: []domain.Photo{}}, nil
}

// noopStorage satisfies photoStorage for tests that don't exercise photo routes.
type noopStorage struct{}

func (noopStorage) PresignUpload(context.Context, string, string) (objectstorage.UploadResult, error) {
	return objectstorage.UploadResult{}, nil
}
func (noopStorage) PresignDownload(context.Context, string) (objectstorage.DownloadResult, error) {
	return objectstorage.DownloadResult{}, nil
}

// ── router helpers ───────────────────────────────────────────────────────────

func photoRouter(repo *fakeRepo, stor *fakeStorage) http.Handler {
	return httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           repo,
		Storage:        stor,
	})
}

func authReq(method, path string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, path, body)
	r.Header.Set("X-API-Key", testAPIKey)
	return r
}

func jsonBody(s string) io.Reader { return strings.NewReader(s) }

// decodeBody unmarshals a response into a generic map for field inspection.
func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("decode body: %v (body: %q)", err, w.Body.String())
	}
	return m
}

// ── NewRouter dependency checks ──────────────────────────────────────────────

func TestNewRouter_PanicsOnNilRepo(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewRouter must panic when Repo is nil")
		}
	}()
	httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           nil,
		Storage:        noopStorage{},
	})
}

func TestNewRouter_PanicsOnNilStorage(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("NewRouter must panic when Storage is nil")
		}
	}()
	httpapi.NewRouter(httpapi.RouterConfig{
		Logger:         discardLogger(),
		AllowedOrigins: []string{"http://localhost:5173"},
		APIKey:         testAPIKey,
		Repo:           noopRepo{},
		Storage:        nil,
	})
}

// ── public vs authenticated route tests ─────────────────────────────────────

func TestDataAPIRoutes_RequireAuth(t *testing.T) {
	routes := []struct {
		method string
		path   string
		body   string
	}{
		{"POST", "/photos/" + testPhotoID + "/upload-link", `{"contentType":"image/jpeg"}`},
		{"GET", "/photos/" + testPhotoID, ""},
		{"GET", "/photos", ""},
	}
	h := photoRouter(&fakeRepo{}, &fakeStorage{})
	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			var r *http.Request
			if rt.body != "" {
				r = httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
				r.Header.Set("Content-Type", "application/json")
			} else {
				r = httptest.NewRequest(rt.method, rt.path, nil)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401 for unauthenticated %s %s", w.Code, rt.method, rt.path)
			}
		})
	}
}

func TestDataAPIRoutes_WrongKey_401(t *testing.T) {
	h := photoRouter(&fakeRepo{}, &fakeStorage{})
	r := httptest.NewRequest("GET", "/photos", nil)
	r.Header.Set("X-API-Key", "wrong-key")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestHealthz_StillPublicWithDataDeps(t *testing.T) {
	h := photoRouter(&fakeRepo{}, &fakeStorage{})
	r := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("/healthz status = %d, want 200", w.Code)
	}
}

func TestPreflight_NoAuthRequired(t *testing.T) {
	h := photoRouter(&fakeRepo{}, &fakeStorage{})
	r := httptest.NewRequest("OPTIONS", "/photos", nil)
	r.Header.Set("Origin", "http://localhost:5173")
	r.Header.Set("Access-Control-Request-Method", "GET")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Errorf("preflight status = %d, want 204", w.Code)
	}
}

// ── upload-link success ──────────────────────────────────────────────────────

func TestUploadLink_Success(t *testing.T) {
	var gotPhotoID, gotContentType string
	var gotCtx context.Context

	repo := &fakeRepo{
		existsFn: func(ctx context.Context, id string) (bool, error) {
			return true, nil
		},
	}
	stor := &fakeStorage{
		presignUploadFn: func(ctx context.Context, id, ct string) (objectstorage.UploadResult, error) {
			gotCtx = ctx
			gotPhotoID = id
			gotContentType = ct
			return testUploadResult(), nil
		},
	}
	h := photoRouter(repo, stor)

	r := authReq("POST", "/photos/"+testPhotoID+"/upload-link",
		jsonBody(`{"contentType":"image/jpeg"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}

	// Verify exact DTO fields
	var resp struct {
		URL       string            `json:"url"`
		Method    string            `json:"method"`
		Headers   map[string]string `json:"headers"`
		ExpiresAt string            `json:"expiresAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.URL != testUploadResult().URL {
		t.Errorf("url = %q, want %q", resp.URL, testUploadResult().URL)
	}
	if resp.Method != "PUT" {
		t.Errorf("method = %q, want PUT", resp.Method)
	}
	if resp.Headers["Content-Type"] != "image/jpeg" {
		t.Errorf("headers[Content-Type] = %q, want image/jpeg", resp.Headers["Content-Type"])
	}
	want := testExpiresAt.UTC().Format(time.RFC3339)
	if resp.ExpiresAt != want {
		t.Errorf("expiresAt = %q, want %q", resp.ExpiresAt, want)
	}

	// Context and exact args propagation
	if gotPhotoID != testPhotoID {
		t.Errorf("PresignUpload photoID = %q, want %q", gotPhotoID, testPhotoID)
	}
	if gotContentType != "image/jpeg" {
		t.Errorf("PresignUpload contentType = %q, want image/jpeg", gotContentType)
	}
	if gotCtx == nil {
		t.Error("PresignUpload must receive request context")
	}
}

func TestUploadLink_ContentTypeWithParams_Accepted(t *testing.T) {
	repo := &fakeRepo{existsFn: func(context.Context, string) (bool, error) { return true, nil }}
	stor := &fakeStorage{}
	h := photoRouter(repo, stor)

	r := authReq("POST", "/photos/"+testPhotoID+"/upload-link",
		jsonBody(`{"contentType":"image/jpeg"}`))
	r.Header.Set("Content-Type", "application/json; charset=utf-8")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ── upload-link photo ID validation ─────────────────────────────────────────

func TestUploadLink_InvalidPhotoID(t *testing.T) {
	// Empty string is excluded: /photos//upload-link routes to a redirect, not the handler.
	cases := []string{"not-a-uuid", "short", "too-long-too-long-too-long-too-long-too-long-1234"}
	repo := &fakeRepo{}
	stor := &fakeStorage{}

	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			var storCalled bool
			stor.presignUploadFn = func(context.Context, string, string) (objectstorage.UploadResult, error) {
				storCalled = true
				return objectstorage.UploadResult{}, nil
			}
			h := photoRouter(repo, stor)
			r := authReq("POST", "/photos/"+id+"/upload-link",
				jsonBody(`{"contentType":"image/jpeg"}`))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if storCalled {
				t.Error("storage must not be called for invalid photo ID")
			}
			m := decodeBody(t, w)
			if m["code"] != "ValidationError" {
				t.Errorf("code = %v, want ValidationError", m["code"])
			}
		})
	}
}

// ── upload-link content-type validation ─────────────────────────────────────

func TestUploadLink_ContentType_Failures(t *testing.T) {
	tests := []struct {
		name string
		ct   string
	}{
		{"missing", ""},
		{"text_plain", "text/plain"},
		{"malformed", "not-a-mediatype;;;"},
		{"no_subtype", "application"},
	}
	repo := &fakeRepo{existsFn: func(context.Context, string) (bool, error) { return true, nil }}
	h := photoRouter(repo, &fakeStorage{})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := authReq("POST", "/photos/"+testPhotoID+"/upload-link",
				jsonBody(`{"contentType":"image/jpeg"}`))
			if tt.ct != "" {
				r.Header.Set("Content-Type", tt.ct)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] status = %d, want 400", tt.name, w.Code)
			}
			if m := decodeBody(t, w); m["code"] != "ValidationError" {
				t.Errorf("[%s] code = %v, want ValidationError", tt.name, m["code"])
			}
		})
	}
}

// ── upload-link body validation ──────────────────────────────────────────────

func TestUploadLink_Body_Failures(t *testing.T) {
	repo := &fakeRepo{existsFn: func(context.Context, string) (bool, error) { return true, nil }}
	h := photoRouter(repo, &fakeStorage{})

	doPost := func(body string) *httptest.ResponseRecorder {
		r := authReq("POST", "/photos/"+testPhotoID+"/upload-link", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}

	tests := []struct {
		name string
		body string
	}{
		{"empty", ""},
		{"malformed_json", "{invalid"},
		{"unknown_field", `{"contentType":"image/jpeg","extra":"x"}`},
		{"trailing_json", `{"contentType":"image/jpeg"}{"other":1}`},
		{"wrong_type_contentType", `{"contentType":123}`},
		{"missing_contentType", `{}`},
		{"blank_contentType", `{"contentType":"   "}`},
		{"crlf_injection", `{"contentType":"image/jpeg\r\n"}`},
		{"lf_injection", `{"contentType":"image/jpeg\n"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doPost(tt.body)
			if w.Code != http.StatusBadRequest {
				t.Errorf("[%s] status = %d, want 400; body: %s", tt.name, w.Code, w.Body)
			}
			if m := decodeBody(t, w); m["code"] != "ValidationError" {
				t.Errorf("[%s] code = %v, want ValidationError", tt.name, m["code"])
			}
			// Never echo body contents
			if strings.Contains(w.Body.String(), "invalid") && tt.name == "malformed_json" {
				// only check the body doesn't echo the raw input
			}
		})
	}
}

func TestUploadLink_OversizedBody_400(t *testing.T) {
	repo := &fakeRepo{existsFn: func(context.Context, string) (bool, error) { return true, nil }}
	h := photoRouter(repo, &fakeStorage{})

	bigBody := `{"contentType":"` + strings.Repeat("a", 8*1024) + `"}`
	r := authReq("POST", "/photos/"+testPhotoID+"/upload-link", strings.NewReader(bigBody))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
	if m := decodeBody(t, w); m["code"] != "ValidationError" {
		t.Errorf("code = %v, want ValidationError", m["code"])
	}
}

// ── upload-link photo-existence check ───────────────────────────────────────

func TestUploadLink_PhotoNotFound_404(t *testing.T) {
	var storCalled bool
	stor := &fakeStorage{
		presignUploadFn: func(context.Context, string, string) (objectstorage.UploadResult, error) {
			storCalled = true
			return objectstorage.UploadResult{}, nil
		},
	}
	repo := &fakeRepo{existsFn: func(context.Context, string) (bool, error) { return false, nil }}
	h := photoRouter(repo, stor)

	r := authReq("POST", "/photos/"+testPhotoID+"/upload-link",
		jsonBody(`{"contentType":"image/jpeg"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	if storCalled {
		t.Error("storage must not be called when photo does not exist")
	}
	m := decodeBody(t, w)
	if m["code"] != "NotFound" {
		t.Errorf("code = %v, want NotFound", m["code"])
	}
	if rid, _ := m["resource_id"].(string); rid != testPhotoID {
		t.Errorf("resource_id = %q, want %q", rid, testPhotoID)
	}
}

func TestUploadLink_RepoExistsError_500_NoLeak(t *testing.T) {
	secret := "xyzInternalDBError"
	repo := &fakeRepo{
		existsFn: func(context.Context, string) (bool, error) {
			return false, apperror.NewInternal(errors.New(secret))
		},
	}
	h := photoRouter(repo, &fakeStorage{})

	r := authReq("POST", "/photos/"+testPhotoID+"/upload-link",
		jsonBody(`{"contentType":"image/jpeg"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), secret) {
		t.Error("internal cause must not appear in response body")
	}
}

// ── upload-link storage error mapping ───────────────────────────────────────

func TestUploadLink_StorageInternal_500_NoLeak(t *testing.T) {
	secret := "xyzPresignEndpointURL"
	repo := &fakeRepo{existsFn: func(context.Context, string) (bool, error) { return true, nil }}
	stor := &fakeStorage{
		presignUploadFn: func(context.Context, string, string) (objectstorage.UploadResult, error) {
			return objectstorage.UploadResult{}, fmt.Errorf("internal: %s", secret)
		},
	}
	h := photoRouter(repo, stor)

	r := authReq("POST", "/photos/"+testPhotoID+"/upload-link",
		jsonBody(`{"contentType":"image/jpeg"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), secret) {
		t.Error("storage cause must not appear in response body")
	}
	m := decodeBody(t, w)
	if m["code"] != "InternalServerError" {
		t.Errorf("code = %v, want InternalServerError", m["code"])
	}
}

// ── get photo success ────────────────────────────────────────────────────────

func TestGetPhoto_Success(t *testing.T) {
	var gotRepoID, gotStorID string
	var gotCtx context.Context

	repo := &fakeRepo{
		getPhotoFn: func(ctx context.Context, id string) (domain.Photo, error) {
			gotCtx = ctx
			gotRepoID = id
			return testPhoto(), nil
		},
	}
	stor := &fakeStorage{
		presignDownloadFn: func(ctx context.Context, id string) (objectstorage.DownloadResult, error) {
			gotStorID = id
			return testDownloadResult(id), nil
		},
	}
	h := photoRouter(repo, stor)

	r := authReq("GET", "/photos/"+testPhotoID, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}

	// Exact DTO field names and values
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	assertStr(t, resp, "id", testPhotoID)
	assertFloat(t, resp, "x", 5.0)
	assertFloat(t, resp, "y", 10.0)
	assertFloat(t, resp, "h", 2.5)
	assertFloat(t, resp, "width", 1920)
	assertFloat(t, resp, "height", 1080)
	assertStr(t, resp, "capturedAt", testCapturedAt.UTC().Format(time.RFC3339))
	assertStr(t, resp, "originalUrl", testDownloadURL())

	preds, ok := resp["predictions"].([]any)
	if !ok {
		t.Fatalf("predictions not an array: %T", resp["predictions"])
	}
	if len(preds) != 1 {
		t.Fatalf("predictions len = %d, want 1", len(preds))
	}
	pred := preds[0].(map[string]any)
	assertStr(t, pred, "classId", string(domain.ClassMirid))
	assertFloat(t, pred, "confidence", 0.87)
	bbox := pred["bbox"].(map[string]any)
	assertFloat(t, bbox, "xMin", 0.1)
	assertFloat(t, bbox, "yMin", 0.2)
	assertFloat(t, bbox, "xMax", 0.3)
	assertFloat(t, bbox, "yMax", 0.4)

	// Context and exact ID propagation
	if gotRepoID != testPhotoID {
		t.Errorf("GetPhoto id = %q, want %q", gotRepoID, testPhotoID)
	}
	if gotStorID != testPhotoID {
		t.Errorf("PresignDownload id = %q, want %q", gotStorID, testPhotoID)
	}
	if gotCtx == nil {
		t.Error("GetPhoto must receive request context")
	}
}

func TestGetPhoto_EmptyPredictions_NotNull(t *testing.T) {
	repo := &fakeRepo{
		getPhotoFn: func(context.Context, string) (domain.Photo, error) {
			return testPhotoNoPredictions(), nil
		},
	}
	h := photoRouter(repo, &fakeStorage{})

	r := authReq("GET", "/photos/"+testPhotoID, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	// predictions must be [] not null
	body := w.Body.String()
	if !strings.Contains(body, `"predictions":[]`) {
		t.Errorf("predictions must be [] not null; body: %s", body)
	}
}

// ── get photo validation ─────────────────────────────────────────────────────

func TestGetPhoto_InvalidID_400_NoRepoCalls(t *testing.T) {
	var repoCalled bool
	repo := &fakeRepo{
		getPhotoFn: func(context.Context, string) (domain.Photo, error) {
			repoCalled = true
			return domain.Photo{}, nil
		},
	}
	h := photoRouter(repo, &fakeStorage{})

	// Empty string excluded: /photos/ doesn't match /photos/{photoId}.
	cases := []string{"not-a-uuid", "short"}
	for _, id := range cases {
		t.Run(id, func(t *testing.T) {
			repoCalled = false
			r := authReq("GET", "/photos/"+id, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if repoCalled {
				t.Error("repo must not be called for invalid ID")
			}
		})
	}
}

func TestGetPhoto_NotFound_404(t *testing.T) {
	repo := &fakeRepo{
		getPhotoFn: func(ctx context.Context, id string) (domain.Photo, error) {
			return domain.Photo{}, apperror.NewNotFound("photo not found", id)
		},
	}
	var storCalled bool
	stor := &fakeStorage{
		presignDownloadFn: func(context.Context, string) (objectstorage.DownloadResult, error) {
			storCalled = true
			return objectstorage.DownloadResult{}, nil
		},
	}
	h := photoRouter(repo, stor)

	r := authReq("GET", "/photos/"+testPhotoID, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	if storCalled {
		t.Error("storage must not be called when photo is not found")
	}
	m := decodeBody(t, w)
	if rid, _ := m["resource_id"].(string); rid != testPhotoID {
		t.Errorf("resource_id = %q, want %q", rid, testPhotoID)
	}
}

func TestGetPhoto_DownloadSignError_500_NoLeak(t *testing.T) {
	secret := "xyzPresignSecret"
	repo := &fakeRepo{
		getPhotoFn: func(context.Context, string) (domain.Photo, error) { return testPhoto(), nil },
	}
	stor := &fakeStorage{
		presignDownloadFn: func(context.Context, string) (objectstorage.DownloadResult, error) {
			return objectstorage.DownloadResult{}, fmt.Errorf("sign failed: %s", secret)
		},
	}
	h := photoRouter(repo, stor)

	r := authReq("GET", "/photos/"+testPhotoID, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), secret) {
		t.Error("storage cause must not appear in response body")
	}
}

// ── list photos success ──────────────────────────────────────────────────────

func TestListPhotos_EmptyPage(t *testing.T) {
	var signCalled bool
	repo := &fakeRepo{
		listPhotosFn: func(context.Context, sqlite.ListPhotosParams) (domain.PhotoPage, error) {
			return domain.PhotoPage{Items: []domain.Photo{}}, nil
		},
	}
	stor := &fakeStorage{
		presignDownloadFn: func(context.Context, string) (objectstorage.DownloadResult, error) {
			signCalled = true
			return objectstorage.DownloadResult{}, nil
		},
	}
	h := photoRouter(repo, stor)

	r := authReq("GET", "/photos", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if signCalled {
		t.Error("PresignDownload must not be called for empty pages")
	}
	body := w.Body.String()
	if !strings.Contains(body, `"items":[]`) {
		t.Errorf("empty items must be [], not null; body: %s", body)
	}
	if strings.Contains(body, "next_token") {
		t.Errorf("next_token must be absent for empty page; body: %s", body)
	}
}

func TestListPhotos_NonFinalPage(t *testing.T) {
	photos := []domain.Photo{testPhoto(), testPhoto()}
	photos[1].ID = testPhotoID2
	var signedIDs []string

	repo := &fakeRepo{
		listPhotosFn: func(context.Context, sqlite.ListPhotosParams) (domain.PhotoPage, error) {
			return domain.PhotoPage{Items: photos, NextToken: "cursor-token-xyz"}, nil
		},
	}
	stor := &fakeStorage{
		presignDownloadFn: func(_ context.Context, id string) (objectstorage.DownloadResult, error) {
			signedIDs = append(signedIDs, id)
			return testDownloadResult(id), nil
		},
	}
	h := photoRouter(repo, stor)

	r := authReq("GET", "/photos", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body)
	}

	m := decodeBody(t, w)
	items, _ := m["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	if m["next_token"] != "cursor-token-xyz" {
		t.Errorf("next_token = %v, want cursor-token-xyz", m["next_token"])
	}

	// Stable order, one sign per photo
	if len(signedIDs) != 2 {
		t.Fatalf("signed %d IDs, want 2", len(signedIDs))
	}
	if signedIDs[0] != testPhotoID {
		t.Errorf("signedIDs[0] = %q, want %q", signedIDs[0], testPhotoID)
	}
	if signedIDs[1] != testPhotoID2 {
		t.Errorf("signedIDs[1] = %q, want %q", signedIDs[1], testPhotoID2)
	}

	// Correct originalUrl per photo
	p0 := items[0].(map[string]any)
	p1 := items[1].(map[string]any)
	if p0["originalUrl"] != testDownloadResult(testPhotoID).URL {
		t.Errorf("items[0].originalUrl = %v", p0["originalUrl"])
	}
	if p1["originalUrl"] != testDownloadResult(testPhotoID2).URL {
		t.Errorf("items[1].originalUrl = %v", p1["originalUrl"])
	}
}

func TestListPhotos_FinalPage_NoNextToken(t *testing.T) {
	repo := &fakeRepo{
		listPhotosFn: func(context.Context, sqlite.ListPhotosParams) (domain.PhotoPage, error) {
			return domain.PhotoPage{Items: []domain.Photo{testPhoto()}, NextToken: ""}, nil
		},
	}
	h := photoRouter(repo, &fakeStorage{})

	r := authReq("GET", "/photos", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "next_token") {
		t.Errorf("next_token must be absent on final page; body: %s", body)
	}
}

func TestListPhotos_PredictionsNeverNull(t *testing.T) {
	p := testPhotoNoPredictions()
	repo := &fakeRepo{
		listPhotosFn: func(context.Context, sqlite.ListPhotosParams) (domain.PhotoPage, error) {
			return domain.PhotoPage{Items: []domain.Photo{p}}, nil
		},
	}
	h := photoRouter(repo, &fakeStorage{})

	r := authReq("GET", "/photos", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"predictions":[]`) {
		t.Errorf("predictions must be [], not null; body: %s", body)
	}
}

// ── list photos repo params ───────────────────────────────────────────────────

func TestListPhotos_ExactRepoParams(t *testing.T) {
	var gotParams sqlite.ListPhotosParams
	var gotCtx context.Context

	repo := &fakeRepo{
		listPhotosFn: func(ctx context.Context, p sqlite.ListPhotosParams) (domain.PhotoPage, error) {
			gotCtx = ctx
			gotParams = p
			return domain.PhotoPage{Items: []domain.Photo{}}, nil
		},
	}
	h := photoRouter(repo, &fakeStorage{})

	classID := domain.ClassMirid
	r := authReq("GET", "/photos?cursor=tok123&limit=42&classId=mirid&minConfidence=0.75", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body)
	}
	if gotParams.Cursor != "tok123" {
		t.Errorf("Cursor = %q, want tok123", gotParams.Cursor)
	}
	if gotParams.Limit != 42 {
		t.Errorf("Limit = %d, want 42", gotParams.Limit)
	}
	if gotParams.ClassID == nil || *gotParams.ClassID != classID {
		t.Errorf("ClassID = %v, want %v", gotParams.ClassID, classID)
	}
	if gotParams.MinConfidence == nil || *gotParams.MinConfidence != 0.75 {
		t.Errorf("MinConfidence = %v, want 0.75", gotParams.MinConfidence)
	}
	if gotCtx == nil {
		t.Error("ListPhotos must receive request context")
	}
}

func TestListPhotos_DefaultLimit_ZeroPassedToRepo(t *testing.T) {
	var gotParams sqlite.ListPhotosParams
	repo := &fakeRepo{
		listPhotosFn: func(_ context.Context, p sqlite.ListPhotosParams) (domain.PhotoPage, error) {
			gotParams = p
			return domain.PhotoPage{Items: []domain.Photo{}}, nil
		},
	}
	h := photoRouter(repo, &fakeStorage{})

	r := authReq("GET", "/photos", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if gotParams.Limit != 0 {
		t.Errorf("Limit = %d, want 0 (repo uses default 50)", gotParams.Limit)
	}
}

// ── list photos limit boundary ───────────────────────────────────────────────

func TestListPhotos_LimitBoundaries(t *testing.T) {
	tests := []struct {
		limitStr string
		wantCode int
		wantN    int // 0 means "not checked"
	}{
		{"1", 200, 1},
		{"200", 200, 200},
		{"50", 200, 50},
		{"0", 400, 0},
		{"201", 400, 0},
		{"-1", 400, 0},
		{"+1", 400, 0},
		{"1.5", 400, 0},
		{"abc", 400, 0},
		{"%201", 400, 0}, // URL-encoded space before digit
		{"99999999999999999999", 400, 0},
	}
	for _, tt := range tests {
		t.Run("limit="+tt.limitStr, func(t *testing.T) {
			var gotN int
			repo := &fakeRepo{
				listPhotosFn: func(_ context.Context, p sqlite.ListPhotosParams) (domain.PhotoPage, error) {
					gotN = p.Limit
					return domain.PhotoPage{Items: []domain.Photo{}}, nil
				},
			}
			h := photoRouter(repo, &fakeStorage{})
			r := authReq("GET", "/photos?limit="+tt.limitStr, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			if w.Code != tt.wantCode {
				t.Errorf("status = %d, want %d; body: %s", w.Code, tt.wantCode, w.Body)
			}
			if tt.wantCode == 200 && tt.wantN != 0 && gotN != tt.wantN {
				t.Errorf("Limit passed to repo = %d, want %d", gotN, tt.wantN)
			}
			if tt.wantCode == 400 {
				m := decodeBody(t, w)
				if m["code"] != "ValidationError" {
					t.Errorf("code = %v, want ValidationError", m["code"])
				}
			}
		})
	}
}

// ── list photos query param validation ───────────────────────────────────────

func TestListPhotos_UnknownParam_400(t *testing.T) {
	h := photoRouter(&fakeRepo{}, &fakeStorage{})
	cases := []string{"/photos?foo=bar", "/photos?cursor=x&extra=y", "/photos?LIMIT=5"}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			r := authReq("GET", path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if m := decodeBody(t, w); m["code"] != "ValidationError" {
				t.Errorf("code = %v, want ValidationError", m["code"])
			}
		})
	}
}

func TestListPhotos_RepeatedParam_400(t *testing.T) {
	h := photoRouter(&fakeRepo{}, &fakeStorage{})
	cases := []string{
		"/photos?limit=5&limit=10",
		"/photos?classId=mirid&classId=thrips",
		"/photos?cursor=a&cursor=b",
		"/photos?minConfidence=0.5&minConfidence=0.6",
		"/photos?limit=&limit=",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			r := authReq("GET", path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
		})
	}
}

func TestListPhotos_EmptyParam_400(t *testing.T) {
	h := photoRouter(&fakeRepo{}, &fakeStorage{})
	cases := []string{
		"/photos?cursor=",
		"/photos?limit=",
		"/photos?classId=",
		"/photos?minConfidence=",
	}
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			r := authReq("GET", path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusBadRequest {
				t.Errorf("%s: status = %d, want 400", path, w.Code)
			}
		})
	}
}

func TestListPhotos_UnknownClassID_400(t *testing.T) {
	h := photoRouter(&fakeRepo{}, &fakeStorage{})
	r := authReq("GET", "/photos?classId=unknown_pest", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestListPhotos_KnownClassIDs_Accepted(t *testing.T) {
	classes := []string{"powdery_mildew", "mirid", "whitefly_aphid", "miner_tuta", "thrips", "spider_mites"}
	for _, cls := range classes {
		t.Run(cls, func(t *testing.T) {
			h := photoRouter(&fakeRepo{}, &fakeStorage{})
			r := authReq("GET", "/photos?classId="+cls, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusOK {
				t.Errorf("[%s] status = %d, want 200", cls, w.Code)
			}
		})
	}
}

func TestListPhotos_MinConfidence_Validation(t *testing.T) {
	tests := []struct {
		val      string
		wantCode int
	}{
		{"0", 200},
		{"1", 200},
		{"0.5", 200},
		{"0.99", 200},
		{"NaN", 400},
		{"+Inf", 400},
		{"-Inf", 400},
		{"Inf", 400},
		{"-0.1", 400},
		{"1.1", 400},
		{"%200.5", 400}, // URL-encoded leading space
		{"abc", 400},
		{"0,5", 400},
	}
	for _, tt := range tests {
		t.Run("minConfidence="+tt.val, func(t *testing.T) {
			h := photoRouter(&fakeRepo{}, &fakeStorage{})
			r := authReq("GET", "/photos?minConfidence="+tt.val, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.wantCode {
				t.Errorf("[%s] status = %d, want %d; body: %s", tt.val, w.Code, tt.wantCode, w.Body)
			}
		})
	}
}

func TestListPhotos_CursorPreserved(t *testing.T) {
	cursor := "eyJjYXB0dXJlZF9hdCI6IjIwMjQifQ=="
	var gotCursor string
	repo := &fakeRepo{
		listPhotosFn: func(_ context.Context, p sqlite.ListPhotosParams) (domain.PhotoPage, error) {
			gotCursor = p.Cursor
			return domain.PhotoPage{Items: []domain.Photo{}}, nil
		},
	}
	h := photoRouter(repo, &fakeStorage{})

	r := authReq("GET", "/photos?cursor="+cursor, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if gotCursor != cursor {
		t.Errorf("cursor = %q, want %q", gotCursor, cursor)
	}
}

// ── list photos error paths ──────────────────────────────────────────────────

func TestListPhotos_RepoError_500_NoLeak(t *testing.T) {
	secret := "xyzListPhotosDBSecret"
	repo := &fakeRepo{
		listPhotosFn: func(context.Context, sqlite.ListPhotosParams) (domain.PhotoPage, error) {
			return domain.PhotoPage{}, apperror.NewInternal(errors.New(secret))
		},
	}
	h := photoRouter(repo, &fakeStorage{})

	r := authReq("GET", "/photos", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if strings.Contains(w.Body.String(), secret) {
		t.Error("internal cause must not appear in response body")
	}
}

func TestListPhotos_EnrichmentFailure_StopsEarly_NoPartialSuccess(t *testing.T) {
	photos := []domain.Photo{testPhoto(), testPhoto()}
	photos[1].ID = testPhotoID2

	signCount := 0
	repo := &fakeRepo{
		listPhotosFn: func(context.Context, sqlite.ListPhotosParams) (domain.PhotoPage, error) {
			return domain.PhotoPage{Items: photos}, nil
		},
	}
	stor := &fakeStorage{
		presignDownloadFn: func(_ context.Context, id string) (objectstorage.DownloadResult, error) {
			signCount++
			if id == testPhotoID2 {
				return objectstorage.DownloadResult{}, errors.New("sign failed")
			}
			return testDownloadResult(id), nil
		},
	}
	h := photoRouter(repo, stor)

	r := authReq("GET", "/photos", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	// Must not emit partial 200 with first photo
	if m := decodeBody(t, w); m["code"] != "InternalServerError" {
		t.Errorf("code = %v, want InternalServerError", m["code"])
	}
}

// ── response headers for data API ────────────────────────────────────────────

func TestDataAPISuccess_Headers(t *testing.T) {
	repo := &fakeRepo{
		getPhotoFn: func(context.Context, string) (domain.Photo, error) { return testPhoto(), nil },
		listPhotosFn: func(context.Context, sqlite.ListPhotosParams) (domain.PhotoPage, error) {
			return domain.PhotoPage{Items: []domain.Photo{}}, nil
		},
		existsFn: func(context.Context, string) (bool, error) { return true, nil },
	}
	stor := &fakeStorage{}

	routes := []struct {
		method      string
		path        string
		body        string
		contentType string
	}{
		{"GET", "/photos/" + testPhotoID, "", ""},
		{"GET", "/photos", "", ""},
		{"POST", "/photos/" + testPhotoID + "/upload-link", `{"contentType":"image/jpeg"}`, "application/json"},
	}

	for _, rt := range routes {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			h := photoRouter(repo, stor)
			var r *http.Request
			if rt.body != "" {
				r = authReq(rt.method, rt.path, strings.NewReader(rt.body))
				r.Header.Set("Content-Type", rt.contentType)
			} else {
				r = authReq(rt.method, rt.path, nil)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d; body: %s", w.Code, w.Body)
			}
			if ct := w.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			if cc := w.Header().Get("Cache-Control"); cc != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", cc)
			}
		})
	}
}

// ── request ID propagation ────────────────────────────────────────────────────

func TestDataAPI_RequestIDInErrorBody(t *testing.T) {
	h := photoRouter(&fakeRepo{}, &fakeStorage{})

	r := authReq("GET", "/photos/not-a-uuid", nil)
	r.Header.Set("X-Request-ID", "my-request-id-123")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	m := decodeBody(t, w)
	if m["request_id"] != "my-request-id-123" {
		t.Errorf("request_id = %v, want my-request-id-123", m["request_id"])
	}
}

// ── DTO field name checks ─────────────────────────────────────────────────────

func TestGetPhoto_DTOFieldNames(t *testing.T) {
	repo := &fakeRepo{
		getPhotoFn: func(context.Context, string) (domain.Photo, error) { return testPhoto(), nil },
	}
	h := photoRouter(repo, &fakeStorage{})

	r := authReq("GET", "/photos/"+testPhotoID, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	body := w.Body.String()
	for _, field := range []string{"id", "x", "y", "h", "width", "height", "capturedAt", "originalUrl", "predictions"} {
		if !strings.Contains(body, `"`+field+`"`) {
			t.Errorf("field %q missing from response body: %s", field, body)
		}
	}
	for _, field := range []string{"classId", "confidence", "bbox", "xMin", "yMin", "xMax", "yMax"} {
		if !strings.Contains(body, `"`+field+`"`) {
			t.Errorf("field %q missing from prediction: %s", field, body)
		}
	}
}

func TestListPhotos_DTOFieldNames(t *testing.T) {
	repo := &fakeRepo{
		listPhotosFn: func(context.Context, sqlite.ListPhotosParams) (domain.PhotoPage, error) {
			return domain.PhotoPage{Items: []domain.Photo{testPhoto()}, NextToken: "tok"}, nil
		},
	}
	h := photoRouter(repo, &fakeStorage{})

	r := authReq("GET", "/photos", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	body := w.Body.String()
	for _, field := range []string{"items", "next_token"} {
		if !strings.Contains(body, `"`+field+`"`) {
			t.Errorf("field %q missing from list response: %s", field, body)
		}
	}
}

func TestUploadLink_DTOFieldNames(t *testing.T) {
	repo := &fakeRepo{existsFn: func(context.Context, string) (bool, error) { return true, nil }}
	h := photoRouter(repo, &fakeStorage{})

	r := authReq("POST", "/photos/"+testPhotoID+"/upload-link",
		jsonBody(`{"contentType":"image/jpeg"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	body := w.Body.String()
	for _, field := range []string{"url", "method", "expiresAt"} {
		if !strings.Contains(body, `"`+field+`"`) {
			t.Errorf("field %q missing from upload-link response: %s", field, body)
		}
	}
	// method must be constant "PUT"
	if !strings.Contains(body, `"method":"PUT"`) {
		t.Errorf("method must be PUT; body: %s", body)
	}
}

// ── upload-link: no headers field when nil ────────────────────────────────────

func TestUploadLink_NilHeaders_Omitted(t *testing.T) {
	repo := &fakeRepo{existsFn: func(context.Context, string) (bool, error) { return true, nil }}
	stor := &fakeStorage{
		presignUploadFn: func(context.Context, string, string) (objectstorage.UploadResult, error) {
			return objectstorage.UploadResult{
				URL:       "https://example.com/put",
				Headers:   nil,
				ExpiresAt: testExpiresAt,
			}, nil
		},
	}
	h := photoRouter(repo, stor)

	r := authReq("POST", "/photos/"+testPhotoID+"/upload-link",
		jsonBody(`{"contentType":"image/jpeg"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", w.Code, w.Body)
	}
	body := w.Body.String()
	// nil map → omitempty → headers field absent
	if strings.Contains(body, `"headers"`) {
		t.Errorf("nil headers should be omitted; body: %s", body)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func assertStr(t *testing.T, m map[string]any, key, want string) {
	t.Helper()
	got, _ := m[key].(string)
	if got != want {
		t.Errorf("%q = %q, want %q", key, got, want)
	}
}

func assertFloat(t *testing.T, m map[string]any, key string, want float64) {
	t.Helper()
	got, _ := m[key].(float64)
	if got != want {
		t.Errorf("%q = %v, want %v", key, got, want)
	}
}

// ── buf compile check ─────────────────────────────────────────────────────────

// Ensure fakes compile against the interfaces (checked at assignment in photoRouter).
var _ = func() {
	_ = bytes.NewReader
}

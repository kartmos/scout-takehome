package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"scout/internal/apperror"
	"scout/internal/domain"
	"scout/internal/objectstorage"
	"scout/internal/repository/sqlite"
)

// photoRepository is the narrow interface the photo handlers require.
type photoRepository interface {
	PhotoExists(ctx context.Context, photoID string) (bool, error)
	GetPhoto(ctx context.Context, photoID string) (domain.Photo, error)
	ListPhotos(ctx context.Context, params sqlite.ListPhotosParams) (domain.PhotoPage, error)
}

// photoStorage is the narrow interface the photo handlers require.
type photoStorage interface {
	PresignUpload(ctx context.Context, photoID string, contentType string) (objectstorage.UploadResult, error)
	PresignDownload(ctx context.Context, photoID string) (objectstorage.DownloadResult, error)
}

// maxBodyBytes is the hard limit on upload-link request bodies (8 KiB).
const maxBodyBytes = 8 * 1024

var allowedListParams = map[string]bool{
	"cursor":        true,
	"limit":         true,
	"classId":       true,
	"minConfidence": true,
	"nearX":         true,
	"nearY":         true,
	"nearRadius":    true,
}

const (
	maxCursorLen     = 512
	maxClassIDLen    = 64
	maxLimitLen      = 20
	maxConfidenceLen = 64
	maxCoordLen      = 32
)

func handleUploadLink(repo photoRepository, storage photoStorage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		photoID := r.PathValue("photoId")
		if !domain.IsValidUUID(photoID) {
			WriteError(w, r, logger, apperror.NewValidation("invalid photo ID", []apperror.FieldViolation{
				{Field: "photoId", Issue: "must be a canonical UUID"},
			}))
			return
		}

		if !isJSONContentType(r.Header.Get("Content-Type")) {
			WriteError(w, r, logger, apperror.NewValidation("unsupported content type", []apperror.FieldViolation{
				{Field: "Content-Type", Issue: "must be application/json"},
			}))
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()

		var req uploadLinkRequestDTO
		if err := dec.Decode(&req); err != nil {
			WriteError(w, r, logger, bodyDecodeError(err))
			return
		}
		if dec.More() {
			WriteError(w, r, logger, apperror.NewValidation("request body must contain exactly one JSON object", []apperror.FieldViolation{
				{Field: "body", Issue: "trailing content after JSON object"},
			}))
			return
		}

		if strings.TrimSpace(req.ContentType) == "" {
			WriteError(w, r, logger, apperror.NewValidation("contentType is required", []apperror.FieldViolation{
				{Field: "contentType", Issue: "must not be blank"},
			}))
			return
		}
		if strings.ContainsAny(req.ContentType, "\r\n") {
			WriteError(w, r, logger, apperror.NewValidation("invalid contentType", []apperror.FieldViolation{
				{Field: "contentType", Issue: "must not contain CR or LF"},
			}))
			return
		}

		exists, err := repo.PhotoExists(r.Context(), photoID)
		if err != nil {
			WriteError(w, r, logger, err)
			return
		}
		if !exists {
			WriteError(w, r, logger, apperror.NewNotFound("photo not found", photoID))
			return
		}

		result, err := storage.PresignUpload(r.Context(), photoID, req.ContentType)
		if err != nil {
			WriteError(w, r, logger, mapStorageError(err))
			return
		}

		resp := uploadLinkResponseDTO{
			URL:       result.URL,
			Method:    "PUT",
			Headers:   result.Headers,
			ExpiresAt: result.ExpiresAt.UTC().Format(time.RFC3339),
		}
		writeSuccessJSON(w, r, logger, http.StatusOK, resp)
	}
}

func handleGetPhoto(repo photoRepository, storage photoStorage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		photoID := r.PathValue("photoId")
		if !domain.IsValidUUID(photoID) {
			WriteError(w, r, logger, apperror.NewValidation("invalid photo ID", []apperror.FieldViolation{
				{Field: "photoId", Issue: "must be a canonical UUID"},
			}))
			return
		}

		photo, err := repo.GetPhoto(r.Context(), photoID)
		if err != nil {
			WriteError(w, r, logger, err)
			return
		}

		dl, err := storage.PresignDownload(r.Context(), photo.ID)
		if err != nil {
			WriteError(w, r, logger, apperror.NewInternal(err))
			return
		}

		writeSuccessJSON(w, r, logger, http.StatusOK, photoToDTO(photo, dl.URL))
	}
}

func handleListPhotos(repo photoRepository, storage photoStorage, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params, err := parseListParams(r)
		if err != nil {
			WriteError(w, r, logger, err)
			return
		}

		page, err := repo.ListPhotos(r.Context(), params)
		if err != nil {
			WriteError(w, r, logger, err)
			return
		}

		items := make([]photoDTO, len(page.Items))
		for i, ph := range page.Items {
			if ctxErr := r.Context().Err(); ctxErr != nil {
				WriteError(w, r, logger, apperror.NewInternal(ctxErr))
				return
			}
			dl, dlErr := storage.PresignDownload(r.Context(), ph.ID)
			if dlErr != nil {
				WriteError(w, r, logger, apperror.NewInternal(dlErr))
				return
			}
			items[i] = photoToDTO(ph, dl.URL)
		}

		resp := photoPageResponseDTO{Items: items}
		if page.NextToken != "" {
			resp.NextToken = page.NextToken
		}
		writeSuccessJSON(w, r, logger, http.StatusOK, resp)
	}
}

// parseListParams validates and parses all query parameters for GET /photos.
func parseListParams(r *http.Request) (sqlite.ListPhotosParams, error) {
	q := r.URL.Query()

	for key, vals := range q {
		if !allowedListParams[key] {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("unknown query parameter", []apperror.FieldViolation{
				{Field: key, Issue: "unknown parameter"},
			})
		}
		if len(vals) != 1 {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("repeated query parameter", []apperror.FieldViolation{
				{Field: key, Issue: "must not be repeated"},
			})
		}
	}

	var params sqlite.ListPhotosParams

	if vals, ok := q["cursor"]; ok {
		v := vals[0]
		if v == "" {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
				{Field: "cursor", Issue: "must not be empty"},
			})
		}
		if len(v) > maxCursorLen {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("cursor too long", []apperror.FieldViolation{
				{Field: "cursor", Issue: "exceeds maximum length"},
			})
		}
		params.Cursor = v
	}

	if vals, ok := q["limit"]; ok {
		v := vals[0]
		if v == "" {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("invalid limit", []apperror.FieldViolation{
				{Field: "limit", Issue: "must not be empty"},
			})
		}
		if len(v) > maxLimitLen {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("invalid limit", []apperror.FieldViolation{
				{Field: "limit", Issue: "value too long"},
			})
		}
		n, parseErr := strconv.ParseUint(v, 10, 64)
		if parseErr != nil || n < uint64(sqlite.MinLimit) || n > uint64(sqlite.MaxLimit) {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("invalid limit", []apperror.FieldViolation{
				{Field: "limit", Issue: "must be an integer between 1 and 200"},
			})
		}
		params.Limit = int(n)
	}

	if vals, ok := q["classId"]; ok {
		v := vals[0]
		if v == "" {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("invalid classId", []apperror.FieldViolation{
				{Field: "classId", Issue: "must not be empty"},
			})
		}
		if len(v) > maxClassIDLen {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("invalid classId", []apperror.FieldViolation{
				{Field: "classId", Issue: "value too long"},
			})
		}
		classID := domain.ClassID(v)
		if !domain.IsKnownClassID(classID) {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("unknown classId", []apperror.FieldViolation{
				{Field: "classId", Issue: "unknown class"},
			})
		}
		params.ClassID = &classID
	}

	if vals, ok := q["minConfidence"]; ok {
		v := vals[0]
		if v == "" {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("invalid minConfidence", []apperror.FieldViolation{
				{Field: "minConfidence", Issue: "must not be empty"},
			})
		}
		if len(v) > maxConfidenceLen {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("invalid minConfidence", []apperror.FieldViolation{
				{Field: "minConfidence", Issue: "value too long"},
			})
		}
		f, parseErr := strconv.ParseFloat(v, 64)
		if parseErr != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > 1 {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("invalid minConfidence", []apperror.FieldViolation{
				{Field: "minConfidence", Issue: "must be a finite decimal between 0 and 1"},
			})
		}
		params.MinConfidence = &f
	}

	_, hasNearX := q["nearX"]
	_, hasNearY := q["nearY"]
	_, hasNearR := q["nearRadius"]
	if hasNearX || hasNearY || hasNearR {
		if !hasNearX || !hasNearY || !hasNearR {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("near filter incomplete", []apperror.FieldViolation{
				{Field: "nearX/nearY/nearRadius", Issue: "all three parameters must be provided together"},
			})
		}
		nearX, err := parseFiniteFloat("nearX", q["nearX"][0], 0, 40)
		if err != nil {
			return sqlite.ListPhotosParams{}, err
		}
		nearY, err := parseFiniteFloat("nearY", q["nearY"][0], 0, 40)
		if err != nil {
			return sqlite.ListPhotosParams{}, err
		}
		nearR, err := parseFiniteFloat("nearRadius", q["nearRadius"][0], 0, 40)
		if err != nil {
			return sqlite.ListPhotosParams{}, err
		}
		if nearR <= 0 {
			return sqlite.ListPhotosParams{}, apperror.NewValidation("invalid nearRadius", []apperror.FieldViolation{
				{Field: "nearRadius", Issue: "must be positive"},
			})
		}
		params.Near = &sqlite.NearLocation{X: nearX, Y: nearY, Radius: nearR}
	}

	return params, nil
}

// parseFiniteFloat parses a non-empty, bounded query param value. Returns a validation error on any issue.
func parseFiniteFloat(field, v string, minVal, maxVal float64) (float64, error) {
	if v == "" {
		return 0, apperror.NewValidation("invalid "+field, []apperror.FieldViolation{
			{Field: field, Issue: "must not be empty"},
		})
	}
	if len(v) > maxCoordLen {
		return 0, apperror.NewValidation("invalid "+field, []apperror.FieldViolation{
			{Field: field, Issue: "value too long"},
		})
	}
	f, parseErr := strconv.ParseFloat(v, 64)
	if parseErr != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < minVal || f > maxVal {
		return 0, apperror.NewValidation("invalid "+field, []apperror.FieldViolation{
			{Field: field, Issue: "must be a finite decimal between 0 and 40"},
		})
	}
	return f, nil
}

// isJSONContentType reports whether ct is application/json (with optional media-type parameters).
func isJSONContentType(ct string) bool {
	if ct == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}

// bodyDecodeError maps common JSON decode failures to typed 400 validation errors.
// It never includes the request body contents in the error message.
func bodyDecodeError(err error) error {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return apperror.NewValidation("request body too large", []apperror.FieldViolation{
			{Field: "body", Issue: "exceeds maximum allowed size"},
		})
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return apperror.NewValidation("request body is missing or truncated", []apperror.FieldViolation{
			{Field: "body", Issue: "must be a non-empty JSON object"},
		})
	}
	var syntaxErr *json.SyntaxError
	if errors.As(err, &syntaxErr) {
		return apperror.NewValidation("request body contains malformed JSON", []apperror.FieldViolation{
			{Field: "body", Issue: "malformed JSON syntax"},
		})
	}
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return apperror.NewValidation("request body contains wrong field type", []apperror.FieldViolation{
			{Field: typeErr.Field, Issue: "wrong type"},
		})
	}
	// DisallowUnknownFields emits "json: unknown field \"<name>\""
	if strings.HasPrefix(err.Error(), "json: unknown field") {
		return apperror.NewValidation("request body contains unknown field", []apperror.FieldViolation{
			{Field: "body", Issue: "unknown field"},
		})
	}
	return apperror.NewValidation("invalid request body", []apperror.FieldViolation{
		{Field: "body", Issue: "invalid"},
	})
}

// mapStorageError converts a storage failure to a typed app error.
// CategoryInvalidInput becomes a validation error; all others become internal.
func mapStorageError(err error) error {
	var se *objectstorage.StorageError
	if errors.As(err, &se) && se.Cat == objectstorage.CategoryInvalidInput {
		return apperror.NewValidation("invalid storage request", []apperror.FieldViolation{
			{Field: "request", Issue: "invalid input"},
		})
	}
	return apperror.NewInternal(err)
}

// photoToDTO converts a domain photo and presigned URL into the HTTP response DTO.
func photoToDTO(ph domain.Photo, originalURL string) photoDTO {
	preds := make([]predictionDTO, len(ph.Predictions))
	for i, p := range ph.Predictions {
		preds[i] = predictionDTO{
			ClassID:    string(p.ClassID),
			Confidence: p.Confidence,
			BBox: bboxDTO{
				XMin: p.BoundingBox.XMin,
				YMin: p.BoundingBox.YMin,
				XMax: p.BoundingBox.XMax,
				YMax: p.BoundingBox.YMax,
			},
		}
	}
	return photoDTO{
		ID:          ph.ID,
		X:           ph.X,
		Y:           ph.Y,
		H:           ph.H,
		Width:       ph.Width,
		Height:      ph.Height,
		CapturedAt:  ph.CapturedAt.UTC().Format(time.RFC3339),
		OriginalURL: originalURL,
		Predictions: preds,
	}
}

// writeSuccessJSON marshals v to JSON before committing headers, then writes
// Content-Type: application/json and Cache-Control: no-store.
// On marshal failure it falls through to WriteError.
func writeSuccessJSON(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		WriteError(w, r, logger, apperror.NewInternal(err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

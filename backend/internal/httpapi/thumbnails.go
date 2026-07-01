package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"scout/internal/apperror"
	"scout/internal/domain"
	"scout/internal/thumbnail"
)

// thumbnailPhotoRepository is the narrow read interface the thumbnail handler uses.
type thumbnailPhotoRepository interface {
	GetPhoto(ctx context.Context, photoID string) (domain.Photo, error)
}

// thumbnailService is the interface the thumbnail handler depends on for generation.
type thumbnailService interface {
	Get(ctx context.Context, photo domain.Photo, req thumbnail.Request) (*thumbnail.ThumbnailResult, error)
}

var allowedThumbnailParams = map[string]bool{
	"width":   true,
	"dpr":     true,
	"quality": true,
}

// handleGetThumbnail serves GET /photos/{photoId}/thumbnail without authentication.
// It validates the request, checks the cache-friendly ETag, and streams the JPEG.
func handleGetThumbnail(repo thumbnailPhotoRepository, svc thumbnailService, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		photoID := r.PathValue("photoId")
		if !domain.IsValidUUID(photoID) {
			WriteError(w, r, logger, apperror.NewValidation("invalid photo ID", []apperror.FieldViolation{
				{Field: "photoId", Issue: "must be a canonical UUID"},
			}))
			return
		}

		q := r.URL.Query()
		if err := validateThumbnailQuery(q); err != nil {
			WriteError(w, r, logger, err)
			return
		}

		req, err := thumbnail.ParseRequest(q.Get("width"), q.Get("dpr"), q.Get("quality"))
		if err != nil {
			WriteError(w, r, logger, mapThumbnailParseError(err))
			return
		}

		photo, err := repo.GetPhoto(r.Context(), photoID)
		if err != nil {
			WriteError(w, r, logger, mapRepoError(err))
			return
		}

		// Derive the stable ETag from the canonical thumbnail identity.
		// This requires resolved dimensions, which depend on source photo size.
		dims := thumbnail.ResolveDims(photo.Width, photo.Height, req.RequestedPixels)
		key := thumbnail.NewKey(photo, req, dims)
		etag := `"` + key.Hash() + `"`

		// Early conditional response avoids cache/generation work for unchanged thumbnails.
		if r.Header.Get("If-None-Match") == etag {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			w.Header().Set("ETag", etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}

		result, err := svc.Get(r.Context(), photo, req)
		if err != nil {
			WriteError(w, r, logger, mapThumbnailGenError(err))
			return
		}
		if result == nil || result.File == nil {
			WriteError(w, r, logger, apperror.NewInternal(errors.New("thumbnail service returned invalid nil result")))
			return
		}
		defer result.File.Close()

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		w.Header().Set("ETag", etag)
		// ServeContent handles Content-Length (via seek), range requests, and
		// conditional GET via the ETag we've already set in the response headers.
		http.ServeContent(w, r, "", time.Time{}, result.File)
	}
}

// validateThumbnailQuery rejects unknown, repeated, or empty query parameters.
func validateThumbnailQuery(q url.Values) error {
	var violations []apperror.FieldViolation
	for key, vals := range q {
		if !allowedThumbnailParams[key] {
			violations = append(violations, apperror.FieldViolation{Field: key, Issue: "unknown query parameter"})
			continue
		}
		if len(vals) > 1 {
			violations = append(violations, apperror.FieldViolation{Field: key, Issue: "must not be repeated"})
		} else if len(vals) == 1 && vals[0] == "" {
			violations = append(violations, apperror.FieldViolation{Field: key, Issue: "must not be empty"})
		}
	}
	if len(violations) > 0 {
		return apperror.NewValidation("invalid query parameters", violations)
	}
	return nil
}

func mapThumbnailParseError(err error) error {
	var inv *thumbnail.ErrInvalidRequest
	if errors.As(err, &inv) {
		return apperror.NewValidation("invalid thumbnail parameters", []apperror.FieldViolation{
			{Field: inv.Field, Issue: inv.Msg},
		})
	}
	return apperror.NewInternal(err)
}

func mapThumbnailGenError(err error) error {
	var notFound *thumbnail.ErrNotFound
	if errors.As(err, &notFound) {
		return apperror.NewNotFound("original photo not found in storage", "original")
	}
	return apperror.NewInternal(err)
}

func mapRepoError(err error) error {
	var appErr *apperror.AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return apperror.NewInternal(err)
}

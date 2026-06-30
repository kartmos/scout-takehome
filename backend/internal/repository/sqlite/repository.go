package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	// Pure-Go SQLite driver; registers the "sqlite" driver name.
	_ "modernc.org/sqlite"

	"scout/internal/apperror"
	"scout/internal/domain"
)

// maxConnections is the connection pool ceiling for one small box (~1 vCPU).
// SQLite allows concurrent reads; keeping the pool small avoids goroutine starvation.
const maxConnections = 4

// DefaultLimit is the page size when the caller supplies zero.
const DefaultLimit = 50

// MinLimit and MaxLimit enforce the OpenAPI contract.
const (
	MinLimit = 1
	MaxLimit = 200
)

// ListPhotosParams carries all optional inputs for ListPhotos.
type ListPhotosParams struct {
	Cursor        string
	Limit         int
	ClassID       *domain.ClassID
	MinConfidence *float64
}

// Repository is a read-only SQLite-backed photo repository.
type Repository struct {
	db *sql.DB
}

// Open resolves the path, opens SQLite in read-only mode, and returns a ready Repository.
func Open(path string) (*Repository, error) {
	if path == "" {
		return nil, fmt.Errorf("sqlite: database path must not be empty")
	}

	// Open read-only via URI query parameter; modernc.org/sqlite honours the standard flags.
	dsn := fmt.Sprintf("file:%s?mode=ro&_foreign_keys=1", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open %q: %w", path, err)
	}

	db.SetMaxOpenConns(maxConnections)
	db.SetMaxIdleConns(maxConnections)

	if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: ping %q: %w", path, err)
	}

	return &Repository{db: db}, nil
}

// Close releases the underlying connection pool.
func (r *Repository) Close() error {
	return r.db.Close()
}

// PhotoExists reports whether a photo with the given ID exists.
func (r *Repository) PhotoExists(ctx context.Context, photoID string) (bool, error) {
	if !domain.IsValidUUID(photoID) {
		return false, apperror.NewValidation("invalid photo ID", []apperror.FieldViolation{
			{Field: "photoId", Issue: "must be a canonical UUID"},
		})
	}

	var exists bool
	const q = `SELECT EXISTS(SELECT 1 FROM photos WHERE id = ?)`
	if err := r.db.QueryRowContext(ctx, q, photoID).Scan(&exists); err != nil {
		return false, apperror.NewInternal(fmt.Errorf("sqlite: PhotoExists: %w", err))
	}
	return exists, nil
}

// GetPhoto returns the photo with all its predictions, or a not-found error.
func (r *Repository) GetPhoto(ctx context.Context, photoID string) (domain.Photo, error) {
	if !domain.IsValidUUID(photoID) {
		return domain.Photo{}, apperror.NewValidation("invalid photo ID", []apperror.FieldViolation{
			{Field: "photoId", Issue: "must be a canonical UUID"},
		})
	}

	const q = `SELECT id, x, y, h, width, height, captured_at FROM photos WHERE id = ?`
	row := r.db.QueryRowContext(ctx, q, photoID)
	photo, err := scanPhoto(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Photo{}, apperror.NewNotFound("photo not found", photoID)
		}
		return domain.Photo{}, err // already wrapped by scanPhoto
	}

	preds, err := r.loadPredictions(ctx, []string{photoID})
	if err != nil {
		return domain.Photo{}, err
	}
	photo.Predictions = preds[photoID]
	if photo.Predictions == nil {
		photo.Predictions = []domain.Prediction{}
	}
	return photo, nil
}

// ListPhotos returns a cursor-paginated page of photos matching the supplied params.
func (r *Repository) ListPhotos(ctx context.Context, params ListPhotosParams) (domain.PhotoPage, error) {
	limit, err := resolveLimit(params.Limit)
	if err != nil {
		return domain.PhotoPage{}, err
	}
	if params.ClassID != nil && !domain.IsKnownClassID(*params.ClassID) {
		return domain.PhotoPage{}, apperror.NewValidation("invalid classId", []apperror.FieldViolation{
			{Field: "classId", Issue: fmt.Sprintf("unknown class %q", string(*params.ClassID))},
		})
	}
	if params.MinConfidence != nil {
		if fe := domain.ValidateConfidence(*params.MinConfidence); fe != nil {
			return domain.PhotoPage{}, apperror.NewValidation("invalid minConfidence", []apperror.FieldViolation{
				{Field: "minConfidence", Issue: fe.Issue},
			})
		}
	}

	var (
		hasCursor bool
		cursorAt  time.Time
		cursorID  string
	)
	if params.Cursor != "" {
		hasCursor = true
		cursorAt, cursorID, err = decodeCursor(params.Cursor)
		if err != nil {
			return domain.PhotoPage{}, err
		}
	}

	photos, hasMore, err := r.queryPhotos(ctx, hasCursor, cursorAt, cursorID, params.ClassID, params.MinConfidence, limit)
	if err != nil {
		return domain.PhotoPage{}, err
	}

	if len(photos) == 0 {
		return domain.PhotoPage{Items: []domain.Photo{}}, nil
	}

	// Load all predictions for this page in one query.
	ids := make([]string, len(photos))
	for i, p := range photos {
		ids[i] = p.ID
	}
	predMap, err := r.loadPredictions(ctx, ids)
	if err != nil {
		return domain.PhotoPage{}, err
	}
	for i := range photos {
		photos[i].Predictions = predMap[photos[i].ID]
		if photos[i].Predictions == nil {
			photos[i].Predictions = []domain.Prediction{}
		}
	}

	page := domain.PhotoPage{Items: photos}
	if hasMore {
		last := photos[len(photos)-1]
		page.NextToken = encodeCursor(last.CapturedAt, last.ID)
	}
	return page, nil
}

// queryPhotos fetches up to limit photo rows (plus one sentinel for hasMore detection).
func (r *Repository) queryPhotos(
	ctx context.Context,
	hasCursor bool,
	cursorAt time.Time,
	cursorID string,
	classID *domain.ClassID,
	minConf *float64,
	limit int,
) ([]domain.Photo, bool, error) {
	var args []any
	var clauses []string

	if hasCursor {
		clauses = append(clauses, "(p.captured_at < ? OR (p.captured_at = ? AND p.id < ?))")
		args = append(args, cursorAt.UTC().Format(time.RFC3339Nano), cursorAt.UTC().Format(time.RFC3339Nano), cursorID)
	}

	if classID != nil || minConf != nil {
		var sub strings.Builder
		sub.WriteString("EXISTS (SELECT 1 FROM predictions pr WHERE pr.photo_id = p.id")
		if classID != nil {
			sub.WriteString(" AND pr.class_id = ?")
			args = append(args, string(*classID))
		}
		if minConf != nil {
			sub.WriteString(" AND pr.confidence >= ?")
			args = append(args, *minConf)
		}
		sub.WriteString(")")
		clauses = append(clauses, sub.String())
	}

	var q strings.Builder
	q.WriteString("SELECT p.id, p.x, p.y, p.h, p.width, p.height, p.captured_at FROM photos p")
	if len(clauses) > 0 {
		q.WriteString(" WHERE ")
		q.WriteString(strings.Join(clauses, " AND "))
	}
	q.WriteString(" ORDER BY p.captured_at DESC, p.id DESC LIMIT ?")
	args = append(args, limit+1)

	rows, err := r.db.QueryContext(ctx, q.String(), args...)
	if err != nil {
		return nil, false, apperror.NewInternal(fmt.Errorf("sqlite: list photos query: %w", err))
	}
	defer rows.Close()

	var photos []domain.Photo
	for rows.Next() {
		ph, err := scanPhotoRow(rows)
		if err != nil {
			return nil, false, err
		}
		photos = append(photos, ph)
	}
	if err = rows.Err(); err != nil {
		return nil, false, apperror.NewInternal(fmt.Errorf("sqlite: list photos rows: %w", err))
	}
	if err = rows.Close(); err != nil {
		return nil, false, apperror.NewInternal(fmt.Errorf("sqlite: list photos close: %w", err))
	}

	hasMore := len(photos) > limit
	if hasMore {
		photos = photos[:limit]
	}
	return photos, hasMore, nil
}

// loadPredictions fetches all predictions for the given photo IDs in one query.
// Returns a map from photo ID to prediction slice.
func (r *Repository) loadPredictions(ctx context.Context, photoIDs []string) (map[string][]domain.Prediction, error) {
	if len(photoIDs) == 0 {
		return map[string][]domain.Prediction{}, nil
	}

	placeholders := strings.Repeat("?,", len(photoIDs))
	placeholders = placeholders[:len(placeholders)-1]
	q := fmt.Sprintf(
		"SELECT photo_id, class_id, confidence, bbox_xmin, bbox_ymin, bbox_xmax, bbox_ymax FROM predictions WHERE photo_id IN (%s) ORDER BY photo_id, rowid",
		placeholders,
	)
	args := make([]any, len(photoIDs))
	for i, id := range photoIDs {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("sqlite: load predictions: %w", err))
	}
	defer rows.Close()

	out := make(map[string][]domain.Prediction)
	for rows.Next() {
		var photoID, classIDStr string
		var conf, xmin, ymin, xmax, ymax float64
		if err = rows.Scan(&photoID, &classIDStr, &conf, &xmin, &ymin, &xmax, &ymax); err != nil {
			return nil, apperror.NewInternal(fmt.Errorf("sqlite: scan prediction: %w", err))
		}

		pred := domain.Prediction{
			ClassID:    domain.ClassID(classIDStr),
			Confidence: conf,
			BoundingBox: domain.BoundingBox{
				XMin: xmin, YMin: ymin,
				XMax: xmax, YMax: ymax,
			},
		}
		if fieldErrs := domain.ValidatePrediction(pred); len(fieldErrs) > 0 {
			return nil, apperror.NewInternal(fmt.Errorf("sqlite: malformed prediction for photo %s: %s", photoID, fieldErrs[0].Error()))
		}
		out[photoID] = append(out[photoID], pred)
	}
	if err = rows.Err(); err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("sqlite: predictions rows: %w", err))
	}
	if err = rows.Close(); err != nil {
		return nil, apperror.NewInternal(fmt.Errorf("sqlite: predictions close: %w", err))
	}
	return out, nil
}

// resolveLimit normalises and validates the caller-supplied limit.
func resolveLimit(n int) (int, error) {
	if n == 0 {
		return DefaultLimit, nil
	}
	if n < MinLimit || n > MaxLimit {
		return 0, apperror.NewValidation("invalid limit", []apperror.FieldViolation{
			{Field: "limit", Issue: fmt.Sprintf("must be between %d and %d", MinLimit, MaxLimit)},
		})
	}
	return n, nil
}

// photoScanner abstracts *sql.Row and *sql.Rows so a single scan function covers both.
type photoScanner interface {
	Scan(dest ...any) error
}

func scanPhoto(row *sql.Row) (domain.Photo, error) {
	return scanPhotoScanner(row)
}

func scanPhotoRow(rows *sql.Rows) (domain.Photo, error) {
	return scanPhotoScanner(rows)
}

func scanPhotoScanner(s photoScanner) (domain.Photo, error) {
	var id, capturedAtStr string
	var x, y, h float64
	var width, height int

	if err := s.Scan(&id, &x, &y, &h, &width, &height, &capturedAtStr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Photo{}, sql.ErrNoRows
		}
		return domain.Photo{}, apperror.NewInternal(fmt.Errorf("sqlite: scan photo: %w", err))
	}

	t, err := time.Parse(time.RFC3339Nano, capturedAtStr)
	if err != nil {
		t, err = time.Parse(time.RFC3339, capturedAtStr)
		if err != nil {
			return domain.Photo{}, apperror.NewInternal(fmt.Errorf("sqlite: malformed captured_at %q: %w", capturedAtStr, err))
		}
	}

	ph := domain.Photo{
		ID:         id,
		X:          x,
		Y:          y,
		H:          h,
		Width:      width,
		Height:     height,
		CapturedAt: t,
	}

	if fieldErrs := domain.ValidatePhoto(ph); len(fieldErrs) > 0 {
		return domain.Photo{}, apperror.NewInternal(fmt.Errorf("sqlite: malformed photo row %s: %s", id, fieldErrs[0].Error()))
	}

	return ph, nil
}

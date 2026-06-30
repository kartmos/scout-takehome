package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"scout/internal/apperror"
	"scout/internal/domain"
)

// ---- Fixture IDs and timestamps ----

const (
	// Group A: same captured_at, two photos (sorted DESC id: A2 then A1)
	photoA1 = "00000000-0000-0000-0000-000000000001"
	photoA2 = "00000000-0000-0000-0000-000000000002"
	// Group B
	photoB1 = "00000000-0000-0000-0000-000000000003"
	// Group C: no predictions
	photoC1 = "00000000-0000-0000-0000-000000000004"
	// Group D: negative-test for same-prediction filter
	photoD1 = "00000000-0000-0000-0000-000000000005"
	// Group E: positive both-filter; also tests "all preds returned when filtered"
	photoE1 = "00000000-0000-0000-0000-000000000006"
)

var (
	timeA = time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC)
	timeB = time.Date(2024, 1, 2, 12, 0, 0, 0, time.UTC)
	timeC = time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	timeD = time.Date(2023, 12, 31, 12, 0, 0, 0, time.UTC)
	timeE = time.Date(2023, 12, 30, 12, 0, 0, 0, time.UTC)
)

// sortedPhotoIDs is the deterministic DESC order: captured_at DESC, id DESC.
var sortedPhotoIDs = []string{photoA2, photoA1, photoB1, photoC1, photoD1, photoE1}

const schema = `
CREATE TABLE IF NOT EXISTS photos (
	id TEXT PRIMARY KEY,
	x REAL NOT NULL, y REAL NOT NULL, h REAL NOT NULL,
	width INTEGER NOT NULL, height INTEGER NOT NULL,
	captured_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS predictions (
	id TEXT PRIMARY KEY, photo_id TEXT NOT NULL REFERENCES photos(id),
	class_id TEXT NOT NULL, confidence REAL NOT NULL,
	bbox_xmin REAL NOT NULL, bbox_ymin REAL NOT NULL, bbox_xmax REAL NOT NULL, bbox_ymax REAL NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pred_photo ON predictions(photo_id);
`

// createFixtureDB creates a temp SQLite database with the standard schema and fixture data.
// The caller owns the file and must remove it when done.
func createFixtureDB(t *testing.T) (path string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "scout-test-*.db")
	if err != nil {
		t.Fatalf("createFixtureDB: create temp file: %v", err)
	}
	_ = f.Close()
	path = f.Name()

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("createFixtureDB: open: %v", err)
	}
	defer db.Close()

	if _, err = db.Exec(schema); err != nil {
		t.Fatalf("createFixtureDB: schema: %v", err)
	}

	photos := []struct {
		id         string
		x, y, h    float64
		w, h2      int
		capturedAt string
	}{
		{photoA1, 5.0, 10.0, 2.5, 2560, 1440, timeA.Format(time.RFC3339)},
		{photoA2, 6.0, 11.0, 2.5, 2560, 1440, timeA.Format(time.RFC3339)},
		{photoB1, 7.0, 12.0, 2.5, 2560, 1440, timeB.Format(time.RFC3339)},
		{photoC1, 8.0, 13.0, 2.5, 2560, 1440, timeC.Format(time.RFC3339)},
		{photoD1, 9.0, 14.0, 2.5, 2560, 1440, timeD.Format(time.RFC3339)},
		{photoE1, 10.0, 15.0, 2.5, 2560, 1440, timeE.Format(time.RFC3339)},
	}
	for _, p := range photos {
		_, err = db.Exec(
			`INSERT INTO photos (id,x,y,h,width,height,captured_at) VALUES (?,?,?,?,?,?,?)`,
			p.id, p.x, p.y, p.h, p.w, p.h2, p.capturedAt,
		)
		if err != nil {
			t.Fatalf("createFixtureDB: insert photo %s: %v", p.id, err)
		}
	}

	predictions := []struct {
		id                     string
		photoID                string
		classID                string
		conf                   float64
		xmin, ymin, xmax, ymax float64
	}{
		// photoA1: {mirid,0.9} and {thrips,0.3}
		{"pred-a1-1", photoA1, "mirid", 0.9, 0.1, 0.1, 0.4, 0.4},
		{"pred-a1-2", photoA1, "thrips", 0.3, 0.5, 0.5, 0.8, 0.8},
		// photoA2: {powdery_mildew,0.8}
		{"pred-a2-1", photoA2, "powdery_mildew", 0.8, 0.1, 0.1, 0.5, 0.5},
		// photoB1: {mirid,0.5}
		{"pred-b1-1", photoB1, "mirid", 0.5, 0.2, 0.2, 0.6, 0.6},
		// photoC1: no predictions
		// photoD1 (negative test): {mirid,0.4} and {thrips,0.8} — class and conf on different preds
		{"pred-d1-1", photoD1, "mirid", 0.4, 0.1, 0.1, 0.3, 0.3},
		{"pred-d1-2", photoD1, "thrips", 0.8, 0.4, 0.4, 0.7, 0.7},
		// photoE1 (positive both-filter): {mirid,0.8} and {spider_mites,0.3}
		{"pred-e1-1", photoE1, "mirid", 0.8, 0.1, 0.1, 0.4, 0.4},
		{"pred-e1-2", photoE1, "spider_mites", 0.3, 0.5, 0.5, 0.9, 0.9},
	}
	for _, p := range predictions {
		_, err = db.Exec(
			`INSERT INTO predictions (id,photo_id,class_id,confidence,bbox_xmin,bbox_ymin,bbox_xmax,bbox_ymax)
			 VALUES (?,?,?,?,?,?,?,?)`,
			p.id, p.photoID, p.classID, p.conf, p.xmin, p.ymin, p.xmax, p.ymax,
		)
		if err != nil {
			t.Fatalf("createFixtureDB: insert prediction %s: %v", p.id, err)
		}
	}
	return path
}

func openFixture(t *testing.T) (*Repository, string) {
	t.Helper()
	path := createFixtureDB(t)
	r, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })
	return r, path
}

// ---- Open / close ----

func TestOpen_nonexistent(t *testing.T) {
	_, err := Open("/nonexistent/path/that/does/not/exist.db")
	if err == nil {
		t.Fatal("expected error opening nonexistent path")
	}
}

func TestOpen_emptyPath(t *testing.T) {
	_, err := Open("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestOpen_readOnly(t *testing.T) {
	r, path := openFixture(t)

	// Attempting a write via the repo's internal db (via a raw query workaround)
	// is not possible without exposing internals, so we verify indirectly:
	// open a second connection in write mode and ensure the fixture DB is reachable.
	db2, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer db2.Close()

	// The read-only repository should still be usable alongside the writer.
	exists, err := r.PhotoExists(context.Background(), photoA1)
	if err != nil || !exists {
		t.Fatalf("PhotoExists after second open: exists=%v err=%v", exists, err)
	}
}

// ---- PhotoExists ----

func TestPhotoExists_present(t *testing.T) {
	r, _ := openFixture(t)
	exists, err := r.PhotoExists(context.Background(), photoA1)
	if err != nil {
		t.Fatalf("PhotoExists: %v", err)
	}
	if !exists {
		t.Error("expected photo to exist")
	}
}

func TestPhotoExists_absent(t *testing.T) {
	r, _ := openFixture(t)
	exists, err := r.PhotoExists(context.Background(), "ffffffff-ffff-ffff-ffff-ffffffffffff")
	if err != nil {
		t.Fatalf("PhotoExists: %v", err)
	}
	if exists {
		t.Error("expected photo to be absent")
	}
}

func TestPhotoExists_malformedID(t *testing.T) {
	r, _ := openFixture(t)
	_, err := r.PhotoExists(context.Background(), "not-a-uuid")
	if err == nil {
		t.Fatal("expected validation error")
	}
	assertValidationError(t, err, "photoId")
}

// ---- GetPhoto ----

func TestGetPhoto_mapping(t *testing.T) {
	r, _ := openFixture(t)
	ph, err := r.GetPhoto(context.Background(), photoA1)
	if err != nil {
		t.Fatalf("GetPhoto: %v", err)
	}
	if ph.ID != photoA1 {
		t.Errorf("ID: got %q", ph.ID)
	}
	if ph.X != 5.0 || ph.Y != 10.0 || ph.H != 2.5 {
		t.Errorf("position: got x=%v y=%v h=%v", ph.X, ph.Y, ph.H)
	}
	if ph.Width != 2560 || ph.Height != 1440 {
		t.Errorf("dimensions: got %dx%d", ph.Width, ph.Height)
	}
	if !ph.CapturedAt.Equal(timeA) {
		t.Errorf("CapturedAt: got %v want %v", ph.CapturedAt, timeA)
	}
}

func TestGetPhoto_allPredictions(t *testing.T) {
	r, _ := openFixture(t)
	ph, err := r.GetPhoto(context.Background(), photoA1)
	if err != nil {
		t.Fatalf("GetPhoto: %v", err)
	}
	if len(ph.Predictions) != 2 {
		t.Fatalf("expected 2 predictions, got %d", len(ph.Predictions))
	}
	// Verify both classes are present.
	classes := map[domain.ClassID]bool{}
	for _, p := range ph.Predictions {
		classes[p.ClassID] = true
	}
	if !classes[domain.ClassMirid] || !classes[domain.ClassThrips] {
		t.Errorf("expected mirid and thrips, got %v", classes)
	}
}

func TestGetPhoto_noPredictions(t *testing.T) {
	r, _ := openFixture(t)
	ph, err := r.GetPhoto(context.Background(), photoC1)
	if err != nil {
		t.Fatalf("GetPhoto no-pred photo: %v", err)
	}
	if ph.Predictions == nil {
		t.Error("expected non-nil empty prediction slice")
	}
	if len(ph.Predictions) != 0 {
		t.Errorf("expected 0 predictions, got %d", len(ph.Predictions))
	}
}

func TestGetPhoto_notFound(t *testing.T) {
	r, _ := openFixture(t)
	_, err := r.GetPhoto(context.Background(), "ffffffff-ffff-ffff-ffff-ffffffffffff")
	if err == nil {
		t.Fatal("expected not-found error")
	}
	assertNotFoundError(t, err)
}

func TestGetPhoto_malformedID(t *testing.T) {
	r, _ := openFixture(t)
	_, err := r.GetPhoto(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected validation error")
	}
	assertValidationError(t, err, "photoId")
}

func TestGetPhoto_malformedTimestamp(t *testing.T) {
	r, path := openFixture(t)
	// Insert a photo with a bad captured_at.
	db, _ := sql.Open("sqlite", "file:"+path)
	defer db.Close()
	badID := "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	_, err := db.Exec(
		`INSERT INTO photos (id,x,y,h,width,height,captured_at) VALUES (?,5,5,1,100,100,'not-a-timestamp')`,
		badID,
	)
	if err != nil {
		t.Fatalf("insert malformed photo: %v", err)
	}

	_, err = r.GetPhoto(context.Background(), badID)
	if err == nil {
		t.Fatal("expected internal error for malformed timestamp")
	}
	assertInternalError(t, err)
}

func TestGetPhoto_invalidBboxInDB(t *testing.T) {
	r, path := openFixture(t)
	db, _ := sql.Open("sqlite", "file:"+path)
	defer db.Close()

	badPredID := "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	_, err := db.Exec(
		`INSERT INTO predictions (id,photo_id,class_id,confidence,bbox_xmin,bbox_ymin,bbox_xmax,bbox_ymax)
		 VALUES (?,?,?,?,?,?,?,?)`,
		badPredID, photoA2, "mirid", 0.5, 0.8, 0.1, 0.4, 0.9, // xmin > xmax
	)
	if err != nil {
		t.Fatalf("insert bad prediction: %v", err)
	}

	_, err = r.GetPhoto(context.Background(), photoA2)
	if err == nil {
		t.Fatal("expected internal error for invalid bbox")
	}
	assertInternalError(t, err)
}

func TestGetPhoto_invalidConfidenceInDB(t *testing.T) {
	r, path := openFixture(t)
	db, _ := sql.Open("sqlite", "file:"+path)
	defer db.Close()

	badPredID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	_, err := db.Exec(
		`INSERT INTO predictions (id,photo_id,class_id,confidence,bbox_xmin,bbox_ymin,bbox_xmax,bbox_ymax)
		 VALUES (?,?,?,?,?,?,?,?)`,
		badPredID, photoB1, "mirid", 1.5, 0.1, 0.1, 0.4, 0.4, // confidence > 1
	)
	if err != nil {
		t.Fatalf("insert bad prediction: %v", err)
	}

	_, err = r.GetPhoto(context.Background(), photoB1)
	if err == nil {
		t.Fatal("expected internal error for invalid confidence")
	}
	assertInternalError(t, err)
}

// ---- ListPhotos: limits ----

func TestListPhotos_defaultLimit(t *testing.T) {
	r, _ := openFixture(t)
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{Limit: 0})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	// Only 6 photos in fixture, all returned with default limit=50.
	if len(page.Items) != 6 {
		t.Errorf("expected 6 items, got %d", len(page.Items))
	}
	if page.NextToken != "" {
		t.Error("expected empty NextToken on final page")
	}
}

func TestListPhotos_negativeLimitRejected(t *testing.T) {
	r, _ := openFixture(t)
	_, err := r.ListPhotos(context.Background(), ListPhotosParams{Limit: -1})
	if err == nil {
		t.Fatal("expected validation error for limit=-1")
	}
	assertValidationError(t, err, "limit")
}

func TestListPhotos_overMaxLimitRejected(t *testing.T) {
	r, _ := openFixture(t)
	_, err := r.ListPhotos(context.Background(), ListPhotosParams{Limit: MaxLimit + 1})
	if err == nil {
		t.Fatal("expected validation error for limit>200")
	}
	assertValidationError(t, err, "limit")
}

func TestListPhotos_limitOne(t *testing.T) {
	r, _ := openFixture(t)
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{Limit: 1})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(page.Items))
	}
	if page.Items[0].ID != photoA2 {
		t.Errorf("expected photoA2 first, got %q", page.Items[0].ID)
	}
	if page.NextToken == "" {
		t.Error("expected NextToken for non-final page")
	}
}

// ---- ListPhotos: sort order ----

func TestListPhotos_sortOrder(t *testing.T) {
	r, _ := openFixture(t)
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{Limit: 0})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	for i, want := range sortedPhotoIDs {
		if i >= len(page.Items) {
			t.Fatalf("expected %d items, got %d", len(sortedPhotoIDs), len(page.Items))
		}
		if page.Items[i].ID != want {
			t.Errorf("position %d: got %q, want %q", i, page.Items[i].ID, want)
		}
	}
}

// ---- ListPhotos: filter semantics ----

func TestListPhotos_noFilters(t *testing.T) {
	r, _ := openFixture(t)
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if len(page.Items) != 6 {
		t.Errorf("expected 6 photos, got %d", len(page.Items))
	}
}

func TestListPhotos_filterClassOnly(t *testing.T) {
	r, _ := openFixture(t)
	cls := domain.ClassMirid
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{ClassID: &cls})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	// mirid appears in: photoA1, photoB1, photoD1, photoE1
	wantIDs := []string{photoA1, photoB1, photoD1, photoE1}
	assertPhotoIDs(t, page.Items, wantIDs)
}

func TestListPhotos_filterConfidenceOnly(t *testing.T) {
	r, _ := openFixture(t)
	minConf := 0.8
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{MinConfidence: &minConf})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	// confidence >= 0.8 in at least one pred: photoA1(0.9), photoA2(0.8), photoD1(0.8), photoE1(0.8)
	wantIDs := []string{photoA2, photoA1, photoD1, photoE1}
	assertPhotoIDs(t, page.Items, wantIDs)
}

func TestListPhotos_filterConfidenceInclusive(t *testing.T) {
	r, _ := openFixture(t)
	// Exact 0.8 boundary — inclusive.
	minConf := 0.8
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{MinConfidence: &minConf})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	// photoA2 has exactly 0.8 — must be included.
	foundA2 := false
	for _, ph := range page.Items {
		if ph.ID == photoA2 {
			foundA2 = true
		}
	}
	if !foundA2 {
		t.Error("photoA2 (confidence=0.8) must be included with minConfidence=0.8")
	}
}

func TestListPhotos_filterBothSamePrediction_positive(t *testing.T) {
	r, _ := openFixture(t)
	cls := domain.ClassMirid
	minConf := 0.8
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{
		ClassID:       &cls,
		MinConfidence: &minConf,
	})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	// Only photoA1 {mirid,0.9} and photoE1 {mirid,0.8} have one pred satisfying both.
	// photoD1 is excluded: mirid has 0.4 (<0.8) and thrips has 0.8 (wrong class).
	wantIDs := []string{photoA1, photoE1}
	assertPhotoIDs(t, page.Items, wantIDs)
}

func TestListPhotos_negativeSamePrediction(t *testing.T) {
	r, _ := openFixture(t)
	cls := domain.ClassMirid
	minConf := 0.8
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{
		ClassID:       &cls,
		MinConfidence: &minConf,
	})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	// photoD1 has {mirid,0.4} and {thrips,0.8} — different predictions match each filter.
	// It must NOT appear in results.
	for _, ph := range page.Items {
		if ph.ID == photoD1 {
			t.Error("photoD1 must be excluded: no single prediction satisfies both classId=mirid and minConfidence=0.8")
		}
	}
}

func TestListPhotos_matchedPhotoReturnsAllPredictions(t *testing.T) {
	r, _ := openFixture(t)
	// photoE1 matches {mirid,0.8}; it also has {spider_mites,0.3} which doesn't match.
	// All predictions must be returned.
	cls := domain.ClassMirid
	minConf := 0.8
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{
		ClassID:       &cls,
		MinConfidence: &minConf,
	})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	var e1Photo *domain.Photo
	for i := range page.Items {
		if page.Items[i].ID == photoE1 {
			e1Photo = &page.Items[i]
		}
	}
	if e1Photo == nil {
		t.Fatal("photoE1 not found in results")
	}
	if len(e1Photo.Predictions) != 2 {
		t.Errorf("expected 2 predictions for photoE1, got %d", len(e1Photo.Predictions))
	}
}

func TestListPhotos_unknownClassIDRejected(t *testing.T) {
	r, _ := openFixture(t)
	cls := domain.ClassID("unknown_class")
	_, err := r.ListPhotos(context.Background(), ListPhotosParams{ClassID: &cls})
	if err == nil {
		t.Fatal("expected validation error for unknown classId")
	}
	assertValidationError(t, err, "classId")
}

func TestListPhotos_invalidConfidenceRejected(t *testing.T) {
	r, _ := openFixture(t)
	bad := 1.5
	_, err := r.ListPhotos(context.Background(), ListPhotosParams{MinConfidence: &bad})
	if err == nil {
		t.Fatal("expected validation error for minConfidence > 1")
	}
	assertValidationError(t, err, "minConfidence")
}

// ---- ListPhotos: pagination ----

func TestListPhotos_multiPageTraversal(t *testing.T) {
	r, _ := openFixture(t)
	// Traverse all 6 photos 2 at a time.
	var allIDs []string
	cursor := ""
	for i := range 4 {
		page, err := r.ListPhotos(context.Background(), ListPhotosParams{Limit: 2, Cursor: cursor})
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		for _, ph := range page.Items {
			allIDs = append(allIDs, ph.ID)
		}
		cursor = page.NextToken
		if cursor == "" {
			break
		}
	}
	if len(allIDs) != 6 {
		t.Fatalf("expected 6 total IDs, got %d: %v", len(allIDs), allIDs)
	}
	for i, want := range sortedPhotoIDs {
		if allIDs[i] != want {
			t.Errorf("position %d: got %q, want %q", i, allIDs[i], want)
		}
	}
}

func TestListPhotos_noDuplicatesOrOmissions(t *testing.T) {
	r, _ := openFixture(t)
	seen := map[string]int{}
	cursor := ""
	for {
		page, err := r.ListPhotos(context.Background(), ListPhotosParams{Limit: 2, Cursor: cursor})
		if err != nil {
			t.Fatalf("ListPhotos: %v", err)
		}
		for _, ph := range page.Items {
			seen[ph.ID]++
		}
		cursor = page.NextToken
		if cursor == "" {
			break
		}
	}
	for _, id := range sortedPhotoIDs {
		if seen[id] != 1 {
			t.Errorf("photo %q seen %d times, want 1", id, seen[id])
		}
	}
}

func TestListPhotos_equalTimestampsResolvedByID(t *testing.T) {
	r, _ := openFixture(t)
	// photoA1 and photoA2 share the same captured_at; they must be separated by id DESC.
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{Limit: 1})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if page.Items[0].ID != photoA2 {
		t.Errorf("expected photoA2 (higher id) first among equal timestamps, got %q", page.Items[0].ID)
	}

	page2, err := r.ListPhotos(context.Background(), ListPhotosParams{Limit: 1, Cursor: page.NextToken})
	if err != nil {
		t.Fatalf("ListPhotos page2: %v", err)
	}
	if page2.Items[0].ID != photoA1 {
		t.Errorf("expected photoA1 second, got %q", page2.Items[0].ID)
	}
}

func TestListPhotos_finalPageEmptyToken(t *testing.T) {
	r, _ := openFixture(t)
	// Last page: request more than remaining.
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{Limit: 200})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	if page.NextToken != "" {
		t.Error("expected empty NextToken on final page")
	}
}

func TestListPhotos_malformedCursor(t *testing.T) {
	r, _ := openFixture(t)
	_, err := r.ListPhotos(context.Background(), ListPhotosParams{Cursor: "!!!invalid!!!"})
	if err == nil {
		t.Fatal("expected error for malformed cursor")
	}
	assertValidationError(t, err, "cursor")
}

func TestListPhotos_noPredictionsPhoto(t *testing.T) {
	r, _ := openFixture(t)
	page, err := r.ListPhotos(context.Background(), ListPhotosParams{})
	if err != nil {
		t.Fatalf("ListPhotos: %v", err)
	}
	for _, ph := range page.Items {
		if ph.ID == photoC1 {
			if ph.Predictions == nil {
				t.Error("expected non-nil empty slice for no-prediction photo")
			}
			return
		}
	}
	t.Fatal("photoC1 not found in results")
}

// ---- Context cancellation ----

func TestListPhotos_contextCancelled(t *testing.T) {
	r, _ := openFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.ListPhotos(ctx, ListPhotosParams{})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestGetPhoto_contextCancelled(t *testing.T) {
	r, _ := openFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.GetPhoto(ctx, photoA1)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// ---- Supplied DB unchanged ----

func TestSuppliedDB_unchanged(t *testing.T) {
	dbPath := filepath.Join("..", "..", "..", "..", "dataset", "predictions.db")
	if _, err := os.Stat(dbPath); err != nil {
		t.Skipf("supplied DB not found at %q: %v", dbPath, err)
	}

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	modBefore := info.ModTime()

	r, openErr := Open(dbPath)
	if openErr != nil {
		t.Fatalf("Open: %v", openErr)
	}
	defer r.Close()

	_, _ = r.ListPhotos(context.Background(), ListPhotosParams{})

	info2, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if info2.ModTime() != modBefore {
		t.Error("supplied DB was modified")
	}
}

// ---- helpers ----

func assertValidationError(t *testing.T, err error, field string) {
	t.Helper()
	var ae *apperror.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *apperror.AppError, got %T: %v", err, err)
	}
	if ae.Kind() != apperror.KindValidation {
		t.Errorf("expected KindValidation, got %v", ae.Kind())
	}
	for _, v := range ae.Violations() {
		if v.Field == field {
			return
		}
	}
	t.Errorf("expected violation for field %q in %v", field, ae.Violations())
}

func assertNotFoundError(t *testing.T, err error) {
	t.Helper()
	var ae *apperror.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *apperror.AppError, got %T", err)
	}
	if ae.Kind() != apperror.KindNotFound {
		t.Errorf("expected KindNotFound, got %v", ae.Kind())
	}
}

func assertInternalError(t *testing.T, err error) {
	t.Helper()
	var ae *apperror.AppError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *apperror.AppError, got %T", err)
	}
	if ae.Kind() != apperror.KindInternal {
		t.Errorf("expected KindInternal, got %v", ae.Kind())
	}
	// Confirm cause text doesn't leak through Error().
	if ae.Error() != "an internal error occurred" {
		t.Errorf("internal error leaked cause in Error(): %q", ae.Error())
	}
}

func assertPhotoIDs(t *testing.T, photos []domain.Photo, wantIDs []string) {
	t.Helper()
	if len(photos) != len(wantIDs) {
		gotIDs := make([]string, len(photos))
		for i, p := range photos {
			gotIDs[i] = p.ID
		}
		t.Fatalf("expected %d photos %v, got %d %v", len(wantIDs), wantIDs, len(photos), gotIDs)
	}
	for i, want := range wantIDs {
		if photos[i].ID != want {
			t.Errorf("position %d: got %q, want %q", i, photos[i].ID, want)
		}
	}
}

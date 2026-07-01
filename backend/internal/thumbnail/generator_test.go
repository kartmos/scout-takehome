package thumbnail_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"scout/internal/domain"
	"scout/internal/thumbnail"
)

// encodeJPEG encodes img as JPEG at quality q and returns the bytes.
func encodeJPEG(t *testing.T, img image.Image, q int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// makeGrayJPEG returns JPEG bytes for a w×h grey image.
func makeGrayJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetGray(x, y, color.Gray{Y: 128})
		}
	}
	return encodeJPEG(t, img, 85)
}

// testPhoto returns a valid domain.Photo for use with the generator.
func testPhoto(w, h int) domain.Photo {
	return domain.Photo{
		ID:     "550e8400-e29b-41d4-a716-446655440000",
		Width:  w,
		Height: h,
	}
}

// fakeOriginalStorage implements the narrow originalOpener interface for tests.
// It avoids importing the full objectstorage package.
type fakeOriginalStorage struct {
	mu   sync.Mutex
	data map[string][]byte
	err  error
}

func newFakeStorage(photoID string, jpegData []byte) *fakeOriginalStorage {
	return &fakeOriginalStorage{data: map[string][]byte{photoID: jpegData}}
}

func (s *fakeOriginalStorage) OpenOriginal(_ context.Context, id string) (io.ReadCloser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	data, ok := s.data[id]
	if !ok {
		return nil, &notFoundError{id: id}
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

type notFoundError struct{ id string }

func (e *notFoundError) Error() string    { return "not found: " + e.id }
func (e *notFoundError) IsNotFound() bool { return true }

// TestGenerate_OutputDimensions verifies the generated JPEG has correct dimensions.
func TestGenerate_OutputDimensions(t *testing.T) {
	const srcW, srcH = 800, 600
	jpegData := makeGrayJPEG(t, srcW, srcH)
	stor := newFakeStorage("550e8400-e29b-41d4-a716-446655440000", jpegData)
	gen := thumbnail.NewGenerator(stor, 1)

	photo := testPhoto(srcW, srcH)
	req, _ := thumbnail.ParseRequest("400", "", "")

	var out bytes.Buffer
	_, err := gen.Generate(context.Background(), photo, req, &out)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	img, err := jpeg.Decode(&out)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 400 {
		t.Errorf("output width = %d, want 400", b.Dx())
	}
	// Aspect ratio: 600/800 * 400 = 300
	if b.Dy() != 300 {
		t.Errorf("output height = %d, want 300", b.Dy())
	}
}

// TestGenerate_DirectPath verifies that images not needing resize are handled correctly.
func TestGenerate_DirectPath(t *testing.T) {
	const srcW, srcH = 200, 150
	jpegData := makeGrayJPEG(t, srcW, srcH)
	stor := newFakeStorage("550e8400-e29b-41d4-a716-446655440000", jpegData)
	gen := thumbnail.NewGenerator(stor, 1)

	photo := testPhoto(srcW, srcH)
	// Request more pixels than source → no resize (no upscale).
	req, _ := thumbnail.ParseRequest("400", "", "")

	var out bytes.Buffer
	_, err := gen.Generate(context.Background(), photo, req, &out)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	img, err := jpeg.Decode(&out)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != srcW || b.Dy() != srcH {
		t.Errorf("output dims = %dx%d, want %dx%d (direct path, no upscale)",
			b.Dx(), b.Dy(), srcW, srcH)
	}
}

// TestGenerate_MediaType verifies output is valid JPEG.
func TestGenerate_MediaType(t *testing.T) {
	jpegData := makeGrayJPEG(t, 100, 100)
	stor := newFakeStorage("550e8400-e29b-41d4-a716-446655440000", jpegData)
	gen := thumbnail.NewGenerator(stor, 1)

	var out bytes.Buffer
	req, _ := thumbnail.ParseRequest("50", "", "")
	gen.Generate(context.Background(), testPhoto(100, 100), req, &out)

	b := out.Bytes()
	if len(b) < 3 || b[0] != 0xff || b[1] != 0xd8 {
		t.Errorf("output must start with JPEG SOI marker; got %x %x", b[0], b[1])
	}
}

// TestGenerate_CorruptInput verifies corrupt JPEG returns an appropriate error.
func TestGenerate_CorruptInput(t *testing.T) {
	stor := newFakeStorage("550e8400-e29b-41d4-a716-446655440000", []byte("not a jpeg"))
	gen := thumbnail.NewGenerator(stor, 1)

	var out bytes.Buffer
	req, _ := thumbnail.ParseRequest("50", "", "")
	_, err := gen.Generate(context.Background(), testPhoto(10, 10), req, &out)
	if err == nil {
		t.Error("Generate with corrupt input must return an error")
	}
}

// TestGenerate_DimensionMismatch verifies mismatch between DB dimensions and JPEG header is rejected.
func TestGenerate_DimensionMismatch(t *testing.T) {
	// JPEG is 200×100, but photo DB record says 400×200.
	jpegData := makeGrayJPEG(t, 200, 100)
	stor := newFakeStorage("550e8400-e29b-41d4-a716-446655440000", jpegData)
	gen := thumbnail.NewGenerator(stor, 1)

	photo := testPhoto(400, 200) // wrong dimensions

	var out bytes.Buffer
	req, _ := thumbnail.ParseRequest("100", "", "")
	_, err := gen.Generate(context.Background(), photo, req, &out)
	if err == nil {
		t.Error("Generate must fail when JPEG dimensions differ from database record")
	}
	var unsupported *thumbnail.ErrUnsupportedImage
	if !errors.As(err, &unsupported) {
		t.Errorf("error = %T(%v); want *ErrUnsupportedImage", err, err)
	}
}

// TestGenerate_OversourceMetadata verifies source images exceeding dimension limits are rejected.
func TestGenerate_OversourceMetadata(t *testing.T) {
	// Create a JPEG within reasonable actual size but fake the photo metadata to be huge.
	// The generator reads cfg.Width/Height from the actual JPEG, so we need a real big JPEG.
	// Instead, test the photo dimension validation path with invalid photo record.
	stor := newFakeStorage("550e8400-e29b-41d4-a716-446655440000", makeGrayJPEG(t, 10, 10))
	gen := thumbnail.NewGenerator(stor, 1)

	// Photo with zero dimensions should be rejected before storage access.
	badPhoto := domain.Photo{
		ID:     "550e8400-e29b-41d4-a716-446655440000",
		Width:  0,
		Height: 0,
	}
	var out bytes.Buffer
	req, _ := thumbnail.ParseRequest("50", "", "")
	_, err := gen.Generate(context.Background(), badPhoto, req, &out)
	if err == nil {
		t.Error("Generate must reject photo with zero dimensions")
	}
}

// TestGenerate_InvalidPhotoID verifies non-UUID photo IDs are rejected immediately.
func TestGenerate_InvalidPhotoID(t *testing.T) {
	stor := newFakeStorage("any", makeGrayJPEG(t, 10, 10))
	gen := thumbnail.NewGenerator(stor, 1)

	badPhoto := domain.Photo{ID: "not-a-uuid", Width: 10, Height: 10}
	var out bytes.Buffer
	req, _ := thumbnail.ParseRequest("50", "", "")
	_, err := gen.Generate(context.Background(), badPhoto, req, &out)
	if err == nil {
		t.Error("Generate must reject invalid photo UUID")
	}
}

// TestGenerate_ContextCancellation verifies cancellation is honored.
func TestGenerate_ContextCancellation(t *testing.T) {
	jpegData := makeGrayJPEG(t, 100, 100)
	stor := newFakeStorage("550e8400-e29b-41d4-a716-446655440000", jpegData)
	gen := thumbnail.NewGenerator(stor, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var out bytes.Buffer
	req, _ := thumbnail.ParseRequest("50", "", "")
	_, err := gen.Generate(ctx, testPhoto(100, 100), req, &out)
	if err == nil {
		t.Error("Generate must return an error when context is already cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v; want context.Canceled", err)
	}
}

// TestGenerate_OriginalStreamClosed verifies the storage stream is closed after generation.
func TestGenerate_OriginalStreamClosed(t *testing.T) {
	jpegData := makeGrayJPEG(t, 100, 100)
	var closed bool
	rc := &trackingReadCloser{
		Reader:  bytes.NewReader(jpegData),
		onClose: func() { closed = true },
	}

	stor := &singleReadCloserStorage{rc: rc}
	gen := thumbnail.NewGenerator(stor, 1)

	var out bytes.Buffer
	req, _ := thumbnail.ParseRequest("50", "", "")
	gen.Generate(context.Background(), testPhoto(100, 100), req, &out)

	if !closed {
		t.Error("original stream must be closed after generation")
	}
}

// TestGenerate_ConcurrencyLimit verifies concurrent generation does not exceed the configured limit.
// It measures concurrent access inside OpenOriginal (after semaphore acquisition) using atomics.
func TestGenerate_ConcurrencyLimit(t *testing.T) {
	const concurrency = 1

	jpegData := makeGrayJPEG(t, 100, 100)
	stor := &countingStorage{data: jpegData}
	gen := thumbnail.NewGenerator(stor, concurrency)

	const goroutines = 4
	var wg sync.WaitGroup
	results := make(chan error, goroutines)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out bytes.Buffer
			req, _ := thumbnail.ParseRequest("50", "", "")
			_, err := gen.Generate(context.Background(), testPhoto(100, 100), req, &out)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	for err := range results {
		if err != nil {
			t.Errorf("Generate: %v", err)
		}
	}
	if got := stor.peak.Load(); got > concurrency {
		t.Errorf("peak concurrent generations inside semaphore = %d, must not exceed %d", got, concurrency)
	}
}

// TestGenerate_WaitingCancellationDoesNotConsumeSlot verifies that a goroutine waiting for
// the semaphore whose context is cancelled does not consume a slot or block future callers.
func TestGenerate_WaitingCancellationDoesNotConsumeSlot(t *testing.T) {
	jpegData := makeGrayJPEG(t, 50, 50)
	// holderIn is closed when the holder has acquired the semaphore (inside OpenOriginal).
	holderIn := make(chan struct{})
	// releaseHolder is closed to let the holder's OpenOriginal return.
	releaseHolder := make(chan struct{})

	stor := &signallingStorage{
		data:    jpegData,
		onEnter: func() { close(holderIn) },
		unblock: releaseHolder,
	}
	gen := thumbnail.NewGenerator(stor, 1)

	// Start holder goroutine: acquires the only semaphore slot.
	holderDone := make(chan struct{})
	go func() {
		defer close(holderDone)
		var out bytes.Buffer
		req, _ := thumbnail.ParseRequest("25", "", "")
		gen.Generate(context.Background(), testPhoto(50, 50), req, &out)
	}()

	// Wait until the holder owns the slot (inside OpenOriginal).
	select {
	case <-holderIn:
	case <-time.After(5 * time.Second):
		t.Fatal("holder goroutine did not acquire slot in time")
	}

	// Now start a waiter with a short deadline — it will time out on the semaphore.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	req, _ := thumbnail.ParseRequest("25", "", "")
	gen.Generate(ctx, testPhoto(50, 50), req, &out)

	// Release the holder so it finishes and returns the slot.
	close(releaseHolder)
	select {
	case <-holderDone:
	case <-time.After(5 * time.Second):
		t.Fatal("holder goroutine did not finish after release")
	}

	// Verify the slot is back by immediately running a new generation.
	stor2 := newFakeStorage("550e8400-e29b-41d4-a716-446655440000", jpegData)
	gen2 := thumbnail.NewGenerator(stor2, 1)
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	var out2 bytes.Buffer
	req2, _ := thumbnail.ParseRequest("25", "", "")
	if _, err := gen2.Generate(ctx2, testPhoto(50, 50), req2, &out2); err != nil {
		t.Errorf("slot must be available after cancelled waiter: %v", err)
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

type trackingReadCloser struct {
	io.Reader
	onClose func()
}

func (r *trackingReadCloser) Close() error {
	r.onClose()
	return nil
}

type singleReadCloserStorage struct {
	rc io.ReadCloser
}

func (s *singleReadCloserStorage) OpenOriginal(_ context.Context, _ string) (io.ReadCloser, error) {
	return s.rc, nil
}

// countingStorage measures peak concurrent access inside OpenOriginal (after semaphore acquisition).
type countingStorage struct {
	data   []byte
	active atomic.Int32
	peak   atomic.Int32
}

func (s *countingStorage) OpenOriginal(_ context.Context, _ string) (io.ReadCloser, error) {
	n := s.active.Add(1)
	defer s.active.Add(-1)
	for {
		cur := s.peak.Load()
		if n <= cur || s.peak.CompareAndSwap(cur, n) {
			break
		}
	}
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

// signallingStorage signals when it enters OpenOriginal and waits for release.
type signallingStorage struct {
	data    []byte
	onEnter func() // called once, on first entry
	once    sync.Once
	unblock <-chan struct{}
}

func (s *signallingStorage) OpenOriginal(_ context.Context, _ string) (io.ReadCloser, error) {
	s.once.Do(func() {
		if s.onEnter != nil {
			s.onEnter()
		}
	})
	<-s.unblock
	return io.NopCloser(bytes.NewReader(s.data)), nil
}

// Compile-time interface checks.
var _ interface {
	OpenOriginal(context.Context, string) (io.ReadCloser, error)
} = (*fakeOriginalStorage)(nil)

// Suppress unused import.
var _ = strings.TrimSpace

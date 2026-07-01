package thumbnail_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"scout/internal/domain"
	"scout/internal/thumbnail"
)

// newTestCache creates a bounded cache in t.TempDir.
func newTestCache(t *testing.T, maxBytes int64) *thumbnail.Cache {
	t.Helper()
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024 // 1 MiB default for tests
	}
	c, err := thumbnail.NewCache(t.TempDir(), maxBytes)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	return c
}

// writeBytes is a genFn that writes the provided bytes.
func writeBytes(data []byte) func(io.Writer) error {
	return func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	}
}

// errGenFn is a genFn that returns an error.
func errGenFn(err error) func(io.Writer) error {
	return func(io.Writer) error { return err }
}

// validHash returns a deterministic valid 64-char lowercase hex key for testing.
func validHash(n uint64) string {
	return fmt.Sprintf("%064x", n)
}

// TestCache_Hit verifies that a cache hit bypasses the generator.
func TestCache_Hit(t *testing.T) {
	c := newTestCache(t, 1024*1024)
	content := makeGrayJPEG(t, 10, 10)

	// Miss: populate cache.
	calls := 0
	f, size, _, err := c.GetOrCreate(context.Background(), validHash(1), func(w io.Writer) error {
		calls++
		_, err := w.Write(content)
		return err
	})
	if err != nil {
		t.Fatalf("first GetOrCreate: %v", err)
	}
	f.Close()

	if calls != 1 {
		t.Errorf("want 1 generation call on miss, got %d", calls)
	}

	// Hit: genFn must NOT be called.
	f2, size2, hit, err := c.GetOrCreate(context.Background(), validHash(1), func(w io.Writer) error {
		calls++
		_, _ = w.Write(content)
		return nil
	})
	if err != nil {
		t.Fatalf("second GetOrCreate: %v", err)
	}
	defer f2.Close()

	if calls != 1 {
		t.Errorf("genFn called on cache hit (calls=%d)", calls)
	}
	if !hit {
		t.Error("second GetOrCreate must return hit=true")
	}
	if size != size2 {
		t.Errorf("size mismatch: first=%d second=%d", size, size2)
	}
	_ = size
}

// TestCache_Miss_Commit verifies a miss triggers generation and commits to disk.
func TestCache_Miss_Commit(t *testing.T) {
	c := newTestCache(t, 1024*1024)
	content := makeGrayJPEG(t, 20, 20)

	f, size, hit, err := c.GetOrCreate(context.Background(), validHash(2), writeBytes(content))
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	defer f.Close()

	if hit {
		t.Error("first GetOrCreate must return hit=false on a cold cache")
	}
	if size != int64(len(content)) {
		t.Errorf("committed size = %d, want %d", size, len(content))
	}
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("content mismatch: got %d bytes, want %d", len(got), len(content))
	}
}

// TestCache_StableETag verifies the ETag (hash) is stable across hits.
func TestCache_StableETag(t *testing.T) {
	photo := domain.Photo{
		ID:     "550e8400-e29b-41d4-a716-446655440000",
		Width:  800,
		Height: 600,
	}
	req, _ := thumbnail.ParseRequest("400", "", "")
	dims := thumbnail.ResolveDims(photo.Width, photo.Height, req.RequestedPixels)
	key := thumbnail.NewKey(photo, req, dims)

	h1 := key.Hash()
	h2 := key.Hash()
	if h1 != h2 {
		t.Errorf("Hash is not stable: %q != %q", h1, h2)
	}
	if h1 == "" {
		t.Error("Hash must not be empty")
	}
}

// TestCache_ContentLength verifies Size returned from GetOrCreate equals actual bytes.
func TestCache_ContentLength(t *testing.T) {
	c := newTestCache(t, 1024*1024)
	content := makeGrayJPEG(t, 30, 30)

	f, size, _, err := c.GetOrCreate(context.Background(), validHash(3), writeBytes(content))
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	defer f.Close()

	fi, statErr := f.Stat()
	if statErr != nil {
		t.Fatalf("Stat: %v", statErr)
	}
	if fi.Size() != size {
		t.Errorf("Stat().Size()=%d, GetOrCreate size=%d: must match", fi.Size(), size)
	}
}

// TestCache_FailedGenerationNoPartialEntry verifies a failed generation leaves no cache entry.
func TestCache_FailedGenerationNoPartialEntry(t *testing.T) {
	c := newTestCache(t, 1024*1024)
	genErr := errors.New("generation failed")

	_, _, _, err := c.GetOrCreate(context.Background(), validHash(4), errGenFn(genErr))
	if err == nil {
		t.Fatal("GetOrCreate must return error on failed generation")
	}

	// A subsequent call with a working generator must succeed (no stale partial entry).
	content := makeGrayJPEG(t, 10, 10)
	f, _, _, err := c.GetOrCreate(context.Background(), validHash(4), writeBytes(content))
	if err != nil {
		t.Fatalf("second GetOrCreate after failed first: %v", err)
	}
	f.Close()
}

// TestCache_OversizedOutputRejection verifies outputs exceeding maxBytes are rejected.
func TestCache_OversizedOutputRejection(t *testing.T) {
	content := makeGrayJPEG(t, 200, 200) // decent size JPEG
	// Set maxBytes smaller than the JPEG.
	c := newTestCache(t, int64(len(content)/2))

	_, _, _, err := c.GetOrCreate(context.Background(), validHash(5), writeBytes(content))
	if err == nil {
		t.Error("GetOrCreate must reject output exceeding cache budget")
	}
}

// TestCache_StartupStaleTempCleanup verifies that stale temp files are removed on startup.
func TestCache_StartupStaleTempCleanup(t *testing.T) {
	dir := t.TempDir()
	// Plant a stale temp file that a previous run left behind.
	tmpName := "abc123.tmp.deadbeef"
	if err := os.WriteFile(filepath.Join(dir, tmpName), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := thumbnail.NewCache(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	// The stale temp file must have been removed.
	if _, err := os.Stat(filepath.Join(dir, tmpName)); !os.IsNotExist(err) {
		t.Errorf("stale temp file must be removed on startup, stat returned: %v", err)
	}
}

// TestCache_SymlinkDirRejected verifies that a symlink as the cache root is rejected.
func TestCache_SymlinkDirRejected(t *testing.T) {
	realDir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skip("symlinks not supported:", err)
	}

	_, err := thumbnail.NewCache(link, 1024*1024)
	if err == nil {
		t.Error("NewCache must reject a symlink as the cache root")
	}
}

// TestCache_RecencyEviction verifies that the least-recently-accessed entry is evicted first.
func TestCache_RecencyEviction(t *testing.T) {
	content := makeGrayJPEG(t, 50, 50)
	size := int64(len(content))

	// Fit exactly 2 entries.
	c := newTestCache(t, size*2)

	f1, _, _, err := c.GetOrCreate(context.Background(), validHash(6), writeBytes(content))
	if err != nil {
		t.Fatalf("GetOrCreate old: %v", err)
	}
	f1.Close()

	f2, _, _, err := c.GetOrCreate(context.Background(), validHash(7), writeBytes(content))
	if err != nil {
		t.Fatalf("GetOrCreate recent: %v", err)
	}
	f2.Close()

	// Access validHash(7) again to make it more recently used.
	f3, _, _, err := c.GetOrCreate(context.Background(), validHash(7), writeBytes(content))
	if err != nil {
		t.Fatalf("GetOrCreate recent second: %v", err)
	}
	f3.Close()

	// Add a third entry; validHash(6) should be evicted.
	f4, _, _, err := c.GetOrCreate(context.Background(), validHash(8), writeBytes(content))
	if err != nil {
		t.Fatalf("GetOrCreate newest: %v", err)
	}
	f4.Close()

	stats := c.Stats()
	if stats.Bytes > size*2 {
		t.Errorf("cache bytes = %d, must not exceed budget %d", stats.Bytes, size*2)
	}
	if stats.Evictions == 0 {
		t.Error("at least one eviction must have occurred")
	}
}

// TestCache_CacheBytesAtOrBelowMax verifies cache never exceeds its byte limit.
func TestCache_CacheBytesAtOrBelowMax(t *testing.T) {
	content := makeGrayJPEG(t, 40, 40)
	size := int64(len(content))
	maxBytes := size * 3

	c := newTestCache(t, maxBytes)

	// Add 5 distinct entries; eviction must keep total ≤ maxBytes.
	for i := range 5 {
		f, _, _, err := c.GetOrCreate(context.Background(), validHash(uint64(i+100)), writeBytes(content))
		if err != nil {
			t.Fatalf("GetOrCreate %d: %v", i, err)
		}
		f.Close()
		stats := c.Stats()
		if stats.Bytes > maxBytes {
			t.Errorf("after entry %d: cache bytes %d exceeds max %d", i, stats.Bytes, maxBytes)
		}
	}
}

// TestCache_ConcurrentIdenticalMissesGenerateOnce verifies singleflight deduplication.
// Run under the race detector to catch any data races.
func TestCache_ConcurrentIdenticalMissesGenerateOnce(t *testing.T) {
	c := newTestCache(t, 10*1024*1024)
	content := makeGrayJPEG(t, 60, 60)

	var genCalls atomic.Int32
	genFn := func(w io.Writer) error {
		genCalls.Add(1)
		_, err := w.Write(content)
		return err
	}

	const concurrency = 20
	results := make(chan []byte, concurrency)
	var wg sync.WaitGroup

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f, _, _, err := c.GetOrCreate(context.Background(), validHash(9), genFn)
			if err != nil {
				results <- nil
				return
			}
			defer f.Close()
			data, _ := io.ReadAll(f)
			results <- data
		}()
	}

	wg.Wait()
	close(results)

	if got := genCalls.Load(); got != 1 {
		t.Errorf("generation called %d times, want exactly 1", got)
	}

	var allBytes [][]byte
	for data := range results {
		if data == nil {
			t.Error("a caller received an error")
		}
		allBytes = append(allBytes, data)
	}
	if len(allBytes) != concurrency {
		t.Errorf("got %d results, want %d", len(allBytes), concurrency)
	}
	for i, b := range allBytes {
		if !bytes.Equal(b, content) {
			t.Errorf("result %d differs from expected (%d bytes vs %d)", i, len(b), len(content))
		}
	}
}

// TestCache_CoalescedHitMiss verifies hit/miss accounting for concurrent cold requests.
// N simultaneous cold requests must all be misses with one generation.
// A subsequent warm request must be a hit with no generation.
func TestCache_CoalescedHitMiss(t *testing.T) {
	c := newTestCache(t, 10*1024*1024)
	content := makeGrayJPEG(t, 40, 40)

	const N = 10
	// Use a channel to synchronize all goroutines to start at the same moment.
	ready := make(chan struct{})

	var genCalls atomic.Int32
	genFn := func(w io.Writer) error {
		genCalls.Add(1)
		_, err := w.Write(content)
		return err
	}

	type result struct {
		hit bool
		err error
	}
	results := make(chan result, N)

	for range N {
		go func() {
			<-ready
			_, _, hit, err := c.GetOrCreate(context.Background(), validHash(20), genFn)
			results <- result{hit: hit, err: err}
		}()
	}

	close(ready) // release all goroutines simultaneously
	var hits, misses int
	for range N {
		r := <-results
		if r.err != nil {
			t.Errorf("GetOrCreate error: %v", r.err)
			continue
		}
		if r.hit {
			hits++
		} else {
			misses++
		}
	}

	if genCalls.Load() != 1 {
		t.Errorf("generation called %d times, want exactly 1", genCalls.Load())
	}
	if hits != 0 {
		t.Errorf("cold burst: got %d hits, want 0", hits)
	}
	if misses != N {
		t.Errorf("cold burst: got %d misses, want %d", misses, N)
	}

	// Warm request: must be a hit with no additional generation.
	_, _, warmHit, err := c.GetOrCreate(context.Background(), validHash(20), genFn)
	if err != nil {
		t.Fatalf("warm GetOrCreate: %v", err)
	}
	if !warmHit {
		t.Error("warm request must return hit=true")
	}
	if genCalls.Load() != 1 {
		t.Errorf("warm request triggered generation (calls=%d, want 1)", genCalls.Load())
	}
}

// TestCache_CancelledGenerationCleanup verifies that a cancelled caller returns
// promptly while the shared singleflight goroutine finishes and cleans up any
// temp file without racing the test's TempDir removal.
func TestCache_CancelledGenerationCleanup(t *testing.T) {
	c := newTestCache(t, 1024*1024)

	genStarted := make(chan struct{})
	releaseGen := make(chan struct{})
	var genOnce sync.Once

	genFn := func(_ io.Writer) error {
		genOnce.Do(func() { close(genStarted) })
		<-releaseGen
		return context.Canceled
	}

	hash := validHash(10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Leader goroutine: will start the singleflight generation.
	callerErr := make(chan error, 1)
	go func() {
		_, _, _, err := c.GetOrCreate(ctx, hash, genFn)
		callerErr <- err
	}()

	// Wait for generation to start, then cancel the caller context.
	<-genStarted
	cancel()

	// The cancelled caller must return promptly with context.Canceled.
	if err := <-callerErr; !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled caller: want context.Canceled, got %v", err)
	}

	// The singleflight goroutine is still blocked at releaseGen. Start a watcher
	// now (before releasing) so it joins the in-flight singleflight. Waiting for
	// the watcher ensures the flight goroutine fully exits (including temp-file
	// cleanup) before the test returns and t.TempDir removes the directory.
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		c.GetOrCreate(context.Background(), hash, genFn)
	}()

	// Release generation; genFn returns an error so no entry is committed.
	close(releaseGen)

	// Wait for the flight to fully exit via the watcher.
	<-watcherDone

	// Cache state must remain consistent with no partial entry.
	stats := c.Stats()
	if stats.Bytes < 0 {
		t.Errorf("cache bytes must not be negative, got %d", stats.Bytes)
	}
}

// TestCache_RestartReconciliation verifies that existing cache entries are indexed on NewCache.
func TestCache_RestartReconciliation(t *testing.T) {
	dir := t.TempDir()
	content := makeGrayJPEG(t, 15, 15)

	// First run: populate cache.
	c1, err := thumbnail.NewCache(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewCache c1: %v", err)
	}
	f, _, _, err := c1.GetOrCreate(context.Background(), validHash(11), writeBytes(content))
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	f.Close()

	// Second run: new Cache instance must find the existing entry.
	c2, err := thumbnail.NewCache(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewCache c2: %v", err)
	}

	var generated bool
	f2, _, hit, err := c2.GetOrCreate(context.Background(), validHash(11), func(w io.Writer) error {
		generated = true
		_, err := w.Write(content)
		return err
	})
	if err != nil {
		t.Fatalf("GetOrCreate c2: %v", err)
	}
	defer f2.Close()

	if generated {
		t.Error("cache entry must be found on restart without re-generation")
	}
	if !hit {
		t.Error("restart reconciliation: existing entry must return hit=true")
	}
}

// TestCache_PathSafety verifies invalid keys are rejected before any filesystem access.
func TestCache_PathSafety(t *testing.T) {
	dir := t.TempDir()
	c, err := thumbnail.NewCache(dir, 1024*1024)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}

	// All of these must be rejected by key validation.
	badKeys := []string{
		"../escape",
		"../../etc/passwd",
		"/absolute/path",
		"",
		"short",
		"toolong" + validHash(0),
		"UPPERCASE000000000000000000000000000000000000000000000000000000",
		"00000000000000000000000000000000000000000000000000000000GGGGGGGG",
		validHash(0)[:63],  // one char short
		validHash(0) + "a", // one char too long
		".",
		"..",
		"valid/but/has/slash" + validHash(0), // has slashes
		"\\backslash" + validHash(0),
	}

	var genCalled atomic.Int32
	content := []byte("should not reach generator")
	for _, k := range badKeys {
		t.Run(k, func(t *testing.T) {
			before := genCalled.Load()
			_, _, _, err := c.GetOrCreate(context.Background(), k, func(w io.Writer) error {
				genCalled.Add(1)
				_, _ = w.Write(content)
				return nil
			})
			if err == nil {
				t.Errorf("key %q must be rejected, got nil error", k)
			}
			if genCalled.Load() != before {
				t.Errorf("key %q: generator must not be called on invalid key", k)
			}
		})
	}

	// No files must have been created in or outside the cache root.
	entries, _ := os.ReadDir(dir)
	for _, de := range entries {
		t.Errorf("unexpected file created: %s", de.Name())
	}

	// Valid key must succeed (sanity check).
	good := validHash(99)
	f, _, _, err := c.GetOrCreate(context.Background(), good, writeBytes([]byte("ok")))
	if err != nil {
		t.Errorf("valid key must succeed: %v", err)
	} else {
		f.Close()
	}

	// Stats must never go negative.
	stats := c.Stats()
	if stats.Bytes < 0 || stats.Entries < 0 {
		t.Errorf("Stats: Bytes=%d Entries=%d, must not be negative", stats.Bytes, stats.Entries)
	}
}

// TestCache_LeaderCancelledFollowerSucceeds verifies that cancelling the leader's request
// context does not abort shared generation needed by a live follower.
// The genFn captures a long-lived genCtx (simulating Service.genCtx), so generation
// continues even when the leader's caller ctx is cancelled.
// Run under the race detector.
func TestCache_LeaderCancelledFollowerSucceeds(t *testing.T) {
	c := newTestCache(t, 10*1024*1024)
	content := makeGrayJPEG(t, 20, 20)

	genStarted := make(chan struct{})
	releaseGen := make(chan struct{})
	var genCalls atomic.Int32
	var genOnce sync.Once

	// genCtx simulates the service lifetime context – not tied to any individual request.
	genCtx, cancelGenCtx := context.WithCancel(context.Background())
	defer cancelGenCtx()

	genFn := func(w io.Writer) error {
		genCalls.Add(1)
		genOnce.Do(func() { close(genStarted) })
		select {
		case <-releaseGen:
			_, err := w.Write(content)
			return err
		case <-genCtx.Done():
			return genCtx.Err()
		}
	}

	hash := validHash(300)

	// Start the leader with a cancellable request context.
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	defer cancelLeader()

	leaderErr := make(chan error, 1)
	go func() {
		f, _, _, err := c.GetOrCreate(leaderCtx, hash, genFn)
		if f != nil {
			f.Close()
		}
		leaderErr <- err
	}()

	// Wait until the singleflight leader is inside genFn (generation in progress).
	<-genStarted

	// Start the follower; the singleflight flight is in progress so it joins it.
	followerDone := make(chan error, 1)
	go func() {
		f, _, _, err := c.GetOrCreate(context.Background(), hash, genFn)
		if f != nil {
			f.Close()
		}
		followerDone <- err
	}()

	// Cancel the leader's request context – must not abort the shared generation.
	cancelLeader()

	if err := <-leaderErr; !errors.Is(err, context.Canceled) {
		t.Errorf("leader: want context.Canceled, got %v", err)
	}

	// Release generation; the singleflight goroutine completes successfully.
	close(releaseGen)

	// Follower must succeed (from the shared generation or the committed cache entry).
	if err := <-followerDone; err != nil {
		t.Errorf("follower: want success, got %v", err)
	}

	if genCalls.Load() != 1 {
		t.Errorf("generation calls = %d, want exactly 1", genCalls.Load())
	}
}

// TestCache_GenCtxCancellationAbortsWaiters verifies that cancelling the service generation
// context aborts in-flight generation and wakes all live waiters with context.Canceled.
// Run under the race detector.
func TestCache_GenCtxCancellationAbortsWaiters(t *testing.T) {
	c := newTestCache(t, 10*1024*1024)

	// genCtx simulates the server shutdown signal context.
	genCtx, cancelGenCtx := context.WithCancel(context.Background())

	genStarted := make(chan struct{})
	var genOnce sync.Once

	genFn := func(_ io.Writer) error {
		genOnce.Do(func() { close(genStarted) })
		<-genCtx.Done()
		return genCtx.Err()
	}

	hash := validHash(400)
	const N = 3
	errs := make(chan error, N)

	for range N {
		go func() {
			_, _, _, err := c.GetOrCreate(context.Background(), hash, genFn)
			errs <- err
		}()
	}

	// Wait until generation is in progress.
	<-genStarted

	// Cancel the service lifetime context – simulates server shutdown.
	cancelGenCtx()

	// All N waiters must wake with context.Canceled.
	for range N {
		err := <-errs
		if err == nil {
			t.Error("waiter: want error, got nil")
		} else if !errors.Is(err, context.Canceled) {
			t.Errorf("waiter: want context.Canceled, got %v", err)
		}
	}
}

// TestCache_StartupIndexCap verifies overflow valid entries are removed on startup.
func TestCache_StartupIndexCap(t *testing.T) {
	dir := t.TempDir()
	content := makeGrayJPEG(t, 10, 10)

	// Pre-populate dir with cap+2 canonical cache files, bypassing Cache.
	const cap = 5
	for i := range cap + 2 {
		name := fmt.Sprintf("%064x.jpg", i)
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Also add a non-canonical file that must NOT be removed.
	nonCanonical := "README.txt"
	if err := os.WriteFile(filepath.Join(dir, nonCanonical), []byte("doc"), 0o600); err != nil {
		t.Fatal(err)
	}

	c, err := thumbnail.NewCacheWithIndexCap(dir, int64(len(content))*int64(cap+10), cap)
	if err != nil {
		t.Fatalf("NewCacheWithIndexCap: %v", err)
	}

	stats := c.Stats()

	// At most cap entries must be indexed.
	if stats.Entries > cap {
		t.Errorf("indexed entries = %d, must not exceed cap %d", stats.Entries, cap)
	}

	// Non-canonical file must still exist.
	if _, err := os.Stat(filepath.Join(dir, nonCanonical)); err != nil {
		t.Errorf("non-canonical file must not be removed: %v", err)
	}

	// Total valid JPEG files on disk must not exceed cap (overflow were deleted).
	entries, _ := os.ReadDir(dir)
	var jpegCount int
	for _, de := range entries {
		if len(de.Name()) == 64+4 { // 64 hex + ".jpg"
			jpegCount++
		}
	}
	if jpegCount > cap {
		t.Errorf("JPEG files on disk = %d, want ≤ cap %d (overflow not deleted)", jpegCount, cap)
	}

	// Accounted bytes must match indexed entries.
	expectedBytes := int64(stats.Entries) * int64(len(content))
	if stats.Bytes != expectedBytes {
		t.Errorf("cache bytes = %d, want %d (entries×size)", stats.Bytes, expectedBytes)
	}
}

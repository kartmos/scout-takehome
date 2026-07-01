package thumbnail

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	cacheExt       = ".jpg"
	cacheTempInfix = ".tmp."
	maxIndexSize   = 100_000
	keyLen         = 64 // SHA-256 hex
)

type cacheEntry struct {
	size       int64
	accessedAt time.Time
}

// Cache is a bounded LRU disk cache for generated JPEG thumbnails.
// All index mutations are serialized through mu. Concurrent misses for the same
// key collapse to a single generation via singleflight.
type Cache struct {
	dir             string
	maxBytes        int64
	maxIndexEntries int
	mu              sync.Mutex
	index           map[string]*cacheEntry
	total           int64
	sf              singleflight.Group
	// evictions counts entries removed by evictLocked; protected by mu.
	evictions int64
}

// CacheStats is a point-in-time snapshot of cache state.
type CacheStats struct {
	Bytes     int64
	Entries   int
	Evictions int64
}

// Stats returns a snapshot of current cache state.
func (c *Cache) Stats() CacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return CacheStats{
		Bytes:     c.total,
		Entries:   len(c.index),
		Evictions: c.evictions,
	}
}

// NewCache initializes a bounded thumbnail disk cache rooted at dir with the given
// max budget. It validates the root (no symlinks), removes stale temp files, indexes
// existing entries, and evicts overflow before returning.
func NewCache(dir string, maxBytes int64) (*Cache, error) {
	return newCacheInternal(dir, maxBytes, maxIndexSize)
}

// NewCacheWithIndexCap is like NewCache but overrides the maximum in-memory index size.
// Use only in tests; production code should use NewCache.
func NewCacheWithIndexCap(dir string, maxBytes int64, maxIndex int) (*Cache, error) {
	if maxIndex <= 0 {
		return nil, fmt.Errorf("thumbnail cache: maxIndex must be positive")
	}
	return newCacheInternal(dir, maxBytes, maxIndex)
}

func newCacheInternal(dir string, maxBytes int64, maxIndex int) (*Cache, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("thumbnail cache: maxBytes must be positive, got %d", maxBytes)
	}
	if err := validateCacheRoot(dir); err != nil {
		return nil, err
	}
	c := &Cache{
		dir:             dir,
		maxBytes:        maxBytes,
		maxIndexEntries: maxIndex,
		index:           make(map[string]*cacheEntry),
	}
	return c, c.startup()
}

func validateCacheRoot(dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("thumbnail cache: directory must not be empty")
	}
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0o700)
	}
	if err != nil {
		return fmt.Errorf("thumbnail cache: stat %s: %w", dir, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("thumbnail cache: %s must not be a symlink", dir)
	}
	if !info.IsDir() {
		return fmt.Errorf("thumbnail cache: %s is not a directory", dir)
	}
	return nil
}

// startup indexes existing cache files and cleans up stale temp files.
// Valid cache-owned files beyond maxIndexEntries are removed so no valid
// entry is left unaccounted on disk.
func (c *Cache) startup() error {
	des, err := os.ReadDir(c.dir)
	if err != nil {
		return fmt.Errorf("thumbnail cache: read dir: %w", err)
	}
	for _, de := range des {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if strings.Contains(name, cacheTempInfix) {
			_ = os.Remove(filepath.Join(c.dir, name))
			continue
		}
		if !strings.HasSuffix(name, cacheExt) {
			continue // not a cache file; leave untouched
		}
		hash := strings.TrimSuffix(name, cacheExt)
		if !isValidKey(hash) {
			continue // not canonical cache-owned format; leave untouched
		}
		fi, serr := de.Info()
		if serr != nil || fi.Size() == 0 {
			_ = os.Remove(filepath.Join(c.dir, name))
			continue
		}
		if len(c.index) >= c.maxIndexEntries {
			// Overflow: remove so no valid entry is left unaccounted.
			_ = os.Remove(filepath.Join(c.dir, name))
			continue
		}
		c.index[hash] = &cacheEntry{size: fi.Size(), accessedAt: fi.ModTime()}
		c.total += fi.Size()
	}
	c.evictLocked()
	return nil
}

// isValidKey returns true if key is exactly 64 lowercase hex characters (SHA-256).
// This is the only form accepted by exported cache operations.
func isValidKey(key string) bool {
	if len(key) != keyLen {
		return false
	}
	for _, c := range key {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// GetOrCreate returns an open *os.File for the thumbnail identified by hash,
// generating it with genFn if necessary. Concurrent misses for the same hash
// collapse to one generation; all callers open the committed entry independently.
// The returned hit is true only when the entry was found on the initial lookup
// (before any generation attempt); every caller that went through singleflight
// returns hit=false regardless of whether it was the leader.
// The returned file is owned by the caller and must be closed.
// hash must be exactly 64 lowercase hex characters; other values are rejected.
func (c *Cache) GetOrCreate(ctx context.Context, hash string, genFn func(io.Writer) error) (*os.File, int64, bool, error) {
	if !isValidKey(hash) {
		return nil, 0, false, fmt.Errorf("thumbnail cache: invalid key %q", hash)
	}

	if f, size, ok := c.openEntry(hash); ok {
		return f, size, true, nil
	}

	ch := c.sf.DoChan(hash, func() (any, error) {
		// Double-check inside singleflight slot to handle races between fast-path miss
		// and a concurrent leader that already committed.
		if f, _, ok := c.openEntry(hash); ok {
			_ = f.Close()
			return nil, nil
		}
		return nil, c.generateAndCommit(hash, genFn)
	})

	select {
	case res := <-ch:
		if res.Err != nil {
			return nil, 0, false, res.Err
		}
	case <-ctx.Done():
		return nil, 0, false, ctx.Err()
	}

	f, size, ok := c.openEntry(hash)
	if !ok {
		return nil, 0, false, fmt.Errorf("thumbnail cache: entry evicted immediately after generation")
	}
	// hit=false: this caller did not find the entry on its initial lookup.
	return f, size, false, nil
}

// openEntry checks the in-memory index and opens the cached file.
// Updates access time in-memory (no filesystem write per hit).
func (c *Cache) openEntry(hash string) (*os.File, int64, bool) {
	c.mu.Lock()
	e, ok := c.index[hash]
	if ok {
		e.accessedAt = time.Now()
	}
	var size int64
	if ok {
		size = e.size
	}
	c.mu.Unlock()

	if !ok {
		return nil, 0, false
	}
	f, err := os.Open(filepath.Join(c.dir, hash+cacheExt))
	if err != nil {
		// File may have been evicted between index check and open.
		c.mu.Lock()
		if cur, still := c.index[hash]; still {
			c.total -= cur.size
			delete(c.index, hash)
		}
		c.mu.Unlock()
		return nil, 0, false
	}
	return f, size, true
}

// generateAndCommit creates a temp file, streams genFn into it with a byte ceiling,
// and atomically renames it to the committed cache path. Cleans up on any failure.
func (c *Cache) generateAndCommit(hash string, genFn func(io.Writer) error) error {
	tmpPath := filepath.Join(c.dir, hash+cacheTempInfix+randHex(8))

	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("thumbnail cache: create temp: %w", err)
	}

	written, err := writeToTemp(f, c.maxBytes, genFn)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	// f is closed by writeToTemp on success.

	dstPath := filepath.Join(c.dir, hash+cacheExt)
	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("thumbnail cache: rename: %w", err)
	}

	c.mu.Lock()
	c.index[hash] = &cacheEntry{size: written, accessedAt: time.Now()}
	c.total += written
	c.evictLocked()
	c.mu.Unlock()
	return nil
}

// writeToTemp streams genFn to f with a maxBytes ceiling, then syncs and closes f.
// On success f is closed; on error f may be open and caller must close and remove it.
func writeToTemp(f *os.File, maxBytes int64, genFn func(io.Writer) error) (int64, error) {
	lw := &limitWriter{w: f, limit: maxBytes}
	if err := genFn(lw); err != nil {
		return 0, err
	}
	if lw.written == 0 {
		return 0, fmt.Errorf("thumbnail cache: generator produced zero bytes")
	}
	if err := f.Sync(); err != nil {
		return 0, fmt.Errorf("thumbnail cache: sync: %w", err)
	}
	if err := f.Close(); err != nil {
		return 0, fmt.Errorf("thumbnail cache: close: %w", err)
	}
	return lw.written, nil
}

// evictLocked removes least-recently-accessed entries until total ≤ maxBytes.
// Must be called with c.mu held.
func (c *Cache) evictLocked() {
	if c.total <= c.maxBytes {
		return
	}
	type item struct {
		hash string
		e    *cacheEntry
	}
	items := make([]item, 0, len(c.index))
	for h, e := range c.index {
		items = append(items, item{hash: h, e: e})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].e.accessedAt.Before(items[j].e.accessedAt)
	})
	for _, it := range items {
		if c.total <= c.maxBytes {
			break
		}
		path := filepath.Join(c.dir, it.hash+cacheExt)
		if err := os.Remove(path); err == nil || os.IsNotExist(err) {
			c.total -= it.e.size
			delete(c.index, it.hash)
			c.evictions++
		}
	}
}

// limitWriter enforces a maximum byte ceiling on writes.
type limitWriter struct {
	w       io.Writer
	limit   int64
	written int64
}

func (lw *limitWriter) Write(p []byte) (int, error) {
	if lw.written+int64(len(p)) > lw.limit {
		return 0, &ErrGeneration{Cause: fmt.Errorf("thumbnail output exceeds cache budget of %d bytes", lw.limit)}
	}
	n, err := lw.w.Write(p)
	lw.written += int64(n)
	return n, err
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

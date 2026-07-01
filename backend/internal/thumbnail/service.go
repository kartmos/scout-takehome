package thumbnail

import (
	"context"
	"io"
	"os"
	"time"

	"scout/internal/domain"
)

// ServiceHooks are optional observability callbacks. All fields may be nil.
type ServiceHooks struct {
	// OnCacheHit is called when the thumbnail was served from disk without generation.
	OnCacheHit func()
	// OnCacheMiss is called on each request whose initial lookup finds no committed entry,
	// including coalesced followers that join an existing singleflight. The leader goroutine
	// is the only one that runs generation; miss count can therefore exceed generation count
	// during concurrent cold bursts. Do not interpret miss count as generation count.
	OnCacheMiss func()
	// OnGenDone is called after each generation attempt with its wall-clock duration and error.
	// It is called even when generation fails, allowing accurate error counting.
	OnGenDone func(time.Duration, error)
}

// Service bridges domain photo metadata with thumbnail generation and disk caching.
type Service struct {
	gen   *Generator
	cache *Cache
	hooks ServiceHooks
	// genCtx is the shared generation lifetime context. Generation closures use this context,
	// not the individual request context, so a cancelled request cannot abort generation
	// needed by other live callers. Cancel genCtx to stop all in-flight generation (e.g., on
	// server shutdown). Must be non-nil.
	genCtx context.Context
}

// NewService constructs a Service with no observability hooks.
// ctx controls shared generation lifetime; cancel it during server shutdown to stop
// in-flight generation. Use signal.NotifyContext or a derived context in production.
func NewService(ctx context.Context, gen *Generator, cache *Cache) *Service {
	return &Service{gen: gen, cache: cache, genCtx: ctx}
}

// NewServiceWithHooks constructs a Service with observability hooks.
// ctx controls shared generation lifetime; cancel it during server shutdown to stop
// in-flight generation. Use signal.NotifyContext or a derived context in production.
func NewServiceWithHooks(ctx context.Context, gen *Generator, cache *Cache, hooks ServiceHooks) *Service {
	return &Service{gen: gen, cache: cache, hooks: hooks, genCtx: ctx}
}

// ThumbnailResult holds an open file for a committed thumbnail entry.
// The caller must close File.
type ThumbnailResult struct {
	File *os.File
	Size int64
}

// Get returns the thumbnail for the given photo and request parameters,
// fetching from disk cache or generating and caching it on miss.
// ctx controls this caller's wait; cancelling it returns promptly without aborting
// generation shared with other callers. Shared generation uses the service lifetime
// context provided at construction.
func (s *Service) Get(ctx context.Context, photo domain.Photo, req Request) (*ThumbnailResult, error) {
	dims := ResolveDims(photo.Width, photo.Height, req.RequestedPixels)
	key := NewKey(photo, req, dims)
	hash := key.Hash()

	genCtx := s.genCtx
	f, size, hit, err := s.cache.GetOrCreate(ctx, hash, func(w io.Writer) error {
		start := time.Now()
		_, genErr := s.gen.Generate(genCtx, photo, req, w)
		if s.hooks.OnGenDone != nil {
			s.hooks.OnGenDone(time.Since(start), genErr)
		}
		return genErr
	})
	if err != nil {
		return nil, err
	}

	if hit {
		if s.hooks.OnCacheHit != nil {
			s.hooks.OnCacheHit()
		}
	} else {
		if s.hooks.OnCacheMiss != nil {
			s.hooks.OnCacheMiss()
		}
	}

	return &ThumbnailResult{
		File: f,
		Size: size,
	}, nil
}

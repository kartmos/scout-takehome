package thumbnail

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"

	"golang.org/x/image/draw"

	"scout/internal/domain"
	"scout/internal/objectstorage"
)

// originalOpener is the narrow interface the Generator requires from object storage.
// It avoids a dependency on the full OriginalStorage interface and simplifies testing.
type originalOpener interface {
	OpenOriginal(ctx context.Context, photoID string) (io.ReadCloser, error)
}

const (
	// MaxGenerationConcurrency is the maximum allowed value for the semaphore.
	// It is kept at 2 to bound peak memory: each concurrent decode holds roughly
	// 14 MiB for a 2560×1440 source plus encode buffers.
	MaxGenerationConcurrency = 2

	// maxSourceWidth / maxSourceHeight are per-axis guards against hostile images.
	maxSourceWidth  = 8192
	maxSourceHeight = 8192

	// maxSourcePixels bounds total decoded pixel count. 20 MP × 4 bytes/pixel ≈ 80 MiB
	// per generation. With MaxGenerationConcurrency=2 that peaks at ~160 MiB, well inside
	// the 512 MiB runtime budget when combined with encode buffers and OS overhead.
	maxSourcePixels = 20 * 1024 * 1024
)

// Result is the immutable metadata describing a generated thumbnail.
type Result struct {
	Width   int
	Height  int
	Quality int
}

// Generator fetches originals from object storage, decodes, resizes, and JPEG-encodes them.
// It uses a bounded semaphore to cap concurrent memory-intensive operations.
type Generator struct {
	storage originalOpener
	// sem is a pre-filled channel used as a counting semaphore.
	// Acquire by receiving; release by sending.
	sem chan struct{}
}

// NewGenerator constructs a Generator. concurrency must be in [1, MaxGenerationConcurrency].
// storage need only implement OpenOriginal; a full objectstorage.OriginalStorage satisfies this.
func NewGenerator(storage originalOpener, concurrency int) *Generator {
	if concurrency < 1 || concurrency > MaxGenerationConcurrency {
		panic(fmt.Sprintf("thumbnail: concurrency %d out of range [1, %d]",
			concurrency, MaxGenerationConcurrency))
	}
	sem := make(chan struct{}, concurrency)
	for i := 0; i < concurrency; i++ {
		sem <- struct{}{}
	}
	return &Generator{storage: storage, sem: sem}
}

// Generate validates the photo and request, acquires the concurrency semaphore,
// fetches the original, decodes and resizes it, and encodes the result as JPEG into w.
//
// Context cancellation is checked before semaphore acquisition, before storage access,
// and before the decode phase. The storage stream is wrapped in a context-aware reader
// so cancellation can interrupt reads.
func (g *Generator) Generate(ctx context.Context, photo domain.Photo, req Request, w io.Writer) (Result, error) {
	// Validate before any I/O to fail fast on bad inputs.
	if !domain.IsValidUUID(photo.ID) {
		return Result{}, &ErrInvalidRequest{Field: "photo.id", Msg: "must be a canonical UUID"}
	}
	if photo.Width <= 0 || photo.Height <= 0 {
		return Result{}, &ErrInvalidRequest{Field: "photo.dimensions", Msg: "persisted dimensions must be positive"}
	}

	// Acquire semaphore before opening the original to bound concurrent decodes.
	select {
	case <-ctx.Done():
		return Result{}, ctx.Err()
	case <-g.sem:
	}
	defer func() { g.sem <- struct{}{} }()

	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	rc, err := g.storage.OpenOriginal(ctx, photo.ID)
	if err != nil {
		if objectstorage.IsNotFound(err) {
			return Result{}, &ErrNotFound{Cause: err}
		}
		return Result{}, &ErrGeneration{Cause: err}
	}

	result, genErr := generate(ctx, photo, req, rc, w)

	// Always close the stream. Preserve the primary error; attach the close error
	// as supplementary context when both are non-nil.
	closeErr := rc.Close()
	if genErr != nil {
		if closeErr != nil {
			return Result{}, fmt.Errorf("%w (close: %v)", genErr, closeErr)
		}
		return Result{}, genErr
	}
	if closeErr != nil {
		return Result{}, &ErrGeneration{Cause: closeErr}
	}
	return result, nil
}

// generate performs the actual decode → resize → encode pipeline.
// src is the raw storage stream; it must not be closed here.
func generate(ctx context.Context, photo domain.Photo, req Request, src io.Reader, w io.Writer) (Result, error) {
	// Wrap src to propagate context cancellation into every Read call.
	cr := &ctxReader{ctx: ctx, r: src}

	// Read the JPEG header config without discarding the stream.
	// io.TeeReader mirrors every byte read by DecodeConfig into headerBuf.
	// io.MultiReader then prepends those bytes before the remaining src bytes,
	// reconstructing the complete stream for jpeg.Decode.
	var headerBuf bytes.Buffer
	teeR := io.TeeReader(cr, &headerBuf)

	cfg, err := jpeg.DecodeConfig(teeR)
	if err != nil {
		return Result{}, &ErrUnsupportedImage{Msg: "cannot read JPEG config", Cause: err}
	}

	// Reject source images that exceed named per-axis and total-pixel limits before decode.
	if cfg.Width > maxSourceWidth || cfg.Height > maxSourceHeight {
		return Result{}, &ErrUnsupportedImage{
			Msg: fmt.Sprintf("source dimensions %dx%d exceed limit %dx%d",
				cfg.Width, cfg.Height, maxSourceWidth, maxSourceHeight),
		}
	}
	if cfg.Width*cfg.Height > maxSourcePixels {
		return Result{}, &ErrUnsupportedImage{
			Msg: fmt.Sprintf("source pixel count %d exceeds limit %d",
				cfg.Width*cfg.Height, maxSourcePixels),
		}
	}

	// Reject images whose stored dimensions differ from the database record.
	if cfg.Width != photo.Width || cfg.Height != photo.Height {
		return Result{}, &ErrUnsupportedImage{
			Msg: fmt.Sprintf("source dimensions %dx%d do not match database %dx%d",
				cfg.Width, cfg.Height, photo.Width, photo.Height),
		}
	}

	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	// Reconstruct the full stream: replay buffered header bytes then continue with cr.
	fullR := io.MultiReader(bytes.NewReader(headerBuf.Bytes()), cr)

	img, err := jpeg.Decode(fullR)
	if err != nil {
		return Result{}, &ErrUnsupportedImage{Msg: "failed to decode JPEG", Cause: err}
	}

	dims := ResolveDims(cfg.Width, cfg.Height, req.RequestedPixels)

	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	// When output dimensions equal source dimensions skip the resize allocation
	// and encode the original in-memory image directly at the requested quality.
	var out image.Image
	if dims.Width == cfg.Width && dims.Height == cfg.Height {
		out = img
	} else {
		dst := image.NewRGBA(image.Rect(0, 0, dims.Width, dims.Height))
		// CatmullRom is the highest-quality scaler in x/image/draw; suitable for
		// downscaling photographs with minimal aliasing.
		draw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
		out = dst
	}

	if err := jpeg.Encode(w, out, &jpeg.Options{Quality: req.Quality}); err != nil {
		return Result{}, &ErrGeneration{Cause: err}
	}

	return Result{Width: dims.Width, Height: dims.Height, Quality: req.Quality}, nil
}

// ctxReader checks the context before each Read to allow cancellation to interrupt
// streaming before the natural EOF, without requiring the underlying reader to support it.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *ctxReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

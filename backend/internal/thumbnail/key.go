package thumbnail

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"scout/internal/domain"
)

const (
	// generatorVersion is incremented whenever the resize or encoding algorithm
	// changes to invalidate cached thumbnails produced by earlier versions.
	generatorVersion = 1

	// thumbnailFormat is the only output format this generator produces.
	thumbnailFormat = "jpeg"
)

// Key is the canonical cache identity for a generated thumbnail.
// It encodes photo UUID, resolved dimensions, quality, format, and generator version.
// Equivalent parameter combinations that resolve to the same dimensions and quality
// share a single key.
type Key struct {
	PhotoID string
	Width   int
	Height  int
	Quality int
}

// NewKey constructs a Key from the resolved parameters.
func NewKey(photo domain.Photo, req Request, dims Dims) Key {
	return Key{
		PhotoID: photo.ID,
		Width:   dims.Width,
		Height:  dims.Height,
		Quality: req.Quality,
	}
}

// Hash returns a stable, hex-encoded SHA-256 hash of the canonical key components.
// Raw request parameters and filesystem-unsafe characters are never exposed.
func (k Key) Hash() string {
	// Lowercase UUID to normalise mixed-case inputs into a single canonical form.
	canonical := fmt.Sprintf("v%d/%s/%s/%dx%d/q%d",
		generatorVersion,
		thumbnailFormat,
		strings.ToLower(k.PhotoID),
		k.Width,
		k.Height,
		k.Quality,
	)
	sum := sha256.Sum256([]byte(canonical))
	return fmt.Sprintf("%x", sum)
}

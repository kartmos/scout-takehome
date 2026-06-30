package thumbnail

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	// Request parameter ranges.
	minWidth       = 1
	maxWidth       = 2048
	minDPR         = 1.0
	maxDPR         = 3.0
	defaultDPR     = 1.0
	minQuality     = 1
	maxQuality     = 100
	defaultQuality = 80

	// maxRequestedPixels is the output-pixel ceiling: maxWidth(2048) × maxDPR(3).
	// It caps the longest edge of any generated thumbnail in device pixels.
	maxRequestedPixels = 6144
)

// Request holds the validated and resolved thumbnail parameters.
type Request struct {
	// Width is the CSS base width in pixels (1..2048).
	Width int
	// DPR is the device pixel ratio (1..3).
	DPR float64
	// Quality is the JPEG quality (1..100).
	Quality int
	// RequestedPixels is the resolved output width in device pixels,
	// computed as round(Width × DPR), capped to maxRequestedPixels.
	RequestedPixels int
}

// ParseRequest validates raw string parameters from an HTTP query or equivalent.
// widthStr is required. dprStr and qualityStr are optional: empty string uses the default.
// Empty widthStr, whitespace-only values, leading sign characters, NaN, Inf, and
// overflow are all rejected.
func ParseRequest(widthStr, dprStr, qualityStr string) (Request, error) {
	width, err := parseWidth(widthStr)
	if err != nil {
		return Request{}, err
	}

	dpr, err := parseDPR(dprStr)
	if err != nil {
		return Request{}, err
	}

	quality, err := parseQuality(qualityStr)
	if err != nil {
		return Request{}, err
	}

	// Round width × dpr to the nearest integer pixel count.
	// math.Round gives unambiguous rounding for fractional DPR values (e.g. 1.5).
	// Width ≤ 2048 and DPR ≤ 3, so px ≤ 6144 — no overflow before conversion.
	px := math.Round(float64(width) * dpr)
	if px < 1 || px > float64(maxRequestedPixels) {
		return Request{}, &ErrInvalidRequest{
			Field: "dpr",
			Msg:   fmt.Sprintf("computed pixel width %.0f exceeds ceiling %d", px, maxRequestedPixels),
		}
	}

	return Request{
		Width:           width,
		DPR:             dpr,
		Quality:         quality,
		RequestedPixels: int(px),
	}, nil
}

// Dims holds the resolved output dimensions in pixels.
type Dims struct {
	Width  int
	Height int
}

// ResolveDims computes the output dimensions from actual source dimensions.
// It preserves aspect ratio, never upscales, and applies the maxRequestedPixels ceiling.
func ResolveDims(srcWidth, srcHeight, requestedPixels int) Dims {
	outWidth := requestedPixels
	if outWidth > srcWidth {
		outWidth = srcWidth // never upscale
	}
	if outWidth > maxRequestedPixels {
		outWidth = maxRequestedPixels // enforce output ceiling
	}
	if outWidth < 1 {
		outWidth = 1
	}

	// Preserve aspect ratio with nearest-integer rounding to minimise accumulated error.
	outHeight := int(math.Round(float64(outWidth) * float64(srcHeight) / float64(srcWidth)))
	if outHeight < 1 {
		outHeight = 1
	}

	return Dims{Width: outWidth, Height: outHeight}
}

func parseWidth(s string) (int, error) {
	if s == "" {
		return 0, &ErrInvalidRequest{Field: "width", Msg: "is required"}
	}
	if err := rejectBlank("width", s); err != nil {
		return 0, err
	}
	if err := rejectSign("width", s); err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, &ErrInvalidRequest{Field: "width", Msg: "must be a base-10 integer"}
	}
	if n < minWidth || n > maxWidth {
		return 0, &ErrInvalidRequest{
			Field: "width",
			Msg:   fmt.Sprintf("must be in [%d, %d]", minWidth, maxWidth),
		}
	}
	return n, nil
}

func parseDPR(s string) (float64, error) {
	if s == "" {
		return defaultDPR, nil
	}
	if err := rejectBlank("dpr", s); err != nil {
		return 0, err
	}
	if err := rejectSign("dpr", s); err != nil {
		return 0, err
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, &ErrInvalidRequest{Field: "dpr", Msg: "must be a finite decimal number"}
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, &ErrInvalidRequest{Field: "dpr", Msg: "must be finite"}
	}
	if f < minDPR || f > maxDPR {
		return 0, &ErrInvalidRequest{
			Field: "dpr",
			Msg:   fmt.Sprintf("must be in [%.0f, %.0f]", minDPR, maxDPR),
		}
	}
	return f, nil
}

func parseQuality(s string) (int, error) {
	if s == "" {
		return defaultQuality, nil
	}
	if err := rejectBlank("quality", s); err != nil {
		return 0, err
	}
	if err := rejectSign("quality", s); err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, &ErrInvalidRequest{Field: "quality", Msg: "must be a base-10 integer"}
	}
	if n < minQuality || n > maxQuality {
		return 0, &ErrInvalidRequest{
			Field: "quality",
			Msg:   fmt.Sprintf("must be in [%d, %d]", minQuality, maxQuality),
		}
	}
	return n, nil
}

// rejectBlank returns an error when s is non-empty but consists entirely of whitespace.
func rejectBlank(field, s string) error {
	if s != "" && strings.TrimSpace(s) == "" {
		return &ErrInvalidRequest{Field: field, Msg: "must not be whitespace-only"}
	}
	return nil
}

// rejectSign returns an error when s begins with + or -, which we treat as ambiguous.
func rejectSign(field, s string) error {
	if len(s) > 0 && (s[0] == '+' || s[0] == '-') {
		return &ErrInvalidRequest{Field: field, Msg: "must not begin with a sign character"}
	}
	return nil
}

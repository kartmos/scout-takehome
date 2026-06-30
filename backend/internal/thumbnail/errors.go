package thumbnail

import "fmt"

// ErrInvalidRequest indicates a malformed or out-of-range thumbnail request parameter.
type ErrInvalidRequest struct {
	Field string
	Msg   string
}

func (e *ErrInvalidRequest) Error() string {
	return fmt.Sprintf("thumbnail: invalid request field %q: %s", e.Field, e.Msg)
}

// ErrNotFound indicates the photo original does not exist in storage.
type ErrNotFound struct {
	Cause error
}

func (e *ErrNotFound) Error() string { return "thumbnail: original not found" }
func (e *ErrNotFound) Unwrap() error { return e.Cause }

// ErrUnsupportedImage indicates the source is not a valid JPEG or its dimensions violate limits.
type ErrUnsupportedImage struct {
	Msg   string
	Cause error
}

func (e *ErrUnsupportedImage) Error() string {
	if e.Msg != "" {
		return "thumbnail: unsupported image: " + e.Msg
	}
	return "thumbnail: unsupported image"
}
func (e *ErrUnsupportedImage) Unwrap() error { return e.Cause }

// ErrGeneration wraps unexpected internal failures during generation or storage access.
type ErrGeneration struct {
	Cause error
}

func (e *ErrGeneration) Error() string { return "thumbnail: generation failed" }
func (e *ErrGeneration) Unwrap() error { return e.Cause }

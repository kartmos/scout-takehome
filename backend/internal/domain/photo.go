package domain

import (
	"fmt"
	"math"
	"time"
)

// GreenhouseSize is the side length of the greenhouse plane in metres.
const GreenhouseSize = 40.0

// ClassID names a pest/disease detection class.
type ClassID string

const (
	ClassPowderyMildew ClassID = "powdery_mildew"
	ClassMirid         ClassID = "mirid"
	ClassWhiteflyAphid ClassID = "whitefly_aphid"
	ClassMinerTuta     ClassID = "miner_tuta"
	ClassThrips        ClassID = "thrips"
	ClassSpiderMites   ClassID = "spider_mites"
)

var knownClasses = [6]ClassID{
	ClassPowderyMildew, ClassMirid, ClassWhiteflyAphid,
	ClassMinerTuta, ClassThrips, ClassSpiderMites,
}

// IsKnownClassID reports whether c is one of the six documented pest/disease classes.
func IsKnownClassID(c ClassID) bool {
	for _, k := range knownClasses {
		if c == k {
			return true
		}
	}
	return false
}

// KnownClassIDs returns a fresh slice of all known class IDs.
func KnownClassIDs() []ClassID {
	out := make([]ClassID, len(knownClasses))
	copy(out, knownClasses[:])
	return out
}

// FieldError describes a single field-level validation failure.
type FieldError struct {
	Field string
	Issue string
}

func (e *FieldError) Error() string { return e.Field + ": " + e.Issue }

// BoundingBox holds normalized [0,1] corner coordinates (top-left to bottom-right).
type BoundingBox struct {
	XMin, YMin, XMax, YMax float64
}

// Prediction is a single pest/disease detection on a photo.
type Prediction struct {
	ClassID     ClassID
	Confidence  float64
	BoundingBox BoundingBox
}

// Photo is the persisted domain model. OriginalURL is not included; it is derived from object storage.
type Photo struct {
	ID          string
	X, Y, H     float64
	Width       int
	Height      int
	CapturedAt  time.Time
	Predictions []Prediction
}

// PhotoPage is a cursor-paginated slice of photos returned by the repository.
type PhotoPage struct {
	Items     []Photo
	NextToken string
}

// IsValidUUID reports whether s is a canonical hyphenated UUID (8-4-4-4-12), case-insensitive.
func IsValidUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < 36; i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !isHexByte(c) {
				return false
			}
		}
	}
	return true
}

func isHexByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// ValidateBoundingBox validates that all coordinates are finite, in [0,1],
// and that XMin < XMax and YMin < YMax.
func ValidateBoundingBox(b BoundingBox) []*FieldError {
	var errs []*FieldError
	coords := []struct {
		name string
		val  float64
	}{
		{"bbox.xMin", b.XMin}, {"bbox.yMin", b.YMin},
		{"bbox.xMax", b.XMax}, {"bbox.yMax", b.YMax},
	}
	for _, c := range coords {
		if !isFinite(c.val) {
			errs = append(errs, &FieldError{c.name, "must be a finite number"})
		} else if c.val < 0 || c.val > 1 {
			errs = append(errs, &FieldError{c.name, "must be in [0,1]"})
		}
	}
	// Check strict ordering only when individual coordinates are all valid.
	if len(errs) == 0 {
		if b.XMin >= b.XMax {
			errs = append(errs, &FieldError{"bbox.xMin", "must be less than xMax"})
		}
		if b.YMin >= b.YMax {
			errs = append(errs, &FieldError{"bbox.yMin", "must be less than yMax"})
		}
	}
	return errs
}

// ValidateConfidence reports a FieldError when f is not a finite number in [0,1].
func ValidateConfidence(f float64) *FieldError {
	if !isFinite(f) {
		return &FieldError{"confidence", "must be a finite number"}
	}
	if f < 0 || f > 1 {
		return &FieldError{"confidence", "must be in [0,1]"}
	}
	return nil
}

// ValidatePrediction validates a single detection prediction.
func ValidatePrediction(p Prediction) []*FieldError {
	var errs []*FieldError
	if !IsKnownClassID(p.ClassID) {
		errs = append(errs, &FieldError{"classId", fmt.Sprintf("unknown class %q", string(p.ClassID))})
	}
	if e := ValidateConfidence(p.Confidence); e != nil {
		errs = append(errs, e)
	}
	errs = append(errs, ValidateBoundingBox(p.BoundingBox)...)
	return errs
}

// ValidatePhoto validates the persisted metadata fields of a photo.
func ValidatePhoto(ph Photo) []*FieldError {
	var errs []*FieldError
	if !IsValidUUID(ph.ID) {
		errs = append(errs, &FieldError{"id", "must be a canonical UUID"})
	}
	for _, f := range []struct {
		name string
		val  float64
	}{
		{"x", ph.X}, {"y", ph.Y}, {"h", ph.H},
	} {
		if !isFinite(f.val) {
			errs = append(errs, &FieldError{f.name, "must be a finite number"})
			continue
		}
		switch f.name {
		case "x", "y":
			if f.val < 0 || f.val > GreenhouseSize {
				errs = append(errs, &FieldError{f.name, fmt.Sprintf("must be in [0,%g]", GreenhouseSize)})
			}
		case "h":
			if f.val < 0 {
				errs = append(errs, &FieldError{f.name, "must be non-negative"})
			}
		}
	}
	if ph.Width <= 0 {
		errs = append(errs, &FieldError{"width", "must be positive"})
	}
	if ph.Height <= 0 {
		errs = append(errs, &FieldError{"height", "must be positive"})
	}
	if ph.CapturedAt.IsZero() {
		errs = append(errs, &FieldError{"capturedAt", "must not be zero"})
	}
	return errs
}

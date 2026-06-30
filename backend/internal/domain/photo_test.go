package domain_test

import (
	"math"
	"testing"
	"time"

	"scout/internal/domain"
)

func TestIsKnownClassID_Known(t *testing.T) {
	known := []domain.ClassID{
		domain.ClassPowderyMildew,
		domain.ClassMirid,
		domain.ClassWhiteflyAphid,
		domain.ClassMinerTuta,
		domain.ClassThrips,
		domain.ClassSpiderMites,
	}
	for _, c := range known {
		if !domain.IsKnownClassID(c) {
			t.Errorf("IsKnownClassID(%q) = false, want true", c)
		}
	}
}

func TestIsKnownClassID_Unknown(t *testing.T) {
	unknown := []domain.ClassID{
		"",
		"THRIPS",
		"Thrips",
		"spider_mites_extra",
		"foo",
		"powdery mildew",
	}
	for _, c := range unknown {
		if domain.IsKnownClassID(c) {
			t.Errorf("IsKnownClassID(%q) = true, want false", c)
		}
	}
}

func TestKnownClassIDs_NoBacking(t *testing.T) {
	a := domain.KnownClassIDs()
	b := domain.KnownClassIDs()
	if len(a) != 6 {
		t.Fatalf("KnownClassIDs() length = %d, want 6", len(a))
	}
	a[0] = "mutated"
	if b[0] == "mutated" {
		t.Error("KnownClassIDs shares backing storage between calls")
	}
}

func TestValidateBoundingBox(t *testing.T) {
	nan := math.NaN()
	pinf := math.Inf(1)
	ninf := math.Inf(-1)

	tests := []struct {
		name  string
		bbox  domain.BoundingBox
		valid bool
	}{
		// Valid — inclusive boundaries
		{"mid-range", domain.BoundingBox{0.1, 0.2, 0.9, 0.8}, true},
		{"xMin=0 boundary", domain.BoundingBox{0, 0.1, 0.5, 0.9}, true},
		{"yMax=1 boundary", domain.BoundingBox{0.1, 0.1, 0.9, 1}, true},
		{"full extent 0-to-1", domain.BoundingBox{0, 0, 1, 1}, true},
		{"narrow box", domain.BoundingBox{0, 0, 0.001, 0.001}, true},
		{"nearly-full y", domain.BoundingBox{0.1, 0, 0.9, 1}, true},

		// NaN in each coordinate
		{"xMin NaN", domain.BoundingBox{nan, 0, 1, 1}, false},
		{"yMin NaN", domain.BoundingBox{0, nan, 1, 1}, false},
		{"xMax NaN", domain.BoundingBox{0, 0, nan, 1}, false},
		{"yMax NaN", domain.BoundingBox{0, 0, 1, nan}, false},

		// Infinity in each coordinate
		{"xMin +Inf", domain.BoundingBox{pinf, 0, 1, 1}, false},
		{"xMin -Inf", domain.BoundingBox{ninf, 0, 1, 1}, false},
		{"yMax +Inf", domain.BoundingBox{0, 0, 1, pinf}, false},

		// Out of range
		{"xMin < 0", domain.BoundingBox{-0.1, 0, 1, 1}, false},
		{"yMin < 0", domain.BoundingBox{0, -0.1, 1, 1}, false},
		{"xMax > 1", domain.BoundingBox{0, 0, 1.1, 1}, false},
		{"yMax > 1", domain.BoundingBox{0, 0, 1, 1.1}, false},

		// Reversed / equal corners
		{"xMin == xMax", domain.BoundingBox{0.5, 0, 0.5, 1}, false},
		{"xMin > xMax", domain.BoundingBox{0.8, 0, 0.2, 1}, false},
		{"yMin == yMax", domain.BoundingBox{0, 0.5, 1, 0.5}, false},
		{"yMin > yMax", domain.BoundingBox{0, 0.8, 1, 0.2}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := domain.ValidateBoundingBox(tt.bbox)
			if tt.valid && len(errs) > 0 {
				t.Errorf("expected valid, got %d error(s): %v", len(errs), errs)
			}
			if !tt.valid && len(errs) == 0 {
				t.Error("expected invalid, got no errors")
			}
		})
	}
}

func TestValidateConfidence(t *testing.T) {
	nan := math.NaN()
	pinf := math.Inf(1)
	ninf := math.Inf(-1)

	tests := []struct {
		name  string
		val   float64
		valid bool
	}{
		{"zero boundary", 0, true},
		{"one boundary", 1, true},
		{"mid value", 0.5, true},
		{"near zero", 0.0001, true},
		{"near one", 0.9999, true},

		{"below zero", -0.001, false},
		{"above one", 1.001, false},
		{"NaN", nan, false},
		{"+Inf", pinf, false},
		{"-Inf", ninf, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := domain.ValidateConfidence(tt.val)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			}
			if !tt.valid && err == nil {
				t.Error("expected invalid, got no error")
			}
		})
	}
}

func TestIsValidUUID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		// Valid canonical UUIDs
		{"lowercase", "550e8400-e29b-41d4-a716-446655440000", true},
		{"uppercase", "550E8400-E29B-41D4-A716-446655440000", true},
		{"mixed case", "550e8400-E29B-41d4-A716-446655440000", true},
		{"all zeros", "00000000-0000-0000-0000-000000000000", true},

		// Malformed
		{"empty string", "", false},
		{"too short by one", "550e8400-e29b-41d4-a716-44665544000", false},
		{"too long by one", "550e8400-e29b-41d4-a716-4466554400000", false},
		{"no dashes", "550e8400e29b41d4a716446655440000", false},
		{"wrong dash position (shifted)", "550e840-0e29b-41d4-a716-446655440000", false},
		{"non-hex char g", "550e8400-e29b-41d4-a716-44665544000g", false},
		{"non-hex char space", "550e8400-e29b-41d4-a716-44665544000 ", false},
		{"curly braces", "{550e8400-e29b-41d4-a716-446655440000}", false},
		{"missing segment", "550e8400-e29b-41d4-a716", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.IsValidUUID(tt.input)
			if got != tt.valid {
				t.Errorf("IsValidUUID(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

func validPhoto() domain.Photo {
	return domain.Photo{
		ID:         "550e8400-e29b-41d4-a716-446655440000",
		X:          10,
		Y:          20,
		H:          2.5,
		Width:      2560,
		Height:     1440,
		CapturedAt: time.Now(),
	}
}

func TestValidatePhoto_Valid(t *testing.T) {
	ph := validPhoto()
	if errs := domain.ValidatePhoto(ph); len(errs) != 0 {
		t.Errorf("expected valid, got %d error(s): %v", len(errs), errs)
	}
}

func TestValidatePhoto_GreenhouseBoundaries(t *testing.T) {
	tests := []struct {
		name  string
		photo domain.Photo
		valid bool
	}{
		// X boundaries
		{"x lower bound 0", func() domain.Photo { p := validPhoto(); p.X = 0; return p }(), true},
		{"x upper bound 40", func() domain.Photo { p := validPhoto(); p.X = 40; return p }(), true},
		{"x below 0", func() domain.Photo { p := validPhoto(); p.X = -0.1; return p }(), false},
		{"x above 40", func() domain.Photo { p := validPhoto(); p.X = 40.1; return p }(), false},

		// Y boundaries
		{"y lower bound 0", func() domain.Photo { p := validPhoto(); p.Y = 0; return p }(), true},
		{"y upper bound 40", func() domain.Photo { p := validPhoto(); p.Y = 40; return p }(), true},
		{"y below 0", func() domain.Photo { p := validPhoto(); p.Y = -0.1; return p }(), false},
		{"y above 40", func() domain.Photo { p := validPhoto(); p.Y = 40.1; return p }(), false},

		// H (non-negative)
		{"h = 0", func() domain.Photo { p := validPhoto(); p.H = 0; return p }(), true},
		{"h positive", func() domain.Photo { p := validPhoto(); p.H = 100; return p }(), true},
		{"h negative", func() domain.Photo { p := validPhoto(); p.H = -0.001; return p }(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := domain.ValidatePhoto(tt.photo)
			if tt.valid && len(errs) > 0 {
				t.Errorf("expected valid, got errors: %v", errs)
			}
			if !tt.valid && len(errs) == 0 {
				t.Error("expected invalid, got no errors")
			}
		})
	}
}

func TestValidatePhoto_InvalidMetadata(t *testing.T) {
	nan := math.NaN()
	pinf := math.Inf(1)

	tests := []struct {
		name  string
		photo domain.Photo
	}{
		{"bad UUID", func() domain.Photo { p := validPhoto(); p.ID = "not-a-uuid"; return p }()},
		{"width zero", func() domain.Photo { p := validPhoto(); p.Width = 0; return p }()},
		{"width negative", func() domain.Photo { p := validPhoto(); p.Width = -1; return p }()},
		{"height zero", func() domain.Photo { p := validPhoto(); p.Height = 0; return p }()},
		{"height negative", func() domain.Photo { p := validPhoto(); p.Height = -1; return p }()},
		{"capturedAt zero", func() domain.Photo { p := validPhoto(); p.CapturedAt = time.Time{}; return p }()},
		{"x NaN", func() domain.Photo { p := validPhoto(); p.X = nan; return p }()},
		{"y Inf", func() domain.Photo { p := validPhoto(); p.Y = pinf; return p }()},
		{"h NaN", func() domain.Photo { p := validPhoto(); p.H = nan; return p }()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := domain.ValidatePhoto(tt.photo)
			if len(errs) == 0 {
				t.Error("expected invalid, got no errors")
			}
		})
	}
}

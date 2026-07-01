package thumbnail_test

import (
	"testing"

	"scout/internal/thumbnail"
)

func TestParseRequest_Defaults(t *testing.T) {
	req, err := thumbnail.ParseRequest("100", "", "")
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.Width != 100 {
		t.Errorf("Width = %d, want 100", req.Width)
	}
	if req.DPR != 1.0 {
		t.Errorf("DPR = %v, want 1.0", req.DPR)
	}
	if req.Quality != 80 {
		t.Errorf("Quality = %d, want 80", req.Quality)
	}
	if req.RequestedPixels != 100 {
		t.Errorf("RequestedPixels = %d, want 100", req.RequestedPixels)
	}
}

func TestParseRequest_Boundaries(t *testing.T) {
	cases := []struct {
		name    string
		width   string
		dpr     string
		quality string
		wantErr bool
	}{
		{"min width", "1", "", "", false},
		{"max width", "2048", "", "", false},
		{"below min width", "0", "", "", true},
		{"above max width", "2049", "", "", true},
		{"min dpr", "100", "1", "", false},
		{"max dpr", "100", "3", "", false},
		{"below min dpr", "100", "0.9", "", true},
		{"above max dpr", "100", "3.1", "", true},
		{"min quality", "100", "", "1", false},
		{"max quality", "100", "", "100", false},
		{"below min quality", "100", "", "0", true},
		{"above max quality", "100", "", "101", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := thumbnail.ParseRequest(tc.width, tc.dpr, tc.quality)
			if tc.wantErr && err == nil {
				t.Errorf("ParseRequest(%q, %q, %q) = nil; want error", tc.width, tc.dpr, tc.quality)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ParseRequest(%q, %q, %q) = %v; want nil", tc.width, tc.dpr, tc.quality, err)
			}
		})
	}
}

func TestParseRequest_MalformedInputs(t *testing.T) {
	cases := []struct {
		name    string
		width   string
		dpr     string
		quality string
	}{
		{"empty width", "", "", ""},
		{"whitespace width", "  ", "", ""},
		{"sign width", "+100", "", ""},
		{"negative width", "-1", "", ""},
		{"float width", "1.5", "", ""},
		{"whitespace dpr", "100", "  ", ""},
		{"sign dpr", "+1.5", "", ""},
		{"whitespace quality", "100", "", "  "},
		{"sign quality", "100", "", "+80"},
		{"nan dpr", "100", "NaN", ""},
		{"inf dpr", "100", "Inf", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := thumbnail.ParseRequest(tc.width, tc.dpr, tc.quality)
			if err == nil {
				t.Errorf("ParseRequest(%q, %q, %q) = nil; want error for malformed input",
					tc.width, tc.dpr, tc.quality)
			}
		})
	}
}

func TestParseRequest_DeterministicRounding(t *testing.T) {
	// width=100 DPR=1.5 → px = round(100 × 1.5) = 150.
	req, err := thumbnail.ParseRequest("100", "1.5", "")
	if err != nil {
		t.Fatalf("ParseRequest: %v", err)
	}
	if req.RequestedPixels != 150 {
		t.Errorf("RequestedPixels = %d, want 150 for width=100 dpr=1.5", req.RequestedPixels)
	}
}

func TestParseRequest_EquivalentCanonicalKeys(t *testing.T) {
	// width=200 dpr=1.0 should produce the same RequestedPixels as width=100 dpr=2.0
	// only when the source is large enough. Here we just verify RequestedPixels.
	a, err := thumbnail.ParseRequest("200", "1.0", "80")
	if err != nil {
		t.Fatalf("a: %v", err)
	}
	b, err := thumbnail.ParseRequest("100", "2.0", "80")
	if err != nil {
		t.Fatalf("b: %v", err)
	}
	if a.RequestedPixels != b.RequestedPixels {
		t.Errorf("RequestedPixels: a=%d b=%d; should be equal", a.RequestedPixels, b.RequestedPixels)
	}
}

func TestResolveDims_NoUpscale(t *testing.T) {
	// Source 200×100, requested 400px → should stay 200×100 (no upscale).
	dims := thumbnail.ResolveDims(200, 100, 400)
	if dims.Width != 200 || dims.Height != 100 {
		t.Errorf("dims = %+v, want {200, 100} (no upscale)", dims)
	}
}

func TestResolveDims_AspectRatio(t *testing.T) {
	// 1920×1080 @ 960px wide → height should be 540.
	dims := thumbnail.ResolveDims(1920, 1080, 960)
	if dims.Width != 960 {
		t.Errorf("Width = %d, want 960", dims.Width)
	}
	if dims.Height != 540 {
		t.Errorf("Height = %d, want 540", dims.Height)
	}
}

func TestResolveDims_OutputPixelCeiling(t *testing.T) {
	// Source is large, requestedPixels exceeds ceiling.
	dims := thumbnail.ResolveDims(10000, 5000, 9999)
	if dims.Width > 6144 {
		t.Errorf("Width = %d, must not exceed output pixel ceiling 6144", dims.Width)
	}
}

func TestResolveDims_DirectPath(t *testing.T) {
	// When requestedPixels equals source width, output should equal source.
	dims := thumbnail.ResolveDims(800, 600, 800)
	if dims.Width != 800 || dims.Height != 600 {
		t.Errorf("dims = %+v, want {800, 600} for direct path", dims)
	}
}

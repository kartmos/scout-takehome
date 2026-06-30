package sqlite

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"scout/internal/apperror"
)

func TestEncodeDecode_roundtrip(t *testing.T) {
	rawTS := "2024-03-15T10:30:00Z"
	id := "123e4567-e89b-12d3-a456-426614174000"

	token := encodeCursor(rawTS, id)
	if token == "" {
		t.Fatal("encodeCursor returned empty string")
	}

	gotRaw, gotID, err := decodeCursor(token)
	if err != nil {
		t.Fatalf("decodeCursor error: %v", err)
	}
	if gotRaw != rawTS {
		t.Errorf("capturedAtRaw: got %q, want %q", gotRaw, rawTS)
	}
	if gotID != id {
		t.Errorf("id: got %q, want %q", gotID, id)
	}
}

func TestEncodeDecode_nanoseconds(t *testing.T) {
	rawTS := "2024-03-15T10:30:00.123456789Z"
	id := "123e4567-e89b-12d3-a456-426614174000"

	token := encodeCursor(rawTS, id)
	gotRaw, _, err := decodeCursor(token)
	if err != nil {
		t.Fatalf("decodeCursor error: %v", err)
	}
	if gotRaw != rawTS {
		t.Errorf("nanosecond precision: got %q, want %q", gotRaw, rawTS)
	}
}

func TestEncodeDecode_nonUTCOffset(t *testing.T) {
	// Encode a non-UTC timestamp and verify the raw string is preserved exactly.
	rawTS := "2024-03-15T15:30:00+05:30"
	id := "123e4567-e89b-12d3-a456-426614174000"

	token := encodeCursor(rawTS, id)
	gotRaw, _, err := decodeCursor(token)
	if err != nil {
		t.Fatalf("decodeCursor error: %v", err)
	}
	if gotRaw != rawTS {
		t.Errorf("non-UTC offset not preserved: got %q, want %q", gotRaw, rawTS)
	}
}

var malformedCursorCases = []struct {
	name  string
	token string
	issue string
}{
	{
		name:  "invalid base64",
		token: "!!!notbase64!!!",
		issue: "invalid base64",
	},
	{
		name:  "malformed JSON",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{not json`)),
		issue: "malformed JSON",
	},
	{
		name:  "trailing non-whitespace text",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"ca":"2024-01-01T00:00:00Z","id":"123e4567-e89b-12d3-a456-426614174000"}extra`)),
		issue: "trailing content",
	},
	{
		name:  "second JSON object",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"ca":"2024-01-01T00:00:00Z","id":"123e4567-e89b-12d3-a456-426614174000"}{"another":"object"}`)),
		issue: "trailing content",
	},
	{
		name:  "unsupported version",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":99,"ca":"2024-01-01T00:00:00Z","id":"123e4567-e89b-12d3-a456-426614174000"}`)),
		issue: "unsupported cursor version",
	},
	{
		name:  "missing capturedAt",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"id":"123e4567-e89b-12d3-a456-426614174000"}`)),
		issue: "missing capturedAt",
	},
	{
		name:  "missing id",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"ca":"2024-01-01T00:00:00Z"}`)),
		issue: "missing id",
	},
	{
		name:  "malformed timestamp",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"ca":"not-a-timestamp","id":"123e4567-e89b-12d3-a456-426614174000"}`)),
		issue: "malformed capturedAt",
	},
	{
		name:  "malformed UUID",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"ca":"2024-01-01T00:00:00Z","id":"not-a-uuid"}`)),
		issue: "malformed id UUID",
	},
}

func TestDecodeCursor_errors(t *testing.T) {
	for _, tc := range malformedCursorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := decodeCursor(tc.token)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var ae *apperror.AppError
			if !errors.As(err, &ae) {
				t.Fatalf("expected *apperror.AppError, got %T", err)
			}
			if ae.Kind() != apperror.KindValidation {
				t.Errorf("expected KindValidation, got %v", ae.Kind())
			}
			found := false
			for _, v := range ae.Violations() {
				if strings.Contains(v.Issue, tc.issue) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected violation issue containing %q, got %v", tc.issue, ae.Violations())
			}
		})
	}
}

func TestDecodeCursor_trailingWhitespace(t *testing.T) {
	// Trailing whitespace after the JSON object must be accepted.
	raw := `{"v":1,"ca":"2024-01-01T00:00:00Z","id":"123e4567-e89b-12d3-a456-426614174000"}   ` + "\n\t"
	token := base64.RawURLEncoding.EncodeToString([]byte(raw))
	_, _, err := decodeCursor(token)
	if err != nil {
		t.Fatalf("expected trailing whitespace to be accepted, got: %v", err)
	}
}

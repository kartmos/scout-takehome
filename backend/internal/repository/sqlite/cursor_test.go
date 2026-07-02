package sqlite

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"scout/internal/apperror"
)

func TestEncodeDecode_roundtrip(t *testing.T) {
	id := "123e4567-e89b-12d3-a456-426614174000"

	token := encodeCursor(id)
	if token == "" {
		t.Fatal("encodeCursor returned empty string")
	}

	gotID, err := decodeCursor(token)
	if err != nil {
		t.Fatalf("decodeCursor error: %v", err)
	}
	if gotID != id {
		t.Errorf("id: got %q, want %q", gotID, id)
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
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":2,"id":"123e4567-e89b-12d3-a456-426614174000"}extra`)),
		issue: "trailing content",
	},
	{
		name:  "second JSON object",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":2,"id":"123e4567-e89b-12d3-a456-426614174000"}{"another":"object"}`)),
		issue: "trailing content",
	},
	{
		name:  "unsupported version",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":99,"id":"123e4567-e89b-12d3-a456-426614174000"}`)),
		issue: "unsupported cursor version",
	},
	{
		// v1 cursors carry "ca" (unknown field in v2) → rejected as malformed JSON by DisallowUnknownFields
		name:  "old v1 cursor rejected",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"ca":"2024-01-01T00:00:00Z","id":"123e4567-e89b-12d3-a456-426614174000"}`)),
		issue: "malformed JSON",
	},
	{
		name:  "missing id",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":2}`)),
		issue: "missing id",
	},
	{
		name:  "malformed UUID",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":2,"id":"not-a-uuid"}`)),
		issue: "malformed id UUID",
	},
}

func TestDecodeCursor_errors(t *testing.T) {
	for _, tc := range malformedCursorCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeCursor(tc.token)
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
	raw := `{"v":2,"id":"123e4567-e89b-12d3-a456-426614174000"}   ` + "\n\t"
	token := base64.RawURLEncoding.EncodeToString([]byte(raw))
	_, err := decodeCursor(token)
	if err != nil {
		t.Fatalf("expected trailing whitespace to be accepted, got: %v", err)
	}
}

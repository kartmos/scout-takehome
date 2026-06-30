package sqlite

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"scout/internal/apperror"
)

func TestEncodeDecode_roundtrip(t *testing.T) {
	at := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	id := "123e4567-e89b-12d3-a456-426614174000"

	token := encodeCursor(at, id)
	if token == "" {
		t.Fatal("encodeCursor returned empty string")
	}

	gotAt, gotID, err := decodeCursor(token)
	if err != nil {
		t.Fatalf("decodeCursor error: %v", err)
	}
	if !gotAt.Equal(at) {
		t.Errorf("capturedAt: got %v, want %v", gotAt, at)
	}
	if gotID != id {
		t.Errorf("id: got %q, want %q", gotID, id)
	}
}

func TestEncodeDecode_nanoseconds(t *testing.T) {
	at := time.Date(2024, 3, 15, 10, 30, 0, 123456789, time.UTC)
	id := "123e4567-e89b-12d3-a456-426614174000"

	token := encodeCursor(at, id)
	gotAt, _, err := decodeCursor(token)
	if err != nil {
		t.Fatalf("decodeCursor error: %v", err)
	}
	if !gotAt.Equal(at) {
		t.Errorf("nanosecond precision lost: got %v, want %v", gotAt, at)
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
		name:  "trailing content",
		token: base64.RawURLEncoding.EncodeToString([]byte(`{"v":1,"ca":"2024-01-01T00:00:00Z","id":"123e4567-e89b-12d3-a456-426614174000"}extra`)),
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

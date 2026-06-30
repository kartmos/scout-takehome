package sqlite

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"scout/internal/apperror"
	"scout/internal/domain"
)

const cursorVersion = 1

type cursorPayload struct {
	Version    int    `json:"v"`
	CapturedAt string `json:"ca"`
	ID         string `json:"id"`
}

// encodeCursor builds an opaque pagination token from the exact raw captured_at
// string scanned from SQLite and the photo ID. The string is stored verbatim so
// that decodeCursor can return it unchanged for direct SQL boundary comparison.
func encodeCursor(capturedAtRaw string, id string) string {
	p := cursorPayload{
		Version:    cursorVersion,
		CapturedAt: capturedAtRaw,
		ID:         id,
	}
	b, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(b)
}

// decodeCursor unpacks a pagination token and returns the validated raw
// captured_at string (for direct SQL comparison without UTC normalisation)
// and the photo ID. The timestamp is validated as RFC 3339 / RFC 3339 Nano
// but is returned as the original string, not converted to time.Time.
func decodeCursor(token string) (capturedAtRaw string, id string, err error) {
	raw, decErr := base64.RawURLEncoding.DecodeString(token)
	if decErr != nil {
		return "", "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "invalid base64"},
		})
	}

	var p cursorPayload
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if decErr = dec.Decode(&p); decErr != nil {
		return "", "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "malformed JSON"},
		})
	}
	// Require exactly one JSON value: a second decode must return io.EOF.
	// This rejects a second valid JSON value and trailing non-whitespace.
	// Trailing whitespace is fine because the decoder skips it and returns io.EOF.
	var extra json.RawMessage
	if decErr = dec.Decode(&extra); decErr != io.EOF {
		return "", "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "trailing content after JSON"},
		})
	}

	if p.Version != cursorVersion {
		return "", "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: fmt.Sprintf("unsupported cursor version %d", p.Version)},
		})
	}
	if p.CapturedAt == "" {
		return "", "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "missing capturedAt"},
		})
	}
	if p.ID == "" {
		return "", "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "missing id"},
		})
	}

	// Validate timestamp is RFC 3339 without normalising it.
	_, parseErr := time.Parse(time.RFC3339Nano, p.CapturedAt)
	if parseErr != nil {
		_, parseErr = time.Parse(time.RFC3339, p.CapturedAt)
		if parseErr != nil {
			return "", "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
				{Field: "cursor", Issue: "malformed capturedAt timestamp"},
			})
		}
	}

	if !domain.IsValidUUID(p.ID) {
		return "", "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "malformed id UUID"},
		})
	}

	return p.CapturedAt, p.ID, nil
}

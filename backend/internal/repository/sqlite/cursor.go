package sqlite

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

func encodeCursor(capturedAt time.Time, id string) string {
	p := cursorPayload{
		Version:    cursorVersion,
		CapturedAt: capturedAt.UTC().Format(time.RFC3339Nano),
		ID:         id,
	}
	b, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(token string) (capturedAt time.Time, id string, err error) {
	raw, decErr := base64.RawURLEncoding.DecodeString(token)
	if decErr != nil {
		return time.Time{}, "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "invalid base64"},
		})
	}

	var p cursorPayload
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if decErr = dec.Decode(&p); decErr != nil {
		return time.Time{}, "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "malformed JSON"},
		})
	}
	// Reject trailing content after the JSON object.
	if dec.More() {
		return time.Time{}, "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "trailing content after JSON"},
		})
	}

	if p.Version != cursorVersion {
		return time.Time{}, "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: fmt.Sprintf("unsupported cursor version %d", p.Version)},
		})
	}
	if p.CapturedAt == "" {
		return time.Time{}, "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "missing capturedAt"},
		})
	}
	if p.ID == "" {
		return time.Time{}, "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "missing id"},
		})
	}

	t, parseErr := time.Parse(time.RFC3339Nano, p.CapturedAt)
	if parseErr != nil {
		// Also try plain RFC3339.
		t, parseErr = time.Parse(time.RFC3339, p.CapturedAt)
		if parseErr != nil {
			return time.Time{}, "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
				{Field: "cursor", Issue: "malformed capturedAt timestamp"},
			})
		}
	}

	if !domain.IsValidUUID(p.ID) {
		return time.Time{}, "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "malformed id UUID"},
		})
	}

	return t, p.ID, nil
}

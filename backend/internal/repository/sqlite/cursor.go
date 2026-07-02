package sqlite

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"scout/internal/apperror"
	"scout/internal/domain"
)

// cursorVersion is incremented when the payload shape changes.
// Clients holding a v1 cursor receive a validation error and must restart pagination.
const cursorVersion = 2

type cursorPayload struct {
	Version int    `json:"v"`
	ID      string `json:"id"`
}

// encodeCursor builds an opaque pagination token from the photo ID.
func encodeCursor(id string) string {
	p := cursorPayload{Version: cursorVersion, ID: id}
	b, _ := json.Marshal(p)
	return base64.RawURLEncoding.EncodeToString(b)
}

// decodeCursor unpacks a pagination token and returns the validated photo ID.
func decodeCursor(token string) (id string, err error) {
	raw, decErr := base64.RawURLEncoding.DecodeString(token)
	if decErr != nil {
		return "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "invalid base64"},
		})
	}

	var p cursorPayload
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if decErr = dec.Decode(&p); decErr != nil {
		return "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "malformed JSON"},
		})
	}
	// Require exactly one JSON value.
	var extra json.RawMessage
	if decErr = dec.Decode(&extra); decErr != io.EOF {
		return "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "trailing content after JSON"},
		})
	}

	if p.Version != cursorVersion {
		return "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: fmt.Sprintf("unsupported cursor version %d", p.Version)},
		})
	}
	if p.ID == "" {
		return "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "missing id"},
		})
	}
	if !domain.IsValidUUID(p.ID) {
		return "", apperror.NewValidation("invalid cursor", []apperror.FieldViolation{
			{Field: "cursor", Issue: "malformed id UUID"},
		})
	}
	return p.ID, nil
}

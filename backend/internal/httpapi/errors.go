package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"scout/internal/apperror"
)

// apiErrorBody is the base for all error responses (request_id, message, code).
type apiErrorBody struct {
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
	Code      string `json:"code"`
}

type validationDetail struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

type validationErrorBody struct {
	RequestID string             `json:"request_id"`
	Message   string             `json:"message"`
	Code      string             `json:"code"`
	Details   []validationDetail `json:"details"`
}

type notFoundErrorBody struct {
	RequestID  string `json:"request_id"`
	Message    string `json:"message"`
	Code       string `json:"code"`
	ResourceID string `json:"resource_id"`
}

type methodNotAllowedErrorBody struct {
	RequestID string `json:"request_id"`
	Message   string `json:"message"`
	Code      string `json:"code"`
	Allowed   string `json:"allowed"`
}

// WriteError maps err to an OpenAPI-compatible JSON response. It sets
// Content-Type and Cache-Control, marshals the body, then writes the status.
// Internal and unknown errors are logged once with the retained cause.
// Client-facing errors are not logged here.
func WriteError(w http.ResponseWriter, r *http.Request, logger *slog.Logger, err error) {
	reqID := RequestIDFromContext(r.Context())
	if reqID == "" {
		reqID = generateRequestID()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")

	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		logger.Error("unhandled error", "request_id", reqID, "error", err)
		writeErrorJSON(w, http.StatusInternalServerError, apiErrorBody{
			RequestID: reqID,
			Message:   "an internal error occurred",
			Code:      "InternalServerError",
		})
		return
	}

	switch appErr.Kind() {
	case apperror.KindValidation:
		raw := appErr.Violations()
		details := make([]validationDetail, len(raw))
		for i, v := range raw {
			details[i] = validationDetail{Field: v.Field, Issue: v.Issue}
		}
		writeErrorJSON(w, http.StatusBadRequest, validationErrorBody{
			RequestID: reqID,
			Message:   appErr.Error(),
			Code:      "ValidationError",
			Details:   details,
		})

	case apperror.KindAuth:
		writeErrorJSON(w, http.StatusUnauthorized, apiErrorBody{
			RequestID: reqID,
			Message:   appErr.Error(),
			Code:      "AuthenticationRequired",
		})

	case apperror.KindNotFound:
		writeErrorJSON(w, http.StatusNotFound, notFoundErrorBody{
			RequestID:  reqID,
			Message:    appErr.Error(),
			Code:       "NotFound",
			ResourceID: appErr.ResourceID(),
		})

	case apperror.KindMethodNotAllowed:
		w.Header().Set("Allow", appErr.Allowed())
		writeErrorJSON(w, http.StatusMethodNotAllowed, methodNotAllowedErrorBody{
			RequestID: reqID,
			Message:   appErr.Error(),
			Code:      "MethodNotAllowed",
			Allowed:   appErr.Allowed(),
		})

	default:
		// KindInternal — log the retained cause but never expose it.
		if cause := appErr.Unwrap(); cause != nil {
			logger.Error("internal error", "request_id", reqID, "error", cause)
		} else {
			logger.Error("internal error", "request_id", reqID)
		}
		writeErrorJSON(w, http.StatusInternalServerError, apiErrorBody{
			RequestID: reqID,
			Message:   appErr.Error(),
			Code:      "InternalServerError",
		})
	}
}

// writeErrorJSON marshals v completely before calling WriteHeader so headers
// are still writable if marshaling fails.
func writeErrorJSON(w http.ResponseWriter, status int, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

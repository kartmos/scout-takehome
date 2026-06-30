package apperror

// Kind identifies the category of an application error.
type Kind int

const (
	KindValidation Kind = iota + 1
	KindAuth
	KindNotFound
	KindInternal
)

// FieldViolation describes a single field-level validation issue.
type FieldViolation struct {
	Field string
	Issue string
}

// AppError is a typed application error with a safe public message.
// Error() returns only the safe message and never leaks the wrapped internal cause.
type AppError struct {
	kind       Kind
	message    string
	violations []FieldViolation
	resourceID string
	cause      error
}

func (e *AppError) Error() string { return e.message }

// Unwrap exposes the wrapped cause for errors.Is chains; only non-nil for internal errors.
func (e *AppError) Unwrap() error {
	if e.kind == KindInternal {
		return e.cause
	}
	return nil
}

func (e *AppError) Kind() Kind         { return e.kind }
func (e *AppError) ResourceID() string { return e.resourceID }

// Violations returns a copy of the field violations (non-nil only for validation errors).
func (e *AppError) Violations() []FieldViolation {
	if len(e.violations) == 0 {
		return nil
	}
	out := make([]FieldViolation, len(e.violations))
	copy(out, e.violations)
	return out
}

const (
	fallbackValidationMsg = "request validation failed"
	fallbackAuthMsg       = "authentication required"
	fallbackNotFoundMsg   = "resource not found"
	fallbackResourceID    = "unknown"
	internalMsg           = "an internal error occurred"
)

// NewValidation constructs a validation error.
// msg defaults to a fallback if empty; violations are defensively copied.
// If violations is empty/nil, a generic violation is substituted.
func NewValidation(msg string, violations []FieldViolation) *AppError {
	if msg == "" {
		msg = fallbackValidationMsg
	}
	v := make([]FieldViolation, len(violations))
	copy(v, violations)
	if len(v) == 0 {
		v = []FieldViolation{{Field: "request", Issue: "invalid"}}
	}
	return &AppError{kind: KindValidation, message: msg, violations: v}
}

// NewAuth constructs an authentication-required error.
func NewAuth(msg string) *AppError {
	if msg == "" {
		msg = fallbackAuthMsg
	}
	return &AppError{kind: KindAuth, message: msg}
}

// NewNotFound constructs a not-found error.
// resourceID defaults to "unknown" if empty.
func NewNotFound(msg string, resourceID string) *AppError {
	if msg == "" {
		msg = fallbackNotFoundMsg
	}
	if resourceID == "" {
		resourceID = fallbackResourceID
	}
	return &AppError{kind: KindNotFound, message: msg, resourceID: resourceID}
}

// NewInternal constructs an internal error wrapping cause (which may be nil).
// Error() always returns a stable generic public message, never the cause text.
func NewInternal(cause error) *AppError {
	return &AppError{kind: KindInternal, message: internalMsg, cause: cause}
}

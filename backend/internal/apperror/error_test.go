package apperror_test

import (
	"errors"
	"strings"
	"testing"

	"scout/internal/apperror"
)

// ---- NewValidation ----

func TestNewValidation_Basic(t *testing.T) {
	v := []apperror.FieldViolation{{Field: "email", Issue: "required"}}
	err := apperror.NewValidation("invalid request", v)

	if err.Error() != "invalid request" {
		t.Errorf("Error() = %q, want %q", err.Error(), "invalid request")
	}
	if err.Kind() != apperror.KindValidation {
		t.Errorf("Kind() = %v, want KindValidation", err.Kind())
	}
	vv := err.Violations()
	if len(vv) != 1 || vv[0].Field != "email" || vv[0].Issue != "required" {
		t.Errorf("unexpected violations: %v", vv)
	}
}

func TestNewValidation_DefensiveCopy(t *testing.T) {
	v := []apperror.FieldViolation{{Field: "email", Issue: "required"}}
	err := apperror.NewValidation("invalid", v)

	// Mutate original; stored violations must not change.
	v[0].Field = "mutated"
	if err.Violations()[0].Field == "mutated" {
		t.Error("NewValidation did not defensively copy violation slice")
	}
}

func TestNewValidation_ViolationsReturnCopy(t *testing.T) {
	v := []apperror.FieldViolation{{Field: "email", Issue: "required"}}
	err := apperror.NewValidation("invalid", v)

	// Mutate the returned slice; re-fetching must still return original data.
	got := err.Violations()
	got[0].Field = "mutated"
	if err.Violations()[0].Field == "mutated" {
		t.Error("Violations() returned a mutable reference to internal storage")
	}
}

func TestNewValidation_EmptyMsg_Fallback(t *testing.T) {
	err := apperror.NewValidation("", []apperror.FieldViolation{{Field: "f", Issue: "i"}})
	if err.Error() == "" {
		t.Error("Error() should not be empty when empty msg was passed")
	}
}

func TestNewValidation_EmptyViolations_Fallback(t *testing.T) {
	err := apperror.NewValidation("bad input", nil)
	if len(err.Violations()) == 0 {
		t.Error("Violations() should not be empty when nil violations were passed")
	}
}

// ---- NewAuth ----

func TestNewAuth_Basic(t *testing.T) {
	err := apperror.NewAuth("missing API key")

	if err.Error() != "missing API key" {
		t.Errorf("Error() = %q", err.Error())
	}
	if err.Kind() != apperror.KindAuth {
		t.Errorf("Kind() = %v, want KindAuth", err.Kind())
	}
}

func TestNewAuth_EmptyMsg_Fallback(t *testing.T) {
	err := apperror.NewAuth("")
	if err.Error() == "" {
		t.Error("Error() should not be empty when empty msg was passed")
	}
}

// ---- NewNotFound ----

func TestNewNotFound_Basic(t *testing.T) {
	err := apperror.NewNotFound("photo not found", "abc-123")

	if err.Error() != "photo not found" {
		t.Errorf("Error() = %q", err.Error())
	}
	if err.Kind() != apperror.KindNotFound {
		t.Errorf("Kind() = %v, want KindNotFound", err.Kind())
	}
	if err.ResourceID() != "abc-123" {
		t.Errorf("ResourceID() = %q, want %q", err.ResourceID(), "abc-123")
	}
}

func TestNewNotFound_EmptyMsg_Fallback(t *testing.T) {
	err := apperror.NewNotFound("", "id")
	if err.Error() == "" {
		t.Error("Error() should not be empty when empty msg was passed")
	}
}

func TestNewNotFound_EmptyResourceID_Fallback(t *testing.T) {
	err := apperror.NewNotFound("not found", "")
	if err.ResourceID() == "" {
		t.Error("ResourceID() should not be empty when empty resourceID was passed")
	}
}

// ---- NewInternal ----

func TestNewInternal_Basic(t *testing.T) {
	cause := errors.New("db connection refused")
	err := apperror.NewInternal(cause)

	if err.Kind() != apperror.KindInternal {
		t.Errorf("Kind() = %v, want KindInternal", err.Kind())
	}
	if strings.Contains(err.Error(), "db connection refused") {
		t.Errorf("Error() leaks cause text: %q", err.Error())
	}
	if err.Error() == "" {
		t.Error("Error() must return a non-empty stable message")
	}
}

func TestNewInternal_NilCause(t *testing.T) {
	err := apperror.NewInternal(nil)
	if err == nil {
		t.Fatal("NewInternal(nil) returned nil")
	}
	if err.Kind() != apperror.KindInternal {
		t.Errorf("Kind() = %v, want KindInternal", err.Kind())
	}
}

func TestNewInternal_StableMessage(t *testing.T) {
	a := apperror.NewInternal(errors.New("foo"))
	b := apperror.NewInternal(errors.New("bar"))
	if a.Error() != b.Error() {
		t.Errorf("internal errors must share a stable public message: %q vs %q", a.Error(), b.Error())
	}
}

// ---- errors.As ----

func TestAppError_ErrorsAs(t *testing.T) {
	var target *apperror.AppError

	cases := []error{
		apperror.NewValidation("v", []apperror.FieldViolation{{Field: "f", Issue: "i"}}),
		apperror.NewAuth("a"),
		apperror.NewNotFound("n", "id"),
		apperror.NewInternal(errors.New("cause")),
	}
	for _, err := range cases {
		if !errors.As(err, &target) {
			t.Errorf("errors.As should match *AppError for %v", err)
		}
	}
}

// ---- errors.Is through Unwrap (internal only) ----

func TestNewInternal_ErrorsIs(t *testing.T) {
	cause := errors.New("original cause")
	err := apperror.NewInternal(cause)

	if !errors.Is(err, cause) {
		t.Error("errors.Is should find the wrapped cause through Unwrap")
	}
}

func TestNonInternalErrors_DoNotExposeUnwrap(t *testing.T) {
	sentinel := errors.New("sentinel")
	cases := []error{
		apperror.NewValidation("v", []apperror.FieldViolation{{Field: "f", Issue: "i"}}),
		apperror.NewAuth("a"),
		apperror.NewNotFound("n", "id"),
	}
	for _, err := range cases {
		if errors.Is(err, sentinel) {
			t.Errorf("non-internal error should not match an arbitrary sentinel via Unwrap: %v", err)
		}
	}
}

// ---- Error() must not leak internal cause ----

func TestAppError_Error_NoLeakCause(t *testing.T) {
	secret := "super-secret-db-password"
	err := apperror.NewInternal(errors.New(secret))
	if strings.Contains(err.Error(), secret) {
		t.Errorf("Error() leaked internal cause text: %q", err.Error())
	}
}

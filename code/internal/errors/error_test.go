package errors

import (
	stderrors "errors"
	"testing"
)

func TestAppErrorErrorIncludesWrappedErrorMessage(t *testing.T) {
	t.Parallel()

	root := stderrors.New("db timeout")
	err := (&AppError{Message: "failed to save"}).WithErr(root)

	if got := err.Error(); got != "failed to save: db timeout" {
		t.Fatalf("expected wrapped error message, got %q", got)
	}
	if !stderrors.Is(err, root) {
		t.Fatal("expected AppError to unwrap to original error")
	}
}

func TestErrInvalidInputWithMessageDerivesSpecificCodeAndKeepsStatus(t *testing.T) {
	t.Parallel()

	root := stderrors.New("bad input")
	err := ErrInvalidInputWithMessage("description too short", root)

	if err.Status != 400 {
		t.Fatalf("expected status 400, got %d", err.Status)
	}
	if err.Code != "validation.description.too.short" {
		t.Fatalf("expected derived validation code, got %q", err.Code)
	}
	if err.Message != "description too short" {
		t.Fatalf("expected custom message to be preserved, got %q", err.Message)
	}
	if !stderrors.Is(err, root) {
		t.Fatal("expected returned error to wrap original error")
	}
}

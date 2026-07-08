package errors

import (
	"fmt"
	"net/http"
	"strings"
	"unicode"
)

type AppError struct {
	Status  int
	Code    string
	Message string
	Err     error
	Details map[string]any
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s", e.Message, e.Err.Error())
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func (e *AppError) WithErr(err error) *AppError {
	e.Err = err
	return e
}

func (e *AppError) WithDetails(details map[string]any) *AppError {
	e.Details = details
	return e
}

func ErrAccountNotFound() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "account.not_found",
		Message: "account not found",
	}
}
func ErrCategoryNotFound() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "category.not_found",
		Message: "category not found",
	}
}
func ErrInvalidInput() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "validation.invalid_input",
		Message: "invalid input",
	}
}
func ErrUserNotFound() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "user.not_found",
		Message: "user not found",
	}
}
func ErrTransactionNotFound() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "transaction.not_found",
		Message: "transaction not found",
	}
}
func ErrDuplicateAccount() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "account.code.duplicate",
		Message: "there's already one account with that code for this user",
	}
}
func ErrDuplicateUser() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "user.email.duplicate",
		Message: "there's already one user with the same email",
	}
}
func ErrDuplicateCategory() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "category.code.duplicate",
		Message: "there's already one category with that code for this user",
	}
}
func ErrSuggestionNotFound() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "suggestion.not_found",
		Message: "suggestion not found",
	}
}
func ErrInvalidLoginPassword() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "auth.login.invalid_credentials",
		Message: "invalid login/password",
	}
}
func ErrTokenGeneration() *AppError {
	return &AppError{
		Status:  http.StatusInternalServerError,
		Code:    "auth.token.generation_failed",
		Message: "token generation failed",
	}
}
func ErrAccountDeactivated() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "account.deactivated",
		Message: "can't change a deactivated account",
	}
}
func ErrCategoryDeactivated() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "category.deactivated",
		Message: "can't change a deactivated category",
	}
}
func ErrInvalidNumberOfTransferTransactions() *AppError {
	return &AppError{
		Status:  http.StatusInternalServerError,
		Code:    "transaction.transfer.invalid_pair",
		Message: "transferID should return only 2 transactions",
	}
}
func ErrInvalidTransactionID() *AppError {
	return &AppError{
		Status:  http.StatusBadRequest,
		Code:    "transaction.id.invalid",
		Message: "transaction ID not a valid UUID",
	}
}
func ErrInvalidInputWithCode(code, msg string, err error) *AppError {
	e := ErrInvalidInput()
	e.Code = code
	e.Message = msg
	e.Err = err
	return e
}

func ErrInvalidInputWithMessage(msg string, err error) *AppError {
	return ErrInvalidInputWithCode(validationCodeFromMessage(msg), msg, err)
}

func validationCodeFromMessage(msg string) string {
	normalized := strings.TrimSpace(strings.ToLower(msg))
	if normalized == "" {
		return "validation.invalid_input"
	}

	var b strings.Builder
	lastWasSeparator := false
	for _, r := range normalized {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastWasSeparator = false
		case lastWasSeparator:
			continue
		default:
			b.WriteRune('.')
			lastWasSeparator = true
		}
	}

	code := strings.Trim(b.String(), ".")
	if code == "" {
		return "validation.invalid_input"
	}
	return "validation." + code
}

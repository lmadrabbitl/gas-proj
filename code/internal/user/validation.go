package user

import (
	"expense-tracker/internal/errors"
	"net/mail"
	"strings"
)

func CheckUserName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.ErrInvalidInputWithMessage("name is required", nil)
	}

	if len(strings.TrimSpace(name)) > 120 {
		return errors.ErrInvalidInputWithMessage("name too long", nil)
	}

	return nil
}

func CheckUserEmail(email string) error {
	if email == "" {
		return errors.ErrInvalidInputWithMessage("email is required", nil)
	}

	_, err := mail.ParseAddress(email)
	if err != nil {
		return errors.ErrInvalidInputWithMessage("email is invalid", nil)
	}
	return nil
}

func CheckUserPassword(password string) error {
	if password == "" {
		return errors.ErrInvalidInputWithMessage("password can't be empty", nil)
	}

	if len(password) < 8 {
		return errors.ErrInvalidInputWithMessage("password must be at least 8 characters", nil)
	}

	if len(password) > 72 {
		return errors.ErrInvalidInputWithMessage("password too long", nil)
	}

	return nil
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

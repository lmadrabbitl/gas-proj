package suggestion

import (
	"expense-tracker/internal/errors"
	"strings"
)

func CheckDescriptionContains(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.ErrInvalidInputWithMessage("description_contains is required", nil)
	}
	return nil
}

func CheckPriority(priority int) error {
	if priority <= 0 {
		return errors.ErrInvalidInputWithMessage("priority must be greater than 0", nil)
	}
	return nil
}

func CheckSuggestionEntryType(entryType SuggestionEntryType) error {
	if !entryType.IsValid() {
		return errors.ErrInvalidInputWithMessage("entry_type must be REVENUE, EXPENSE or TRANSFER", nil)
	}
	return nil
}

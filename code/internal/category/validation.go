package category

import (
	"expense-tracker/internal/errors"
)

func CheckCategoryName(name string) error {
	if name == "" {
		return errors.ErrInvalidInputWithMessage("name is required", nil)
	}
	return nil
}

func CheckCategoryCode(code string) error {
	if code == "" {
		return errors.ErrInvalidInputWithMessage("code is required", nil)
	}
	return nil
}

func CheckCategoryCodes(codes []string) error {
	for _, str := range codes {
		if str == "" {
			return errors.ErrInvalidInputWithMessage("code is required", nil)
		}
	}
	return nil
}

func CheckCategoryType(accType CategoryType) error {
	switch accType {
	case CategoryTypeIncome, CategoryTypeExpense, CategoryTypeMovement:
		return nil
	default:
		return errors.ErrInvalidInputWithMessage("type needs to be INCOME, EXPENSE or MOVEMENT", nil)
	}
}

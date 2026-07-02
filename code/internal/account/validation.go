package account

import (
	"expense-tracker/internal/errors"
)

func CheckAccountName(name string) error {
	if name == "" {
		return errors.ErrInvalidInputWithMessage("name is required", nil)
	}
	return nil
}

func CheckAccountCode(code string) error {
	if code == "" {
		return errors.ErrInvalidInputWithMessage("code is required", nil)
	}
	return nil
}

func CheckAccountCodes(codes []string) error {
	for _, str := range codes {
		if str == "" {
			return errors.ErrInvalidInputWithMessage("code is required", nil)
		}
	}
	return nil
}

func CheckAccountType(accType AccountType) error {
	switch accType {
	case AccountTypeAsset, AccountTypeLiability:
		return nil
	default:
		return errors.ErrInvalidInputWithMessage("type needs to be ASSET or LIABILITY", nil)
	}
}

func CheckAccountCurrency(currency string) error {
	if len(currency) != 3 {
		return errors.ErrInvalidInputWithMessage("currency needs to be 3 letters", nil)
	}

	return nil
}

func CheckAccountSortOrder(sortOrder int) error {
	if sortOrder <= 0 {
		return errors.ErrInvalidInputWithMessage("sort_order needs to be greater than 0", nil)
	}

	return nil
}

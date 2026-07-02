package investment

import (
	"expense-tracker/internal/errors"
	"regexp"
	"strings"
)

var assetCodePattern = regexp.MustCompile(`^[A-Z0-9.-]+$`)

func CheckAssetCode(code string) error {
	trimmed := strings.TrimSpace(strings.ToUpper(code))
	if trimmed == "" {
		return errors.ErrInvalidInputWithMessage("asset code is required", nil)
	}
	if len(trimmed) > 20 {
		return errors.ErrInvalidInputWithMessage("asset code must have at most 20 characters", nil)
	}
	if !assetCodePattern.MatchString(trimmed) {
		return errors.ErrInvalidInputWithMessage("asset code has invalid characters", nil)
	}
	return nil
}

func CheckAssetName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.ErrInvalidInputWithMessage("asset name is required", nil)
	}
	return nil
}

func CheckAssetType(assetType AssetType) error {
	switch assetType {
	case AssetTypeStock, AssetTypeFII, AssetTypeETF:
		return nil
	default:
		return errors.ErrInvalidInputWithMessage("asset type needs to be STOCK, FII or ETF", nil)
	}
}

func CheckPortfolioCode(code string) error {
	if strings.TrimSpace(code) == "" {
		return errors.ErrInvalidInputWithMessage("portfolio code is required", nil)
	}
	return nil
}

func CheckPortfolioName(name string) error {
	if strings.TrimSpace(name) == "" {
		return errors.ErrInvalidInputWithMessage("portfolio name is required", nil)
	}
	return nil
}

func CheckOperationType(operationType OperationType) error {
	switch operationType {
	case OperationTypeBuy, OperationTypeSell, OperationTypeBonification:
		return nil
	default:
		return errors.ErrInvalidInputWithMessage("operation type needs to be BUY, SELL or BONIFICATION", nil)
	}
}

func CheckQuantity(quantity int64) error {
	if quantity <= 0 {
		return errors.ErrInvalidInputWithMessage("quantity needs to be greater than 0", nil)
	}
	return nil
}

func CheckMoney(name string, value int64, allowZero bool) error {
	if value < 0 {
		return errors.ErrInvalidInputWithMessage(name+" can't be negative", nil)
	}
	if !allowZero && value == 0 {
		return errors.ErrInvalidInputWithMessage(name+" needs to be greater than 0", nil)
	}
	return nil
}

func CheckTargetAllocationBPS(bps int) error {
	if bps < 0 || bps > 10000 {
		return errors.ErrInvalidInputWithMessage("target allocation needs to be between 0 and 100 percent", nil)
	}
	return nil
}

func CheckPortfolioAssetSortOrder(sortOrder int) error {
	if sortOrder <= 0 {
		return errors.ErrInvalidInputWithMessage("sort order needs to be greater than 0", nil)
	}
	return nil
}

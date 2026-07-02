package transaction

import (
	"expense-tracker/internal/errors"
	"strings"
)

const MaxTransactionPageSize = 1000

func CheckLimitQueryParam(limit int) error {
	if limit < 1 {
		return errors.ErrInvalidInputWithCode("transaction.filter.limit.min", "query param limit has to be at least 1", nil)
	}
	if limit > MaxTransactionPageSize {
		return errors.ErrInvalidInputWithCode("transaction.filter.limit.max", "query param limit has to be at most 1000", nil)
	}
	return nil
}

func CheckUpdateRequest(req UpdateTransactionRequest, txPair transactionPair) error {

	// is final result a transfer
	isFinalTransfer := txPair.isTransfer
	if req.IsTransfer != nil {
		isFinalTransfer = *req.IsTransfer
	}

	//if TransferAccountCode will be update, it needs to be a transfer
	//either was a transfer and this won't change
	//or wasn't a transfer and it will become one
	if req.TransferAccountCode != nil && !isFinalTransfer {
		return errors.ErrInvalidInputWithMessage("Can't change transferAccountID on a non-transfer", nil)
	}

	if isFinalTransfer && txPair.t1.TransferAccountID == nil && req.TransferAccountCode == nil {
		return errors.ErrInvalidInputWithMessage("Can't change to transfer without transferAccountID", nil)
	}

	return nil
}

func CheckSortQueryParam(sort string) error {
	sortUpper := strings.ToUpper(sort)
	switch sortUpper {
	case string(SortByAmount):
	case string(SortByTransactionDate):
	case string(SortByUpdatedDate):
	default:
		return errors.ErrInvalidInputWithMessage("invalid query param sort", nil)
	}
	return nil
}

func CheckAccountQueryParam(account_code []string) error {
	if len(account_code) > 0 {
		for _, acc := range account_code {
			if len(acc) > 50 {
				return errors.ErrInvalidInputWithMessage("account code has to be less than 50 characters", nil)
			}
		}
	}
	return nil
}

func CheckCategoryQueryParam(category_code []string) error {
	if len(category_code) > 0 {
		for _, cat := range category_code {
			if len(cat) > 50 {
				return errors.ErrInvalidInputWithMessage("category code has to be less than 50 characters", nil)
			}
		}
	}
	return nil
}

func CheckDescriptionQueryParam(descriptionTerms []string) error {
	if len(descriptionTerms) > 0 {
		for _, des := range descriptionTerms {
			normalized := strings.TrimPrefix(strings.TrimSpace(des), QUERY_DESCRIPTION_EXCLUDE_PREFIX)
			if len(normalized) < 2 {
				return errors.ErrInvalidInputWithMessage("description term has to be equal or more than 2 characters", nil)
			}
		}
	}
	return nil
}

func CheckOperationQueryParam(operationTypes []string) error {
	if len(operationTypes) > 0 {
		for _, op := range operationTypes {
			switch op {
			case string(CreditOperation), string(DebitOperation), string(TransferOperation):
			default:
				return errors.ErrInvalidInputWithCode("transaction.filter.operation.invalid", "invalid operation type", nil)
			}
		}
	}
	return nil
}

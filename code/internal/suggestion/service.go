package suggestion

import (
	"expense-tracker/internal/account"
	"expense-tracker/internal/category"
	"expense-tracker/internal/errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service interface {
	AddSuggestion(userID uuid.UUID, req CreateSuggestionRequest) (*SuggestionResponseItem, error)
	GetSuggestions(userID uuid.UUID) ([]SuggestionResponseItem, error)
	GetSuggestionByID(userID, suggestionID uuid.UUID) (*SuggestionResponseItem, error)
	UpdateSuggestion(userID, suggestionID uuid.UUID, req UpdateSuggestionRequest) (*SuggestionResponseItem, error)
	DeleteSuggestion(userID, suggestionID uuid.UUID) error
}

type AccountReader interface {
	GetAccountByCode(userID uuid.UUID, code string) (*account.Account, error)
}

type CategoryReader interface {
	GetCategoryByCode(userID uuid.UUID, code string) (*category.Category, error)
}

type service struct {
	repo       Repository
	accounts   AccountReader
	categories CategoryReader
}

type CreateSuggestionRequest struct {
	DescriptionContains string
	Priority            int
	EntryType           *SuggestionEntryType
	CategoryCode        *string
	AccountCode         *string
	TransferAccountCode *string
}

type UpdateSuggestionRequest struct {
	DescriptionContains *string
	Priority            *int
	EntryType           *string
	CategoryCode        *string
	AccountCode         *string
	TransferAccountCode *string
}

type SuggestionResponseItem struct {
	ID                  uuid.UUID            `json:"id"`
	DescriptionContains string               `json:"description_contains"`
	Priority            int                  `json:"priority"`
	EntryType           *SuggestionEntryType `json:"entry_type"`
	CategoryCode        *string              `json:"category_code"`
	AccountCode         *string              `json:"account_code"`
	TransferAccountCode *string              `json:"transfer_account_code"`
	CreatedAt           time.Time            `json:"created_at"`
	UpdatedAt           time.Time            `json:"updated_at"`
}

func NewService(repo Repository, accounts AccountReader, categories CategoryReader) Service {
	return &service{
		repo:       repo,
		accounts:   accounts,
		categories: categories,
	}
}

func (serv *service) AddSuggestion(userID uuid.UUID, req CreateSuggestionRequest) (*SuggestionResponseItem, error) {
	entryType, categoryID, accountID, transferAccountID, description, err := serv.resolveSuggestionFields(
		userID,
		req.DescriptionContains,
		req.Priority,
		req.EntryType,
		req.CategoryCode,
		req.AccountCode,
		req.TransferAccountCode,
	)
	if err != nil {
		return nil, err
	}

	suggestion := &Suggestion{
		ID:                  uuid.New(),
		UserID:              userID,
		DescriptionContains: description,
		Priority:            req.Priority,
		EntryType:           entryType,
		CategoryID:          categoryID,
		AccountID:           accountID,
		TransferAccountID:   transferAccountID,
	}

	created, err := serv.repo.Create(suggestion)
	if err != nil {
		return nil, err
	}

	return serv.repo.GetDTOByID(userID, created.ID)
}

func (serv *service) GetSuggestions(userID uuid.UUID) ([]SuggestionResponseItem, error) {
	return serv.repo.GetByUser(userID)
}

func (serv *service) GetSuggestionByID(userID, suggestionID uuid.UUID) (*SuggestionResponseItem, error) {
	return serv.repo.GetDTOByID(userID, suggestionID)
}

func (serv *service) UpdateSuggestion(userID, suggestionID uuid.UUID, req UpdateSuggestionRequest) (*SuggestionResponseItem, error) {
	if req.DescriptionContains == nil &&
		req.Priority == nil &&
		req.EntryType == nil &&
		req.CategoryCode == nil &&
		req.AccountCode == nil &&
		req.TransferAccountCode == nil {
		return nil, errors.ErrInvalidInputWithMessage("at least one suggestion field must be provided", nil)
	}

	current, err := serv.repo.GetByID(userID, suggestionID)
	if err != nil {
		return nil, err
	}

	description := current.DescriptionContains
	if req.DescriptionContains != nil {
		description = *req.DescriptionContains
	}

	priority := current.Priority
	if req.Priority != nil {
		priority = *req.Priority
	}

	entryTypeText := ""
	if current.EntryType != nil {
		entryTypeText = string(*current.EntryType)
	}
	if req.EntryType != nil {
		entryTypeText = *req.EntryType
	}

	categoryCode := ""
	accountCode := ""
	transferAccountCode := ""
	dto, err := serv.repo.GetDTOByID(userID, suggestionID)
	if err != nil {
		return nil, err
	}
	if dto.CategoryCode != nil {
		categoryCode = *dto.CategoryCode
	}
	if dto.AccountCode != nil {
		accountCode = *dto.AccountCode
	}
	if dto.TransferAccountCode != nil {
		transferAccountCode = *dto.TransferAccountCode
	}

	if req.CategoryCode != nil {
		categoryCode = *req.CategoryCode
	}
	if req.AccountCode != nil {
		accountCode = *req.AccountCode
	}
	if req.TransferAccountCode != nil {
		transferAccountCode = *req.TransferAccountCode
	}

	var createEntryType *SuggestionEntryType
	if entryTypeText != "" {
		normalizedEntryType := SuggestionEntryType(strings.ToUpper(strings.TrimSpace(entryTypeText)))
		createEntryType = &normalizedEntryType
	}

	var categoryCodePtr *string
	if categoryCode != "" {
		normalized := categoryCode
		categoryCodePtr = &normalized
	}
	var accountCodePtr *string
	if accountCode != "" {
		normalized := accountCode
		accountCodePtr = &normalized
	}
	var transferCodePtr *string
	if transferAccountCode != "" {
		normalized := transferAccountCode
		transferCodePtr = &normalized
	}

	entryType, categoryID, accountID, transferAccountID, normalizedDescription, err := serv.resolveSuggestionFields(
		userID,
		description,
		priority,
		createEntryType,
		categoryCodePtr,
		accountCodePtr,
		transferCodePtr,
	)
	if err != nil {
		return nil, err
	}

	updateSuggestion := &UpdateSuggestion{}
	if req.DescriptionContains != nil {
		updateSuggestion.DescriptionContains = &normalizedDescription
	}
	if req.Priority != nil {
		updateSuggestion.Priority = &priority
	}
	if req.EntryType != nil {
		updateSuggestion.SetEntryType = true
		updateSuggestion.EntryType = entryType
	}
	if req.CategoryCode != nil {
		updateSuggestion.SetCategoryID = true
		updateSuggestion.CategoryID = categoryID
	}
	if req.AccountCode != nil {
		updateSuggestion.SetAccountID = true
		updateSuggestion.AccountID = accountID
	}
	if req.TransferAccountCode != nil {
		updateSuggestion.SetTransferAccountID = true
		updateSuggestion.TransferAccountID = transferAccountID
	}

	if req.EntryType != nil && strings.TrimSpace(*req.EntryType) == "" {
		updateSuggestion.SetEntryType = true
		updateSuggestion.EntryType = nil
	}
	if req.CategoryCode != nil && strings.TrimSpace(*req.CategoryCode) == "" {
		updateSuggestion.SetCategoryID = true
		updateSuggestion.CategoryID = nil
	}
	if req.AccountCode != nil && strings.TrimSpace(*req.AccountCode) == "" {
		updateSuggestion.SetAccountID = true
		updateSuggestion.AccountID = nil
	}
	if req.TransferAccountCode != nil && strings.TrimSpace(*req.TransferAccountCode) == "" {
		updateSuggestion.SetTransferAccountID = true
		updateSuggestion.TransferAccountID = nil
	}

	if _, err := serv.repo.Update(userID, suggestionID, updateSuggestion); err != nil {
		return nil, err
	}

	return serv.repo.GetDTOByID(userID, suggestionID)
}

func (serv *service) DeleteSuggestion(userID, suggestionID uuid.UUID) error {
	return serv.repo.Delete(userID, suggestionID)
}

func (serv *service) resolveSuggestionFields(
	userID uuid.UUID,
	description string,
	priority int,
	entryType *SuggestionEntryType,
	categoryCode *string,
	accountCode *string,
	transferAccountCode *string,
) (*SuggestionEntryType, *uuid.UUID, *uuid.UUID, *uuid.UUID, string, error) {
	trimmedDescription := strings.TrimSpace(description)
	if err := CheckDescriptionContains(trimmedDescription); err != nil {
		return nil, nil, nil, nil, "", err
	}
	if err := CheckPriority(priority); err != nil {
		return nil, nil, nil, nil, "", err
	}

	var normalizedEntryType *SuggestionEntryType
	if entryType != nil && strings.TrimSpace(string(*entryType)) != "" {
		value := SuggestionEntryType(strings.ToUpper(strings.TrimSpace(string(*entryType))))
		if err := CheckSuggestionEntryType(value); err != nil {
			return nil, nil, nil, nil, "", err
		}
		normalizedEntryType = &value
	}

	var categoryID *uuid.UUID
	var categoryValue *category.Category
	if categoryCode != nil && strings.TrimSpace(*categoryCode) != "" {
		foundCategory, err := serv.categories.GetCategoryByCode(userID, strings.TrimSpace(*categoryCode))
		if err != nil {
			return nil, nil, nil, nil, "", err
		}
		if foundCategory.ParentID == nil {
			return nil, nil, nil, nil, "", errors.ErrInvalidInputWithMessage("category_code must reference a leaf category", nil)
		}
		categoryID = &foundCategory.ID
		categoryValue = foundCategory
	}

	var accountID *uuid.UUID
	if accountCode != nil && strings.TrimSpace(*accountCode) != "" {
		foundAccount, err := serv.accounts.GetAccountByCode(userID, strings.TrimSpace(*accountCode))
		if err != nil {
			return nil, nil, nil, nil, "", err
		}
		accountID = &foundAccount.ID
	}

	var transferAccountID *uuid.UUID
	if transferAccountCode != nil && strings.TrimSpace(*transferAccountCode) != "" {
		foundTransferAccount, err := serv.accounts.GetAccountByCode(userID, strings.TrimSpace(*transferAccountCode))
		if err != nil {
			return nil, nil, nil, nil, "", err
		}
		transferAccountID = &foundTransferAccount.ID
	}

	if normalizedEntryType == nil && categoryID == nil && accountID == nil && transferAccountID == nil {
		return nil, nil, nil, nil, "", errors.ErrInvalidInputWithMessage("at least one target field must be provided", nil)
	}

	if transferAccountID != nil && (normalizedEntryType == nil || *normalizedEntryType != SuggestionEntryTypeTransfer) {
		return nil, nil, nil, nil, "", errors.ErrInvalidInputWithMessage("transfer_account_code requires entry_type TRANSFER", nil)
	}

	if normalizedEntryType != nil && categoryValue != nil {
		switch *normalizedEntryType {
		case SuggestionEntryTypeRevenue:
			if categoryValue.Type != category.CategoryTypeIncome {
				return nil, nil, nil, nil, "", errors.ErrInvalidInputWithMessage("category_code must be an INCOME category for entry_type REVENUE", nil)
			}
		case SuggestionEntryTypeExpense:
			if categoryValue.Type != category.CategoryTypeExpense {
				return nil, nil, nil, nil, "", errors.ErrInvalidInputWithMessage("category_code must be an EXPENSE category for entry_type EXPENSE", nil)
			}
		case SuggestionEntryTypeTransfer:
			if categoryValue.Type != category.CategoryTypeMovement {
				return nil, nil, nil, nil, "", errors.ErrInvalidInputWithMessage("category_code must be a MOVEMENT category for entry_type TRANSFER", nil)
			}
		}
	}

	return normalizedEntryType, categoryID, accountID, transferAccountID, trimmedDescription, nil
}

package account

import (
	"expense-tracker/internal/errors"
	"expense-tracker/internal/slugutil"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service interface {
	AddAccount(userID uuid.UUID, req CreateAccountRequest) (*Account, error)
	GetAccounts(userID uuid.UUID) ([]Account, error)
	GetAccountByCode(userID uuid.UUID, code string) (*Account, error)
	GetAccountsByCode(userID uuid.UUID, codes []string) ([]Account, error)
	GetAccountsByID(userID uuid.UUID, id []uuid.UUID) ([]Account, error)
	UpdateAccount(userID uuid.UUID, code string, req UpdateAccountRequest) (*Account, error)
	ReorderAccounts(userID uuid.UUID, codes []string) error
	DeactivateAccount(userID uuid.UUID, code string) error
	DeleteAccountPermanently(userID uuid.UUID, code string) error
	UpdateBalance(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error
}

type service struct {
	repo Repository
}

type CreateAccountRequest struct {
	Name              string
	Type              AccountType
	Currency          string
	AssetRole         AccountAssetRole
	HideFromDashboard bool
}

type UpdateAccountRequest struct {
	Name              *string
	Type              *AccountType
	Currency          *string
	AssetRole         *AccountAssetRole
	HideFromDashboard *bool
}

func NewService(repo Repository) Service {
	return &service{
		repo: repo,
	}
}

func (serv *service) AddAccount(userID uuid.UUID, req CreateAccountRequest) (*Account, error) {
	if err := CheckAccountName(req.Name); err != nil {
		return nil, err
	}

	if err := CheckAccountCurrency(req.Currency); err != nil {
		return nil, err
	}

	if err := CheckAccountType(req.Type); err != nil {
		return nil, err
	}
	assetRole, err := NormalizeAccountAssetRole(req.Type, req.AssetRole)
	if err != nil {
		return nil, err
	}

	existingAccounts, err := serv.repo.GetByUser(userID)
	if err != nil {
		return nil, err
	}
	if hasDuplicateActiveAccountName(existingAccounts, req.Name, uuid.Nil) {
		return nil, errors.ErrInvalidInputWithMessage("there's already one active account with that name for this user", nil)
	}

	existingCodes := make(map[string]struct{}, len(existingAccounts))
	for _, account := range existingAccounts {
		existingCodes[account.Code] = struct{}{}
	}

	account := &Account{
		ID:                uuid.New(),
		UserID:            userID,
		Code:              slugutil.GenerateUnique(req.Name, "account", existingCodes),
		Name:              req.Name,
		Type:              req.Type,
		Currency:          req.Currency,
		AssetRole:         assetRole,
		HideFromDashboard: req.HideFromDashboard,
	}

	if account.DeactivatedAt == nil && account.SortOrder == nil {
		nextSortOrder, err := serv.repo.GetNextSortOrder(userID)
		if err != nil {
			return nil, err
		}
		account.SortOrder = &nextSortOrder
	}

	return serv.repo.Create(account)
}

func (serv *service) GetAccounts(userID uuid.UUID) ([]Account, error) {

	return serv.repo.GetByUser(userID)
}

func (serv *service) GetAccountByCode(userID uuid.UUID, code string) (*Account, error) {

	if err := CheckAccountCode(code); err != nil {
		return nil, err
	}

	return serv.repo.GetByCode(userID, strings.ToLower(code))
}

func (serv *service) GetAccountsByCode(userID uuid.UUID, codes []string) ([]Account, error) {

	if err := CheckAccountCodes(codes); err != nil {
		return nil, err
	}

	lowercaseCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		lowercaseCodes = append(lowercaseCodes, strings.ToLower(code))
	}

	return serv.repo.GetByCodes(userID, lowercaseCodes)
}

func (serv *service) GetAccountsByID(userID uuid.UUID, ids []uuid.UUID) ([]Account, error) {
	return serv.repo.GetByIDs(userID, ids)
}

func (serv *service) UpdateAccount(userID uuid.UUID, code string, req UpdateAccountRequest) (*Account, error) {

	if req.Currency == nil && req.Name == nil && req.Type == nil && req.AssetRole == nil && req.HideFromDashboard == nil {
		return nil, errors.ErrInvalidInputWithMessage("at least one of these can't be empty: name, currency, type, asset_role or hide_from_dashboard", nil)
	}

	if err := CheckAccountCode(code); err != nil {
		return nil, err
	}
	account, err := serv.repo.GetByCode(userID, strings.ToLower(code))
	if err != nil {
		return nil, err
	}

	if account.DeactivatedAt != nil {
		return nil, errors.ErrAccountDeactivated()
	}

	if req.Name != nil {
		existingAccounts, err := serv.repo.GetByUser(userID)
		if err != nil {
			return nil, err
		}
		if hasDuplicateActiveAccountName(existingAccounts, *req.Name, account.ID) {
			return nil, errors.ErrInvalidInputWithMessage("there's already one active account with that name for this user", nil)
		}
	}

	updateAccount := &UpdateAccount{}
	nextType := account.Type
	if req.Type != nil {
		if err := CheckAccountType(*req.Type); err != nil {
			return nil, err
		}
		nextType = *req.Type
	}
	nextRole := account.AssetRole
	if req.AssetRole != nil {
		nextRole = *req.AssetRole
	}
	normalizedRole, err := NormalizeAccountAssetRole(nextType, nextRole)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		if err := CheckAccountName(*req.Name); err != nil {
			return nil, err
		}
		updateAccount.Name = req.Name
	}
	if req.Currency != nil {
		if err := CheckAccountCurrency(*req.Currency); err != nil {
			return nil, err
		}
		updateAccount.Currency = req.Currency
	}
	if req.Type != nil {
		updateAccount.Type = req.Type
	}
	if req.AssetRole != nil || (req.Type != nil && account.AssetRole != normalizedRole) {
		updateAccount.AssetRole = &normalizedRole
	}
	if req.HideFromDashboard != nil {
		updateAccount.HideFromDashboard = req.HideFromDashboard
	}
	return serv.repo.Update(userID, strings.ToLower(code), updateAccount)

}

func (serv *service) ReorderAccounts(userID uuid.UUID, codes []string) error {
	if len(codes) == 0 {
		return errors.ErrInvalidInputWithMessage("account order cannot be empty", nil)
	}

	seenCodes := make(map[string]struct{}, len(codes))
	normalizedCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		if err := CheckAccountCode(code); err != nil {
			return err
		}

		lowerCode := strings.ToLower(code)
		if _, exists := seenCodes[lowerCode]; exists {
			return errors.ErrInvalidInputWithMessage("account order cannot contain duplicate codes", nil)
		}

		seenCodes[lowerCode] = struct{}{}
		normalizedCodes = append(normalizedCodes, lowerCode)
	}

	accounts, err := serv.repo.GetByUser(userID)
	if err != nil {
		return err
	}

	activeCodes := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if account.DeactivatedAt == nil {
			activeCodes = append(activeCodes, account.Code)
		}
	}

	if len(activeCodes) != len(normalizedCodes) {
		return errors.ErrInvalidInputWithMessage("account order must include all active accounts exactly once", nil)
	}

	for _, activeCode := range activeCodes {
		if _, exists := seenCodes[activeCode]; !exists {
			return errors.ErrInvalidInputWithMessage("account order must include all active accounts exactly once", nil)
		}
	}

	return serv.repo.Reorder(userID, normalizedCodes)
}

func (serv *service) DeactivateAccount(userID uuid.UUID, code string) error {

	if err := CheckAccountCode(code); err != nil {
		return err
	}

	return serv.repo.Deactivate(userID, strings.ToLower(code))
}

func (serv *service) DeleteAccountPermanently(userID uuid.UUID, code string) error {
	if err := CheckAccountCode(code); err != nil {
		return err
	}

	normalizedCode := strings.ToLower(code)
	account, err := serv.repo.GetByCode(userID, normalizedCode)
	if err != nil {
		return err
	}
	if account.DeactivatedAt == nil {
		return errors.ErrInvalidInputWithMessage("only deactivated accounts can be permanently deleted", nil)
	}

	hasTransactions, err := serv.repo.HasTransactions(userID, account.ID)
	if err != nil {
		return err
	}
	if hasTransactions {
		return errors.ErrInvalidInputWithMessage(
			"account can only be permanently deleted when it has no associated transactions",
			nil,
		)
	}

	return serv.repo.DeletePermanently(userID, account.ID)
}

func (serv *service) UpdateBalance(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error {

	if err := serv.repo.UpdateBalance(db, userID, code, newBalance); err != nil {
		return err
	}

	return nil

}

func hasDuplicateActiveAccountName(accounts []Account, name string, currentID uuid.UUID) bool {
	normalizedName := strings.TrimSpace(name)
	for _, account := range accounts {
		if account.DeactivatedAt != nil {
			continue
		}
		if currentID != uuid.Nil && account.ID == currentID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(account.Name), normalizedName) {
			return true
		}
	}
	return false
}

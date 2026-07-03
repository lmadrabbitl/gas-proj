package account

import (
	"errors"
	appErr "expense-tracker/internal/errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	Create(account *Account) (*Account, error)
	GetNextSortOrder(userID uuid.UUID) (int, error)
	GetByCode(userID uuid.UUID, code string) (*Account, error)
	GetByCodes(userID uuid.UUID, codes []string) ([]Account, error)
	GetByIDs(userID uuid.UUID, ids []uuid.UUID) ([]Account, error)
	GetByUser(userID uuid.UUID) ([]Account, error)
	Update(userID uuid.UUID, code string, account *UpdateAccount) (*Account, error)
	Reorder(userID uuid.UUID, codes []string) error
	Deactivate(userID uuid.UUID, code string) error
	HasTransactions(userID, accountID uuid.UUID) (bool, error)
	DeletePermanently(userID, accountID uuid.UUID) error
	UpdateBalance(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error
}

type UpdateAccount struct {
	Name               *string
	Type               *AccountType
	Currency           *string
	IsBrokerageAccount *bool
	HideFromDashboard  *bool
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (repo *repository) Create(account *Account) (*Account, error) {
	if err := repo.db.Create(account).Error; err != nil {
		var pgErr *pgconn.PgError
		log.Printf("Error in creation: %s", err.Error())
		switch {
		case errors.As(err, &pgErr):
			switch pgErr.Code {
			case "23503":
				return nil, appErr.ErrUserNotFound()
			case "23505":
				return nil, appErr.ErrDuplicateAccount()
			default:
				return nil, err
			}
		default:
			return nil, err
		}
	}
	return account, nil
}

func (repo *repository) GetByCode(userID uuid.UUID, code string) (*Account, error) {
	codes := []string{code}

	accounts, err := repo.GetByCodes(userID, codes)
	if err != nil {
		return nil, err
	}

	if len(accounts) == 0 {
		return nil, appErr.ErrAccountNotFound()
	}

	return &accounts[0], nil
}

func (repo *repository) GetByCodes(userID uuid.UUID, codes []string) ([]Account, error) {
	var accounts []Account

	//checking len to not cause issues when using 'IN'
	if len(codes) == 0 {
		return []Account{}, nil
	}

	if err := repo.db.Where("user_id = ? and code IN ?", userID, codes).Find(&accounts).Error; err != nil {
		return nil, err
	}

	return accounts, nil
}

func (repo *repository) GetByIDs(userID uuid.UUID, ids []uuid.UUID) ([]Account, error) {
	var accounts []Account

	//checking len to not cause issues when using 'IN'
	if len(ids) == 0 {
		return []Account{}, nil
	}

	if err := repo.db.Where("user_id = ? and id IN ?", userID, ids).Find(&accounts).Error; err != nil {
		return nil, err
	}

	return accounts, nil
}

func (repo *repository) GetByUser(userID uuid.UUID) ([]Account, error) {
	var accounts []Account

	if err := repo.db.
		Where("user_id = ?", userID).
		Order("CASE WHEN deactivated_at IS NULL THEN 0 ELSE 1 END").
		Order("sort_order ASC NULLS LAST").
		Order("created_at ASC").
		Find(&accounts).Error; err != nil {
		return nil, err
	}

	return accounts, nil
}

func (repo *repository) Update(userID uuid.UUID, code string, account *UpdateAccount) (*Account, error) {

	var updated Account

	updates := map[string]interface{}{}

	if account.Name != nil {
		updates["name"] = *account.Name
	}
	if account.Currency != nil {
		updates["currency"] = *account.Currency
	}
	if account.Type != nil {
		updates["type"] = *account.Type
	}
	if account.IsBrokerageAccount != nil {
		updates["is_brokerage_account"] = *account.IsBrokerageAccount
	}
	if account.HideFromDashboard != nil {
		updates["hide_from_dashboard"] = *account.HideFromDashboard
	}
	if len(updates) == 0 {
		return nil, fmt.Errorf("%w: no fields to update", appErr.ErrInvalidInput())
	}

	// limit update to the editable account fields only.
	result := repo.db.Model(&Account{}).
		Where("user_id = ? and code = ?", userID, code).
		Clauses(clause.Returning{}).
		Updates(updates).Scan(&updated)
	if result.Error != nil {
		return nil, result.Error
	}

	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return &updated, nil
}

func (repo *repository) Reorder(userID uuid.UUID, codes []string) error {
	return repo.db.Transaction(func(tx *gorm.DB) error {
		for index, code := range codes {
			result := tx.Model(&Account{}).
				Where("user_id = ? and code = ? and deactivated_at is null", userID, code).
				Update("sort_order", index+1)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return appErr.ErrAccountNotFound()
			}
		}

		return nil
	})
}

func (repo *repository) Deactivate(userID uuid.UUID, code string) error {

	result := repo.db.Model(&Account{}).
		Where("user_id = ? and code = ? and deactivated_at is null", userID, code).
		Updates(map[string]interface{}{
			"deactivated_at": time.Now(),
			"sort_order":     nil})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return appErr.ErrAccountNotFound()
	}

	return nil
}

func (repo *repository) HasTransactions(userID, accountID uuid.UUID) (bool, error) {
	var count int64
	err := repo.db.
		Table("transactions").
		Where("user_id = ? AND (account_id = ? OR transfer_account_id = ?)", userID, accountID, accountID).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func (repo *repository) DeletePermanently(userID, accountID uuid.UUID) error {
	return repo.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Table("suggestions").
			Where("user_id = ? AND account_id = ?", userID, accountID).
			Update("account_id", nil).Error; err != nil {
			return err
		}

		if err := tx.
			Table("suggestions").
			Where("user_id = ? AND transfer_account_id = ?", userID, accountID).
			Update("transfer_account_id", nil).Error; err != nil {
			return err
		}

		result := tx.
			Where("user_id = ? AND id = ? AND deactivated_at is not null", userID, accountID).
			Delete(&Account{})
		if result.Error != nil {
			var pgErr *pgconn.PgError
			if errors.As(result.Error, &pgErr) && pgErr.Code == "23503" {
				return appErr.ErrInvalidInputWithMessage(
					"account can only be permanently deleted when it has no associated transactions",
					nil,
				)
			}
			return result.Error
		}
		if result.RowsAffected == 0 {
			return appErr.ErrAccountNotFound()
		}

		return nil
	})
}

func (repo *repository) GetNextSortOrder(userID uuid.UUID) (int, error) {
	var nextSortOrder int

	err := repo.db.
		Raw(
			`SELECT COALESCE(MAX(sort_order), 0) + 1
			FROM accounts
			WHERE user_id = ? AND deactivated_at IS NULL`,
			userID,
		).
		Scan(&nextSortOrder).Error
	if err != nil {
		return 0, err
	}

	return nextSortOrder, nil
}

func (repo *repository) UpdateBalance(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error {
	if db == nil {
		db = repo.db
	}

	updates := map[string]interface{}{}
	updates["balance"] = newBalance

	var updated Account

	result := db.Model(&Account{}).
		Where("user_id = ? and code = ?", userID, code).
		Clauses(clause.Returning{}).
		Updates(updates).Scan(&updated)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

/*
CREATE
func (repo *AccountRepository) Create(account *Account) error {
	return repo.db.Create(account).Error
}

GET BY ID - one record
func (repo *AccountRepository) GetByID(id uuid.UUID) (*Account, error) {
	var account Account

	err := repo.db.
		Where("id = ?", id).
		First(&account).Error

	if err != nil {
		return nil, err
	}

	return &account, nil
}

GET BY ID - MANY
func (repo *AccountRepository) GetByUserID(userID uuid.UUID) ([]Account, error) {
	var accounts []Account

	err := repo.db.
		Where("user_id = ?", userID).
		Find(&accounts).Error

	return accounts, err
}

UPDATE
func (repo *AccountRepository) Update(account *Account) error {
	return repo.db.Save(account).Error
}

UPDATE SPECIFIC FIELDS
func (repo *AccountRepository) UpdateName(id uuid.UUID, name string) error {
	return repo.db.
		Model(&Account{}).
		Where("id = ?", id).
		Update("name", name).Error
}

MULTIPLE FIELDS
repo.db.
	Model(&Account{}).
	Where("id = ?", id).
	Updates(map[string]interface{}{
		"name": name,
		"code": code,
	})

HARD DELETE
func (repo *AccountRepository) Delete(id uuid.UUID) error {
	return repo.db.
		Where("id = ?", id).
		Delete(&Account{}).Error
}

SOFT DELETE
func (repo *AccountRepository) Deactivate(id uuid.UUID) error {
	return repo.db.
		Model(&Account{}).
		Where("id = ?", id).
		Update("deactivated_at", time.Now()).Error
}

DB TRANSACTION
err := repo.db.Transaction(func(tx *gorm.DB) error {
	// use tx instead of repo.db

	if err := tx.Create(&account1).Error; err != nil {
		return err
	}

	if err := tx.Create(&account2).Error; err != nil {
		return err
	}

	return nil
})



*/

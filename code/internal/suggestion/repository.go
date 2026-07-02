package suggestion

import (
	"errors"
	appErr "expense-tracker/internal/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	Create(suggestion *Suggestion) (*Suggestion, error)
	GetByID(userID, suggestionID uuid.UUID) (*Suggestion, error)
	GetDTOByID(userID, suggestionID uuid.UUID) (*SuggestionResponseItem, error)
	GetByUser(userID uuid.UUID) ([]SuggestionResponseItem, error)
	Update(userID, suggestionID uuid.UUID, suggestion *UpdateSuggestion) (*Suggestion, error)
	Delete(userID, suggestionID uuid.UUID) error
}

type UpdateSuggestion struct {
	DescriptionContains  *string
	Priority             *int
	EntryType            *SuggestionEntryType
	SetEntryType         bool
	CategoryID           *uuid.UUID
	SetCategoryID        bool
	AccountID            *uuid.UUID
	SetAccountID         bool
	TransferAccountID    *uuid.UUID
	SetTransferAccountID bool
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (repo *repository) Create(suggestion *Suggestion) (*Suggestion, error) {
	if err := repo.db.Create(suggestion).Error; err != nil {
		return nil, err
	}
	return suggestion, nil
}

func (repo *repository) GetByID(userID, suggestionID uuid.UUID) (*Suggestion, error) {
	var suggestion Suggestion

	err := repo.db.
		Where("user_id = ? AND id = ?", userID, suggestionID).
		First(&suggestion).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrSuggestionNotFound()
		}
		return nil, err
	}

	return &suggestion, nil
}

func (repo *repository) GetDTOByID(userID, suggestionID uuid.UUID) (*SuggestionResponseItem, error) {
	var suggestions []SuggestionResponseItem

	err := repo.baseQuery().
		Where("s.user_id = ? AND s.id = ?", userID, suggestionID).
		Find(&suggestions).Error
	if err != nil {
		return nil, err
	}
	if len(suggestions) == 0 {
		return nil, appErr.ErrSuggestionNotFound()
	}

	return &suggestions[0], nil
}

func (repo *repository) GetByUser(userID uuid.UUID) ([]SuggestionResponseItem, error) {
	var suggestions []SuggestionResponseItem

	if err := repo.baseQuery().
		Where("s.user_id = ?", userID).
		Find(&suggestions).Error; err != nil {
		return nil, err
	}

	return suggestions, nil
}

func (repo *repository) Update(userID, suggestionID uuid.UUID, suggestion *UpdateSuggestion) (*Suggestion, error) {
	updates := map[string]interface{}{}

	if suggestion.DescriptionContains != nil {
		updates["description_contains"] = *suggestion.DescriptionContains
	}
	if suggestion.Priority != nil {
		updates["priority"] = *suggestion.Priority
	}
	if suggestion.SetEntryType {
		updates["entry_type"] = suggestion.EntryType
	}
	if suggestion.SetCategoryID {
		updates["category_id"] = suggestion.CategoryID
	}
	if suggestion.SetAccountID {
		updates["account_id"] = suggestion.AccountID
	}
	if suggestion.SetTransferAccountID {
		updates["transfer_account_id"] = suggestion.TransferAccountID
	}
	if len(updates) == 0 {
		return nil, appErr.ErrInvalidInputWithMessage("error on updating suggestion: no fields to update", nil)
	}

	var updated Suggestion
	result := repo.db.Model(&Suggestion{}).
		Where("user_id = ? AND id = ?", userID, suggestionID).
		Clauses(clause.Returning{}).
		Updates(updates).
		Scan(&updated)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, appErr.ErrSuggestionNotFound()
	}

	return &updated, nil
}

func (repo *repository) Delete(userID, suggestionID uuid.UUID) error {
	result := repo.db.
		Where("user_id = ? AND id = ?", userID, suggestionID).
		Delete(&Suggestion{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return appErr.ErrSuggestionNotFound()
	}
	return nil
}

func (repo *repository) baseQuery() *gorm.DB {
	return repo.db.
		Table("suggestions s").
		Select(`s.id,
			s.description_contains,
			s.priority,
			s.entry_type,
			c.code AS category_code,
			a.code AS account_code,
			ta.code AS transfer_account_code,
			s.created_at,
			s.updated_at`).
		Joins("LEFT JOIN categories c ON c.id = s.category_id").
		Joins("LEFT JOIN accounts a ON a.id = s.account_id").
		Joins("LEFT JOIN accounts ta ON ta.id = s.transfer_account_id")
}

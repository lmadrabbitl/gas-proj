package category

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
	Create(category *Category) (*Category, error)
	GetNextSortOrder(userID uuid.UUID, parentID *uuid.UUID) (int, error)
	GetByCode(userID uuid.UUID, code string, getDeactivated bool) (*Category, error)
	GetByCodes(userID uuid.UUID, codes []string, getDeactivated bool) ([]Category, error)
	GetByParentID(userID, parentID uuid.UUID, getDeactivated bool) ([]Category, error)
	GetByUser(userID uuid.UUID, getDeactivated bool) ([]Category, error)
	Update(userID uuid.UUID, code string, category *UpdateCategory) (*Category, error)
	Reorder(userID uuid.UUID, parentID *uuid.UUID, codes []string) error
	Deactivate(userID uuid.UUID, code string) error
}

type UpdateCategory struct {
	Name        *string
	Type        *CategoryType
	Description *string
	ParentID    *uuid.UUID
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (repo *repository) Create(category *Category) (*Category, error) {
	if err := repo.db.Create(category).Error; err != nil {
		var pgErr *pgconn.PgError
		log.Printf("Error in creation: %s", err.Error())
		switch {
		case errors.As(err, &pgErr):
			switch pgErr.Code {
			case "23503":
				return nil, appErr.ErrUserNotFound()
			case "23505":
				return nil, appErr.ErrDuplicateCategory()
			default:
				return nil, err
			}
		default:
			return nil, err
		}
	}
	return category, nil
}

func (repo *repository) GetByCode(userID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
	codes := []string{code}
	categories, err := repo.GetByCodes(userID, codes, getDeactivated)
	if err != nil {
		return nil, err
	}

	if len(categories) == 0 {
		return nil, appErr.ErrCategoryNotFound()
	}

	return &categories[0], nil
}

func (repo *repository) GetByCodes(userID uuid.UUID, codes []string, getDeactivated bool) ([]Category, error) {
	var categories []Category

	if len(codes) == 0 {
		return nil, appErr.ErrCategoryNotFound()
	}

	whereClause := "user_id = ? and code IN ?"
	if !getDeactivated {
		whereClause = whereClause + " and deactivated_at is null"
	}

	if err := repo.db.Where(whereClause, userID, codes).Find(&categories).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrInvalidInputWithMessage(
				fmt.Sprintf("no categories found with code %v", codes), err)
		}
		return nil, err
	}

	return categories, nil
}

func (repo *repository) GetByParentID(userID, parentID uuid.UUID, getDeactivated bool) ([]Category, error) {
	var categories []Category

	whereClause := "user_id = ? and parent_id = ?"
	if !getDeactivated {
		whereClause = whereClause + " and deactivated_at is null"
	}

	if err := repo.db.
		Where(whereClause, userID, parentID).
		Order("sort_order ASC NULLS LAST").
		Order("created_at ASC").
		Find(&categories).Error; err != nil {
		return nil, err
	}

	return categories, nil
}

func (repo *repository) GetByUser(userID uuid.UUID, getDeactivated bool) ([]Category, error) {
	var categories []Category

	whereClause := "user_id = ?"
	if !getDeactivated {
		whereClause = whereClause + " and deactivated_at is null"
	}

	if err := repo.db.
		Where(whereClause, userID).
		Order("CASE WHEN parent_id IS NULL THEN 0 ELSE 1 END").
		Order("CASE WHEN parent_id IS NULL THEN sort_order END ASC NULLS LAST").
		Order("CASE WHEN parent_id IS NULL THEN created_at END ASC").
		Order("CASE WHEN parent_id IS NOT NULL THEN parent_id::text END ASC NULLS LAST").
		Order("CASE WHEN parent_id IS NOT NULL THEN sort_order END ASC NULLS LAST").
		Order("CASE WHEN parent_id IS NOT NULL THEN created_at END ASC").
		Find(&categories).Error; err != nil {
		return nil, err
	}

	return categories, nil
}

func (repo *repository) Update(userID uuid.UUID, code string, category *UpdateCategory) (*Category, error) {

	var updated Category

	updates := map[string]interface{}{}

	if category.Name != nil {
		updates["name"] = *category.Name
	}
	if category.Description != nil {
		updates["description"] = *category.Description
	}
	if category.Type != nil {
		updates["type"] = *category.Type
	}
	if category.ParentID != nil {
		//TODO currently not allowing parent removal (parent_id = null)
		updates["parent_id"] = *category.ParentID
	}

	if len(updates) == 0 {
		return nil, appErr.ErrInvalidInputWithMessage("error on updating category: no fields to update", nil)
	}

	//limit update to name type, description and parent only
	result := repo.db.Model(&Category{}).
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

func (repo *repository) Reorder(userID uuid.UUID, parentID *uuid.UUID, codes []string) error {
	return repo.db.Transaction(func(tx *gorm.DB) error {
		for index, code := range codes {
			query := tx.Model(&Category{}).
				Where("user_id = ? and code = ? and deactivated_at is null", userID, code)

			if parentID == nil {
				query = query.Where("parent_id is null")
			} else {
				query = query.Where("parent_id = ?", *parentID)
			}

			result := query.Update("sort_order", index+1)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return appErr.ErrCategoryNotFound()
			}
		}

		return nil
	})
}

func (repo *repository) Deactivate(userID uuid.UUID, code string) error {

	result := repo.db.Model(&Category{}).
		Where("user_id = ? and code = ? and deactivated_at is null", userID, code).
		Updates(map[string]interface{}{
			"deactivated_at": time.Now(),
			"sort_order":     nil})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return appErr.ErrCategoryNotFound()
	}

	return nil
}

func (repo *repository) GetNextSortOrder(userID uuid.UUID, parentID *uuid.UUID) (int, error) {
	var nextSortOrder int

	query := repo.db.Raw(
		`SELECT COALESCE(MAX(sort_order), 0) + 1
		FROM categories
		WHERE user_id = ? AND deactivated_at IS NULL`,
		userID,
	)

	if parentID == nil {
		query = repo.db.Raw(
			`SELECT COALESCE(MAX(sort_order), 0) + 1
			FROM categories
			WHERE user_id = ? AND deactivated_at IS NULL AND parent_id IS NULL`,
			userID,
		)
	} else {
		query = repo.db.Raw(
			`SELECT COALESCE(MAX(sort_order), 0) + 1
			FROM categories
			WHERE user_id = ? AND deactivated_at IS NULL AND parent_id = ?`,
			userID,
			*parentID,
		)
	}

	if err := query.Scan(&nextSortOrder).Error; err != nil {
		return 0, err
	}

	return nextSortOrder, nil
}

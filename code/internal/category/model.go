package category

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID            uuid.UUID    `gorm:"type:uuid;primaryKey"`
	UserID        uuid.UUID    `gorm:"type:uuid;not null"`
	ParentID      *uuid.UUID   `gorm:"type:uuid"`
	Code          string       `gorm:"type:varchar(50);not null"`
	Name          string       `gorm:"type:text;not null"`
	Type          CategoryType `gorm:"type:text;not null"`
	Description   string       `gorm:"type:text"`
	SortOrder     *int         `gorm:"type:integer"`
	CreatedAt     time.Time    `gorm:"type:timestamptz;not null"`
	UpdatedAt     time.Time    `gorm:"type:timestamptz;not null"`
	DeactivatedAt *time.Time   `gorm:"type:timestamptz"`
	SubCategories []Category   `gorm:"-"`
}

type CategoryType string

const (
	CategoryTypeIncome   CategoryType = "INCOME"
	CategoryTypeExpense  CategoryType = "EXPENSE"
	CategoryTypeMovement CategoryType = "MOVEMENT" //special, can't be created via endpoint
)

func (c *Category) IsActive() bool {
	return (c != nil && c.DeactivatedAt == nil)
}

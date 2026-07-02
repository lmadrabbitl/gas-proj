package suggestion

import (
	"time"

	"github.com/google/uuid"
)

type Suggestion struct {
	ID                  uuid.UUID            `gorm:"type:uuid;primaryKey"`
	UserID              uuid.UUID            `gorm:"type:uuid;not null"`
	DescriptionContains string               `gorm:"type:text;not null"`
	Priority            int                  `gorm:"type:integer;not null"`
	EntryType           *SuggestionEntryType `gorm:"type:text"`
	CategoryID          *uuid.UUID           `gorm:"type:uuid"`
	AccountID           *uuid.UUID           `gorm:"type:uuid"`
	TransferAccountID   *uuid.UUID           `gorm:"type:uuid"`
	CreatedAt           time.Time            `gorm:"type:timestamptz;not null"`
	UpdatedAt           time.Time            `gorm:"type:timestamptz;not null"`
}

type SuggestionEntryType string

const (
	SuggestionEntryTypeRevenue  SuggestionEntryType = "REVENUE"
	SuggestionEntryTypeExpense  SuggestionEntryType = "EXPENSE"
	SuggestionEntryTypeTransfer SuggestionEntryType = "TRANSFER"
)

func (t SuggestionEntryType) IsValid() bool {
	switch t {
	case SuggestionEntryTypeRevenue, SuggestionEntryTypeExpense, SuggestionEntryTypeTransfer:
		return true
	default:
		return false
	}
}

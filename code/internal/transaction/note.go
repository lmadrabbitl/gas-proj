package transaction

import (
	"time"

	"github.com/google/uuid"
)

type TransactionNote struct {
	TransactionID uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID        uuid.UUID `gorm:"type:uuid;not null"`
	Notes         string    `gorm:"type:text;not null;default:''"`
	CreatedAt     time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt     time.Time `gorm:"type:timestamptz;not null"`
}

package transaction

import (
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID               uuid.UUID  `gorm:"type:uuid;not null"`
	CategoryID           uuid.UUID  `gorm:"type:uuid;not null"`
	Description          string     `gorm:"type:text;not null"`
	Date                 time.Time  `gorm:"type:date;not null"`
	AccountID            uuid.UUID  `gorm:"type:uuid;not null"`
	Amount               int64      `gorm:"type:bigint;not null"`
	TransferID           *int64     `gorm:"type:bigint"`
	TransferAccountID    *uuid.UUID `gorm:"type:uuid"`
	IsVisible            *bool      `gorm:"type:boolean;not null;default:true"`
	ExcludeFromDashboard bool       `gorm:"type:boolean;not null;default:false"`
	CreatedAt            time.Time  `gorm:"type:timestamptz;not null"`
	UpdatedAt            time.Time  `gorm:"type:timestamptz;not null"`
}

package account

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID                uuid.UUID        `gorm:"type:uuid;primaryKey"`
	UserID            uuid.UUID        `gorm:"type:uuid;not null"`
	Code              string           `gorm:"type:varchar(50);not null"`
	Name              string           `gorm:"type:text;not null"`
	Type              AccountType      `gorm:"type:text;not null"`
	Balance           int64            `gorm:"type:bigint;not null"`
	Currency          string           `gorm:"type:char(3);not null"`
	AssetRole         AccountAssetRole `gorm:"column:asset_role;type:text;not null;default:'NORMAL'" json:"asset_role"`
	HideFromDashboard bool             `gorm:"type:boolean;not null;default:false" json:"hide_from_dashboard"`
	SortOrder         *int             `gorm:"type:integer"`
	CreatedAt         time.Time        `gorm:"type:timestamptz;not null"`
	UpdatedAt         time.Time        `gorm:"type:timestamptz;not null"`
	DeactivatedAt     *time.Time       `gorm:"type:timestamptz"`
}

type AccountType string

const (
	AccountTypeAsset     AccountType = "ASSET"
	AccountTypeLiability AccountType = "LIABILITY"
)

type AccountAssetRole string

const (
	AccountAssetRoleNormal     AccountAssetRole = "NORMAL"
	AccountAssetRoleBrokerage  AccountAssetRole = "BROKERAGE"
	AccountAssetRoleInvestment AccountAssetRole = "INVESTMENT"
)

func (a *Account) IsActive() bool {
	return (a != nil && a.DeactivatedAt == nil)
}

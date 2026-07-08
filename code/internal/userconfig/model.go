package userconfig

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

const DefaultLanguage = "pt-BR"
const DefaultTransactionPageSize = 50
const MaxTransactionPageSize = 1000
const DefaultInvestmentRebalanceToleranceBPS = 50
const DefaultInvestmentSuggestionStrategy = InvestmentSuggestionStrategyBestNextShare

type InvestmentSuggestionStrategy string

const (
	InvestmentSuggestionStrategyBestNextShare   InvestmentSuggestionStrategy = "BEST_NEXT_SHARE"
	InvestmentSuggestionStrategyProportionalGap InvestmentSuggestionStrategy = "PROPORTIONAL_GAP"
)

type UserConfig struct {
	UserID    uuid.UUID       `gorm:"type:uuid;primaryKey"`
	Language  string          `gorm:"type:text;not null;default:'pt-BR'"`
	Settings  json.RawMessage `gorm:"type:jsonb;not null;default:'{}'"`
	CreatedAt time.Time       `gorm:"type:timestamptz;not null"`
	UpdatedAt time.Time       `gorm:"type:timestamptz;not null"`
}

type Config struct {
	Language string       `json:"language"`
	Settings ConfigValues `json:"settings"`
}

type ConfigValues struct {
	Transactions TransactionsConfig `json:"transactions"`
	Reports      ReportsConfig      `json:"reports"`
	Investments  InvestmentsConfig  `json:"investments"`
	UI           UIConfig           `json:"ui"`
}

type TransactionsConfig struct {
	List TransactionListConfig `json:"list"`
}

type TransactionListConfig struct {
	PageSize  int  `json:"page_size"`
	ShowTotal bool `json:"show_total"`
}

type ReportsConfig struct {
	ShowEmptyCategories bool `json:"show_empty_categories"`
}

type InvestmentsConfig struct {
	Portfolios  InvestmentPortfoliosConfig  `json:"portfolios"`
	Integration InvestmentIntegrationConfig `json:"integration"`
}

type InvestmentPortfoliosConfig struct {
	RebalanceToleranceBPS int                          `json:"rebalance_tolerance_basis_point"`
	SuggestionStrategy    InvestmentSuggestionStrategy `json:"suggestion_strategy"`
}

type InvestmentIntegrationConfig struct {
	WatchedCategoryIDs []uuid.UUID `json:"watched_category_ids"`
	SellGainCategoryID *uuid.UUID  `json:"sell_gain_category_id"`
	SellLossCategoryID *uuid.UUID  `json:"sell_loss_category_id"`
}

type UIConfig struct {
	HideAmounts bool `json:"hide_amounts"`
}

type UpdateConfigRequest struct {
	Language *string             `json:"language"`
	Settings *UpdateConfigValues `json:"settings"`
}

type UpdateConfigValues struct {
	Transactions *UpdateTransactionsConfig `json:"transactions"`
	Reports      *UpdateReportsConfig      `json:"reports"`
	Investments  *UpdateInvestmentsConfig  `json:"investments"`
	UI           *UpdateUIConfig           `json:"ui"`
}

type UpdateTransactionsConfig struct {
	List *UpdateTransactionListConfig `json:"list"`
}

type UpdateTransactionListConfig struct {
	PageSize  *int  `json:"page_size"`
	ShowTotal *bool `json:"show_total"`
}

type UpdateReportsConfig struct {
	ShowEmptyCategories *bool `json:"show_empty_categories"`
}

type UpdateInvestmentsConfig struct {
	Portfolios  *UpdateInvestmentPortfoliosConfig  `json:"portfolios"`
	Integration *UpdateInvestmentIntegrationConfig `json:"integration"`
}

type UpdateInvestmentPortfoliosConfig struct {
	RebalanceToleranceBPS *int                          `json:"rebalance_tolerance_basis_point"`
	SuggestionStrategy    *InvestmentSuggestionStrategy `json:"suggestion_strategy"`
}

type UpdateInvestmentIntegrationConfig struct {
	WatchedCategoryIDs []uuid.UUID `json:"watched_category_ids"`
	SellGainCategoryID *uuid.UUID  `json:"sell_gain_category_id"`
	SellLossCategoryID *uuid.UUID  `json:"sell_loss_category_id"`
}

type UpdateUIConfig struct {
	HideAmounts *bool `json:"hide_amounts"`
}

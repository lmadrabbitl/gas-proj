package userconfig

import (
	"encoding/json"
	"expense-tracker/internal/errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Service interface {
	GetConfig(userID uuid.UUID) (*Config, error)
	UpdateConfig(userID uuid.UUID, req UpdateConfigRequest) (*Config, error)
	GetTransactionListConfig(userID uuid.UUID) (TransactionListConfig, error)
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (serv *service) GetConfig(userID uuid.UUID) (*Config, error) {
	configRow, err := serv.repo.GetByUserID(userID)
	if err != nil {
		return nil, err
	}
	if configRow == nil {
		configRow, err = serv.createDefaultConfig(userID)
		if err != nil {
			return nil, err
		}
	}

	return decodeConfig(configRow)
}

func (serv *service) UpdateConfig(userID uuid.UUID, req UpdateConfigRequest) (*Config, error) {
	config, err := serv.GetConfig(userID)
	if err != nil {
		return nil, err
	}

	if req.Language != nil {
		language := strings.TrimSpace(*req.Language)
		if language == "" {
			return nil, errors.ErrInvalidInputWithCode(
				"user_config.language.required",
				"language is required",
				nil,
			)
		}
		if language != DefaultLanguage {
			return nil, errors.ErrInvalidInputWithCode(
				"user_config.language.unsupported",
				"unsupported language",
				nil,
			)
		}
		config.Language = language
	}

	if req.Settings != nil && req.Settings.Transactions != nil && req.Settings.Transactions.List != nil {
		if req.Settings.Transactions.List.PageSize != nil {
			config.Settings.Transactions.List.PageSize = clampPageSize(*req.Settings.Transactions.List.PageSize)
		}
		if req.Settings.Transactions.List.ShowTotal != nil {
			config.Settings.Transactions.List.ShowTotal = *req.Settings.Transactions.List.ShowTotal
		}
	}

	if req.Settings != nil && req.Settings.Reports != nil && req.Settings.Reports.ShowEmptyCategories != nil {
		config.Settings.Reports.ShowEmptyCategories = *req.Settings.Reports.ShowEmptyCategories
	}
	if req.Settings != nil && req.Settings.Investments != nil && req.Settings.Investments.Portfolios != nil && req.Settings.Investments.Portfolios.RebalanceToleranceBPS != nil {
		config.Settings.Investments.Portfolios.RebalanceToleranceBPS = clampRebalanceToleranceBPS(*req.Settings.Investments.Portfolios.RebalanceToleranceBPS)
	}
	if req.Settings != nil && req.Settings.Investments != nil && req.Settings.Investments.Portfolios != nil && req.Settings.Investments.Portfolios.SuggestionStrategy != nil {
		config.Settings.Investments.Portfolios.SuggestionStrategy = clampInvestmentSuggestionStrategy(*req.Settings.Investments.Portfolios.SuggestionStrategy)
	}
	if req.Settings != nil && req.Settings.Investments != nil && req.Settings.Investments.Integration != nil {
		config.Settings.Investments.Integration.WatchedCategoryIDs = normalizeWatchedCategoryIDs(req.Settings.Investments.Integration.WatchedCategoryIDs)
		config.Settings.Investments.Integration.SellGainCategoryID = normalizeOptionalUUID(req.Settings.Investments.Integration.SellGainCategoryID)
		config.Settings.Investments.Integration.SellLossCategoryID = normalizeOptionalUUID(req.Settings.Investments.Integration.SellLossCategoryID)
		config.Settings.Investments.Integration.BonificationIncomeCategoryID = normalizeOptionalUUID(req.Settings.Investments.Integration.BonificationIncomeCategoryID)
	}
	if req.Settings != nil && req.Settings.UI != nil && req.Settings.UI.HideAmounts != nil {
		config.Settings.UI.HideAmounts = *req.Settings.UI.HideAmounts
	}

	row, err := encodeConfig(userID, config)
	if err != nil {
		return nil, err
	}
	row.UpdatedAt = time.Now()
	if _, err := serv.repo.Upsert(row); err != nil {
		return nil, err
	}

	return config, nil
}

func (serv *service) GetTransactionListConfig(userID uuid.UUID) (TransactionListConfig, error) {
	config, err := serv.GetConfig(userID)
	if err != nil {
		return TransactionListConfig{}, err
	}
	return config.Settings.Transactions.List, nil
}

func (serv *service) createDefaultConfig(userID uuid.UUID) (*UserConfig, error) {
	config := defaultConfig()
	row, err := encodeConfig(userID, config)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	row.CreatedAt = now
	row.UpdatedAt = now
	return serv.repo.Upsert(row)
}

func defaultConfig() *Config {
	return &Config{
		Language: DefaultLanguage,
		Settings: ConfigValues{
			Transactions: TransactionsConfig{
				List: TransactionListConfig{
					PageSize:  DefaultTransactionPageSize,
					ShowTotal: false,
				},
			},
			Reports: ReportsConfig{
				ShowEmptyCategories: true,
			},
			Investments: InvestmentsConfig{
				Portfolios: InvestmentPortfoliosConfig{
					RebalanceToleranceBPS: DefaultInvestmentRebalanceToleranceBPS,
					SuggestionStrategy:    DefaultInvestmentSuggestionStrategy,
				},
				Integration: InvestmentIntegrationConfig{
					WatchedCategoryIDs: []uuid.UUID{},
					SellGainCategoryID: nil,
					SellLossCategoryID: nil,
					BonificationIncomeCategoryID: nil,
				},
			},
			UI: UIConfig{
				HideAmounts: false,
			},
		},
	}
}

func decodeConfig(row *UserConfig) (*Config, error) {
	config := defaultConfig()
	if row == nil {
		return config, nil
	}
	config.Language = row.Language
	if config.Language == "" {
		config.Language = DefaultLanguage
	}
	if len(row.Settings) == 0 {
		config.Settings = defaultConfig().Settings
		return config, nil
	}

	if err := json.Unmarshal(row.Settings, &config.Settings); err != nil {
		return nil, errors.ErrInvalidInputWithCode("user_config.settings.decode_failed", "failed to decode user config", err)
	}
	config.Settings.Transactions.List.PageSize = clampPageSize(config.Settings.Transactions.List.PageSize)
	if !rowSettingsHasReportsConfig(row.Settings) && !config.Settings.Reports.ShowEmptyCategories {
		config.Settings.Reports.ShowEmptyCategories = true
	}
	config.Settings.Investments.Portfolios.RebalanceToleranceBPS = clampRebalanceToleranceBPS(config.Settings.Investments.Portfolios.RebalanceToleranceBPS)
	config.Settings.Investments.Portfolios.SuggestionStrategy = clampInvestmentSuggestionStrategy(config.Settings.Investments.Portfolios.SuggestionStrategy)
	config.Settings.Investments.Integration.WatchedCategoryIDs = normalizeWatchedCategoryIDs(config.Settings.Investments.Integration.WatchedCategoryIDs)
	config.Settings.Investments.Integration.SellGainCategoryID = normalizeOptionalUUID(config.Settings.Investments.Integration.SellGainCategoryID)
	config.Settings.Investments.Integration.SellLossCategoryID = normalizeOptionalUUID(config.Settings.Investments.Integration.SellLossCategoryID)
	config.Settings.Investments.Integration.BonificationIncomeCategoryID = normalizeOptionalUUID(config.Settings.Investments.Integration.BonificationIncomeCategoryID)

	return config, nil
}

func encodeConfig(userID uuid.UUID, config *Config) (*UserConfig, error) {
	raw, err := json.Marshal(config.Settings)
	if err != nil {
		return nil, errors.ErrInvalidInputWithCode("user_config.settings.encode_failed", "failed to encode user config", err)
	}

	return &UserConfig{
		UserID:   userID,
		Language: config.Language,
		Settings: raw,
	}, nil
}

func clampPageSize(value int) int {
	if value <= 0 {
		return DefaultTransactionPageSize
	}
	if value > MaxTransactionPageSize {
		return MaxTransactionPageSize
	}
	return value
}

func rowSettingsHasReportsConfig(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}

	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return false
	}
	_, ok := decoded["reports"]
	return ok
}

func clampRebalanceToleranceBPS(value int) int {
	if value <= 0 {
		return DefaultInvestmentRebalanceToleranceBPS
	}
	if value > 500 {
		return 500
	}
	return value
}

func clampInvestmentSuggestionStrategy(value InvestmentSuggestionStrategy) InvestmentSuggestionStrategy {
	switch value {
	case InvestmentSuggestionStrategyBestNextShare, InvestmentSuggestionStrategyProportionalGap:
		return value
	default:
		return DefaultInvestmentSuggestionStrategy
	}
}

func normalizeWatchedCategoryIDs(ids []uuid.UUID) []uuid.UUID {
	if len(ids) == 0 {
		return []uuid.UUID{}
	}

	seen := make(map[uuid.UUID]struct{}, len(ids))
	normalized := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if id == uuid.Nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}

func normalizeOptionalUUID(id *uuid.UUID) *uuid.UUID {
	if id == nil || *id == uuid.Nil {
		return nil
	}
	copy := *id
	return &copy
}

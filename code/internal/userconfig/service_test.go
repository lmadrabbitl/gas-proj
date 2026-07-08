package userconfig

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

type repositoryStub struct {
	config *UserConfig
}

func (repo *repositoryStub) GetByUserID(userID uuid.UUID) (*UserConfig, error) {
	if repo.config == nil {
		return nil, nil
	}
	copy := *repo.config
	return &copy, nil
}

func (repo *repositoryStub) Upsert(config *UserConfig) (*UserConfig, error) {
	copy := *config
	repo.config = &copy
	return &copy, nil
}

func TestGetConfigDefaultsShowEmptyCategoriesForLegacyRows(t *testing.T) {
	userID := uuid.New()
	settings, err := json.Marshal(map[string]any{
		"transactions": map[string]any{
			"list": map[string]any{
				"page_size":  75,
				"show_total": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}

	serv := NewService(&repositoryStub{
		config: &UserConfig{
			UserID:    userID,
			Language:  DefaultLanguage,
			Settings:  settings,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	})

	config, err := serv.GetConfig(userID)
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}

	if !config.Settings.Reports.ShowEmptyCategories {
		t.Fatalf("expected legacy config to default show_empty_categories to true")
	}
}

func TestUpdateConfigPersistsReportsVisibilitySetting(t *testing.T) {
	userID := uuid.New()
	repo := &repositoryStub{}
	serv := NewService(repo)
	showEmptyCategories := false

	config, err := serv.UpdateConfig(userID, UpdateConfigRequest{
		Settings: &UpdateConfigValues{
			Reports: &UpdateReportsConfig{
				ShowEmptyCategories: &showEmptyCategories,
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}

	if config.Settings.Reports.ShowEmptyCategories {
		t.Fatalf("expected updated config to persist show_empty_categories=false")
	}

	stored, err := serv.GetConfig(userID)
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}

	if stored.Settings.Reports.ShowEmptyCategories {
		t.Fatalf("expected stored config to keep show_empty_categories=false")
	}
}

func TestUpdateConfigPersistsHideAmountsSetting(t *testing.T) {
	userID := uuid.New()
	repo := &repositoryStub{}
	serv := NewService(repo)
	hideAmounts := true

	config, err := serv.UpdateConfig(userID, UpdateConfigRequest{
		Settings: &UpdateConfigValues{
			UI: &UpdateUIConfig{
				HideAmounts: &hideAmounts,
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}

	if !config.Settings.UI.HideAmounts {
		t.Fatalf("expected updated config to persist hide_amounts=true")
	}

	stored, err := serv.GetConfig(userID)
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}

	if !stored.Settings.UI.HideAmounts {
		t.Fatalf("expected stored config to keep hide_amounts=true")
	}
}

func TestUpdateConfigPersistsInvestmentToleranceSetting(t *testing.T) {
	userID := uuid.New()
	repo := &repositoryStub{}
	serv := NewService(repo)
	tolerance := 125

	config, err := serv.UpdateConfig(userID, UpdateConfigRequest{
		Settings: &UpdateConfigValues{
			Investments: &UpdateInvestmentsConfig{
				Portfolios: &UpdateInvestmentPortfoliosConfig{
					RebalanceToleranceBPS: &tolerance,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}

	if config.Settings.Investments.Portfolios.RebalanceToleranceBPS != 125 {
		t.Fatalf("expected updated config to persist tolerance=125, got %d", config.Settings.Investments.Portfolios.RebalanceToleranceBPS)
	}

	stored, err := serv.GetConfig(userID)
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}

	if stored.Settings.Investments.Portfolios.RebalanceToleranceBPS != 125 {
		t.Fatalf("expected stored config to keep tolerance=125, got %d", stored.Settings.Investments.Portfolios.RebalanceToleranceBPS)
	}
}

func TestUpdateConfigPersistsInvestmentSuggestionStrategy(t *testing.T) {
	userID := uuid.New()
	repo := &repositoryStub{}
	serv := NewService(repo)
	strategy := InvestmentSuggestionStrategyProportionalGap

	config, err := serv.UpdateConfig(userID, UpdateConfigRequest{
		Settings: &UpdateConfigValues{
			Investments: &UpdateInvestmentsConfig{
				Portfolios: &UpdateInvestmentPortfoliosConfig{
					SuggestionStrategy: &strategy,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}

	if config.Settings.Investments.Portfolios.SuggestionStrategy != InvestmentSuggestionStrategyProportionalGap {
		t.Fatalf("expected updated config to persist strategy=%s, got %s", InvestmentSuggestionStrategyProportionalGap, config.Settings.Investments.Portfolios.SuggestionStrategy)
	}

	stored, err := serv.GetConfig(userID)
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}

	if stored.Settings.Investments.Portfolios.SuggestionStrategy != InvestmentSuggestionStrategyProportionalGap {
		t.Fatalf("expected stored config to keep strategy=%s, got %s", InvestmentSuggestionStrategyProportionalGap, stored.Settings.Investments.Portfolios.SuggestionStrategy)
	}
}

func TestUpdateConfigPersistsWatchedInvestmentCategoryIDs(t *testing.T) {
	userID := uuid.New()
	repo := &repositoryStub{}
	serv := NewService(repo)
	categoryID1 := uuid.New()
	categoryID2 := uuid.New()

	config, err := serv.UpdateConfig(userID, UpdateConfigRequest{
		Settings: &UpdateConfigValues{
			Investments: &UpdateInvestmentsConfig{
				Integration: &UpdateInvestmentIntegrationConfig{
					WatchedCategoryIDs: []uuid.UUID{categoryID1, categoryID1, uuid.Nil, categoryID2},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}

	if len(config.Settings.Investments.Integration.WatchedCategoryIDs) != 2 {
		t.Fatalf("expected 2 normalized watched category ids, got %d", len(config.Settings.Investments.Integration.WatchedCategoryIDs))
	}

	stored, err := serv.GetConfig(userID)
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}

	if len(stored.Settings.Investments.Integration.WatchedCategoryIDs) != 2 {
		t.Fatalf("expected stored config to keep 2 watched category ids, got %d", len(stored.Settings.Investments.Integration.WatchedCategoryIDs))
	}
}

func TestUpdateConfigPersistsSellPnLCategoryIDs(t *testing.T) {
	userID := uuid.New()
	repo := &repositoryStub{}
	serv := NewService(repo)
	gainCategoryID := uuid.New()
	lossCategoryID := uuid.New()

	config, err := serv.UpdateConfig(userID, UpdateConfigRequest{
		Settings: &UpdateConfigValues{
			Investments: &UpdateInvestmentsConfig{
				Integration: &UpdateInvestmentIntegrationConfig{
					WatchedCategoryIDs: []uuid.UUID{},
					SellGainCategoryID: &gainCategoryID,
					SellLossCategoryID: &lossCategoryID,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}

	if config.Settings.Investments.Integration.SellGainCategoryID == nil || *config.Settings.Investments.Integration.SellGainCategoryID != gainCategoryID {
		t.Fatalf("expected updated config to persist gain category id")
	}
	if config.Settings.Investments.Integration.SellLossCategoryID == nil || *config.Settings.Investments.Integration.SellLossCategoryID != lossCategoryID {
		t.Fatalf("expected updated config to persist loss category id")
	}

	stored, err := serv.GetConfig(userID)
	if err != nil {
		t.Fatalf("GetConfig returned error: %v", err)
	}

	if stored.Settings.Investments.Integration.SellGainCategoryID == nil || *stored.Settings.Investments.Integration.SellGainCategoryID != gainCategoryID {
		t.Fatalf("expected stored config to keep gain category id")
	}
	if stored.Settings.Investments.Integration.SellLossCategoryID == nil || *stored.Settings.Investments.Integration.SellLossCategoryID != lossCategoryID {
		t.Fatalf("expected stored config to keep loss category id")
	}
}

func TestUpdateConfigNormalizesNilSellPnLCategoryIDs(t *testing.T) {
	userID := uuid.New()
	repo := &repositoryStub{}
	serv := NewService(repo)
	nilUUID := uuid.Nil

	config, err := serv.UpdateConfig(userID, UpdateConfigRequest{
		Settings: &UpdateConfigValues{
			Investments: &UpdateInvestmentsConfig{
				Integration: &UpdateInvestmentIntegrationConfig{
					WatchedCategoryIDs: []uuid.UUID{},
					SellGainCategoryID: &nilUUID,
					SellLossCategoryID: nil,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}

	if config.Settings.Investments.Integration.SellGainCategoryID != nil {
		t.Fatalf("expected nil UUID gain category to normalize to nil")
	}
	if config.Settings.Investments.Integration.SellLossCategoryID != nil {
		t.Fatalf("expected nil loss category to stay nil")
	}
}

package investment

import (
	"context"
	"expense-tracker/internal/transaction"
	"expense-tracker/internal/userconfig"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestParseGoogleFinanceQuote(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.June, 22, 18, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	body := `<html><body><div>R$ 15,15</div><div>Jun 22, 5:45:00 PM GMT-3</div></body></html>`

	quote, err := parseGoogleFinanceQuote("WEGE3", body, now)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if quote.Ticker != "WEGE3" {
		t.Fatalf("expected ticker WEGE3, got %s", quote.Ticker)
	}
	if quote.CurrentPrice != 1515 {
		t.Fatalf("expected 1515 cents, got %d", quote.CurrentPrice)
	}
	expectedTimestamp := time.Date(2026, time.June, 22, 17, 45, 0, 0, now.Location())
	if !quote.Timestamp.Equal(expectedTimestamp) {
		t.Fatalf("expected timestamp %s, got %s", expectedTimestamp, quote.Timestamp)
	}
}

func TestParseBrazilianPriceToCentsSupportsDotDecimal(t *testing.T) {
	t.Parallel()

	value, err := parseBrazilianPriceToCents("11.92")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if value != 1192 {
		t.Fatalf("expected 1192 cents, got %d", value)
	}
}

func TestParseBrazilianPriceToCentsSupportsCommaDecimal(t *testing.T) {
	t.Parallel()

	value, err := parseBrazilianPriceToCents("15,15")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if value != 1515 {
		t.Fatalf("expected 1515 cents, got %d", value)
	}
}

func TestParseB3Quote(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.June, 23, 18, 0, 0, 0, time.FixedZone("BRT", -3*60*60))
	body := `<html><body><div class="asset__info__value">107,20</div><div>Atualizado às 23/06/2026 13h08</div></body></html>`

	quote, err := parseB3Quote("SPYI11", body, now)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if quote.Ticker != "SPYI11" {
		t.Fatalf("expected ticker SPYI11, got %s", quote.Ticker)
	}
	if quote.CurrentPrice != 10720 {
		t.Fatalf("expected 10720 cents, got %d", quote.CurrentPrice)
	}
	expectedTimestamp := time.Date(2026, time.June, 23, 13, 8, 0, 0, time.Local)
	if !quote.Timestamp.Equal(expectedTimestamp) {
		t.Fatalf("expected timestamp %s, got %s", expectedTimestamp, quote.Timestamp)
	}
}

func TestServiceListPositionsReturnsBaseRowsWithoutQuotes(t *testing.T) {
	t.Parallel()

	repo := &serviceTestRepo{
		listPositionsFn: func(userID uuid.UUID) ([]PositionRow, error) {
			return []PositionRow{{
				AssetCode:       "WEGE3",
				AssetName:       "WEG S.A.",
				CurrentQuantity: 200,
				AveragePrice:    1515,
			}}, nil
		},
		listPortfolioAssetsFn: func(userID uuid.UUID) ([]PortfolioAssetRow, error) {
			return []PortfolioAssetRow{{
				PortfolioCode: "dividendos",
				PortfolioName: "Dividendos",
				AssetCode:     "WEGE3",
			}}, nil
		},
	}
	service := NewService(repo, nil, nil, nil, nil)
	rows, err := service.ListPositions(uuid.New())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].CurrentPrice != nil || rows[0].QuoteUpdatedAt != nil {
		t.Fatalf("expected positions endpoint without quotes, got %+v", rows[0])
	}
	if len(rows[0].PortfolioNames) != 1 || rows[0].PortfolioNames[0] != "Dividendos" {
		t.Fatalf("expected portfolio names on positions row, got %+v", rows[0].PortfolioNames)
	}
}

func TestServiceListPositionsIncludesMatchedDividends(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categoryID := uuid.New()
	repo := &serviceTestRepo{
		listPositionsFn: func(callUserID uuid.UUID) ([]PositionRow, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			return []PositionRow{
				{AssetCode: "WEGE3", AssetName: "WEG S.A.", AssetType: AssetTypeStock, CurrentQuantity: 10, TotalCostBasis: 10000},
				{AssetCode: "TAEE11", AssetName: "Taesa", AssetType: AssetTypeStock, CurrentQuantity: 5, TotalCostBasis: 5000},
			}, nil
		},
	}
	service := NewService(repo, nil, nil, userConfigServiceStub{
		getConfigFn: func(callUserID uuid.UUID) (*userconfig.Config, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			return &userconfig.Config{
				Settings: userconfig.ConfigValues{
					Investments: userconfig.InvestmentsConfig{
						Integration: userconfig.InvestmentIntegrationConfig{
							WatchedCategoryIDs: []uuid.UUID{categoryID},
						},
					},
				},
			}, nil
		},
	}, transactionReaderStub{
		listVisibleByCategoryIDsFn: func(callUserID uuid.UUID, categoryIDs []uuid.UUID) ([]transaction.TransactionCategoryMatchRow, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			if len(categoryIDs) != 1 || categoryIDs[0] != categoryID {
				t.Fatalf("unexpected watched categories %+v", categoryIDs)
			}
			return []transaction.TransactionCategoryMatchRow{
				{Description: "DIVIDENDOS WEGE3", Amount: 1250},
				{Description: "DIVIDENDOS SEM TICKER", Amount: 300},
			}, nil
		},
	})

	rows, err := service.ListPositions(userID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if rows[0].MatchedDividends != 1250 {
		t.Fatalf("expected WEGE3 matched dividends 1250, got %d", rows[0].MatchedDividends)
	}
	if rows[1].MatchedDividends != 0 {
		t.Fatalf("expected TAEE11 matched dividends 0, got %d", rows[1].MatchedDividends)
	}
}

func TestServiceListPositionQuotesReturnsQuotes(t *testing.T) {
	t.Parallel()

	repo := &serviceTestRepo{
		listPositionsFn: func(userID uuid.UUID) ([]PositionRow, error) {
			return []PositionRow{{AssetCode: "WEGE3", CurrentQuantity: 10}}, nil
		},
	}
	quoteTime := time.Date(2026, time.June, 22, 18, 30, 0, 0, time.UTC)
	provider := quoteProviderStub{
		fetchQuotesFn: func(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
			return map[string]AssetQuote{
				"WEGE3": {
					Ticker:       "WEGE3",
					CurrentPrice: 1632,
					Timestamp:    quoteTime,
				},
			}, nil
		},
	}

	service := NewService(repo, provider, nil, nil, nil)
	rows, err := service.ListPositionQuotes(uuid.New())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].CurrentPrice != 1632 || !rows[0].QuoteUpdatedAt.Equal(quoteTime) {
		t.Fatalf("unexpected quote row %+v", rows[0])
	}
}

func TestServiceAnalyzePortfolioBuildsRows(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	quoteTime := time.Date(2026, time.June, 24, 2, 0, 0, 0, time.UTC)
	repo := &serviceTestRepo{
		getPortfolioByCodeFn: func(callUserID uuid.UUID, code string) (*Portfolio, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			if code != "dividendos" {
				t.Fatalf("expected code dividendos, got %s", code)
			}
			return &Portfolio{Code: "dividendos", Name: "Dividendos"}, nil
		},
		listPortfolioAssetsFn: func(callUserID uuid.UUID) ([]PortfolioAssetRow, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			return []PortfolioAssetRow{
				{
					PortfolioCode:       "dividendos",
					AssetCode:           "WEGE3",
					AssetName:           "WEG S.A.",
					AssetType:           AssetTypeStock,
					TargetAllocationBPS: 6000,
				},
				{
					PortfolioCode:       "dividendos",
					AssetCode:           "TAEE11",
					AssetName:           "Taesa",
					AssetType:           AssetTypeStock,
					TargetAllocationBPS: 4000,
				},
			}, nil
		},
		listPositionsFn: func(callUserID uuid.UUID) ([]PositionRow, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			return []PositionRow{
				{AssetCode: "WEGE3", AssetName: "WEG S.A.", CurrentQuantity: 10, AveragePrice: 1000, TotalCostBasis: 10000},
				{AssetCode: "TAEE11", AssetName: "Taesa", CurrentQuantity: 20, AveragePrice: 500, TotalCostBasis: 10000},
			}, nil
		},
	}
	provider := quoteProviderStub{
		fetchQuotesFn: func(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
			return map[string]AssetQuote{
				"WEGE3":  {Ticker: "WEGE3", CurrentPrice: 1200, Timestamp: quoteTime},
				"TAEE11": {Ticker: "TAEE11", CurrentPrice: 400, Timestamp: quoteTime},
			}, nil
		},
	}

	service := NewService(repo, provider, nil, nil, nil)
	analysis, err := service.AnalyzePortfolio(userID, "dividendos")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if analysis.TotalCurrentValue != 20000 {
		t.Fatalf("expected total current value 20000, got %d", analysis.TotalCurrentValue)
	}
	if analysis.TargetAllocationBasisPointTotal != 10000 {
		t.Fatalf("expected target total 10000, got %d", analysis.TargetAllocationBasisPointTotal)
	}
	if analysis.RebalanceToleranceBasisPoint != 50 {
		t.Fatalf("expected tolerance 50, got %d", analysis.RebalanceToleranceBasisPoint)
	}
	if len(analysis.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(analysis.Rows))
	}
	if analysis.Rows[0].AssetCode != "WEGE3" || analysis.Rows[0].CurrentAllocationBasisPoint != 6000 {
		t.Fatalf("unexpected first row %+v", analysis.Rows[0])
	}
	if analysis.Rows[1].AssetCode != "TAEE11" || analysis.Rows[1].AllocationDriftBasisPoint != 0 {
		t.Fatalf("unexpected second row %+v", analysis.Rows[1])
	}
}

func TestServiceAnalyzePortfolioReturnsMinimumSuggestedInvestment(t *testing.T) {
	t.Parallel()

	repo := &serviceTestRepo{
		listAssetsFn: func(callUserID uuid.UUID) ([]Asset, error) {
			return []Asset{
				{Code: "ITUB3", Name: "Itau", AssetType: AssetTypeStock},
				{Code: "BBSE3", Name: "BB Seguridade", AssetType: AssetTypeStock},
			}, nil
		},
		getPortfolioByCodeFn: func(callUserID uuid.UUID, code string) (*Portfolio, error) {
			return &Portfolio{Code: "dividendos", Name: "Dividendos"}, nil
		},
		listPortfolioAssetsFn: func(callUserID uuid.UUID) ([]PortfolioAssetRow, error) {
			return []PortfolioAssetRow{
				{PortfolioCode: "dividendos", AssetCode: "VALE3", AssetName: "Vale", AssetType: AssetTypeStock, TargetAllocationBPS: 4000, SortOrder: 1},
				{PortfolioCode: "dividendos", AssetCode: "WEGE3", AssetName: "WEG S.A.", AssetType: AssetTypeStock, TargetAllocationBPS: 3000, SortOrder: 2},
				{PortfolioCode: "dividendos", AssetCode: "TAEE11", AssetName: "Taesa", AssetType: AssetTypeStock, TargetAllocationBPS: 3000, SortOrder: 3},
			}, nil
		},
		listPositionsFn: func(callUserID uuid.UUID) ([]PositionRow, error) {
			return []PositionRow{
				{AssetCode: "VALE3", CurrentQuantity: 10, TotalCostBasis: 20000},
				{AssetCode: "WEGE3", CurrentQuantity: 1, TotalCostBasis: 1000},
				{AssetCode: "TAEE11", CurrentQuantity: 1, TotalCostBasis: 500},
			}, nil
		},
	}

	service := NewService(repo, quoteProviderStub{
		fetchQuotesFn: func(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
			return map[string]AssetQuote{
				"VALE3":  {Ticker: "VALE3", CurrentPrice: 2000, Timestamp: time.Now().UTC()},
				"WEGE3":  {Ticker: "WEGE3", CurrentPrice: 1000, Timestamp: time.Now().UTC()},
				"TAEE11": {Ticker: "TAEE11", CurrentPrice: 500, Timestamp: time.Now().UTC()},
			}, nil
		},
	}, nil, nil, nil)

	analysis, err := service.AnalyzePortfolio(uuid.New(), "dividendos")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if analysis.MinimumSuggestedInvestment == nil || *analysis.MinimumSuggestedInvestment <= 0 {
		t.Fatalf("expected positive minimum suggested investment, got %+v", analysis.MinimumSuggestedInvestment)
	}
}

func TestServiceAnalyzePortfolioRequiresQuotesForAllAssets(t *testing.T) {
	t.Parallel()

	repo := &serviceTestRepo{
		listAssetsFn: func(callUserID uuid.UUID) ([]Asset, error) {
			return []Asset{
				{Code: "ITUB3", Name: "Itau", AssetType: AssetTypeStock},
				{Code: "BBSE3", Name: "BB Seguridade", AssetType: AssetTypeStock},
			}, nil
		},
		getPortfolioByCodeFn: func(callUserID uuid.UUID, code string) (*Portfolio, error) {
			return &Portfolio{Code: "dividendos", Name: "Dividendos"}, nil
		},
		listPortfolioAssetsFn: func(callUserID uuid.UUID) ([]PortfolioAssetRow, error) {
			return []PortfolioAssetRow{
				{PortfolioCode: "dividendos", AssetCode: "WEGE3", AssetName: "WEG S.A.", AssetType: AssetTypeStock, TargetAllocationBPS: 10000},
			}, nil
		},
		listPositionsFn: func(callUserID uuid.UUID) ([]PositionRow, error) {
			return []PositionRow{{AssetCode: "WEGE3", CurrentQuantity: 10}}, nil
		},
	}

	service := NewService(repo, quoteProviderStub{
		fetchQuotesFn: func(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
			return map[string]AssetQuote{}, nil
		},
	}, nil, nil, nil)

	if _, err := service.AnalyzePortfolio(uuid.New(), "dividendos"); err == nil {
		t.Fatal("expected missing quote error")
	}
}

func TestServiceAnalyzePortfolioNormalizesTargetsToOneHundredPercent(t *testing.T) {
	t.Parallel()

	repo := &serviceTestRepo{
		getPortfolioByCodeFn: func(callUserID uuid.UUID, code string) (*Portfolio, error) {
			return &Portfolio{Code: "dividendos", Name: "Dividendos"}, nil
		},
		listPortfolioAssetsFn: func(callUserID uuid.UUID) ([]PortfolioAssetRow, error) {
			return []PortfolioAssetRow{
				{PortfolioCode: "dividendos", AssetCode: "WEGE3", AssetName: "WEG S.A.", AssetType: AssetTypeStock, TargetAllocationBPS: 6000, SortOrder: 1},
				{PortfolioCode: "dividendos", AssetCode: "TAEE11", AssetName: "Taesa", AssetType: AssetTypeStock, TargetAllocationBPS: 3415, SortOrder: 2},
			}, nil
		},
		listPositionsFn: func(callUserID uuid.UUID) ([]PositionRow, error) {
			return []PositionRow{
				{AssetCode: "WEGE3", CurrentQuantity: 10, TotalCostBasis: 10000},
				{AssetCode: "TAEE11", CurrentQuantity: 10, TotalCostBasis: 10000},
			}, nil
		},
	}

	service := NewService(repo, quoteProviderStub{
		fetchQuotesFn: func(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
			return map[string]AssetQuote{
				"WEGE3":  {Ticker: "WEGE3", CurrentPrice: 1000, Timestamp: time.Now().UTC()},
				"TAEE11": {Ticker: "TAEE11", CurrentPrice: 1000, Timestamp: time.Now().UTC()},
			}, nil
		},
	}, nil, nil, nil)

	analysis, err := service.AnalyzePortfolio(uuid.New(), "dividendos")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if analysis.TargetAllocationBasisPointTotal != 10000 {
		t.Fatalf("expected normalized target total 10000, got %d", analysis.TargetAllocationBasisPointTotal)
	}
	if analysis.Rows[0].TargetAllocationBasisPoint != 6373 {
		t.Fatalf("expected normalized target 6373 for WEGE3, got %d", analysis.Rows[0].TargetAllocationBasisPoint)
	}
	if analysis.Rows[1].TargetAllocationBasisPoint != 3627 {
		t.Fatalf("expected normalized target 3627 for TAEE11, got %d", analysis.Rows[1].TargetAllocationBasisPoint)
	}
}

func TestServiceAnalyzePortfolioBuildsIncomeSummaryFromWatchedTransactions(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	watchedCategoryID := uuid.New()
	repo := &serviceTestRepo{
		listAssetsFn: func(callUserID uuid.UUID) ([]Asset, error) {
			return []Asset{
				{Code: "ITUB3", Name: "Itau", AssetType: AssetTypeStock},
				{Code: "BBSE3", Name: "BB Seguridade", AssetType: AssetTypeStock},
			}, nil
		},
		getPortfolioByCodeFn: func(callUserID uuid.UUID, code string) (*Portfolio, error) {
			return &Portfolio{Code: "dividendos", Name: "Dividendos"}, nil
		},
		listPortfolioAssetsFn: func(callUserID uuid.UUID) ([]PortfolioAssetRow, error) {
			return []PortfolioAssetRow{
				{PortfolioCode: "dividendos", AssetCode: "ITUB3", AssetName: "Itau", AssetType: AssetTypeStock, TargetAllocationBPS: 5000, SortOrder: 1},
				{PortfolioCode: "dividendos", AssetCode: "BBSE3", AssetName: "BB Seguridade", AssetType: AssetTypeStock, TargetAllocationBPS: 5000, SortOrder: 2},
			}, nil
		},
		listPositionsFn: func(callUserID uuid.UUID) ([]PositionRow, error) {
			return []PositionRow{
				{AssetCode: "ITUB3", CurrentQuantity: 1, TotalCostBasis: 1000},
				{AssetCode: "BBSE3", CurrentQuantity: 1, TotalCostBasis: 1000},
			}, nil
		},
	}

	service := NewService(repo, quoteProviderStub{
		fetchQuotesFn: func(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
			return map[string]AssetQuote{
				"ITUB3": {Ticker: "ITUB3", CurrentPrice: 1000, Timestamp: time.Now().UTC()},
				"BBSE3": {Ticker: "BBSE3", CurrentPrice: 1000, Timestamp: time.Now().UTC()},
			}, nil
		},
	}, nil, userConfigServiceStub{
		getConfigFn: func(callUserID uuid.UUID) (*userconfig.Config, error) {
			return &userconfig.Config{
				Settings: userconfig.ConfigValues{
					Investments: userconfig.InvestmentsConfig{
						Integration: userconfig.InvestmentIntegrationConfig{
							WatchedCategoryIDs: []uuid.UUID{watchedCategoryID},
						},
					},
				},
			}, nil
		},
	}, transactionReaderStub{
		listVisibleByCategoryIDsFn: func(callUserID uuid.UUID, categoryIDs []uuid.UUID) ([]transaction.TransactionCategoryMatchRow, error) {
			return []transaction.TransactionCategoryMatchRow{
				{ID: uuid.New(), CategoryID: watchedCategoryID, Description: "DIVIDENDOS ITUB3", Amount: 1250},
				{ID: uuid.New(), CategoryID: watchedCategoryID, Description: "RENDIMENTO BBSE3", Amount: 850},
				{ID: uuid.New(), CategoryID: watchedCategoryID, Description: "DIVIDENDOS SEM TICKER", Amount: 700},
				{ID: uuid.New(), CategoryID: watchedCategoryID, Description: "ITUB3 BBSE3", Amount: 500},
			}, nil
		},
	})

	analysis, err := service.AnalyzePortfolio(userID, "dividendos")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if analysis.IncomeSummary.MatchedDividendsTotal != 3100 {
		t.Fatalf("expected matched dividends total 3100, got %d", analysis.IncomeSummary.MatchedDividendsTotal)
	}
	if analysis.IncomeSummary.MatchedTransactionsCount != 4 || analysis.IncomeSummary.UnmatchedTransactionsCount != 1 || analysis.IncomeSummary.AmbiguousTransactionsCount != 1 {
		t.Fatalf("unexpected income summary counters %+v", analysis.IncomeSummary)
	}
	if len(analysis.IncomeSummary.Rows) != 2 {
		t.Fatalf("expected 2 income rows, got %d", len(analysis.IncomeSummary.Rows))
	}
	if analysis.IncomeSummary.Rows[0].AssetCode != "ITUB3" || analysis.IncomeSummary.Rows[0].Amount != 1750 || analysis.IncomeSummary.Rows[0].TransactionCount != 2 {
		t.Fatalf("unexpected first income row %+v", analysis.IncomeSummary.Rows[0])
	}
	if analysis.IncomeSummary.Rows[1].AssetCode != "BBSE3" || analysis.IncomeSummary.Rows[1].Amount != 1350 || analysis.IncomeSummary.Rows[1].TransactionCount != 2 {
		t.Fatalf("unexpected second income row %+v", analysis.IncomeSummary.Rows[1])
	}
}

func TestServiceAnalyzePortfolioUsesAllInvestmentAssetsForUnmatchedCounters(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	watchedCategoryID := uuid.New()
	repo := &serviceTestRepo{
		listAssetsFn: func(callUserID uuid.UUID) ([]Asset, error) {
			return []Asset{
				{Code: "ITUB3", Name: "Itau", AssetType: AssetTypeStock},
				{Code: "BBSE3", Name: "BB Seguridade", AssetType: AssetTypeStock},
				{Code: "VALE3", Name: "Vale", AssetType: AssetTypeStock},
			}, nil
		},
		getPortfolioByCodeFn: func(callUserID uuid.UUID, code string) (*Portfolio, error) {
			return &Portfolio{Code: "dividendos", Name: "Dividendos"}, nil
		},
		listPortfolioAssetsFn: func(callUserID uuid.UUID) ([]PortfolioAssetRow, error) {
			return []PortfolioAssetRow{
				{PortfolioCode: "dividendos", AssetCode: "ITUB3", AssetName: "Itau", AssetType: AssetTypeStock, TargetAllocationBPS: 5000, SortOrder: 1},
				{PortfolioCode: "dividendos", AssetCode: "BBSE3", AssetName: "BB Seguridade", AssetType: AssetTypeStock, TargetAllocationBPS: 5000, SortOrder: 2},
			}, nil
		},
		listPositionsFn: func(callUserID uuid.UUID) ([]PositionRow, error) {
			return []PositionRow{
				{AssetCode: "ITUB3", CurrentQuantity: 1, TotalCostBasis: 1000},
				{AssetCode: "BBSE3", CurrentQuantity: 1, TotalCostBasis: 1000},
			}, nil
		},
	}

	service := NewService(repo, quoteProviderStub{
		fetchQuotesFn: func(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
			return map[string]AssetQuote{
				"ITUB3": {Ticker: "ITUB3", CurrentPrice: 1000, Timestamp: time.Now().UTC()},
				"BBSE3": {Ticker: "BBSE3", CurrentPrice: 1000, Timestamp: time.Now().UTC()},
			}, nil
		},
	}, nil, userConfigServiceStub{
		getConfigFn: func(callUserID uuid.UUID) (*userconfig.Config, error) {
			return &userconfig.Config{
				Settings: userconfig.ConfigValues{
					Investments: userconfig.InvestmentsConfig{
						Integration: userconfig.InvestmentIntegrationConfig{
							WatchedCategoryIDs: []uuid.UUID{watchedCategoryID},
						},
					},
				},
			}, nil
		},
	}, transactionReaderStub{
		listVisibleByCategoryIDsFn: func(callUserID uuid.UUID, categoryIDs []uuid.UUID) ([]transaction.TransactionCategoryMatchRow, error) {
			return []transaction.TransactionCategoryMatchRow{
				{Description: "DIVIDENDOS ITUB3", Amount: 100},
				{Description: "DIVIDENDOS VALE3", Amount: 200},
				{Description: "DIVIDENDOS SEM TICKER", Amount: 300},
			}, nil
		},
	})

	analysis, err := service.AnalyzePortfolio(userID, "dividendos")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if analysis.IncomeSummary.MatchedDividendsTotal != 100 {
		t.Fatalf("expected selected portfolio matched dividends total 100, got %d", analysis.IncomeSummary.MatchedDividendsTotal)
	}
	if analysis.IncomeSummary.MatchedTransactionsCount != 1 {
		t.Fatalf("expected selected portfolio matched transactions 1, got %d", analysis.IncomeSummary.MatchedTransactionsCount)
	}
	if analysis.IncomeSummary.UnmatchedTransactionsCount != 1 {
		t.Fatalf("expected unmatched transactions 1, got %d", analysis.IncomeSummary.UnmatchedTransactionsCount)
	}
	if analysis.IncomeSummary.AmbiguousTransactionsCount != 0 {
		t.Fatalf("expected ambiguous transactions 0, got %d", analysis.IncomeSummary.AmbiguousTransactionsCount)
	}
	if len(analysis.IncomeSummary.Rows) != 1 || analysis.IncomeSummary.Rows[0].AssetCode != "ITUB3" {
		t.Fatalf("unexpected filtered income rows %+v", analysis.IncomeSummary.Rows)
	}
}

func TestServiceSuggestPortfolioInvestmentBuildsPlan(t *testing.T) {
	t.Parallel()

	repo := &serviceTestRepo{
		getPortfolioByCodeFn: func(callUserID uuid.UUID, code string) (*Portfolio, error) {
			return &Portfolio{Code: "dividendos", Name: "Dividendos"}, nil
		},
		listPortfolioAssetsFn: func(callUserID uuid.UUID) ([]PortfolioAssetRow, error) {
			return []PortfolioAssetRow{
				{PortfolioCode: "dividendos", AssetCode: "VALE3", AssetName: "Vale", AssetType: AssetTypeStock, TargetAllocationBPS: 4000, SortOrder: 1},
				{PortfolioCode: "dividendos", AssetCode: "WEGE3", AssetName: "WEG S.A.", AssetType: AssetTypeStock, TargetAllocationBPS: 3000, SortOrder: 2},
				{PortfolioCode: "dividendos", AssetCode: "TAEE11", AssetName: "Taesa", AssetType: AssetTypeStock, TargetAllocationBPS: 3000, SortOrder: 3},
			}, nil
		},
		listPositionsFn: func(callUserID uuid.UUID) ([]PositionRow, error) {
			return []PositionRow{
				{AssetCode: "VALE3", CurrentQuantity: 10, TotalCostBasis: 20000},
				{AssetCode: "WEGE3", CurrentQuantity: 1, TotalCostBasis: 1000},
				{AssetCode: "TAEE11", CurrentQuantity: 1, TotalCostBasis: 500},
			}, nil
		},
	}

	service := NewService(repo, quoteProviderStub{
		fetchQuotesFn: func(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
			return map[string]AssetQuote{
				"VALE3":  {Ticker: "VALE3", CurrentPrice: 2000, Timestamp: time.Now().UTC()},
				"WEGE3":  {Ticker: "WEGE3", CurrentPrice: 1000, Timestamp: time.Now().UTC()},
				"TAEE11": {Ticker: "TAEE11", CurrentPrice: 500, Timestamp: time.Now().UTC()},
			}, nil
		},
	}, nil, nil, nil)

	suggestion, err := service.SuggestPortfolioInvestment(uuid.New(), "dividendos", 3000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if suggestion.PlannedSpend != 3000 || suggestion.CashRemainder != 0 {
		t.Fatalf("unexpected spend totals %+v", suggestion)
	}
	if len(suggestion.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(suggestion.Rows))
	}
	rowsByCode := map[string]PortfolioSuggestionRow{}
	for _, row := range suggestion.Rows {
		rowsByCode[row.AssetCode] = row
	}
	if rowsByCode["WEGE3"].BuyShares != 3 || rowsByCode["TAEE11"].BuyShares != 0 {
		t.Fatalf("expected default best-next-share strategy to concentrate on WEGE3, got %+v", suggestion.Rows)
	}
	if rowsByCode["VALE3"].BuyShares != 0 {
		t.Fatalf("expected overweight asset to receive no buys, got %+v", rowsByCode["VALE3"])
	}
}

func TestServiceSuggestPortfolioInvestmentRespectsMaxBuyPrice(t *testing.T) {
	t.Parallel()

	maxBuy := int64(900)
	repo := &serviceTestRepo{
		getPortfolioByCodeFn: func(callUserID uuid.UUID, code string) (*Portfolio, error) {
			return &Portfolio{Code: "dividendos", Name: "Dividendos"}, nil
		},
		listPortfolioAssetsFn: func(callUserID uuid.UUID) ([]PortfolioAssetRow, error) {
			return []PortfolioAssetRow{
				{PortfolioCode: "dividendos", AssetCode: "WEGE3", AssetName: "WEG S.A.", AssetType: AssetTypeStock, TargetAllocationBPS: 10000, MaxBuyPrice: &maxBuy, SortOrder: 1},
			}, nil
		},
		listPositionsFn: func(callUserID uuid.UUID) ([]PositionRow, error) {
			return []PositionRow{{AssetCode: "WEGE3", CurrentQuantity: 1, TotalCostBasis: 1000}}, nil
		},
	}

	service := NewService(repo, quoteProviderStub{
		fetchQuotesFn: func(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
			return map[string]AssetQuote{
				"WEGE3": {Ticker: "WEGE3", CurrentPrice: 1000, Timestamp: time.Now().UTC()},
			}, nil
		},
	}, nil, nil, nil)

	suggestion, err := service.SuggestPortfolioInvestment(uuid.New(), "dividendos", 3000)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if suggestion.Rows[0].BuyShares != 0 || !suggestion.Rows[0].BlockedByMaxBuyPrice {
		t.Fatalf("expected blocked suggestion row, got %+v", suggestion.Rows[0])
	}
}

func TestServiceSuggestPortfolioInvestmentUsesConfiguredStrategy(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := &serviceTestRepo{
		getPortfolioByCodeFn: func(callUserID uuid.UUID, code string) (*Portfolio, error) {
			return &Portfolio{Code: "dividendos", Name: "Dividendos"}, nil
		},
		listPortfolioAssetsFn: func(callUserID uuid.UUID) ([]PortfolioAssetRow, error) {
			return []PortfolioAssetRow{
				{PortfolioCode: "dividendos", AssetCode: "VALE3", AssetName: "Vale", AssetType: AssetTypeStock, TargetAllocationBPS: 4000, SortOrder: 1},
				{PortfolioCode: "dividendos", AssetCode: "WEGE3", AssetName: "WEG S.A.", AssetType: AssetTypeStock, TargetAllocationBPS: 3000, SortOrder: 2},
				{PortfolioCode: "dividendos", AssetCode: "TAEE11", AssetName: "Taesa", AssetType: AssetTypeStock, TargetAllocationBPS: 3000, SortOrder: 3},
			}, nil
		},
		listPositionsFn: func(callUserID uuid.UUID) ([]PositionRow, error) {
			return []PositionRow{
				{AssetCode: "VALE3", CurrentQuantity: 10, TotalCostBasis: 20000},
				{AssetCode: "WEGE3", CurrentQuantity: 1, TotalCostBasis: 1000},
				{AssetCode: "TAEE11", CurrentQuantity: 1, TotalCostBasis: 500},
			}, nil
		},
	}
	quoteProvider := quoteProviderStub{
		fetchQuotesFn: func(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
			return map[string]AssetQuote{
				"VALE3":  {Ticker: "VALE3", CurrentPrice: 2000, Timestamp: time.Now().UTC()},
				"WEGE3":  {Ticker: "WEGE3", CurrentPrice: 1000, Timestamp: time.Now().UTC()},
				"TAEE11": {Ticker: "TAEE11", CurrentPrice: 500, Timestamp: time.Now().UTC()},
			}, nil
		},
	}

	proportionalService := NewService(repo, quoteProvider, nil, userConfigServiceStub{
		getConfigFn: func(callUserID uuid.UUID) (*userconfig.Config, error) {
			return &userconfig.Config{
				Settings: userconfig.ConfigValues{
					Investments: userconfig.InvestmentsConfig{
						Portfolios: userconfig.InvestmentPortfoliosConfig{
							SuggestionStrategy: userconfig.InvestmentSuggestionStrategyProportionalGap,
						},
					},
				},
			}, nil
		},
	}, nil)
	bestNextShareService := NewService(repo, quoteProvider, nil, userConfigServiceStub{
		getConfigFn: func(callUserID uuid.UUID) (*userconfig.Config, error) {
			return &userconfig.Config{
				Settings: userconfig.ConfigValues{
					Investments: userconfig.InvestmentsConfig{
						Portfolios: userconfig.InvestmentPortfoliosConfig{
							SuggestionStrategy: userconfig.InvestmentSuggestionStrategyBestNextShare,
						},
					},
				},
			}, nil
		},
	}, nil)

	proportional, err := proportionalService.SuggestPortfolioInvestment(userID, "dividendos", 3000)
	if err != nil {
		t.Fatalf("expected no error for proportional strategy, got %v", err)
	}
	bestNextShare, err := bestNextShareService.SuggestPortfolioInvestment(userID, "dividendos", 3000)
	if err != nil {
		t.Fatalf("expected no error for best-next-share strategy, got %v", err)
	}

	proportionalByCode := map[string]PortfolioSuggestionRow{}
	for _, row := range proportional.Rows {
		proportionalByCode[row.AssetCode] = row
	}
	bestNextShareByCode := map[string]PortfolioSuggestionRow{}
	for _, row := range bestNextShare.Rows {
		bestNextShareByCode[row.AssetCode] = row
	}

	if proportionalByCode["WEGE3"].BuyShares == 0 || proportionalByCode["TAEE11"].BuyShares == 0 {
		t.Fatalf("expected proportional strategy to split between WEGE3 and TAEE11, got %+v", proportional.Rows)
	}
	if bestNextShareByCode["WEGE3"].BuyShares != 3 || bestNextShareByCode["TAEE11"].BuyShares != 0 {
		t.Fatalf("expected best-next-share strategy to concentrate on WEGE3, got %+v", bestNextShare.Rows)
	}
}

type quoteProviderStub struct {
	fetchQuotesFn func(ctx context.Context, tickers []string) (map[string]AssetQuote, error)
}

func (s quoteProviderStub) FetchQuotes(ctx context.Context, tickers []string) (map[string]AssetQuote, error) {
	return s.fetchQuotesFn(ctx, tickers)
}

type userConfigServiceStub struct {
	getConfigFn func(userID uuid.UUID) (*userconfig.Config, error)
}

func (s userConfigServiceStub) GetConfig(userID uuid.UUID) (*userconfig.Config, error) {
	return s.getConfigFn(userID)
}

type transactionReaderStub struct {
	listVisibleByCategoryIDsFn func(userID uuid.UUID, categoryIDs []uuid.UUID) ([]transaction.TransactionCategoryMatchRow, error)
}

func (s transactionReaderStub) ListVisibleByCategoryIDs(userID uuid.UUID, categoryIDs []uuid.UUID) ([]transaction.TransactionCategoryMatchRow, error) {
	return s.listVisibleByCategoryIDsFn(userID, categoryIDs)
}

type serviceTestRepo struct {
	listAssetsFn            func(userID uuid.UUID) ([]Asset, error)
	getPortfolioByCodeFn    func(userID uuid.UUID, code string) (*Portfolio, error)
	listPortfolioAssetsFn   func(userID uuid.UUID) ([]PortfolioAssetRow, error)
	listPositionsFn         func(userID uuid.UUID) ([]PositionRow, error)
	listAssetQuoteCachesFn  func(assetCodes []string) ([]AssetQuoteCache, error)
	upsertAssetQuoteCacheFn func(cache *AssetQuoteCache) error
	listAssetOperationsFn   func(userID, assetID uuid.UUID) ([]Operation, error)
	upsertPositionFn        func(position *Position) error
}

func (r *serviceTestRepo) CreateAsset(asset *Asset) (*Asset, error) { panic("unexpected call") }
func (r *serviceTestRepo) ListAssets(userID uuid.UUID) ([]Asset, error) {
	if r.listAssetsFn != nil {
		return r.listAssetsFn(userID)
	}
	return []Asset{}, nil
}
func (r *serviceTestRepo) GetAssetByCode(userID uuid.UUID, code string) (*Asset, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) UpdateAsset(userID uuid.UUID, code string, update *UpdateAsset) (*Asset, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) UpdateAssetMetadata(userID uuid.UUID, code string, update *UpdateAssetMetadata) (*Asset, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) CreatePortfolio(portfolio *Portfolio) (*Portfolio, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) ListPortfolios(userID uuid.UUID) ([]Portfolio, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) GetPortfolioByCode(userID uuid.UUID, code string) (*Portfolio, error) {
	return r.getPortfolioByCodeFn(userID, code)
}
func (r *serviceTestRepo) UpdatePortfolio(userID uuid.UUID, code string, update *UpdatePortfolio) (*Portfolio, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) DeletePortfolio(userID uuid.UUID, code string) error {
	panic("unexpected call")
}
func (r *serviceTestRepo) GetNextPortfolioSortOrder(userID uuid.UUID) (int, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) ListPortfolioAssets(userID uuid.UUID) ([]PortfolioAssetRow, error) {
	if r.listPortfolioAssetsFn != nil {
		return r.listPortfolioAssetsFn(userID)
	}
	return []PortfolioAssetRow{}, nil
}
func (r *serviceTestRepo) GetPortfolioAsset(userID, portfolioID, assetID uuid.UUID) (*PortfolioAsset, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) GetNextPortfolioAssetSortOrder(userID, portfolioID uuid.UUID) (int, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) UpsertPortfolioAsset(membership *PortfolioAsset) (*PortfolioAsset, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) DeletePortfolioAsset(userID, portfolioID, assetID uuid.UUID) error {
	panic("unexpected call")
}
func (r *serviceTestRepo) ReorderPortfolioAssets(userID, portfolioID uuid.UUID, codes []string) error {
	panic("unexpected call")
}
func (r *serviceTestRepo) CreateOperation(db *gorm.DB, operation *Operation) (*Operation, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) ListOperations(userID uuid.UUID) ([]OperationRow, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) GetOperationByID(userID, operationID uuid.UUID) (*Operation, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) UpdateOperation(db *gorm.DB, userID, operationID uuid.UUID, update *UpdateOperationModel) (*Operation, error) {
	panic("unexpected call")
}
func (r *serviceTestRepo) DeleteOperation(db *gorm.DB, userID, operationID uuid.UUID) error {
	panic("unexpected call")
}
func (r *serviceTestRepo) ListAssetOperations(db *gorm.DB, userID, assetID uuid.UUID) ([]Operation, error) {
	if r.listAssetOperationsFn != nil {
		return r.listAssetOperationsFn(userID, assetID)
	}
	panic("unexpected call")
}
func (r *serviceTestRepo) UpsertPosition(db *gorm.DB, position *Position) error {
	if r.upsertPositionFn != nil {
		return r.upsertPositionFn(position)
	}
	panic("unexpected call")
}
func (r *serviceTestRepo) DeletePosition(db *gorm.DB, userID, assetID uuid.UUID) error {
	panic("unexpected call")
}
func (r *serviceTestRepo) ListPositions(userID uuid.UUID) ([]PositionRow, error) {
	return r.listPositionsFn(userID)
}
func (r *serviceTestRepo) ListAssetQuoteCaches(assetCodes []string) ([]AssetQuoteCache, error) {
	if r.listAssetQuoteCachesFn != nil {
		return r.listAssetQuoteCachesFn(assetCodes)
	}
	return []AssetQuoteCache{}, nil
}
func (r *serviceTestRepo) UpsertAssetQuoteCache(cache *AssetQuoteCache) error {
	if r.upsertAssetQuoteCacheFn != nil {
		return r.upsertAssetQuoteCacheFn(cache)
	}
	return nil
}
func (r *serviceTestRepo) DB() *gorm.DB { panic("unexpected call") }

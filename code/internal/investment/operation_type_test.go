package investment

import (
	"errors"
	"testing"
	"time"

	appErr "expense-tracker/internal/errors"

	"github.com/google/uuid"
)

func TestCheckOperationTypeAllowsBonification(t *testing.T) {
	t.Parallel()

	if err := CheckOperationType(OperationTypeBonification); err != nil {
		t.Fatalf("expected bonification to be valid, got %v", err)
	}
}

func TestComputeNetAmountUsesBonificationCostBasis(t *testing.T) {
	t.Parallel()

	got := computeNetAmount(OperationTypeBonification, 1_000, 25)
	if got != 1_025 {
		t.Fatalf("expected bonification net amount 1025, got %d", got)
	}
}

func TestRebuildPositionAppliesBonificationToQuantityAndAveragePrice(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	assetID := uuid.New()
	var saved *Position
	repo := &serviceTestRepo{
		listAssetOperationsFn: func(userIDArg, assetIDArg uuid.UUID) ([]Operation, error) {
			if userIDArg != userID {
				t.Fatalf("expected userID %s, got %s", userID, userIDArg)
			}
			if assetIDArg != assetID {
				t.Fatalf("expected assetID %s, got %s", assetID, assetIDArg)
			}
			return []Operation{
				{OperationType: OperationTypeBuy, Quantity: 10, NetAmount: 1_000},
				{OperationType: OperationTypeBonification, Quantity: 2, NetAmount: 100},
			}, nil
		},
		upsertPositionFn: func(position *Position) error {
			saved = position
			return nil
		},
	}
	svc := &service{repo: repo}

	if err := svc.rebuildPosition(nil, userID, assetID); err != nil {
		t.Fatalf("rebuildPosition returned error: %v", err)
	}
	if saved == nil {
		t.Fatal("expected position to be saved")
	}
	if saved.CurrentQuantity != 12 {
		t.Fatalf("expected quantity 12, got %d", saved.CurrentQuantity)
	}
	if saved.TotalCostBasis != 1_100 {
		t.Fatalf("expected total cost basis 1100, got %d", saved.TotalCostBasis)
	}
	if saved.AveragePrice != 92 {
		t.Fatalf("expected average price 92, got %d", saved.AveragePrice)
	}
	if saved.RealizedPNL != 0 {
		t.Fatalf("expected realized pnl 0, got %d", saved.RealizedPNL)
	}
}

func TestRebuildPositionAllowsSameDayBuyBeforeSellForHistoryOrdering(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	assetID := uuid.New()
	var saved *Position
	sameDay := time.Date(2026, time.July, 8, 0, 0, 0, 0, time.UTC)
	repo := &serviceTestRepo{
		listAssetOperationsFn: func(userIDArg, assetIDArg uuid.UUID) ([]Operation, error) {
			if userIDArg != userID {
				t.Fatalf("expected userID %s, got %s", userID, userIDArg)
			}
			if assetIDArg != assetID {
				t.Fatalf("expected assetID %s, got %s", assetID, assetIDArg)
			}
			return []Operation{
				{ID: uuid.MustParse("00000000-0000-0000-0000-000000000003"), Date: sameDay, OperationType: OperationTypeSell, Quantity: 10, NetAmount: 1_000},
				{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001"), Date: sameDay, OperationType: OperationTypeBuy, Quantity: 9, NetAmount: 900},
				{ID: uuid.MustParse("00000000-0000-0000-0000-000000000002"), Date: sameDay, OperationType: OperationTypeBuy, Quantity: 10, NetAmount: 1_100},
			}, nil
		},
		upsertPositionFn: func(position *Position) error {
			saved = position
			return nil
		},
	}
	svc := &service{repo: repo}

	if err := svc.rebuildPosition(nil, userID, assetID); err != nil {
		t.Fatalf("rebuildPosition returned error: %v", err)
	}
	if saved == nil {
		t.Fatal("expected position to be saved")
	}
	if saved.CurrentQuantity != 9 {
		t.Fatalf("expected quantity 9, got %d", saved.CurrentQuantity)
	}
}

func TestAnnotateOperationCashMovementGroupsGroupsByBrokerageDateTickerAndType(t *testing.T) {
	t.Parallel()

	brokerageCode := "btg"
	investmentCode := "invest-a"
	date := time.Date(2026, time.July, 4, 0, 0, 0, 0, time.UTC)
	rows := []OperationRow{
		{
			ID:                    uuid.New(),
			AssetCode:             "PETR4",
			BrokerageAccountCode:  &brokerageCode,
			InvestmentAccountCode: &investmentCode,
			OperationType:         OperationTypeBuy,
			Date:                  date,
			Quantity:              100,
			GrossAmount:           1_500_00,
			NetAmount:             1_501_00,
		},
		{
			ID:                    uuid.New(),
			AssetCode:             "PETR4",
			BrokerageAccountCode:  &brokerageCode,
			InvestmentAccountCode: &investmentCode,
			OperationType:         OperationTypeBuy,
			Date:                  date,
			Quantity:              50,
			GrossAmount:           800_00,
			NetAmount:             801_00,
		},
		{
			ID:                    uuid.New(),
			AssetCode:             "PETR4",
			BrokerageAccountCode:  &brokerageCode,
			InvestmentAccountCode: &investmentCode,
			OperationType:         OperationTypeSell,
			Date:                  date,
			Quantity:              10,
			GrossAmount:           200_00,
			NetAmount:             199_00,
		},
	}

	annotateOperationCashMovementGroups(rows)

	if rows[0].CashMovementGroupKey == "" {
		t.Fatal("expected first row to receive a group key")
	}
	if rows[0].CashMovementGroupKey != rows[1].CashMovementGroupKey {
		t.Fatalf("expected matching buy rows to share group key, got %q and %q", rows[0].CashMovementGroupKey, rows[1].CashMovementGroupKey)
	}
	if rows[0].CashMovementGroupKey == rows[2].CashMovementGroupKey {
		t.Fatalf("expected sell row to have different group key, got %q", rows[2].CashMovementGroupKey)
	}
	if rows[0].CashMovementGroupSize != 2 || rows[1].CashMovementGroupSize != 2 {
		t.Fatalf("expected grouped buy size 2, got %d and %d", rows[0].CashMovementGroupSize, rows[1].CashMovementGroupSize)
	}
	if rows[0].CashMovementGroupQty != 150 || rows[1].CashMovementGroupQty != 150 {
		t.Fatalf("expected grouped buy quantity 150, got %d and %d", rows[0].CashMovementGroupQty, rows[1].CashMovementGroupQty)
	}
	if rows[0].CashMovementGroupGross != 2_300_00 || rows[0].CashMovementGroupNet != 2_302_00 {
		t.Fatalf("unexpected grouped buy totals gross=%d net=%d", rows[0].CashMovementGroupGross, rows[0].CashMovementGroupNet)
	}
	if rows[2].CashMovementGroupSize != 1 {
		t.Fatalf("expected sell row to remain single-operation group, got %d", rows[2].CashMovementGroupSize)
	}
}

func TestAnnotateOperationCashMovementGroupsSeparatesInvestmentAccounts(t *testing.T) {
	t.Parallel()

	brokerageCode := "btg"
	firstInvestmentCode := "invest-a"
	secondInvestmentCode := "invest-b"
	date := time.Date(2026, time.July, 4, 0, 0, 0, 0, time.UTC)
	rows := []OperationRow{
		{
			ID:                    uuid.New(),
			AssetCode:             "PETR4",
			BrokerageAccountCode:  &brokerageCode,
			InvestmentAccountCode: &firstInvestmentCode,
			OperationType:         OperationTypeBuy,
			Date:                  date,
			Quantity:              100,
			GrossAmount:           1_500_00,
			NetAmount:             1_501_00,
		},
		{
			ID:                    uuid.New(),
			AssetCode:             "PETR4",
			BrokerageAccountCode:  &brokerageCode,
			InvestmentAccountCode: &secondInvestmentCode,
			OperationType:         OperationTypeBuy,
			Date:                  date,
			Quantity:              50,
			GrossAmount:           800_00,
			NetAmount:             801_00,
		},
	}

	annotateOperationCashMovementGroups(rows)

	if rows[0].CashMovementGroupKey == rows[1].CashMovementGroupKey {
		t.Fatalf("expected different investment accounts to produce different group keys, got %q", rows[0].CashMovementGroupKey)
	}
	if rows[0].CashMovementGroupSize != 1 || rows[1].CashMovementGroupSize != 1 {
		t.Fatalf("expected both rows to remain single-operation groups, got %d and %d", rows[0].CashMovementGroupSize, rows[1].CashMovementGroupSize)
	}
}

func TestSummarizeOperationCashMovementGroupAggregatesQuantityAndNetAmount(t *testing.T) {
	t.Parallel()

	firstID := uuid.New()
	secondID := uuid.New()
	investmentID := uuid.New()
	date := time.Date(2026, time.July, 4, 0, 0, 0, 0, time.UTC)

	summary := summarizeOperationCashMovementGroup([]Operation{
		{
			ID:                  firstID,
			InvestmentAccountID: &investmentID,
			OperationType:       OperationTypeBuy,
			Date:                date,
			Quantity:            100,
			NetAmount:           1_501_00,
		},
		{
			ID:                  secondID,
			InvestmentAccountID: &investmentID,
			OperationType:       OperationTypeBuy,
			Date:                date,
			Quantity:            50,
			NetAmount:           801_00,
		},
	}, "PETR4")

	if summary.AssetCode != "PETR4" {
		t.Fatalf("expected PETR4 asset code, got %q", summary.AssetCode)
	}
	if summary.OperationType != OperationTypeBuy {
		t.Fatalf("expected BUY operation type, got %q", summary.OperationType)
	}
	if summary.Date != date {
		t.Fatalf("expected date %v, got %v", date, summary.Date)
	}
	if summary.Quantity != 150 {
		t.Fatalf("expected quantity 150, got %d", summary.Quantity)
	}
	if summary.NetAmount != 2_302_00 {
		t.Fatalf("expected net amount 230200, got %d", summary.NetAmount)
	}
	if summary.InvestmentAccountID == nil || *summary.InvestmentAccountID != investmentID {
		t.Fatalf("expected investment account id %s, got %v", investmentID, summary.InvestmentAccountID)
	}
	if len(summary.OperationIDs) != 2 || summary.OperationIDs[0] != firstID || summary.OperationIDs[1] != secondID {
		t.Fatalf("unexpected operation ids: %v", summary.OperationIDs)
	}
}

func TestBuildMirrorCashMovementSummaryForSellUsesReleasedCostBasisAndRealizedPnL(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	assetID := uuid.New()
	brokerageID := uuid.New()
	buyID := uuid.New()
	sellID := uuid.New()
	date := time.Date(2026, time.July, 4, 0, 0, 0, 0, time.UTC)

	repo := &serviceTestRepo{
		listAssetOperationsFn: func(callUserID, callAssetID uuid.UUID) ([]Operation, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			if callAssetID != assetID {
				t.Fatalf("expected assetID %s, got %s", assetID, callAssetID)
			}
			return []Operation{
				{ID: buyID, AssetID: assetID, OperationType: OperationTypeBuy, Date: date.AddDate(0, -1, 0), Quantity: 10, NetAmount: 1_000},
				{ID: sellID, AssetID: assetID, BrokerageAccountID: &brokerageID, OperationType: OperationTypeSell, Date: date, Quantity: 4, NetAmount: 520},
			}, nil
		},
	}
	svc := &service{repo: repo}

	summary, err := svc.buildMirrorCashMovementSummary(nil, userID, []Operation{
		{ID: sellID, AssetID: assetID, BrokerageAccountID: &brokerageID, OperationType: OperationTypeSell, Date: date, Quantity: 4, NetAmount: 520},
	}, "PETR4")
	if err != nil {
		t.Fatalf("buildMirrorCashMovementSummary returned error: %v", err)
	}

	if summary.ReleasedCostBasis != 400 {
		t.Fatalf("expected released cost basis 400, got %d", summary.ReleasedCostBasis)
	}
	if summary.RealizedPNL != 120 {
		t.Fatalf("expected realized pnl 120, got %d", summary.RealizedPNL)
	}
	if summary.BrokerageAccountID == nil || *summary.BrokerageAccountID != brokerageID {
		t.Fatalf("expected brokerage account id %s, got %v", brokerageID, summary.BrokerageAccountID)
	}
}

func TestBuildMirrorCashMovementSummaryForFinalSellConsumesRemainingCostBasis(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	assetID := uuid.New()
	sellID := uuid.New()
	date := time.Date(2026, time.July, 4, 0, 0, 0, 0, time.UTC)

	repo := &serviceTestRepo{
		listAssetOperationsFn: func(callUserID, callAssetID uuid.UUID) ([]Operation, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			if callAssetID != assetID {
				t.Fatalf("expected assetID %s, got %s", assetID, callAssetID)
			}
			return []Operation{
				{ID: uuid.New(), AssetID: assetID, OperationType: OperationTypeBuy, Date: date.AddDate(0, -1, 0), Quantity: 3, NetAmount: 100},
				{ID: sellID, AssetID: assetID, OperationType: OperationTypeSell, Date: date, Quantity: 3, NetAmount: 90},
			}, nil
		},
	}
	svc := &service{repo: repo}

	summary, err := svc.buildMirrorCashMovementSummary(nil, userID, []Operation{
		{ID: sellID, AssetID: assetID, OperationType: OperationTypeSell, Date: date, Quantity: 3, NetAmount: 90},
	}, "ITSA4")
	if err != nil {
		t.Fatalf("buildMirrorCashMovementSummary returned error: %v", err)
	}

	if summary.ReleasedCostBasis != 100 {
		t.Fatalf("expected final sell to consume exact remaining cost basis 100, got %d", summary.ReleasedCostBasis)
	}
	if summary.RealizedPNL != -10 {
		t.Fatalf("expected realized pnl -10, got %d", summary.RealizedPNL)
	}
}

func TestPreviewImportOperationsReturnsProjectedPositionRowForBuy(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	assetID := uuid.New()
	asset := &Asset{ID: assetID, Code: "MXRF11", Name: "MXRF11", AssetType: AssetTypeFII}
	repo := &serviceTestRepo{
		getAssetByCodeFn: func(callUserID uuid.UUID, code string) (*Asset, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			if code != "MXRF11" {
				t.Fatalf("expected asset code MXRF11, got %s", code)
			}
			return asset, nil
		},
		listAssetOperationsFn: func(callUserID, callAssetID uuid.UUID) ([]Operation, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			if callAssetID != assetID {
				t.Fatalf("expected assetID %s, got %s", assetID, callAssetID)
			}
			return []Operation{
				{AssetID: assetID, OperationType: OperationTypeBuy, Quantity: 10, NetAmount: 1_000},
			}, nil
		},
	}
	svc := &service{repo: repo}

	preview, err := svc.PreviewImportOperations(userID, ImportOperationsRequest{
		Operations: []ImportOperationRequest{{
			ClientRowID:           "row-1",
			AssetCode:             "mxrf11",
			BrokerageAccountCode:  "btg",
			InvestmentAccountCode: "investimentos",
			OperationType:         OperationTypeBuy,
			Date:                  time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC),
			Quantity:              2,
			UnitPrice:             150,
			TotalFeeAmount:        20,
		}},
	})
	if err != nil {
		t.Fatalf("PreviewImportOperations returned error: %v", err)
	}
	if len(preview.PositionPreviewRows) != 1 {
		t.Fatalf("expected 1 preview row, got %d", len(preview.PositionPreviewRows))
	}

	row := preview.PositionPreviewRows[0]
	if row.AssetCode != "MXRF11" {
		t.Fatalf("expected asset code MXRF11, got %s", row.AssetCode)
	}
	if row.CurrentQuantity != 10 || row.ProjectedQuantity != 12 {
		t.Fatalf("unexpected quantities current=%d projected=%d", row.CurrentQuantity, row.ProjectedQuantity)
	}
	if row.DraftChange != 2 {
		t.Fatalf("expected draft change 2, got %d", row.DraftChange)
	}
	if row.CurrentAveragePrice != 100 {
		t.Fatalf("expected current average price 100, got %d", row.CurrentAveragePrice)
	}
	if row.ProjectedAveragePrice != 110 {
		t.Fatalf("expected projected average price 110, got %d", row.ProjectedAveragePrice)
	}
}

func TestPreviewImportOperationsReplaysHistoryForBackdatedBuyBeforeLaterSell(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	assetID := uuid.New()
	asset := &Asset{ID: assetID, Code: "PETR4", Name: "PETR4", AssetType: AssetTypeStock}
	buyDate := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	sellDate := time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)
	repo := &serviceTestRepo{
		getAssetByCodeFn: func(callUserID uuid.UUID, code string) (*Asset, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			return asset, nil
		},
		listAssetOperationsFn: func(callUserID, callAssetID uuid.UUID) ([]Operation, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			if callAssetID != assetID {
				t.Fatalf("expected assetID %s, got %s", assetID, callAssetID)
			}
			return []Operation{
				{AssetID: assetID, OperationType: OperationTypeBuy, Date: buyDate, Quantity: 10, NetAmount: 1_000},
				{AssetID: assetID, OperationType: OperationTypeSell, Date: sellDate, Quantity: 5, NetAmount: 600},
			}, nil
		},
	}
	svc := &service{repo: repo}

	preview, err := svc.PreviewImportOperations(userID, ImportOperationsRequest{
		Operations: []ImportOperationRequest{{
			ClientRowID:           "row-1",
			AssetCode:             "PETR4",
			BrokerageAccountCode:  "btg",
			InvestmentAccountCode: "investimentos",
			OperationType:         OperationTypeBuy,
			Date:                  time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC),
			Quantity:              10,
			UnitPrice:             200,
			TotalFeeAmount:        0,
		}},
	})
	if err != nil {
		t.Fatalf("PreviewImportOperations returned error: %v", err)
	}

	row := preview.PositionPreviewRows[0]
	if row.CurrentQuantity != 5 || row.CurrentAveragePrice != 100 {
		t.Fatalf("unexpected current state qty=%d avg=%d", row.CurrentQuantity, row.CurrentAveragePrice)
	}
	if row.ProjectedQuantity != 15 {
		t.Fatalf("expected projected quantity 15, got %d", row.ProjectedQuantity)
	}
	if row.ProjectedAveragePrice != 150 {
		t.Fatalf("expected projected average price 150 after historical replay, got %d", row.ProjectedAveragePrice)
	}
}

func TestPreviewImportOperationsRejectsSellThatExceedsHistoricalPosition(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	assetID := uuid.New()
	asset := &Asset{ID: assetID, Code: "VALE3", Name: "VALE3", AssetType: AssetTypeStock}
	repo := &serviceTestRepo{
		getAssetByCodeFn: func(callUserID uuid.UUID, code string) (*Asset, error) {
			return asset, nil
		},
		listAssetOperationsFn: func(callUserID, callAssetID uuid.UUID) ([]Operation, error) {
			return []Operation{
				{AssetID: assetID, OperationType: OperationTypeBuy, Date: time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), Quantity: 5, NetAmount: 500},
			}, nil
		},
	}
	svc := &service{repo: repo}

	_, err := svc.PreviewImportOperations(userID, ImportOperationsRequest{
		Operations: []ImportOperationRequest{{
			ClientRowID:           "row-1",
			AssetCode:             "VALE3",
			BrokerageAccountCode:  "btg",
			InvestmentAccountCode: "investimentos",
			OperationType:         OperationTypeSell,
			Date:                  time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC),
			Quantity:              10,
			UnitPrice:             100,
			TotalFeeAmount:        0,
		}},
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var inputErr *appErr.AppError
	if !errors.As(err, &inputErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if inputErr.Code != "investment.operation.sell.exceeds.position" {
		t.Fatalf("expected sell exceeds position code, got %q", inputErr.Code)
	}
	if inputErr.Details["client_row_id"] != "row-1" {
		t.Fatalf("expected client_row_id row-1, got %#v", inputErr.Details)
	}
	if inputErr.Details["asset_code"] != "VALE3" {
		t.Fatalf("expected asset_code VALE3, got %#v", inputErr.Details)
	}
	if inputErr.Details["attempted_quantity"] != int64(10) {
		t.Fatalf("expected attempted_quantity 10, got %#v", inputErr.Details)
	}
	if inputErr.Details["available_quantity"] != int64(5) {
		t.Fatalf("expected available_quantity 5, got %#v", inputErr.Details)
	}
}

func TestPreviewImportOperationsAllowsSameDayDayTradeWhenBuysOffsetSell(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	assetID := uuid.New()
	asset := &Asset{ID: assetID, Code: "OPPR3", Name: "OPPR3", AssetType: AssetTypeStock}
	sameDay := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	repo := &serviceTestRepo{
		getAssetByCodeFn: func(callUserID uuid.UUID, code string) (*Asset, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			return asset, nil
		},
		listAssetOperationsFn: func(callUserID, callAssetID uuid.UUID) ([]Operation, error) {
			return []Operation{}, nil
		},
	}
	svc := &service{repo: repo}

	preview, err := svc.PreviewImportOperations(userID, ImportOperationsRequest{
		Operations: []ImportOperationRequest{
			{
				ClientRowID:           "row-1",
				AssetCode:             "OPPR3",
				BrokerageAccountCode:  "btg",
				InvestmentAccountCode: "investimentos",
				OperationType:         OperationTypeSell,
				Date:                  sameDay,
				Quantity:              10,
				UnitPrice:             100,
				TotalFeeAmount:        10,
			},
			{
				ClientRowID:           "row-2",
				AssetCode:             "OPPR3",
				BrokerageAccountCode:  "btg",
				InvestmentAccountCode: "investimentos",
				OperationType:         OperationTypeBuy,
				Date:                  sameDay,
				Quantity:              9,
				UnitPrice:             100,
				TotalFeeAmount:        10,
			},
			{
				ClientRowID:           "row-3",
				AssetCode:             "OPPR3",
				BrokerageAccountCode:  "btg",
				InvestmentAccountCode: "investimentos",
				OperationType:         OperationTypeBuy,
				Date:                  sameDay,
				Quantity:              10,
				UnitPrice:             100,
				TotalFeeAmount:        10,
			},
		},
	})
	if err != nil {
		t.Fatalf("PreviewImportOperations returned error: %v", err)
	}
	if len(preview.PositionPreviewRows) != 1 {
		t.Fatalf("expected 1 preview row, got %d", len(preview.PositionPreviewRows))
	}
	row := preview.PositionPreviewRows[0]
	if row.ProjectedQuantity != 9 {
		t.Fatalf("expected projected quantity 9, got %d", row.ProjectedQuantity)
	}
}

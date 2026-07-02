package investment

import (
	"testing"

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

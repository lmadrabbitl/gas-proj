package transaction

import (
	"testing"

	"github.com/google/uuid"
)

func TestCheckDescriptionQueryParamAllowsExcludedTerms(t *testing.T) {
	t.Parallel()

	err := CheckDescriptionQueryParam([]string{"-juros", "ir", "dividendo"})
	if err != nil {
		t.Fatalf("expected excluded terms to be valid, got %v", err)
	}
}

func TestCheckDescriptionQueryParamRejectsSingleCharacterExcludedTerms(t *testing.T) {
	t.Parallel()

	err := CheckDescriptionQueryParam([]string{"-a"})
	if err == nil {
		t.Fatal("expected short excluded term to be rejected")
	}
}

func TestCheckLimitQueryParamRejectsValuesAboveMax(t *testing.T) {
	t.Parallel()

	err := CheckLimitQueryParam(MaxTransactionPageSize + 1)
	if err == nil {
		t.Fatal("expected oversized page limit to be rejected")
	}
}

func TestCheckLimitQueryParamAllowsConfiguredMax(t *testing.T) {
	t.Parallel()

	err := CheckLimitQueryParam(MaxTransactionPageSize)
	if err != nil {
		t.Fatalf("expected configured max limit to be valid, got %v", err)
	}
}

func TestCheckUpdateRequestRejectsTransferAccountChangeForNonTransfer(t *testing.T) {
	t.Parallel()

	transferCode := "inter"
	err := CheckUpdateRequest(UpdateTransactionRequest{
		TransferAccountCode: &transferCode,
	}, transactionPair{
		t1:         &Transaction{},
		isTransfer: false,
	})
	if err == nil {
		t.Fatal("expected non-transfer update to reject transfer account changes")
	}
}

func TestCheckUpdateRequestRejectsTransferWithoutTransferAccount(t *testing.T) {
	t.Parallel()

	isTransfer := true
	err := CheckUpdateRequest(UpdateTransactionRequest{
		IsTransfer: &isTransfer,
	}, transactionPair{
		t1:         &Transaction{},
		isTransfer: false,
	})
	if err == nil {
		t.Fatal("expected transfer update without transfer account to be rejected")
	}
}

func TestCheckUpdateRequestAllowsExistingTransferWithoutChangingTransferAccount(t *testing.T) {
	t.Parallel()

	transferAccountID := uuid.New()
	err := CheckUpdateRequest(UpdateTransactionRequest{}, transactionPair{
		t1: &Transaction{
			TransferAccountID: &transferAccountID,
		},
		isTransfer: true,
	})
	if err != nil {
		t.Fatalf("expected existing transfer to remain valid, got %v", err)
	}
}

func TestCheckSortQueryParamAllowsCaseInsensitiveKnownValue(t *testing.T) {
	t.Parallel()

	if err := CheckSortQueryParam("amount"); err != nil {
		t.Fatalf("expected lowercase sort param to be accepted, got %v", err)
	}
}

func TestCheckOperationQueryParamRejectsUnknownOperation(t *testing.T) {
	t.Parallel()

	if err := CheckOperationQueryParam([]string{"credit", "mystery"}); err == nil {
		t.Fatal("expected unknown operation type to be rejected")
	}
}

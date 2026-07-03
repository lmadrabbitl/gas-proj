package transaction

import (
	"reflect"
	"testing"
)

func TestBuildOperationConditionsReturnsGroupedClausesForValidOperations(t *testing.T) {
	t.Parallel()

	got := buildOperationConditions([]OperationTransaction{
		CreditOperation,
		DebitOperation,
		TransferOperation,
		InvestmentOperation,
	})

	want := []string{
		"t.amount > 0 AND t.transfer_id IS NULL",
		"t.amount < 0 AND t.transfer_id IS NULL",
		"t.transfer_id IS NOT NULL",
		"iotl.transaction_id IS NOT NULL",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected operation conditions: got %v want %v", got, want)
	}
}

func TestBuildOperationConditionsSkipsUnknownOperations(t *testing.T) {
	t.Parallel()

	got := buildOperationConditions([]OperationTransaction{
		CreditOperation,
		OperationTransaction("mystery"),
	})

	want := []string{
		"t.amount > 0 AND t.transfer_id IS NULL",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected operation conditions: got %v want %v", got, want)
	}
}

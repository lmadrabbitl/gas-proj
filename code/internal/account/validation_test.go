package account

import "testing"

func TestCheckAccountName(t *testing.T) {
	t.Parallel()

	if err := CheckAccountName("Cash"); err != nil {
		t.Fatalf("expected valid name, got error: %v", err)
	}

	if err := CheckAccountName(""); err == nil {
		t.Fatal("expected empty name to fail validation")
	}
}

func TestCheckAccountCode(t *testing.T) {
	t.Parallel()

	if err := CheckAccountCode("wallet"); err != nil {
		t.Fatalf("expected valid code, got error: %v", err)
	}

	if err := CheckAccountCode(""); err == nil {
		t.Fatal("expected empty code to fail validation")
	}
}

func TestCheckAccountCodes(t *testing.T) {
	t.Parallel()

	if err := CheckAccountCodes([]string{"cash", "brokerage"}); err != nil {
		t.Fatalf("expected valid codes, got error: %v", err)
	}

	if err := CheckAccountCodes([]string{"cash", ""}); err == nil {
		t.Fatal("expected empty code in slice to fail validation")
	}
}

func TestCheckAccountType(t *testing.T) {
	t.Parallel()

	validTypes := []AccountType{AccountTypeAsset, AccountTypeLiability}
	for _, accountType := range validTypes {
		if err := CheckAccountType(accountType); err != nil {
			t.Fatalf("expected account type %q to be valid, got error: %v", accountType, err)
		}
	}

	if err := CheckAccountType(AccountType("OTHER")); err == nil {
		t.Fatal("expected unsupported account type to fail validation")
	}
}

func TestCheckAccountCurrency(t *testing.T) {
	t.Parallel()

	if err := CheckAccountCurrency("BRL"); err != nil {
		t.Fatalf("expected valid currency, got error: %v", err)
	}

	for _, currency := range []string{"", "US", "USDT"} {
		if err := CheckAccountCurrency(currency); err == nil {
			t.Fatalf("expected currency %q to fail validation", currency)
		}
	}
}

func TestCheckAccountSortOrder(t *testing.T) {
	t.Parallel()

	if err := CheckAccountSortOrder(1); err != nil {
		t.Fatalf("expected positive sort order to be valid, got error: %v", err)
	}

	for _, sortOrder := range []int{0, -1} {
		if err := CheckAccountSortOrder(sortOrder); err == nil {
			t.Fatalf("expected sort order %d to fail validation", sortOrder)
		}
	}
}

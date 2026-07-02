package category

import (
	"testing"
	"time"
)

func TestCheckCategoryName(t *testing.T) {
	t.Parallel()

	if err := CheckCategoryName("Salary"); err != nil {
		t.Fatalf("expected valid name, got error: %v", err)
	}

	if err := CheckCategoryName(""); err == nil {
		t.Fatal("expected empty name to fail validation")
	}
}

func TestCheckCategoryCode(t *testing.T) {
	t.Parallel()

	if err := CheckCategoryCode("salary"); err != nil {
		t.Fatalf("expected valid code, got error: %v", err)
	}

	if err := CheckCategoryCode(""); err == nil {
		t.Fatal("expected empty code to fail validation")
	}
}

func TestCheckCategoryCodes(t *testing.T) {
	t.Parallel()

	if err := CheckCategoryCodes([]string{"salary", "rent"}); err != nil {
		t.Fatalf("expected valid codes, got error: %v", err)
	}

	if err := CheckCategoryCodes([]string{"salary", ""}); err == nil {
		t.Fatal("expected empty code in slice to fail validation")
	}
}

func TestCheckCategoryType(t *testing.T) {
	t.Parallel()

	validTypes := []CategoryType{CategoryTypeIncome, CategoryTypeExpense, CategoryTypeMovement}
	for _, categoryType := range validTypes {
		if err := CheckCategoryType(categoryType); err != nil {
			t.Fatalf("expected category type %q to be valid, got error: %v", categoryType, err)
		}
	}

	if err := CheckCategoryType(CategoryType("OTHER")); err == nil {
		t.Fatal("expected unsupported category type to fail validation")
	}
}

func TestCategoryIsActive(t *testing.T) {
	t.Parallel()

	if (*Category)(nil).IsActive() {
		t.Fatal("expected nil category to be inactive")
	}

	active := &Category{}
	if !active.IsActive() {
		t.Fatal("expected category without deactivated timestamp to be active")
	}

	now := ptrTimeForValidation()
	inactive := &Category{DeactivatedAt: now}
	if inactive.IsActive() {
		t.Fatal("expected category with deactivated timestamp to be inactive")
	}
}

func ptrTimeForValidation() *time.Time {
	now := time.Now()
	return &now
}

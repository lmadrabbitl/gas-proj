package account

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	appErr "expense-tracker/internal/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type accountRepoStub struct {
	createFn          func(account *Account) (*Account, error)
	getNextSortFn     func(userID uuid.UUID) (int, error)
	getByCodeFn       func(userID uuid.UUID, code string) (*Account, error)
	getByCodesFn      func(userID uuid.UUID, codes []string) ([]Account, error)
	getByIDsFn        func(userID uuid.UUID, ids []uuid.UUID) ([]Account, error)
	getByUserFn       func(userID uuid.UUID) ([]Account, error)
	updateFn          func(userID uuid.UUID, code string, account *UpdateAccount) (*Account, error)
	reorderFn         func(userID uuid.UUID, codes []string) error
	deactivateFn      func(userID uuid.UUID, code string) error
	hasTransactionsFn func(userID, accountID uuid.UUID) (bool, error)
	deleteFn          func(userID, accountID uuid.UUID) error
	updateBalanceFn   func(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error
}

func (s *accountRepoStub) Create(account *Account) (*Account, error) {
	return s.createFn(account)
}

func (s *accountRepoStub) GetNextSortOrder(userID uuid.UUID) (int, error) {
	if s.getNextSortFn == nil {
		return 0, nil
	}
	return s.getNextSortFn(userID)
}

func (s *accountRepoStub) GetByCode(userID uuid.UUID, code string) (*Account, error) {
	return s.getByCodeFn(userID, code)
}

func (s *accountRepoStub) GetByCodes(userID uuid.UUID, codes []string) ([]Account, error) {
	return s.getByCodesFn(userID, codes)
}

func (s *accountRepoStub) GetByIDs(userID uuid.UUID, ids []uuid.UUID) ([]Account, error) {
	return s.getByIDsFn(userID, ids)
}

func (s *accountRepoStub) GetByUser(userID uuid.UUID) ([]Account, error) {
	return s.getByUserFn(userID)
}

func (s *accountRepoStub) Update(userID uuid.UUID, code string, account *UpdateAccount) (*Account, error) {
	return s.updateFn(userID, code, account)
}

func (s *accountRepoStub) Reorder(userID uuid.UUID, codes []string) error {
	return s.reorderFn(userID, codes)
}

func (s *accountRepoStub) Deactivate(userID uuid.UUID, code string) error {
	return s.deactivateFn(userID, code)
}

func (s *accountRepoStub) HasTransactions(userID, accountID uuid.UUID) (bool, error) {
	return s.hasTransactionsFn(userID, accountID)
}

func (s *accountRepoStub) DeletePermanently(userID, accountID uuid.UUID) error {
	return s.deleteFn(userID, accountID)
}

func (s *accountRepoStub) UpdateBalance(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error {
	return s.updateBalanceFn(db, userID, code, newBalance)
}

func TestServiceAddAccountGeneratesCodeAndDelegates(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	var created *Account

	service := NewService(&accountRepoStub{
		getByUserFn: func(gotUserID uuid.UUID) ([]Account, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			return []Account{
				{ID: uuid.New(), Code: "wallet", Name: "Wallet", DeactivatedAt: ptrTime(time.Now())},
			}, nil
		},
		getNextSortFn: func(gotUserID uuid.UUID) (int, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			return 7, nil
		},
		createFn: func(account *Account) (*Account, error) {
			created = account
			return account, nil
		},
	})

	account, err := service.AddAccount(userID, CreateAccountRequest{
		Name:              "Wallet",
		Type:              AccountTypeAsset,
		Currency:          "BRL",
		HideFromDashboard: true,
	})
	if err != nil {
		t.Fatalf("expected account creation to succeed, got error: %v", err)
	}

	if created == nil {
		t.Fatal("expected repository create to be called")
	}
	if created.UserID != userID {
		t.Fatalf("expected user ID %s, got %s", userID, created.UserID)
	}
	if created.Code != "wallet-2" {
		t.Fatalf("expected generated unique code, got %q", created.Code)
	}
	if !created.HideFromDashboard {
		t.Fatal("expected hide_from_dashboard to be preserved")
	}
	if created.SortOrder == nil || *created.SortOrder != 7 {
		t.Fatalf("expected service to assign sort order 7, got %+v", created.SortOrder)
	}
	if account != created {
		t.Fatal("expected returned account to be repository result")
	}
}

func TestServiceAddAccountRejectsInvalidInputBeforeRepository(t *testing.T) {
	t.Parallel()

	called := false
	service := NewService(&accountRepoStub{
		createFn: func(account *Account) (*Account, error) {
			called = true
			return account, nil
		},
	})

	_, err := service.AddAccount(uuid.New(), CreateAccountRequest{
		Name:     "",
		Type:     AccountTypeAsset,
		Currency: "BRL",
	})
	if err == nil {
		t.Fatal("expected invalid account name to fail")
	}
	if called {
		t.Fatal("expected repository create not to be called for invalid input")
	}
}

func TestServiceAddAccountRejectsDuplicateActiveName(t *testing.T) {
	t.Parallel()

	service := NewService(&accountRepoStub{
		getByUserFn: func(userID uuid.UUID) ([]Account, error) {
			return []Account{
				{ID: uuid.New(), Code: "bradesco", Name: "Bradesco"},
				{ID: uuid.New(), Code: "bradesco-antigo", Name: "Bradesco", DeactivatedAt: ptrTime(time.Now())},
			}, nil
		},
		createFn: func(account *Account) (*Account, error) {
			t.Fatal("expected create not to be called for duplicate active name")
			return nil, nil
		},
	})

	_, err := service.AddAccount(uuid.New(), CreateAccountRequest{
		Name:     "bradesco",
		Type:     AccountTypeAsset,
		Currency: "BRL",
	})
	if err == nil || !strings.Contains(err.Error(), "active account") {
		t.Fatalf("expected duplicate active name error, got %v", err)
	}
}

func TestServiceAddAccountAllowsReuseOfDeactivatedNameAndNormalizesAccents(t *testing.T) {
	t.Parallel()

	var created *Account
	service := NewService(&accountRepoStub{
		getByUserFn: func(userID uuid.UUID) ([]Account, error) {
			return []Account{
				{ID: uuid.New(), Code: "cartao-bradesco", Name: "Cartão Bradesco", DeactivatedAt: ptrTime(time.Now())},
			}, nil
		},
		getNextSortFn: func(userID uuid.UUID) (int, error) {
			return 4, nil
		},
		createFn: func(account *Account) (*Account, error) {
			created = account
			return account, nil
		},
	})

	_, err := service.AddAccount(uuid.New(), CreateAccountRequest{
		Name:     "Cartão Bradesco",
		Type:     AccountTypeLiability,
		Currency: "BRL",
	})
	if err != nil {
		t.Fatalf("expected create to succeed, got %v", err)
	}
	if created == nil || created.Code != "cartao-bradesco-2" {
		t.Fatalf("expected accent-normalized code with suffix, got %+v", created)
	}
}

func TestServiceGetAccountByCodeNormalizesLookup(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	called := false

	service := NewService(&accountRepoStub{
		getByCodeFn: func(gotUserID uuid.UUID, code string) (*Account, error) {
			called = true
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			if code != "cash" {
				t.Fatalf("expected lowercase code, got %q", code)
			}
			return &Account{UserID: gotUserID, Code: code}, nil
		},
	})

	account, err := service.GetAccountByCode(userID, "CaSh")
	if err != nil {
		t.Fatalf("expected lookup to succeed, got error: %v", err)
	}
	if !called {
		t.Fatal("expected repository lookup to be called")
	}
	if account.Code != "cash" {
		t.Fatalf("expected returned account code to be lowercase, got %q", account.Code)
	}
}

func TestServiceGetAccountsDelegatesToRepository(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	expected := []Account{{Code: "cash"}, {Code: "broker"}}

	service := NewService(&accountRepoStub{
		getByUserFn: func(gotUserID uuid.UUID) ([]Account, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			return expected, nil
		},
	})

	got, err := service.GetAccounts(userID)
	if err != nil {
		t.Fatalf("expected account list to succeed, got %v", err)
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestServiceGetAccountsByCodeNormalizesAllCodes(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	service := NewService(&accountRepoStub{
		getByCodesFn: func(gotUserID uuid.UUID, codes []string) ([]Account, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			expected := []string{"cash", "broker"}
			if !reflect.DeepEqual(codes, expected) {
				t.Fatalf("expected normalized codes %v, got %v", expected, codes)
			}
			return []Account{{Code: "cash"}, {Code: "broker"}}, nil
		},
	})

	got, err := service.GetAccountsByCode(userID, []string{"CaSh", "BROKER"})
	if err != nil {
		t.Fatalf("expected account lookup to succeed, got %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(got))
	}
}

func TestServiceGetAccountsByIDDelegates(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountIDs := []uuid.UUID{uuid.New(), uuid.New()}
	service := NewService(&accountRepoStub{
		getByIDsFn: func(gotUserID uuid.UUID, ids []uuid.UUID) ([]Account, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			if !reflect.DeepEqual(ids, accountIDs) {
				t.Fatalf("expected ids %v, got %v", accountIDs, ids)
			}
			return []Account{{ID: ids[0]}, {ID: ids[1]}}, nil
		},
	})

	got, err := service.GetAccountsByID(userID, accountIDs)
	if err != nil {
		t.Fatalf("expected account ID lookup to succeed, got %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(got))
	}
}

func TestServiceUpdateAccountRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()

	service := NewService(&accountRepoStub{})

	_, err := service.UpdateAccount(uuid.New(), "cash", UpdateAccountRequest{})
	if err == nil {
		t.Fatal("expected empty update to fail")
	}
	if !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("expected at-least-one-field validation error, got: %v", err)
	}
}

func TestServiceUpdateAccountRejectsDeactivatedAccount(t *testing.T) {
	t.Parallel()

	now := time.Now()
	service := NewService(&accountRepoStub{
		getByCodeFn: func(userID uuid.UUID, code string) (*Account, error) {
			return &Account{Code: code, DeactivatedAt: &now}, nil
		},
		updateFn: func(userID uuid.UUID, code string, account *UpdateAccount) (*Account, error) {
			t.Fatal("expected repository update not to be called for deactivated account")
			return nil, nil
		},
	})

	name := "Updated"
	_, err := service.UpdateAccount(uuid.New(), "cash", UpdateAccountRequest{Name: &name})
	var appError *appErr.AppError
	if !errors.As(err, &appError) || appError.Code != appErr.ErrAccountDeactivated().Code {
		t.Fatalf("expected account deactivated error, got: %v", err)
	}
}

func TestServiceUpdateAccountBuildsEditableFieldsOnly(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	existing := &Account{UserID: userID, Code: "cash"}
	name := "Updated"
	currency := "USD"
	accountType := AccountTypeLiability
	hide := true

	var updatedPayload *UpdateAccount
	var gotCode string

	service := NewService(&accountRepoStub{
		getByCodeFn: func(gotUserID uuid.UUID, code string) (*Account, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			return existing, nil
		},
		getByUserFn: func(gotUserID uuid.UUID) ([]Account, error) {
			return []Account{{ID: existing.ID, Code: "cash", Name: "Wallet"}}, nil
		},
		updateFn: func(gotUserID uuid.UUID, code string, account *UpdateAccount) (*Account, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			gotCode = code
			updatedPayload = account
			return &Account{UserID: gotUserID, Code: code, Name: *account.Name, Currency: *account.Currency, Type: *account.Type, HideFromDashboard: *account.HideFromDashboard}, nil
		},
	})

	account, err := service.UpdateAccount(userID, "CaSh", UpdateAccountRequest{
		Name:              &name,
		Currency:          &currency,
		Type:              &accountType,
		HideFromDashboard: &hide,
	})
	if err != nil {
		t.Fatalf("expected update to succeed, got error: %v", err)
	}
	if gotCode != "cash" {
		t.Fatalf("expected lowercase update code, got %q", gotCode)
	}
	if updatedPayload == nil || updatedPayload.Name == nil || *updatedPayload.Name != name {
		t.Fatal("expected update payload to include name")
	}
	if updatedPayload.Currency == nil || *updatedPayload.Currency != currency {
		t.Fatal("expected update payload to include currency")
	}
	if updatedPayload.Type == nil || *updatedPayload.Type != accountType {
		t.Fatal("expected update payload to include type")
	}
	if updatedPayload.HideFromDashboard == nil || *updatedPayload.HideFromDashboard != hide {
		t.Fatal("expected update payload to include hide_from_dashboard")
	}
	if account.Type != accountType {
		t.Fatalf("expected returned account type %q, got %q", accountType, account.Type)
	}
}

func TestServiceUpdateAccountRejectsDuplicateActiveName(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	currentID := uuid.New()
	name := "Bradesco"

	service := NewService(&accountRepoStub{
		getByCodeFn: func(gotUserID uuid.UUID, code string) (*Account, error) {
			return &Account{ID: currentID, UserID: gotUserID, Code: code, Name: "Conta antiga"}, nil
		},
		getByUserFn: func(gotUserID uuid.UUID) ([]Account, error) {
			return []Account{
				{ID: currentID, Code: "conta-antiga", Name: "Conta antiga"},
				{ID: uuid.New(), Code: "bradesco", Name: "Bradesco"},
			}, nil
		},
		updateFn: func(userID uuid.UUID, code string, account *UpdateAccount) (*Account, error) {
			t.Fatal("expected update not to be called for duplicate active name")
			return nil, nil
		},
	})

	_, err := service.UpdateAccount(userID, "conta-antiga", UpdateAccountRequest{Name: &name})
	if err == nil || !strings.Contains(err.Error(), "active account") {
		t.Fatalf("expected duplicate active name error, got %v", err)
	}
}

func TestServiceReorderAccountsRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	testCases := []struct {
		name  string
		codes []string
	}{
		{name: "empty list", codes: nil},
		{name: "duplicate code", codes: []string{"cash", "CASH"}},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := NewService(&accountRepoStub{
				getByUserFn: func(userID uuid.UUID) ([]Account, error) {
					t.Fatal("expected account list not to be fetched for invalid input")
					return nil, nil
				},
			})

			err := service.ReorderAccounts(userID, tc.codes)
			if err == nil {
				t.Fatal("expected reorder validation to fail")
			}
		})
	}
}

func TestServiceReorderAccountsRequiresAllActiveAccountsExactlyOnce(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	service := NewService(&accountRepoStub{
		getByUserFn: func(gotUserID uuid.UUID) ([]Account, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			return []Account{
				{Code: "cash"},
				{Code: "broker"},
				{Code: "old", DeactivatedAt: ptrTime(time.Now())},
			}, nil
		},
		reorderFn: func(userID uuid.UUID, codes []string) error {
			t.Fatal("expected reorder repository not to be called for incomplete active list")
			return nil
		},
	})

	err := service.ReorderAccounts(userID, []string{"cash"})
	if err == nil {
		t.Fatal("expected reorder to fail when active accounts are missing")
	}
	if !strings.Contains(err.Error(), "all active accounts exactly once") {
		t.Fatalf("expected missing active accounts error, got: %v", err)
	}
}

func TestServiceReorderAccountsNormalizesCodesAndCallsRepository(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	var reordered []string

	service := NewService(&accountRepoStub{
		getByUserFn: func(userID uuid.UUID) ([]Account, error) {
			return []Account{
				{Code: "cash"},
				{Code: "broker"},
				{Code: "old", DeactivatedAt: ptrTime(time.Now())},
			}, nil
		},
		reorderFn: func(gotUserID uuid.UUID, codes []string) error {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			reordered = append([]string(nil), codes...)
			return nil
		},
	})

	err := service.ReorderAccounts(userID, []string{"BROKER", "Cash"})
	if err != nil {
		t.Fatalf("expected reorder to succeed, got error: %v", err)
	}
	if len(reordered) != 2 || reordered[0] != "broker" || reordered[1] != "cash" {
		t.Fatalf("expected normalized reorder payload, got %v", reordered)
	}
}

func TestServiceDeactivateAccountNormalizesCode(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	var gotCode string

	service := NewService(&accountRepoStub{
		deactivateFn: func(gotUserID uuid.UUID, code string) error {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			gotCode = code
			return nil
		},
	})

	if err := service.DeactivateAccount(userID, "CaSh"); err != nil {
		t.Fatalf("expected deactivate to succeed, got error: %v", err)
	}
	if gotCode != "cash" {
		t.Fatalf("expected lowercase deactivate code, got %q", gotCode)
	}
}

func TestServiceDeleteAccountPermanentlyNormalizesCodeAndDelegates(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	var gotAccountID uuid.UUID

	service := NewService(&accountRepoStub{
		getByCodeFn: func(gotUserID uuid.UUID, code string) (*Account, error) {
			if gotUserID != userID || code != "cash" {
				t.Fatalf("unexpected get account args: %s %q", gotUserID, code)
			}
			return &Account{ID: accountID, Code: code, DeactivatedAt: ptrTime(time.Now())}, nil
		},
		hasTransactionsFn: func(gotUserID, gotAccountID uuid.UUID) (bool, error) {
			if gotUserID != userID || gotAccountID != accountID {
				t.Fatalf("unexpected has transactions args: %s %s", gotUserID, gotAccountID)
			}
			return false, nil
		},
		deleteFn: func(gotUserID, accountID uuid.UUID) error {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			gotAccountID = accountID
			return nil
		},
	})

	if err := service.DeleteAccountPermanently(userID, "CaSh"); err != nil {
		t.Fatalf("expected permanent delete to succeed, got error: %v", err)
	}
	if gotAccountID != accountID {
		t.Fatalf("expected delete to receive account ID %s, got %s", accountID, gotAccountID)
	}
}

func TestServiceDeleteAccountPermanentlyRejectsAccountsWithTransactions(t *testing.T) {
	t.Parallel()

	accountID := uuid.New()
	service := NewService(&accountRepoStub{
		getByCodeFn: func(userID uuid.UUID, code string) (*Account, error) {
			return &Account{ID: accountID, Code: code, DeactivatedAt: ptrTime(time.Now())}, nil
		},
		hasTransactionsFn: func(userID, gotAccountID uuid.UUID) (bool, error) {
			if gotAccountID != accountID {
				t.Fatalf("expected account ID %s, got %s", accountID, gotAccountID)
			}
			return true, nil
		},
		deleteFn: func(userID, accountID uuid.UUID) error {
			t.Fatal("expected delete not to be called when account has transactions")
			return nil
		},
	})

	err := service.DeleteAccountPermanently(uuid.New(), "cash")
	if err == nil || !strings.Contains(err.Error(), "associated transactions") {
		t.Fatalf("expected transaction association validation error, got %v", err)
	}
}

func TestServiceDeleteAccountPermanentlyRejectsActiveAccounts(t *testing.T) {
	t.Parallel()

	service := NewService(&accountRepoStub{
		getByCodeFn: func(userID uuid.UUID, code string) (*Account, error) {
			return &Account{Code: code}, nil
		},
		hasTransactionsFn: func(userID, accountID uuid.UUID) (bool, error) {
			t.Fatal("expected transaction lookup not to be called for active account")
			return false, nil
		},
		deleteFn: func(userID, accountID uuid.UUID) error {
			t.Fatal("expected delete not to be called for active account")
			return nil
		},
	})

	err := service.DeleteAccountPermanently(uuid.New(), "cash")
	if err == nil || !strings.Contains(err.Error(), "only deactivated accounts") {
		t.Fatalf("expected deactivated account validation error, got %v", err)
	}
}

func TestServiceUpdateBalanceDelegatesRepositoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("boom")
	service := NewService(&accountRepoStub{
		updateBalanceFn: func(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error {
			return expectedErr
		},
	})

	err := service.UpdateBalance(nil, uuid.New(), "cash", 10)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected repository error to be returned, got: %v", err)
	}
}

func TestServiceUpdateBalanceDelegatesSuccess(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	service := NewService(&accountRepoStub{
		updateBalanceFn: func(db *gorm.DB, gotUserID uuid.UUID, code string, newBalance int64) error {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			if code != "cash" || newBalance != 12345 {
				t.Fatalf("unexpected balance update args: %q %d", code, newBalance)
			}
			return nil
		},
	})

	if err := service.UpdateBalance(nil, userID, "cash", 12345); err != nil {
		t.Fatalf("expected balance update to succeed, got %v", err)
	}
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

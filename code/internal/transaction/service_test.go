package transaction

import (
	"expense-tracker/internal/account"
	"expense-tracker/internal/category"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type transactionRepoStub struct {
	createSingleFn             func(db *gorm.DB, userID uuid.UUID, transaction *Transaction) (*TransactionResponseItem, error)
	getByIDFn                  func(db *gorm.DB, userID, transactionID uuid.UUID) (*Transaction, error)
	getByIDsFn                 func(db *gorm.DB, userID uuid.UUID, transactionIDs []uuid.UUID) ([]*Transaction, error)
	getDTOByIDFn               func(db *gorm.DB, userID, transactionID uuid.UUID) (*TransactionResponseItem, error)
	getByTransferIDFn          func(db *gorm.DB, userID uuid.UUID, transferID int64) ([]Transaction, error)
	getByUserFn                func(db *gorm.DB, userID uuid.UUID, filter *FilterTransactionQuery, showAll bool) (*TransactionResponse, error)
	updateFn                   func(db *gorm.DB, userID, transactionID uuid.UUID, transaction *UpdateTransaction) (*TransactionResponseItem, error)
	deleteFn                   func(db *gorm.DB, userID, transactionID uuid.UUID) error
	getNextTransferIDFn        func(db *gorm.DB) (int64, error)
	calculateAccountBalanceFn  func(db *gorm.DB, userID uuid.UUID, accCode string) (int64, error)
	listVisibleByCategoryIDsFn func(userID uuid.UUID, categoryIDs []uuid.UUID) ([]TransactionCategoryMatchRow, error)
	upsertTransactionNoteFn    func(db *gorm.DB, userID, transactionID uuid.UUID, notes string) error
	deleteTransactionNoteFn    func(db *gorm.DB, userID, transactionID uuid.UUID) error
}

func (s *transactionRepoStub) CreateSingle(db *gorm.DB, userID uuid.UUID, transaction *Transaction) (*TransactionResponseItem, error) {
	if s.createSingleFn == nil {
		return nil, nil
	}
	return s.createSingleFn(db, userID, transaction)
}

func (s *transactionRepoStub) CreateMany(db *gorm.DB, userID uuid.UUID, transaction []*Transaction) ([]*TransactionResponseItem, error) {
	return nil, nil
}

func (s *transactionRepoStub) GetByID(db *gorm.DB, userID, transactionID uuid.UUID) (*Transaction, error) {
	return s.getByIDFn(db, userID, transactionID)
}

func (s *transactionRepoStub) GetByIDs(db *gorm.DB, userID uuid.UUID, transactionIDs []uuid.UUID) ([]*Transaction, error) {
	if s.getByIDsFn == nil {
		return nil, nil
	}
	return s.getByIDsFn(db, userID, transactionIDs)
}

func (s *transactionRepoStub) GetDTOByID(db *gorm.DB, userID, transactionID uuid.UUID) (*TransactionResponseItem, error) {
	if s.getDTOByIDFn == nil {
		return &TransactionResponseItem{ID: transactionID}, nil
	}
	return s.getDTOByIDFn(db, userID, transactionID)
}

func (s *transactionRepoStub) GetDTOByIDs(db *gorm.DB, userID uuid.UUID, transactionIDs []uuid.UUID) ([]*TransactionResponseItem, error) {
	return nil, nil
}

func (s *transactionRepoStub) GetByTransferID(db *gorm.DB, userID uuid.UUID, transferID int64) ([]Transaction, error) {
	return s.getByTransferIDFn(db, userID, transferID)
}

func (s *transactionRepoStub) GetByUser(db *gorm.DB, userID uuid.UUID, filter *FilterTransactionQuery, showAll bool) (*TransactionResponse, error) {
	return s.getByUserFn(db, userID, filter, showAll)
}

func (s *transactionRepoStub) Update(db *gorm.DB, userID, transactionID uuid.UUID, transaction *UpdateTransaction) (*TransactionResponseItem, error) {
	if s.updateFn == nil {
		return nil, nil
	}
	return s.updateFn(db, userID, transactionID, transaction)
}

func (s *transactionRepoStub) Delete(db *gorm.DB, userID, transactionID uuid.UUID) error {
	if s.deleteFn == nil {
		return nil
	}
	return s.deleteFn(db, userID, transactionID)
}

func (s *transactionRepoStub) GetNextTransferID(db *gorm.DB) (int64, error) {
	if s.getNextTransferIDFn == nil {
		return 0, nil
	}
	return s.getNextTransferIDFn(db)
}

func (s *transactionRepoStub) CalculateAccountBalance(db *gorm.DB, userID uuid.UUID, accCode string) (int64, error) {
	return s.calculateAccountBalanceFn(db, userID, accCode)
}

func (s *transactionRepoStub) ListVisibleByCategoryIDs(userID uuid.UUID, categoryIDs []uuid.UUID) ([]TransactionCategoryMatchRow, error) {
	if s.listVisibleByCategoryIDsFn == nil {
		return []TransactionCategoryMatchRow{}, nil
	}
	return s.listVisibleByCategoryIDsFn(userID, categoryIDs)
}

func (s *transactionRepoStub) UpsertTransactionNote(db *gorm.DB, userID, transactionID uuid.UUID, notes string) error {
	if s.upsertTransactionNoteFn == nil {
		return nil
	}
	return s.upsertTransactionNoteFn(db, userID, transactionID, notes)
}

func (s *transactionRepoStub) DeleteTransactionNote(db *gorm.DB, userID, transactionID uuid.UUID) error {
	if s.deleteTransactionNoteFn == nil {
		return nil
	}
	return s.deleteTransactionNoteFn(db, userID, transactionID)
}

type transactionAccountServiceStub struct {
	getAccountByCodeFn  func(userID uuid.UUID, code string) (*account.Account, error)
	getAccountsByCodeFn func(userID uuid.UUID, codes []string) ([]account.Account, error)
	getAccountsByIDFn   func(userID uuid.UUID, ids []uuid.UUID) ([]account.Account, error)
	updateBalanceFn     func(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error
}

func (s *transactionAccountServiceStub) GetAccountByCode(userID uuid.UUID, code string) (*account.Account, error) {
	return s.getAccountByCodeFn(userID, code)
}

func (s *transactionAccountServiceStub) GetAccountsByCode(userID uuid.UUID, codes []string) ([]account.Account, error) {
	return s.getAccountsByCodeFn(userID, codes)
}

func (s *transactionAccountServiceStub) GetAccountsByID(userID uuid.UUID, ids []uuid.UUID) ([]account.Account, error) {
	return s.getAccountsByIDFn(userID, ids)
}

func (s *transactionAccountServiceStub) UpdateBalance(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error {
	return s.updateBalanceFn(db, userID, code, newBalance)
}

type transactionCategoryReaderStub struct {
	getCategoryByCodeFn   func(userID uuid.UUID, code string) (*category.Category, error)
	getCategoriesByCodeFn func(userID uuid.UUID, codes []string) ([]category.Category, error)
}

func (s *transactionCategoryReaderStub) GetCategoryByCode(userID uuid.UUID, code string) (*category.Category, error) {
	return s.getCategoryByCodeFn(userID, code)
}

func (s *transactionCategoryReaderStub) GetCategoriesByCode(userID uuid.UUID, codes []string) ([]category.Category, error) {
	return s.getCategoriesByCodeFn(userID, codes)
}

func TestNormalizeDescriptionForStorageUppercasesValue(t *testing.T) {
	got := normalizeDescriptionForStorage("Mc Donald's")

	if got != "MC DONALD'S" {
		t.Fatalf("expected uppercased description, got %q", got)
	}
}

func TestNormalizeDescriptionForStorageKeepsCaseInsensitiveUnicode(t *testing.T) {
	got := normalizeDescriptionForStorage("pão de açúcar")

	if got != "PÃO DE AÇÚCAR" {
		t.Fatalf("expected unicode uppercasing, got %q", got)
	}
}

func TestTransactionSortFieldValidationAndFallback(t *testing.T) {
	if !SortByAmount.IsValid() || !SortByTransactionDate.IsValid() || !SortByUpdatedDate.IsValid() {
		t.Fatal("expected supported sort fields to be valid")
	}
	if TransactionSortField("unknown").IsValid() {
		t.Fatal("expected unknown sort field to be invalid")
	}
	if got := NewTransactionSortField("amount"); got != SortByAmount {
		t.Fatalf("expected amount sort, got %q", got)
	}
	if got := NewTransactionSortField("something-else"); got != SortByUpdatedDate {
		t.Fatalf("expected fallback to updated sort, got %q", got)
	}
}

func TestDefaultFilterTransactionQueryUsesExpectedDefaults(t *testing.T) {
	filter := defaultFilterTransactionQuery()

	if filter.PageNumber != 1 || filter.PageSize != 50 {
		t.Fatalf("unexpected pagination defaults: %+v", filter)
	}
	if filter.SortColumn != SortByTransactionDate || filter.AscSortOrder {
		t.Fatalf("unexpected sort defaults: %+v", filter)
	}
}

func TestGetAccountsToUpdateReturnsUniqueAccountCodes(t *testing.T) {
	transferAccount := "savings"
	transactions := []*TransactionResponseItem{
		{AccountCode: "checking"},
		{AccountCode: "checking", TransferAccountCode: &transferAccount},
		{AccountCode: "wallet"},
	}

	got := getAccountsToUpdate(transactions)
	gotSet := map[string]bool{}
	for _, accountCode := range got {
		gotSet[accountCode] = true
	}

	for _, expected := range []string{"checking", "savings", "wallet"} {
		if !gotSet[expected] {
			t.Fatalf("expected account code %q in %v", expected, got)
		}
	}
	if len(gotSet) != 3 {
		t.Fatalf("expected exactly 3 unique account codes, got %v", got)
	}
}

func TestGetTransactionsNormalizesFilterBeforeRepositoryCall(t *testing.T) {
	userID := uuid.New()
	accountID := uuid.New()
	categoryID := uuid.New()
	startDate := time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, time.March, 4, 0, 0, 0, 0, time.UTC)
	minAmount := int64(-15)
	maxAmount := int64(75)
	pageNumber := 3
	pageSize := 25
	sortColumn := "amount"
	ascSort := true

	service := &transactionService{
		repo: &transactionRepoStub{
			getByUserFn: func(db *gorm.DB, gotUserID uuid.UUID, filter *FilterTransactionQuery, showAll bool) (*TransactionResponse, error) {
				if db != nil {
					t.Fatalf("expected nil db, got %v", db)
				}
				if gotUserID != userID {
					t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
				}
				if showAll {
					t.Fatal("expected hidden transactions to remain filtered out")
				}
				if filter.PageNumber != pageNumber || filter.PageSize != pageSize {
					t.Fatalf("unexpected pagination: %+v", filter)
				}
				if filter.SortColumn != SortByAmount || !filter.AscSortOrder {
					t.Fatalf("unexpected sorting: %+v", filter)
				}
				if filter.MinAmount == nil || *filter.MinAmount != 15 {
					t.Fatalf("expected normalized positive min amount, got %+v", filter.MinAmount)
				}
				if filter.MaxAmount == nil || *filter.MaxAmount != 75 {
					t.Fatalf("unexpected max amount: %+v", filter.MaxAmount)
				}
				if filter.StartDate != &startDate || filter.EndDate != &endDate {
					t.Fatalf("unexpected date pointers: %+v", filter)
				}
				if !reflect.DeepEqual(filter.AccountIDs, []uuid.UUID{accountID}) {
					t.Fatalf("unexpected account IDs: %v", filter.AccountIDs)
				}
				if !reflect.DeepEqual(filter.CategoryIDs, []uuid.UUID{categoryID}) {
					t.Fatalf("unexpected category IDs: %v", filter.CategoryIDs)
				}
				if !reflect.DeepEqual(filter.Description, []string{"-juros", "bonus"}) {
					t.Fatalf("unexpected description terms: %v", filter.Description)
				}
				if !reflect.DeepEqual(filter.OperationType, []OperationTransaction{CreditOperation, TransferOperation}) {
					t.Fatalf("unexpected operation types: %v", filter.OperationType)
				}
				return &TransactionResponse{}, nil
			},
		},
		accService: &transactionAccountServiceStub{
			getAccountByCodeFn: func(userID uuid.UUID, code string) (*account.Account, error) { return nil, nil },
			getAccountsByCodeFn: func(gotUserID uuid.UUID, codes []string) ([]account.Account, error) {
				if gotUserID != userID {
					t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
				}
				if !reflect.DeepEqual(codes, []string{"wallet"}) {
					t.Fatalf("unexpected account codes: %v", codes)
				}
				return []account.Account{{ID: accountID, Code: "wallet"}}, nil
			},
			getAccountsByIDFn: func(userID uuid.UUID, ids []uuid.UUID) ([]account.Account, error) { return nil, nil },
			updateBalanceFn:   func(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error { return nil },
		},
		catReader: &transactionCategoryReaderStub{
			getCategoryByCodeFn: func(userID uuid.UUID, code string) (*category.Category, error) { return nil, nil },
			getCategoriesByCodeFn: func(gotUserID uuid.UUID, codes []string) ([]category.Category, error) {
				if gotUserID != userID {
					t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
				}
				if !reflect.DeepEqual(codes, []string{"food"}) {
					t.Fatalf("unexpected category codes: %v", codes)
				}
				return []category.Category{{ID: categoryID, Code: "food"}}, nil
			},
		},
	}

	_, err := service.GetTransactions(userID, FilterTransactionRequest{
		PageNumber:       &pageNumber,
		PageSize:         &pageSize,
		SortColumn:       &sortColumn,
		AscSortOrder:     &ascSort,
		OperationType:    []string{"credit", "transfer"},
		AccountCodes:     []string{"wallet"},
		CategoryCodes:    []string{"food"},
		DescriptionTerms: []string{"-juros", "bonus"},
		MinAmount:        &minAmount,
		MaxAmount:        &maxAmount,
		StartDate:        &startDate,
		EndDate:          &endDate,
	})
	if err != nil {
		t.Fatalf("GetTransactions returned error: %v", err)
	}
}

func TestGetTransactionsRejectsStartDateAfterEndDate(t *testing.T) {
	startDate := time.Date(2025, time.March, 5, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, time.March, 4, 0, 0, 0, 0, time.UTC)

	service := &transactionService{}
	_, err := service.GetTransactions(uuid.New(), FilterTransactionRequest{
		StartDate: &startDate,
		EndDate:   &endDate,
	})
	if err == nil {
		t.Fatal("expected invalid date range to be rejected")
	}
}

func TestGetTransactionsRejectsMinAmountHigherThanMaxAfterNormalization(t *testing.T) {
	minAmount := int64(-200)
	maxAmount := int64(100)

	service := &transactionService{}
	_, err := service.GetTransactions(uuid.New(), FilterTransactionRequest{
		MinAmount: &minAmount,
		MaxAmount: &maxAmount,
	})
	if err == nil {
		t.Fatal("expected invalid amount range to be rejected")
	}
}

func TestGetTransactionsRejectsUnknownOperationType(t *testing.T) {
	service := &transactionService{}
	_, err := service.GetTransactions(uuid.New(), FilterTransactionRequest{
		OperationType: []string{"credit", "mystery"},
	})
	if err == nil {
		t.Fatal("expected unknown operation type to be rejected")
	}
}

func TestGetTransactionByIDDelegatesToRepository(t *testing.T) {
	userID := uuid.New()
	txID := uuid.New()
	service := &transactionService{
		repo: &transactionRepoStub{
			getDTOByIDFn: func(db *gorm.DB, gotUserID, transactionID uuid.UUID) (*TransactionResponseItem, error) {
				if gotUserID != userID || transactionID != txID {
					t.Fatalf("unexpected args: %s %s", gotUserID, transactionID)
				}
				return &TransactionResponseItem{ID: txID, Description: "Salary"}, nil
			},
		},
	}

	got, err := service.GetTransactionByID(userID, txID)
	if err != nil {
		t.Fatalf("expected transaction lookup to succeed, got %v", err)
	}
	if got.ID != txID {
		t.Fatalf("expected returned transaction ID %s, got %s", txID, got.ID)
	}
}

func TestGetTransactionPairLoadsHiddenTransferSibling(t *testing.T) {
	userID := uuid.New()
	txID := uuid.New()
	otherID := uuid.New()
	transferID := int64(88)
	service := &transactionService{
		repo: &transactionRepoStub{
			getByIDFn: func(db *gorm.DB, gotUserID, transactionID uuid.UUID) (*Transaction, error) {
				return &Transaction{ID: txID, UserID: gotUserID, TransferID: &transferID}, nil
			},
			getByTransferIDFn: func(db *gorm.DB, gotUserID uuid.UUID, gotTransferID int64) ([]Transaction, error) {
				return []Transaction{
					{ID: txID, UserID: gotUserID, TransferID: &transferID},
					{ID: otherID, UserID: gotUserID, TransferID: &transferID},
				}, nil
			},
		},
	}

	pair, err := service.getTransactionPair(userID, txID)
	if err != nil {
		t.Fatalf("expected transaction pair lookup to succeed, got %v", err)
	}
	if !pair.isTransfer || pair.t2 == nil || pair.t2.ID != otherID {
		t.Fatalf("expected hidden transfer pair, got %+v", pair)
	}
}

func TestGetAccountCodesFromIdsDelegatesToAccountService(t *testing.T) {
	userID := uuid.New()
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	service := &transactionService{
		accService: &transactionAccountServiceStub{
			getAccountByCodeFn:  func(userID uuid.UUID, code string) (*account.Account, error) { return nil, nil },
			getAccountsByCodeFn: func(userID uuid.UUID, codes []string) ([]account.Account, error) { return nil, nil },
			getAccountsByIDFn: func(gotUserID uuid.UUID, gotIDs []uuid.UUID) ([]account.Account, error) {
				if gotUserID != userID || !reflect.DeepEqual(gotIDs, ids) {
					t.Fatalf("unexpected args: %s %v", gotUserID, gotIDs)
				}
				return []account.Account{{Code: "cash"}, {Code: "broker"}}, nil
			},
			updateBalanceFn: func(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error { return nil },
		},
	}

	got, err := service.getAccountCodesFromIds(userID, ids)
	if err != nil {
		t.Fatalf("expected account code lookup to succeed, got %v", err)
	}
	if !reflect.DeepEqual(got, []string{"cash", "broker"}) {
		t.Fatalf("unexpected codes: %v", got)
	}
}

func TestBoolPtrAndCoalesceReturnExpectedValues(t *testing.T) {
	if got := BoolPtr(true); got == nil || !*got {
		t.Fatalf("expected BoolPtr(true) to return true pointer, got %+v", got)
	}
	a, b := 1, 2
	if got := coalesce[int](nil, &a, &b); got == nil || *got != 1 {
		t.Fatalf("expected coalesce to return first non-nil pointer, got %+v", got)
	}
}

func TestUpdateAllAccountBalancesUsesCalculatedBalances(t *testing.T) {
	userID := uuid.New()
	calculatedBalances := map[string]int64{
		"wallet": 120,
		"bank":   -45,
	}
	updatedBalances := map[string]int64{}

	service := &transactionService{
		repo: &transactionRepoStub{
			getByUserFn: func(db *gorm.DB, userID uuid.UUID, filter *FilterTransactionQuery, showAll bool) (*TransactionResponse, error) {
				return nil, nil
			},
			calculateAccountBalanceFn: func(db *gorm.DB, gotUserID uuid.UUID, accCode string) (int64, error) {
				if gotUserID != userID {
					t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
				}
				return calculatedBalances[accCode], nil
			},
		},
		accService: &transactionAccountServiceStub{
			getAccountByCodeFn:  func(userID uuid.UUID, code string) (*account.Account, error) { return nil, nil },
			getAccountsByCodeFn: func(userID uuid.UUID, codes []string) ([]account.Account, error) { return nil, nil },
			getAccountsByIDFn:   func(userID uuid.UUID, ids []uuid.UUID) ([]account.Account, error) { return nil, nil },
			updateBalanceFn: func(db *gorm.DB, gotUserID uuid.UUID, code string, newBalance int64) error {
				if gotUserID != userID {
					t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
				}
				updatedBalances[code] = newBalance
				return nil
			},
		},
	}

	err := service.updateAllAccountBalances(nil, userID, []string{"wallet", "bank"})
	if err != nil {
		t.Fatalf("updateAllAccountBalances returned error: %v", err)
	}

	if !reflect.DeepEqual(updatedBalances, calculatedBalances) {
		t.Fatalf("expected updated balances %v, got %v", calculatedBalances, updatedBalances)
	}
}

func TestUpdateTransactionsBulkRejectsMixedTransferSelection(t *testing.T) {
	userID := uuid.New()
	visible := true
	firstID := uuid.New()
	secondID := uuid.New()
	transferID := int64(10)

	service := &transactionService{
		repo: &transactionRepoStub{
			getByIDFn: func(db *gorm.DB, gotUserID, transactionID uuid.UUID) (*Transaction, error) {
				if transactionID == firstID {
					return &Transaction{ID: firstID, UserID: gotUserID, AccountID: uuid.New(), IsVisible: &visible}, nil
				}
				return &Transaction{ID: secondID, UserID: gotUserID, AccountID: uuid.New(), TransferID: &transferID, IsVisible: &visible}, nil
			},
			getByTransferIDFn: func(db *gorm.DB, gotUserID uuid.UUID, gotTransferID int64) ([]Transaction, error) {
				otherID := uuid.New()
				return []Transaction{
					{ID: secondID, UserID: gotUserID, AccountID: uuid.New(), TransferID: &transferID, IsVisible: &visible},
					{ID: otherID, UserID: gotUserID, AccountID: uuid.New(), TransferID: &transferID, IsVisible: BoolPtr(false)},
				}, nil
			},
		},
	}

	description := "mercado"
	_, err := service.UpdateTransactionsBulk(userID, []uuid.UUID{firstID, secondID}, UpdateTransactionRequest{
		Description: &description,
	})
	if err == nil {
		t.Fatal("expected mixed transfer selection to be rejected")
	}
}

func TestUpdateTransactionsBulkUpdatesUniqueAccountsOnce(t *testing.T) {
	userID := uuid.New()
	visible := true
	accountID := uuid.New()
	firstID := uuid.New()
	secondID := uuid.New()
	updateCalls := 0
	calcCalls := 0
	balanceCalls := 0

	service := &transactionService{
		repo: &transactionRepoStub{
			getByIDFn: func(db *gorm.DB, gotUserID, transactionID uuid.UUID) (*Transaction, error) {
				return &Transaction{
					ID:          transactionID,
					UserID:      gotUserID,
					AccountID:   accountID,
					Description: "OLD",
					IsVisible:   &visible,
				}, nil
			},
			updateFn: func(db *gorm.DB, gotUserID, transactionID uuid.UUID, transaction *UpdateTransaction) (*TransactionResponseItem, error) {
				updateCalls += 1
				if transaction.Description == nil || *transaction.Description != "MERCADO" {
					t.Fatalf("expected normalized description update, got %+v", transaction.Description)
				}
				return &TransactionResponseItem{ID: transactionID, Description: "MERCADO"}, nil
			},
			calculateAccountBalanceFn: func(db *gorm.DB, gotUserID uuid.UUID, accCode string) (int64, error) {
				calcCalls += 1
				if accCode != "wallet" {
					t.Fatalf("unexpected account code: %s", accCode)
				}
				return 999, nil
			},
		},
		accService: &transactionAccountServiceStub{
			getAccountByCodeFn:  func(userID uuid.UUID, code string) (*account.Account, error) { return nil, nil },
			getAccountsByCodeFn: func(userID uuid.UUID, codes []string) ([]account.Account, error) { return nil, nil },
			getAccountsByIDFn: func(gotUserID uuid.UUID, ids []uuid.UUID) ([]account.Account, error) {
				if len(ids) != 1 || ids[0] != accountID {
					t.Fatalf("unexpected account ids: %v", ids)
				}
				return []account.Account{{ID: accountID, Code: "wallet"}}, nil
			},
			updateBalanceFn: func(db *gorm.DB, gotUserID uuid.UUID, code string, newBalance int64) error {
				balanceCalls += 1
				if code != "wallet" || newBalance != 999 {
					t.Fatalf("unexpected balance update: %s %d", code, newBalance)
				}
				return nil
			},
		},
	}

	description := "mercado"
	updatedCount, err := service.UpdateTransactionsBulk(userID, []uuid.UUID{firstID, secondID}, UpdateTransactionRequest{
		Description: &description,
	})
	if err != nil {
		t.Fatalf("expected bulk update to succeed, got %v", err)
	}
	if updatedCount != 2 {
		t.Fatalf("expected updated count 2, got %d", updatedCount)
	}
	if updateCalls != 2 {
		t.Fatalf("expected 2 update calls, got %d", updateCalls)
	}
	if calcCalls != 1 {
		t.Fatalf("expected 1 balance calculation, got %d", calcCalls)
	}
	if balanceCalls != 1 {
		t.Fatalf("expected 1 balance update, got %d", balanceCalls)
	}
}

func TestEnsureMirrorTransactionUpdateAllowedRejectsAccountChanges(t *testing.T) {
	userID := uuid.New()
	transactionID := uuid.New()
	accountCode := "brokerage"
	transferAccountCode := "investment"

	service := &transactionService{
		repo: &transactionRepoStub{
			getDTOByIDFn: func(db *gorm.DB, gotUserID, gotTransactionID uuid.UUID) (*TransactionResponseItem, error) {
				if gotUserID != userID || gotTransactionID != transactionID {
					t.Fatalf("unexpected args: %s %s", gotUserID, gotTransactionID)
				}
				return &TransactionResponseItem{
					ID:                 transactionID,
					IsInvestmentMirror: true,
				}, nil
			},
		},
	}

	err := service.ensureMirrorTransactionUpdateAllowed(userID, transactionID, UpdateTransactionRequest{
		AccountCode: &accountCode,
	})
	if err == nil {
		t.Fatal("expected linked mirror account change to be rejected")
	}

	err = service.ensureMirrorTransactionUpdateAllowed(userID, transactionID, UpdateTransactionRequest{
		TransferAccountCode: &transferAccountCode,
	})
	if err == nil {
		t.Fatal("expected linked mirror transfer account change to be rejected")
	}
}

func TestEnsureMirrorTransactionUpdateAllowedAllowsUnprotectedNoop(t *testing.T) {
	userID := uuid.New()
	transactionID := uuid.New()

	service := &transactionService{
		repo: &transactionRepoStub{
			getDTOByIDFn: func(db *gorm.DB, gotUserID, gotTransactionID uuid.UUID) (*TransactionResponseItem, error) {
				if gotUserID != userID || gotTransactionID != transactionID {
					t.Fatalf("unexpected args: %s %s", gotUserID, gotTransactionID)
				}
				return &TransactionResponseItem{
					ID:                 transactionID,
					IsInvestmentMirror: true,
				}, nil
			},
		},
	}

	if err := service.ensureMirrorTransactionUpdateAllowed(userID, transactionID, UpdateTransactionRequest{}); err != nil {
		t.Fatalf("expected noop linked mirror update check to pass, got %v", err)
	}
}

func TestDeleteTransactionsBulkDeletesTransferPairOnceAndUpdatesBalances(t *testing.T) {
	userID := uuid.New()
	firstID := uuid.New()
	secondID := uuid.New()
	transferID := int64(22)
	accountA := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	accountB := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	deleted := []uuid.UUID{}
	updatedBalances := []string{}

	service := &transactionService{
		repo: &transactionRepoStub{
			getDTOByIDFn: func(db *gorm.DB, gotUserID, transactionID uuid.UUID) (*TransactionResponseItem, error) {
				if gotUserID != userID {
					t.Fatalf("unexpected user ID: %s", gotUserID)
				}
				return &TransactionResponseItem{ID: transactionID, IsInvestmentMirror: false}, nil
			},
			getByIDFn: func(db *gorm.DB, gotUserID, transactionID uuid.UUID) (*Transaction, error) {
				if gotUserID != userID {
					t.Fatalf("unexpected user ID: %s", gotUserID)
				}
				switch transactionID {
				case firstID:
					return &Transaction{ID: firstID, UserID: gotUserID, AccountID: accountA, TransferID: &transferID, TransferAccountID: uuidPtr(accountB)}, nil
				case secondID:
					return &Transaction{ID: secondID, UserID: gotUserID, AccountID: accountB, TransferID: &transferID, TransferAccountID: uuidPtr(accountA)}, nil
				default:
					t.Fatalf("unexpected transaction ID %s", transactionID)
					return nil, nil
				}
			},
			getByTransferIDFn: func(db *gorm.DB, gotUserID uuid.UUID, gotTransferID int64) ([]Transaction, error) {
				if gotUserID != userID || gotTransferID != transferID {
					t.Fatalf("unexpected transfer lookup: %s %d", gotUserID, gotTransferID)
				}
				return []Transaction{
					{ID: firstID, UserID: gotUserID, AccountID: accountA, TransferID: &transferID, TransferAccountID: uuidPtr(accountB)},
					{ID: secondID, UserID: gotUserID, AccountID: accountB, TransferID: &transferID, TransferAccountID: uuidPtr(accountA)},
				}, nil
			},
			deleteFn: func(db *gorm.DB, gotUserID, transactionID uuid.UUID) error {
				if gotUserID != userID {
					t.Fatalf("unexpected user ID: %s", gotUserID)
				}
				deleted = append(deleted, transactionID)
				return nil
			},
			calculateAccountBalanceFn: func(db *gorm.DB, gotUserID uuid.UUID, accCode string) (int64, error) {
				if gotUserID != userID {
					t.Fatalf("unexpected user ID: %s", gotUserID)
				}
				return 0, nil
			},
		},
		accService: &transactionAccountServiceStub{
			getAccountByCodeFn:  func(userID uuid.UUID, code string) (*account.Account, error) { return nil, nil },
			getAccountsByCodeFn: func(userID uuid.UUID, codes []string) ([]account.Account, error) { return nil, nil },
			getAccountsByIDFn: func(gotUserID uuid.UUID, ids []uuid.UUID) ([]account.Account, error) {
				if gotUserID != userID {
					t.Fatalf("unexpected user ID: %s", gotUserID)
				}
				return []account.Account{
					{ID: accountA, Code: "cash"},
					{ID: accountB, Code: "broker"},
				}, nil
			},
			updateBalanceFn: func(db *gorm.DB, gotUserID uuid.UUID, code string, newBalance int64) error {
				if gotUserID != userID {
					t.Fatalf("unexpected user ID: %s", gotUserID)
				}
				updatedBalances = append(updatedBalances, code)
				return nil
			},
		},
	}

	if err := service.DeleteTransactionsBulk(userID, []uuid.UUID{firstID, secondID}); err != nil {
		t.Fatalf("expected bulk delete to succeed, got %v", err)
	}

	if len(deleted) != 2 {
		t.Fatalf("expected 2 deleted rows, got %v", deleted)
	}
	if !containsUUID(deleted, firstID) || !containsUUID(deleted, secondID) {
		t.Fatalf("unexpected deleted rows: %v", deleted)
	}
	if len(updatedBalances) != 2 {
		t.Fatalf("expected 2 balance updates, got %v", updatedBalances)
	}
}

func TestDeleteTransactionsBulkRejectsInvestmentMirror(t *testing.T) {
	userID := uuid.New()
	transactionID := uuid.New()

	service := &transactionService{
		repo: &transactionRepoStub{
			getDTOByIDFn: func(db *gorm.DB, gotUserID, gotTransactionID uuid.UUID) (*TransactionResponseItem, error) {
				if gotUserID != userID || gotTransactionID != transactionID {
					t.Fatalf("unexpected args: %s %s", gotUserID, gotTransactionID)
				}
				return &TransactionResponseItem{
					ID:                 transactionID,
					IsInvestmentMirror: true,
				}, nil
			},
		},
	}

	if err := service.DeleteTransactionsBulk(userID, []uuid.UUID{transactionID}); err == nil {
		t.Fatal("expected linked mirror bulk delete to be rejected")
	}
}

func TestUpdateTransactionCanUpdateNotesWithoutChangingTransactionFields(t *testing.T) {
	userID := uuid.New()
	transactionID := uuid.New()
	savedNotes := ""

	service := &transactionService{
		repo: &transactionRepoStub{
			getDTOByIDFn: func(db *gorm.DB, gotUserID, gotTransactionID uuid.UUID) (*TransactionResponseItem, error) {
				if gotUserID != userID || gotTransactionID != transactionID {
					t.Fatalf("unexpected dto lookup args: %s %s", gotUserID, gotTransactionID)
				}
				return &TransactionResponseItem{ID: transactionID, Notes: savedNotes}, nil
			},
			upsertTransactionNoteFn: func(db *gorm.DB, gotUserID, gotTransactionID uuid.UUID, notes string) error {
				if gotUserID != userID || gotTransactionID != transactionID {
					t.Fatalf("unexpected note args: %s %s", gotUserID, gotTransactionID)
				}
				savedNotes = notes
				return nil
			},
		},
	}

	updated, err := service.UpdateTransaction(userID, transactionID, UpdateTransactionRequest{
		Notes: stringPtr("observação nova"),
	})
	if err != nil {
		t.Fatalf("expected notes-only update to succeed, got error: %v", err)
	}
	if updated == nil || updated.Notes != "observação nova" {
		t.Fatalf("unexpected updated dto: %+v", updated)
	}
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

func stringPtr(value string) *string {
	return &value
}

func containsUUID(values []uuid.UUID, target uuid.UUID) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

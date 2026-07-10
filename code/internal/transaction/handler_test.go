package transaction

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"expense-tracker/internal/userconfig"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type handlerServiceStub struct {
	addFn        func(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error)
	listFn       func(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error)
	getFn        func(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error)
	updateFn     func(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error)
	bulkUpdateFn func(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error)
	deleteFn     func(userID uuid.UUID, transactionID uuid.UUID) error
	bulkDeleteFn func(userID uuid.UUID, transactionIDs []uuid.UUID) error
}

type handlerConfigServiceStub struct{}

func (handlerConfigServiceStub) GetTransactionListConfig(userID uuid.UUID) (userconfig.TransactionListConfig, error) {
	return userconfig.TransactionListConfig{PageSize: 50, ShowTotal: false}, nil
}

func (s *handlerServiceStub) AddTransactions(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {
	return s.addFn(userID, req)
}

func (s *handlerServiceStub) GetTransactions(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error) {
	return s.listFn(userID, filter)
}

func (s *handlerServiceStub) GetTransactionByID(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) {
	return s.getFn(userID, transactionID)
}

func (s *handlerServiceStub) UpdateTransaction(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
	return s.updateFn(userID, transactionID, req)
}

func (s *handlerServiceStub) UpdateTransactionsBulk(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
	return s.bulkUpdateFn(userID, transactionIDs, req)
}

func (s *handlerServiceStub) DeleteTransaction(userID uuid.UUID, transactionID uuid.UUID) error {
	return s.deleteFn(userID, transactionID)
}

func (s *handlerServiceStub) DeleteTransactionsBulk(userID uuid.UUID, transactionIDs []uuid.UUID) error {
	return s.bulkDeleteFn(userID, transactionIDs)
}

func TestParseDescriptionQueryTermsSplitsByWhitespaceAndComma(t *testing.T) {
	t.Parallel()

	got := parseDescriptionQueryTerms("  -juros dividendo,bonus\nprovento  ")
	want := []string{"-juros", "dividendo", "bonus", "provento"}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestGetTransactionsParsesQueryIntoFilter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	expectedUserID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/transactions?limit=100&page=2&sort=-amount&account_code=wallet,bank&category_code=food,income&description=-juros+dividendo,bonus&operation=credit,transfer&min_amount=15&max_amount=75&from_date=2025-01-02&to_date=2025-03-04", nil)
	context.Set("userID", expectedUserID)

	handler := NewHandler(&handlerServiceStub{
		addFn: func(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {
			return nil, nil
		},
		listFn: func(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error) {
			if userID != expectedUserID {
				t.Fatalf("expected user ID %s, got %s", expectedUserID, userID)
			}
			if filter.PageSize == nil || *filter.PageSize != 100 {
				t.Fatalf("unexpected page size: %+v", filter.PageSize)
			}
			if filter.PageNumber == nil || *filter.PageNumber != 2 {
				t.Fatalf("unexpected page number: %+v", filter.PageNumber)
			}
			if filter.SortColumn == nil || *filter.SortColumn != "amount" {
				t.Fatalf("unexpected sort column: %+v", filter.SortColumn)
			}
			if filter.AscSortOrder == nil || *filter.AscSortOrder {
				t.Fatalf("expected descending sort order, got %+v", filter.AscSortOrder)
			}
			if !reflect.DeepEqual(filter.AccountCodes, []string{"wallet", "bank"}) {
				t.Fatalf("unexpected account codes: %v", filter.AccountCodes)
			}
			if !reflect.DeepEqual(filter.CategoryCodes, []string{"food", "income"}) {
				t.Fatalf("unexpected category codes: %v", filter.CategoryCodes)
			}
			if !reflect.DeepEqual(filter.DescriptionTerms, []string{"-juros", "dividendo", "bonus"}) {
				t.Fatalf("unexpected description terms: %v", filter.DescriptionTerms)
			}
			if !reflect.DeepEqual(filter.OperationType, []string{"credit", "transfer"}) {
				t.Fatalf("unexpected operation types: %v", filter.OperationType)
			}
			if filter.MinAmount == nil || *filter.MinAmount != 15 {
				t.Fatalf("unexpected min amount: %+v", filter.MinAmount)
			}
			if filter.MaxAmount == nil || *filter.MaxAmount != 75 {
				t.Fatalf("unexpected max amount: %+v", filter.MaxAmount)
			}
			if filter.StartDate == nil || filter.StartDate.Format(QUERY_DATE_FORMAT) != "2025-01-02" {
				t.Fatalf("unexpected start date: %+v", filter.StartDate)
			}
			if filter.EndDate == nil || filter.EndDate.Format(QUERY_DATE_FORMAT) != "2025-03-04" {
				t.Fatalf("unexpected end date: %+v", filter.EndDate)
			}
			return &TransactionResponse{}, nil
		},
		getFn: func(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) { return nil, nil },
		updateFn: func(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
			return nil, nil
		},
		bulkUpdateFn: func(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
			return 0, nil
		},
		deleteFn: func(userID uuid.UUID, transactionID uuid.UUID) error { return nil },
	}, handlerConfigServiceStub{})

	handler.GetTransactions(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestGetTransactionsRejectsInvalidLimitBeforeCallingService(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/transactions?limit=not-a-number", nil)
	context.Set("userID", uuid.New())

	handler := NewHandler(&handlerServiceStub{
		addFn: func(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {
			return nil, nil
		},
		listFn: func(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error) {
			t.Fatal("service should not be called for invalid limit")
			return nil, nil
		},
		getFn: func(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) { return nil, nil },
		updateFn: func(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
			return nil, nil
		},
		bulkUpdateFn: func(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
			return 0, nil
		},
		deleteFn: func(userID uuid.UUID, transactionID uuid.UUID) error { return nil },
	}, handlerConfigServiceStub{})

	handler.GetTransactions(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestUpdateTransactionRejectsEmptyPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: "11111111-1111-1111-1111-111111111111"}}
	context.Request = httptest.NewRequest(http.MethodPatch, "/transactions/11111111-1111-1111-1111-111111111111", strings.NewReader(`{}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("userID", uuid.New())

	handler := NewHandler(&handlerServiceStub{
		addFn: func(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {
			return nil, nil
		},
		listFn: func(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error) { return nil, nil },
		getFn:  func(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) { return nil, nil },
		updateFn: func(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
			t.Fatal("service should not be called for empty update")
			return nil, nil
		},
		bulkUpdateFn: func(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
			return 0, nil
		},
		deleteFn: func(userID uuid.UUID, transactionID uuid.UUID) error { return nil },
	}, handlerConfigServiceStub{})

	handler.UpdateTransaction(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestUpdateTransactionBuildsServiceRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: "11111111-1111-1111-1111-111111111111"}}
	context.Request = httptest.NewRequest(http.MethodPatch, "/transactions/11111111-1111-1111-1111-111111111111", strings.NewReader(`{"description":"mercado","amount":99}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("userID", uuid.MustParse("22222222-2222-2222-2222-222222222222"))

	handler := NewHandler(&handlerServiceStub{
		addFn: func(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {
			return nil, nil
		},
		listFn: func(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error) { return nil, nil },
		getFn:  func(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) { return nil, nil },
		updateFn: func(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
			if req.Description == nil || *req.Description != "mercado" {
				t.Fatalf("unexpected description: %+v", req.Description)
			}
			if req.Amount == nil || *req.Amount != 99 {
				t.Fatalf("unexpected amount: %+v", req.Amount)
			}
			return &TransactionResponseItem{
				ID:          transactionID,
				Description: "mercado",
				Amount:      99,
				Date:        time.Now(),
			}, nil
		},
		bulkUpdateFn: func(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
			return 0, nil
		},
		deleteFn: func(userID uuid.UUID, transactionID uuid.UUID) error { return nil },
	}, handlerConfigServiceStub{})

	handler.UpdateTransaction(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestUpdateTransactionsBulkRejectsEmptyPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/transactions/bulk", strings.NewReader(`{"ids":["11111111-1111-1111-1111-111111111111"]}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("userID", uuid.New())

	handler := NewHandler(&handlerServiceStub{
		addFn: func(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {
			return nil, nil
		},
		listFn: func(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error) { return nil, nil },
		getFn:  func(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) { return nil, nil },
		updateFn: func(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
			return nil, nil
		},
		bulkUpdateFn: func(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
			t.Fatal("service should not be called for empty bulk update")
			return 0, nil
		},
		deleteFn: func(userID uuid.UUID, transactionID uuid.UUID) error { return nil },
	}, handlerConfigServiceStub{})

	handler.UpdateTransactionsBulk(context)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestUpdateTransactionsBulkBuildsServiceRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPatch, "/transactions/bulk", strings.NewReader(`{"ids":["11111111-1111-1111-1111-111111111111","22222222-2222-2222-2222-222222222222"],"category_code":"mercado","is_transfer":false}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("userID", uuid.MustParse("33333333-3333-3333-3333-333333333333"))

	handler := NewHandler(&handlerServiceStub{
		addFn: func(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {
			return nil, nil
		},
		listFn: func(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error) { return nil, nil },
		getFn:  func(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) { return nil, nil },
		updateFn: func(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
			return nil, nil
		},
		bulkUpdateFn: func(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
			if len(transactionIDs) != 2 {
				t.Fatalf("expected 2 ids, got %d", len(transactionIDs))
			}
			if req.CategoryCode == nil || *req.CategoryCode != "mercado" {
				t.Fatalf("unexpected category code: %+v", req.CategoryCode)
			}
			if req.IsTransfer == nil || *req.IsTransfer {
				t.Fatalf("unexpected is_transfer: %+v", req.IsTransfer)
			}
			return 2, nil
		},
		deleteFn: func(userID uuid.UUID, transactionID uuid.UUID) error { return nil },
	}, handlerConfigServiceStub{})

	handler.UpdateTransactionsBulk(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestCreateTransactionBuildsBatchRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/transactions", strings.NewReader(`{"transactions":[{"date":"2026-01-02T00:00:00Z","category_code":"salary","description":"Salary","amount":1500,"account_code":"cash","is_transfer":false}]}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("userID", userID)

	handler := NewHandler(&handlerServiceStub{
		addFn: func(gotUserID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			if len(req.Transactions) != 1 || req.Transactions[0].Description != "Salary" || req.Transactions[0].AccountCode != "cash" {
				t.Fatalf("unexpected request: %+v", req)
			}
			return []*TransactionResponseItem{{ID: uuid.New(), Description: "Salary", Amount: 1500, Date: time.Now()}}, nil
		},
		listFn: func(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error) { return nil, nil },
		getFn:  func(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) { return nil, nil },
		updateFn: func(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
			return nil, nil
		},
		bulkUpdateFn: func(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
			return 0, nil
		},
		deleteFn: func(userID uuid.UUID, transactionID uuid.UUID) error { return nil },
	}, handlerConfigServiceStub{})

	handler.CreateTransaction(context)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", recorder.Code)
	}
}

func TestGetTransactionByIDReturnsPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	txID := uuid.New()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: txID.String()}}
	context.Request = httptest.NewRequest(http.MethodGet, "/transactions/"+txID.String(), nil)
	context.Set("userID", userID)

	handler := NewHandler(&handlerServiceStub{
		addFn: func(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {
			return nil, nil
		},
		listFn: func(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error) { return nil, nil },
		getFn: func(gotUserID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) {
			if gotUserID != userID || transactionID != txID {
				t.Fatalf("unexpected args: %s %s", gotUserID, transactionID)
			}
			return &TransactionResponseItem{ID: txID, Description: "Salary", Amount: 123, Date: time.Now()}, nil
		},
		updateFn: func(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
			return nil, nil
		},
		bulkUpdateFn: func(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
			return 0, nil
		},
		deleteFn: func(userID uuid.UUID, transactionID uuid.UUID) error { return nil },
	}, handlerConfigServiceStub{})

	handler.GetTransactionByID(context)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestDeleteTransactionReturnsNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	txID := uuid.New()
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = gin.Params{{Key: "id", Value: txID.String()}}
	context.Request = httptest.NewRequest(http.MethodDelete, "/transactions/"+txID.String(), nil)
	context.Set("userID", userID)

	handler := NewHandler(&handlerServiceStub{
		addFn: func(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {
			return nil, nil
		},
		listFn: func(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error) { return nil, nil },
		getFn:  func(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) { return nil, nil },
		updateFn: func(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
			return nil, nil
		},
		bulkUpdateFn: func(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
			return 0, nil
		},
		deleteFn: func(gotUserID uuid.UUID, transactionID uuid.UUID) error {
			if gotUserID != userID || transactionID != txID {
				t.Fatalf("unexpected args: %s %s", gotUserID, transactionID)
			}
			return nil
		},
	}, handlerConfigServiceStub{})

	handler.DeleteTransaction(context)

	if context.Writer.Status() != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", context.Writer.Status())
	}
}

func TestDeleteTransactionsBulkForwardsPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	expectedIDs := []uuid.UUID{
		uuid.MustParse("aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
		uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"),
	}
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/transactions/bulk-delete", strings.NewReader(`{"ids":["aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa","bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"]}`))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("userID", userID)

	called := false
	handler := NewHandler(&handlerServiceStub{
		addFn: func(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {
			return nil, nil
		},
		listFn: func(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error) { return nil, nil },
		getFn:  func(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) { return nil, nil },
		updateFn: func(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
			return nil, nil
		},
		bulkUpdateFn: func(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
			return 0, nil
		},
		deleteFn: func(userID uuid.UUID, transactionID uuid.UUID) error { return nil },
		bulkDeleteFn: func(gotUserID uuid.UUID, transactionIDs []uuid.UUID) error {
			called = true
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			if !reflect.DeepEqual(transactionIDs, expectedIDs) {
				t.Fatalf("unexpected ids: %v", transactionIDs)
			}
			return nil
		},
	}, handlerConfigServiceStub{})

	handler.DeleteTransactionsBulk(context)

	if !called {
		t.Fatal("expected bulk delete service to be called")
	}
	if context.Writer.Status() != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", context.Writer.Status())
	}
}

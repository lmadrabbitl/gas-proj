package account

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appErr "expense-tracker/internal/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type accountServiceStub struct {
	addAccountFn        func(userID uuid.UUID, req CreateAccountRequest) (*Account, error)
	getAccountsFn       func(userID uuid.UUID) ([]Account, error)
	getAccountByCodeFn  func(userID uuid.UUID, code string) (*Account, error)
	getAccountsByCodeFn func(userID uuid.UUID, codes []string) ([]Account, error)
	getAccountsByIDFn   func(userID uuid.UUID, ids []uuid.UUID) ([]Account, error)
	updateAccountFn     func(userID uuid.UUID, code string, req UpdateAccountRequest) (*Account, error)
	reorderAccountsFn   func(userID uuid.UUID, codes []string) error
	deactivateFn        func(userID uuid.UUID, code string) error
	deletePermanentFn   func(userID uuid.UUID, code string) error
	updateBalanceFn     func(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error
}

func (s *accountServiceStub) AddAccount(userID uuid.UUID, req CreateAccountRequest) (*Account, error) {
	return s.addAccountFn(userID, req)
}

func (s *accountServiceStub) GetAccounts(userID uuid.UUID) ([]Account, error) {
	return s.getAccountsFn(userID)
}

func (s *accountServiceStub) GetAccountByCode(userID uuid.UUID, code string) (*Account, error) {
	return s.getAccountByCodeFn(userID, code)
}

func (s *accountServiceStub) GetAccountsByCode(userID uuid.UUID, codes []string) ([]Account, error) {
	return s.getAccountsByCodeFn(userID, codes)
}

func (s *accountServiceStub) GetAccountsByID(userID uuid.UUID, ids []uuid.UUID) ([]Account, error) {
	return s.getAccountsByIDFn(userID, ids)
}

func (s *accountServiceStub) UpdateAccount(userID uuid.UUID, code string, req UpdateAccountRequest) (*Account, error) {
	return s.updateAccountFn(userID, code, req)
}

func (s *accountServiceStub) ReorderAccounts(userID uuid.UUID, codes []string) error {
	return s.reorderAccountsFn(userID, codes)
}

func (s *accountServiceStub) DeactivateAccount(userID uuid.UUID, code string) error {
	return s.deactivateFn(userID, code)
}

func (s *accountServiceStub) DeleteAccountPermanently(userID uuid.UUID, code string) error {
	return s.deletePermanentFn(userID, code)
}

func (s *accountServiceStub) UpdateBalance(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error {
	return s.updateBalanceFn(db, userID, code, newBalance)
}

func TestHandlerCreateAccountRequiresUserID(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	service := &accountServiceStub{
		addAccountFn: func(userID uuid.UUID, req CreateAccountRequest) (*Account, error) {
			t.Fatal("expected service not to be called without user ID")
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBufferString(`{"name":"Wallet","type":"ASSET","currency":"BRL"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).CreateAccount(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if !strings.Contains(w.Body.String(), appErr.ErrInvalidLoginPassword().Code) {
		t.Fatalf("expected invalid login error, got body: %s", w.Body.String())
	}
}

func TestHandlerCreateAccountMapsRequestToService(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	var gotReq CreateAccountRequest
	var gotUserID uuid.UUID

	service := &accountServiceStub{
		addAccountFn: func(callUserID uuid.UUID, req CreateAccountRequest) (*Account, error) {
			gotUserID = callUserID
			gotReq = req
			return &Account{UserID: callUserID, Code: "wallet", Name: req.Name, Type: req.Type, Currency: req.Currency, HideFromDashboard: req.HideFromDashboard}, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Request = httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBufferString(`{"name":"Wallet","type":"ASSET","currency":"BRL","hide_from_dashboard":true}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).CreateAccount(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
	if gotUserID != userID {
		t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
	}
	if gotReq.Type != AccountTypeAsset {
		t.Fatalf("expected account type %q, got %q", AccountTypeAsset, gotReq.Type)
	}
	if gotReq.AssetRole != AccountAssetRoleNormal {
		t.Fatalf("expected default asset role %q, got %q", AccountAssetRoleNormal, gotReq.AssetRole)
	}
	if !gotReq.HideFromDashboard {
		t.Fatal("expected hide_from_dashboard to be forwarded")
	}

	var body map[string]map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON response, got error: %v", err)
	}
	if _, ok := body["account"]; !ok {
		t.Fatalf("expected account response body, got %v", body)
	}
}

func TestHandlerUpdateAccountRejectsInvalidType(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	service := &accountServiceStub{
		updateAccountFn: func(userID uuid.UUID, code string, req UpdateAccountRequest) (*Account, error) {
			t.Fatal("expected service not to be called for invalid account type")
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uuid.New())
	c.Params = []gin.Param{{Key: "code", Value: "cash"}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/accounts/cash", bytes.NewBufferString(`{"type":"OTHER"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).UpdateAccount(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if !strings.Contains(w.Body.String(), "validation.type.needs.to.be.asset.or.liability") {
		t.Fatalf("expected validation.type.needs.to.be.asset.or.liability response, got body: %s", w.Body.String())
	}
}

func TestHandlerUpdateAccountRejectsInvalidAssetRole(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	service := &accountServiceStub{
		updateAccountFn: func(userID uuid.UUID, code string, req UpdateAccountRequest) (*Account, error) {
			t.Fatal("expected service not to be called for invalid asset role")
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uuid.New())
	c.Params = []gin.Param{{Key: "code", Value: "cash"}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/accounts/cash", bytes.NewBufferString(`{"asset_role":"OTHER"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).UpdateAccount(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if !strings.Contains(w.Body.String(), "validation.asset.role.needs.to.be.normal.brokerage.or.investment") {
		t.Fatalf("expected asset role validation response, got body: %s", w.Body.String())
	}
}

func TestHandlerReorderAccountsReturnsNoContent(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	var gotCodes []string

	service := &accountServiceStub{
		reorderAccountsFn: func(callUserID uuid.UUID, codes []string) error {
			if callUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, callUserID)
			}
			gotCodes = append([]string(nil), codes...)
			return nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Request = httptest.NewRequest(http.MethodPatch, "/accounts/reorder", bytes.NewBufferString(`{"codes":["cash","broker"]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).ReorderAccounts(c)

	if c.Writer.Status() != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, c.Writer.Status())
	}
	if len(gotCodes) != 2 || gotCodes[0] != "cash" || gotCodes[1] != "broker" {
		t.Fatalf("expected reorder codes to be forwarded, got %v", gotCodes)
	}
}

func TestHandlerGetAccountListReturnsAccounts(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	service := &accountServiceStub{
		getAccountsFn: func(callUserID uuid.UUID) ([]Account, error) {
			if callUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, callUserID)
			}
			return []Account{{Code: "cash"}, {Code: "broker"}}, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Request = httptest.NewRequest(http.MethodGet, "/accounts", nil)

	NewHandler(service).GetAccountList(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if !strings.Contains(w.Body.String(), `"accounts"`) {
		t.Fatalf("expected accounts payload, got %s", w.Body.String())
	}
}

func TestHandlerGetAccountByCodeReturnsAccount(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	service := &accountServiceStub{
		getAccountByCodeFn: func(callUserID uuid.UUID, code string) (*Account, error) {
			if callUserID != userID || code != "cash" {
				t.Fatalf("unexpected args: %s %q", callUserID, code)
			}
			return &Account{Code: code}, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "code", Value: "cash"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/accounts/cash", nil)

	NewHandler(service).GetAccountByCode(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if !strings.Contains(w.Body.String(), `"account"`) {
		t.Fatalf("expected account payload, got %s", w.Body.String())
	}
}

func TestHandlerDeactivateAccountReturnsNoContent(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	service := &accountServiceStub{
		deactivateFn: func(callUserID uuid.UUID, code string) error {
			if callUserID != userID || code != "cash" {
				t.Fatalf("unexpected args: %s %q", callUserID, code)
			}
			return nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "code", Value: "cash"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/accounts/cash", nil)

	NewHandler(service).DeactivateAccount(c)

	if c.Writer.Status() != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, c.Writer.Status())
	}
}

func TestHandlerDeleteAccountPermanentlyReturnsNoContent(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	service := &accountServiceStub{
		deletePermanentFn: func(callUserID uuid.UUID, code string) error {
			if callUserID != userID || code != "cash" {
				t.Fatalf("unexpected args: %s %q", callUserID, code)
			}
			return nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "code", Value: "cash"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/accounts/cash/permanent", nil)

	NewHandler(service).DeleteAccountPermanently(c)

	if c.Writer.Status() != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, c.Writer.Status())
	}
}

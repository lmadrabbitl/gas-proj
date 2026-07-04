package investment

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appErr "expense-tracker/internal/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type serviceStub struct {
	createAssetFn        func(userID uuid.UUID, req CreateAssetRequest) (*Asset, error)
	listAssetsFn         func(userID uuid.UUID) ([]Asset, error)
	updateAssetFn        func(userID uuid.UUID, code string, req UpdateAssetRequest) (*Asset, error)
	refreshAssetFn       func(userID uuid.UUID, code string) (*Asset, error)
	refreshMissingFn     func(userID uuid.UUID) (int, error)
	createPortfolioFn    func(userID uuid.UUID, req CreatePortfolioRequest) (*Portfolio, error)
	listPortfoliosFn     func(userID uuid.UUID) ([]PortfolioResponse, error)
	updatePortfolioFn    func(userID uuid.UUID, code string, req UpdatePortfolioRequest) (*Portfolio, error)
	deletePortfolioFn    func(userID uuid.UUID, code string) error
	analyzePortfolioFn   func(userID uuid.UUID, portfolioCode string) (*PortfolioAnalysisResponse, error)
	suggestPortfolioFn   func(userID uuid.UUID, portfolioCode string, investmentAmount int64) (*PortfolioSuggestionResponse, error)
	savePortfolioFn      func(userID uuid.UUID, portfolioCode, assetCode string, req SavePortfolioAssetRequest) error
	deleteMemberFn       func(userID uuid.UUID, portfolioCode, assetCode string) error
	reorderAssetsFn      func(userID uuid.UUID, portfolioCode string, assetCodes []string) error
	createOpFn           func(userID uuid.UUID, req CreateOperationRequest) (*OperationRow, error)
	createBulkOpsFn      func(userID uuid.UUID, req CreateBulkOperationsRequest) ([]OperationRow, error)
	importOpsFn          func(userID uuid.UUID, req ImportOperationsRequest) (*ImportOperationsResponse, error)
	listOpsFn            func(userID uuid.UUID) ([]OperationRow, error)
	createMirrorFn       func(userID, operationID uuid.UUID, req CreateOperationMirrorRequest) (*OperationRow, error)
	createMirrorsBulkFn  func(userID uuid.UUID, req CreateOperationMirrorsBulkRequest) ([]OperationRow, error)
	updateOpFn           func(userID uuid.UUID, operationID uuid.UUID, req UpdateOperationRequest) (*OperationRow, error)
	deleteOpFn           func(userID uuid.UUID, operationID uuid.UUID) error
	listPositionsFn      func(userID uuid.UUID) ([]PositionRow, error)
	listPositionQuotesFn func(userID uuid.UUID) ([]PositionQuoteRow, error)
}

func (s *serviceStub) CreateAsset(userID uuid.UUID, req CreateAssetRequest) (*Asset, error) {
	return s.createAssetFn(userID, req)
}
func (s *serviceStub) ListAssets(userID uuid.UUID) ([]Asset, error) { return s.listAssetsFn(userID) }
func (s *serviceStub) UpdateAsset(userID uuid.UUID, code string, req UpdateAssetRequest) (*Asset, error) {
	return s.updateAssetFn(userID, code, req)
}
func (s *serviceStub) RefreshAssetMetadata(userID uuid.UUID, code string) (*Asset, error) {
	return s.refreshAssetFn(userID, code)
}
func (s *serviceStub) RefreshMissingAssetMetadata(userID uuid.UUID) (int, error) {
	return s.refreshMissingFn(userID)
}
func (s *serviceStub) CreatePortfolio(userID uuid.UUID, req CreatePortfolioRequest) (*Portfolio, error) {
	return s.createPortfolioFn(userID, req)
}
func (s *serviceStub) ListPortfolios(userID uuid.UUID) ([]PortfolioResponse, error) {
	return s.listPortfoliosFn(userID)
}
func (s *serviceStub) UpdatePortfolio(userID uuid.UUID, code string, req UpdatePortfolioRequest) (*Portfolio, error) {
	return s.updatePortfolioFn(userID, code, req)
}
func (s *serviceStub) DeletePortfolio(userID uuid.UUID, code string) error {
	return s.deletePortfolioFn(userID, code)
}
func (s *serviceStub) AnalyzePortfolio(userID uuid.UUID, portfolioCode string) (*PortfolioAnalysisResponse, error) {
	return s.analyzePortfolioFn(userID, portfolioCode)
}
func (s *serviceStub) SuggestPortfolioInvestment(userID uuid.UUID, portfolioCode string, investmentAmount int64) (*PortfolioSuggestionResponse, error) {
	return s.suggestPortfolioFn(userID, portfolioCode, investmentAmount)
}
func (s *serviceStub) SavePortfolioAsset(userID uuid.UUID, portfolioCode, assetCode string, req SavePortfolioAssetRequest) error {
	return s.savePortfolioFn(userID, portfolioCode, assetCode, req)
}
func (s *serviceStub) DeletePortfolioAsset(userID uuid.UUID, portfolioCode, assetCode string) error {
	return s.deleteMemberFn(userID, portfolioCode, assetCode)
}
func (s *serviceStub) ReorderPortfolioAssets(userID uuid.UUID, portfolioCode string, assetCodes []string) error {
	return s.reorderAssetsFn(userID, portfolioCode, assetCodes)
}
func (s *serviceStub) CreateOperation(userID uuid.UUID, req CreateOperationRequest) (*OperationRow, error) {
	return s.createOpFn(userID, req)
}
func (s *serviceStub) CreateOperationsBulk(userID uuid.UUID, req CreateBulkOperationsRequest) ([]OperationRow, error) {
	return s.createBulkOpsFn(userID, req)
}
func (s *serviceStub) ImportOperations(userID uuid.UUID, req ImportOperationsRequest) (*ImportOperationsResponse, error) {
	return s.importOpsFn(userID, req)
}
func (s *serviceStub) ListOperations(userID uuid.UUID) ([]OperationRow, error) {
	return s.listOpsFn(userID)
}
func (s *serviceStub) CreateOperationMirror(userID, operationID uuid.UUID, req CreateOperationMirrorRequest) (*OperationRow, error) {
	return s.createMirrorFn(userID, operationID, req)
}
func (s *serviceStub) CreateOperationMirrorsBulk(userID uuid.UUID, req CreateOperationMirrorsBulkRequest) ([]OperationRow, error) {
	return s.createMirrorsBulkFn(userID, req)
}
func (s *serviceStub) UpdateOperation(userID uuid.UUID, operationID uuid.UUID, req UpdateOperationRequest) (*OperationRow, error) {
	return s.updateOpFn(userID, operationID, req)
}
func (s *serviceStub) DeleteOperation(userID uuid.UUID, operationID uuid.UUID) error {
	return s.deleteOpFn(userID, operationID)
}
func (s *serviceStub) ListPositions(userID uuid.UUID) ([]PositionRow, error) {
	return s.listPositionsFn(userID)
}
func (s *serviceStub) ListPositionQuotes(userID uuid.UUID) ([]PositionQuoteRow, error) {
	return s.listPositionQuotesFn(userID)
}

func TestCreateOperationForwardsPayload(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	var got CreateOperationRequest
	service := &serviceStub{
		createOpFn: func(callUserID uuid.UUID, req CreateOperationRequest) (*OperationRow, error) {
			got = req
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			return &OperationRow{ID: uuid.New(), AssetCode: req.AssetCode}, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Request = httptest.NewRequest(http.MethodPost, "/investments/operations", bytes.NewBufferString(`{"asset_code":"mxrf11","brokerage_account_code":"btg-invest","operation_type":"buy","date":"2026-06-20T00:00:00Z","quantity":10,"unit_price":1234,"fee_amount":10,"notes":"teste"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).CreateOperation(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if got.AssetCode != "mxrf11" || got.BrokerageAccountCode != "btg-invest" || got.OperationType != OperationTypeBuy || got.Quantity != 10 || got.UnitPrice != 1234 {
		t.Fatalf("unexpected mapped request: %+v", got)
	}
}

func TestUpdateOperationRejectsInvalidUUID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	service := &serviceStub{
		updateOpFn: func(userID uuid.UUID, operationID uuid.UUID, req UpdateOperationRequest) (*OperationRow, error) {
			t.Fatal("service should not be called for invalid id")
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uuid.New())
	c.Params = []gin.Param{{Key: "id", Value: "bad-id"}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/investments/operations/bad-id", bytes.NewBufferString(`{"quantity":2}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).UpdateOperation(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "investment.operation.id.invalid") {
		t.Fatalf("expected invalid id code, got %s", w.Body.String())
	}
}

func TestListPositionsReturnsPayload(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	service := &serviceStub{
		listPositionsFn: func(callUserID uuid.UUID) ([]PositionRow, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			return []PositionRow{{AssetCode: "MXRF11", CurrentQuantity: 12}}, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Request = httptest.NewRequest(http.MethodGet, "/investments/positions", nil)

	NewHandler(service).ListPositions(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"positions"`) {
		t.Fatalf("expected positions payload, got %s", w.Body.String())
	}
}

func TestAnalyzePortfolioReturnsPayload(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	service := &serviceStub{
		analyzePortfolioFn: func(callUserID uuid.UUID, portfolioCode string) (*PortfolioAnalysisResponse, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			if portfolioCode != "dividendos" {
				t.Fatalf("expected portfolio code dividendos, got %s", portfolioCode)
			}
			return &PortfolioAnalysisResponse{PortfolioCode: portfolioCode}, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "code", Value: "dividendos"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/investments/portfolios/dividendos/analysis", nil)

	NewHandler(service).AnalyzePortfolio(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"analysis"`) || !strings.Contains(w.Body.String(), `"portfolio_code":"dividendos"`) {
		t.Fatalf("expected analysis payload, got %s", w.Body.String())
	}
}

func TestSuggestPortfolioInvestmentReturnsPayload(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	service := &serviceStub{
		suggestPortfolioFn: func(callUserID uuid.UUID, portfolioCode string, investmentAmount int64) (*PortfolioSuggestionResponse, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			if portfolioCode != "dividendos" {
				t.Fatalf("expected portfolio code dividendos, got %s", portfolioCode)
			}
			if investmentAmount != 50000 {
				t.Fatalf("expected investment amount 50000, got %d", investmentAmount)
			}
			return &PortfolioSuggestionResponse{PortfolioCode: portfolioCode, InvestmentAmount: investmentAmount}, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "code", Value: "dividendos"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/investments/portfolios/dividendos/suggestions", bytes.NewBufferString(`{"investment_amount":50000}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).SuggestPortfolioInvestment(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"suggestion"`) || !strings.Contains(w.Body.String(), `"investment_amount":50000`) {
		t.Fatalf("expected suggestion payload, got %s", w.Body.String())
	}
}

func TestCreateAssetRequiresUserID(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	service := &serviceStub{
		createAssetFn: func(userID uuid.UUID, req CreateAssetRequest) (*Asset, error) {
			t.Fatal("service should not be called without user id")
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/investments/assets", bytes.NewBufferString(`{"code":"MXRF11","name":"MXRF","asset_type":"FII"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).CreateAsset(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), appErr.ErrInvalidLoginPassword().Code) {
		t.Fatalf("expected invalid login body, got %s", w.Body.String())
	}
}

func TestCreateOperationParsesDate(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	service := &serviceStub{
		createOpFn: func(userID uuid.UUID, req CreateOperationRequest) (*OperationRow, error) {
			expected := time.Date(2026, time.June, 20, 0, 0, 0, 0, time.UTC)
			if !req.Date.Equal(expected) {
				t.Fatalf("expected %s, got %s", expected, req.Date)
			}
			return &OperationRow{ID: uuid.New(), AssetCode: req.AssetCode}, nil
		},
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uuid.New())
	c.Request = httptest.NewRequest(http.MethodPost, "/investments/operations", bytes.NewBufferString(`{"asset_code":"mxrf11","brokerage_account_code":"btg-invest","operation_type":"buy","date":"2026-06-20T00:00:00Z","quantity":1,"unit_price":100,"fee_amount":0}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).CreateOperation(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
}

func TestCreateBulkOperationsForwardsRows(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	var got CreateBulkOperationsRequest
	service := &serviceStub{
		createBulkOpsFn: func(callUserID uuid.UUID, req CreateBulkOperationsRequest) ([]OperationRow, error) {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			got = req
			return []OperationRow{{ID: uuid.New(), AssetCode: "MXRF11"}}, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Request = httptest.NewRequest(http.MethodPost, "/investments/operations/bulk", bytes.NewBufferString(`{"operations":[{"asset_code":"mxrf11","brokerage_account_code":"btg-invest","operation_type":"buy","date":"2026-06-20T00:00:00Z","quantity":10,"unit_price":1234,"total_fee_amount":100}]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).CreateOperationsBulk(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	if len(got.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(got.Operations))
	}
	if got.Operations[0].BrokerageAccountCode != "btg-invest" || got.Operations[0].OperationType != OperationTypeBuy || got.Operations[0].TotalFeeAmount != 100 {
		t.Fatalf("unexpected request payload: %+v", got.Operations[0])
	}
}

func TestSavePortfolioAssetAllowsZeroTargetAllocation(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	called := false
	service := &serviceStub{
		savePortfolioFn: func(callUserID uuid.UUID, portfolioCode, assetCode string, req SavePortfolioAssetRequest) error {
			called = true
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			if portfolioCode != "dividendos" {
				t.Fatalf("expected portfolio code dividendos, got %s", portfolioCode)
			}
			if assetCode != "SPYI11" {
				t.Fatalf("expected asset code SPYI11, got %s", assetCode)
			}
			if req.TargetAllocationBasisPoint != 0 {
				t.Fatalf("expected zero allocation, got %d", req.TargetAllocationBasisPoint)
			}
			return nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{
		{Key: "code", Value: "dividendos"},
		{Key: "assetCode", Value: "SPYI11"},
	}
	c.Request = httptest.NewRequest(http.MethodPut, "/investments/portfolios/dividendos/assets/SPYI11", bytes.NewBufferString(`{"target_allocation_basis_point":0,"max_buy_price":null}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).SavePortfolioAsset(c)
	c.Writer.WriteHeaderNow()

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", w.Code, w.Body.String())
	}
	if !called {
		t.Fatal("expected service to be called")
	}
}

func TestReorderPortfolioAssetsForwardsCodes(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	var gotPortfolioCode string
	var gotCodes []string
	service := &serviceStub{
		reorderAssetsFn: func(callUserID uuid.UUID, portfolioCode string, assetCodes []string) error {
			if callUserID != userID {
				t.Fatalf("expected userID %s, got %s", userID, callUserID)
			}
			gotPortfolioCode = portfolioCode
			gotCodes = append([]string(nil), assetCodes...)
			return nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "code", Value: "dividendos"}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/investments/portfolios/dividendos/assets/reorder", bytes.NewBufferString(`{"codes":["SPYI11","VALE3"]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).ReorderPortfolioAssets(c)
	c.Writer.WriteHeaderNow()

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", w.Code, w.Body.String())
	}
	if gotPortfolioCode != "dividendos" {
		t.Fatalf("expected portfolio code dividendos, got %s", gotPortfolioCode)
	}
	if len(gotCodes) != 2 || gotCodes[0] != "SPYI11" || gotCodes[1] != "VALE3" {
		t.Fatalf("expected asset reorder codes to be forwarded, got %v", gotCodes)
	}
}

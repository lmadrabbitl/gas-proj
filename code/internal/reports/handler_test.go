package reports

import (
	"encoding/json"
	appErr "expense-tracker/internal/errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type handlerServiceStub struct {
	getReportFn               func(userID uuid.UUID, year int) ([]*CategoryYearlyBalance, error)
	getBalanceByAccountCodeFn func(userID uuid.UUID, code string) (*AccountBalance, error)
	getAllBalancesFn          func(userID uuid.UUID) ([]*AccountBalance, error)
	getDashboardFn            func(userID uuid.UUID, year, month int) (*DashboardData, error)
}

func (s *handlerServiceStub) GetReport(userID uuid.UUID, year int) ([]*CategoryYearlyBalance, error) {
	return s.getReportFn(userID, year)
}

func (s *handlerServiceStub) GetBalanceByAccountCode(userID uuid.UUID, code string) (*AccountBalance, error) {
	return s.getBalanceByAccountCodeFn(userID, code)
}

func (s *handlerServiceStub) GetAllBalances(userID uuid.UUID) ([]*AccountBalance, error) {
	return s.getAllBalancesFn(userID)
}

func (s *handlerServiceStub) GetDashboard(userID uuid.UUID, year, month int) (*DashboardData, error) {
	return s.getDashboardFn(userID, year, month)
}

func TestParseDashboardPeriodDefaultsToCurrentYearAndMonth(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/reports/dashboard", nil)
	ctx.Request = req

	now := time.Now()
	year, month, err := parseDashboardPeriod(ctx)
	if err != nil {
		t.Fatalf("parseDashboardPeriod returned error: %v", err)
	}

	if year != now.Year() || month != int(now.Month()) {
		t.Fatalf("expected current period %d/%d, got %d/%d", now.Month(), now.Year(), month, year)
	}
}

func TestParseDashboardPeriodRejectsFutureMonthInCurrentYear(t *testing.T) {
	t.Parallel()

	currentYear := time.Now().Year()
	currentMonth := int(time.Now().Month())
	futureMonth := currentMonth + 1
	if futureMonth > 12 {
		t.Skip("current month is already December; no future month remains in the current year")
	}

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(http.MethodGet, "/reports/dashboard", nil)
	req.URL.RawQuery = fmt.Sprintf("year=%d&month=%d", currentYear, futureMonth)
	ctx.Request = req

	_, _, err := parseDashboardPeriod(ctx)
	if err == nil {
		t.Fatal("expected future month validation error")
	}
}

func TestGetDashboardReturnsPayloadFromService(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
	})

	stub := &handlerServiceStub{
		getReportFn:               func(userID uuid.UUID, year int) ([]*CategoryYearlyBalance, error) { return nil, nil },
		getBalanceByAccountCodeFn: func(userID uuid.UUID, code string) (*AccountBalance, error) { return nil, nil },
		getAllBalancesFn:          func(userID uuid.UUID) ([]*AccountBalance, error) { return nil, nil },
		getDashboardFn: func(gotUserID uuid.UUID, year, month int) (*DashboardData, error) {
			if gotUserID != userID {
				t.Fatalf("unexpected user id: %s", gotUserID)
			}
			if year != 2026 || month != 4 {
				t.Fatalf("unexpected dashboard period: %d/%d", month, year)
			}
			return &DashboardData{Year: year, Month: month}, nil
		},
	}

	router.GET("/reports/dashboard", NewHandler(stub).GetDashboard)

	req := httptest.NewRequest(http.MethodGet, "/reports/dashboard?year=2026&month=4", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	var payload struct {
		Dashboard DashboardData `json:"dashboard"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Dashboard.Year != 2026 || payload.Dashboard.Month != 4 {
		t.Fatalf("unexpected response payload: %#v", payload.Dashboard)
	}
}

func TestGetYearlyReportRejectsInvalidYearQuery(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", uuid.New())
	})
	router.GET("/reports/yearly", NewHandler(&handlerServiceStub{
		getReportFn:               func(userID uuid.UUID, year int) ([]*CategoryYearlyBalance, error) { return nil, nil },
		getBalanceByAccountCodeFn: func(userID uuid.UUID, code string) (*AccountBalance, error) { return nil, nil },
		getAllBalancesFn:          func(userID uuid.UUID) ([]*AccountBalance, error) { return nil, nil },
		getDashboardFn:            func(userID uuid.UUID, year, month int) (*DashboardData, error) { return nil, nil },
	}).GetYearlyReport)

	req := httptest.NewRequest(http.MethodGet, "/reports/yearly?year=1999", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestGetAllBalancesRequiresUserContext(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/reports/balance", NewHandler(&handlerServiceStub{
		getReportFn:               func(userID uuid.UUID, year int) ([]*CategoryYearlyBalance, error) { return nil, nil },
		getBalanceByAccountCodeFn: func(userID uuid.UUID, code string) (*AccountBalance, error) { return nil, nil },
		getAllBalancesFn:          func(userID uuid.UUID) ([]*AccountBalance, error) { return nil, nil },
		getDashboardFn:            func(userID uuid.UUID, year, month int) (*DashboardData, error) { return nil, nil },
	}).GetAllBalances)

	req := httptest.NewRequest(http.MethodGet, "/reports/balance", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}

	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error.Code != appErr.ErrInvalidLoginPassword().Code {
		t.Fatalf("expected invalid login error code, got %q", payload.Error.Code)
	}
}

func TestGetBalanceByCodeReturnsPayloadFromService(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", userID)
	})
	router.GET("/reports/balance/:code", NewHandler(&handlerServiceStub{
		getReportFn: func(userID uuid.UUID, year int) ([]*CategoryYearlyBalance, error) { return nil, nil },
		getBalanceByAccountCodeFn: func(gotUserID uuid.UUID, code string) (*AccountBalance, error) {
			if gotUserID != userID || code != "cash" {
				t.Fatalf("unexpected args: %s %q", gotUserID, code)
			}
			return &AccountBalance{AccountCode: code, Balance: 12345}, nil
		},
		getAllBalancesFn: func(userID uuid.UUID) ([]*AccountBalance, error) { return nil, nil },
		getDashboardFn:   func(userID uuid.UUID, year, month int) (*DashboardData, error) { return nil, nil },
	}).GetBalanceByCode)

	req := httptest.NewRequest(http.MethodGet, "/reports/balance/cash", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"account"`) {
		t.Fatalf("expected account payload, got %s", recorder.Body.String())
	}
}

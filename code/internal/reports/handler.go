package reports

import (
	"expense-tracker/internal/errors"
	"expense-tracker/internal/middleware"
	appHttp "expense-tracker/internal/transport/http"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

type DashboardResponse struct {
	Dashboard *DashboardData `json:"dashboard"`
}

type YearlyReportResponse struct {
	Balances []*CategoryYearlyBalance `json:"balances"`
}

type AllBalancesResponse struct {
	Accounts []*AccountBalance `json:"accounts"`
}

type BalanceByCodeResponse struct {
	Account *AccountBalance `json:"account"`
}

func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

const MinYear = 2000
const MinMonth = 1
const MaxMonth = 12

func (h *Handler) RegisterRoutes(authMw *middleware.AuthMiddleware, r gin.IRoutes) {
	r.GET("/reports/dashboard", authMw.CheckAuthMiddleware(), h.GetDashboard)
	r.GET("/reports/yearly", authMw.CheckAuthMiddleware(), h.GetYearlyReport)
	r.GET("/reports/balance", authMw.CheckAuthMiddleware(), h.GetAllBalances)
	r.GET("/reports/balance/:code", authMw.CheckAuthMiddleware(), h.GetBalanceByCode)

}

// GetYearlyReport godoc
// @Summary Get yearly report
// @Description Returns all balances for the specified year (default current year)
// @Tags reports
// @Produce json
// @Security BearerAuth
// @Param year query int false "Year to filter" minimum(2000)
// @Success 200 {object} YearlyReportResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /reports/yearly [get]
func (h *Handler) GetYearlyReport(c *gin.Context) {

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	year, _, err := parseDashboardPeriod(c)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	balances, err := h.service.GetReport(userID, year)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"balances": balances})
}

// GetDashboard godoc
// @Summary Get dashboard
// @Description Returns dashboard data for the selected month and year
// @Tags reports
// @Produce json
// @Security BearerAuth
// @Param year query int false "Year to filter" minimum(2000)
// @Param month query int false "Month to filter" minimum(1) maximum(12)
// @Success 200 {object} DashboardResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /reports/dashboard [get]
func (h *Handler) GetDashboard(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	year, month, err := parseDashboardPeriod(c)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	dashboard, err := h.service.GetDashboard(userID, year, month)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"dashboard": dashboard,
	})
}

// GetAllBalances godoc
// @Summary List balances
// @Description Returns balances for all accounts
// @Tags reports
// @Produce json
// @Security BearerAuth
// @Success 200 {object} AllBalancesResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /reports/balance [get]
func (h *Handler) GetAllBalances(c *gin.Context) {

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	balances, err := h.service.GetAllBalances(userID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accounts": balances,
	})
}

// GetBalanceByCode godoc
// @Summary Get balance by account code
// @Description Returns a single account balance
// @Tags reports
// @Produce json
// @Security BearerAuth
// @Param code path string true "Account code"
// @Success 200 {object} BalanceByCodeResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /reports/balance/{code} [get]
func (h *Handler) GetBalanceByCode(c *gin.Context) {

	code := c.Param("code")
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	accountBalance, err := h.service.GetBalanceByAccountCode(userID, code)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"account": accountBalance,
	})

}

func parseDashboardPeriod(c *gin.Context) (int, int, error) {
	now := time.Now()
	currentYear := now.Year()
	year := currentYear
	month := int(now.Month())

	if yearStr := c.Query("year"); yearStr != "" {
		parsedYear, err := strconv.Atoi(yearStr)
		if err != nil {
			return 0, 0, errors.ErrInvalidInputWithCode("report.period.year.invalid", "invalid year query param", err)
		}
		if parsedYear < MinYear || parsedYear > currentYear {
			return 0, 0, errors.ErrInvalidInputWithCode("report.period.year.out_of_range", fmt.Sprintf("year needs to be from 2020 until %d", currentYear), nil)
		}
		year = parsedYear
	}

	if monthStr := c.Query("month"); monthStr != "" {
		parsedMonth, err := strconv.Atoi(monthStr)
		if err != nil {
			return 0, 0, errors.ErrInvalidInputWithCode("report.period.month.invalid", "invalid month query param", err)
		}
		if parsedMonth < MinMonth || parsedMonth > MaxMonth {
			return 0, 0, errors.ErrInvalidInputWithMessage("month needs to be between 1 and 12", nil)
		}
		month = parsedMonth
	}

	if year == currentYear && month > int(now.Month()) {
		return 0, 0, errors.ErrInvalidInputWithMessage("month cannot be in the future", nil)
	}

	return year, month, nil
}

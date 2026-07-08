package transaction

import (
	"expense-tracker/internal/errors"
	appErr "expense-tracker/internal/errors"
	"expense-tracker/internal/middleware"
	appHttp "expense-tracker/internal/transport/http"
	"expense-tracker/internal/userconfig"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service       Service
	configService TransactionConfigService
}

type TransactionConfigService interface {
	GetTransactionListConfig(userID uuid.UUID) (userconfig.TransactionListConfig, error)
}

type CreateTransactionInput struct {
	Transactions []CreateSingleTransaction `json:"transactions" binding:"required"`
}

type CreateSingleTransaction struct {
	Date                 time.Time `json:"date" binding:"required"`
	CategoryCode         string    `json:"category_code" binding:"required"`
	Description          string    `json:"description" binding:"required"`
	Amount               int64     `json:"amount" binding:"required"`
	AccountCode          string    `json:"account_code" binding:"required"`
	IsTransfer           bool      `json:"is_transfer" binding:"required"`
	TransferAccountCode  *string   `json:"account_transfer"`
	ExcludeFromDashboard bool      `json:"exclude_from_dashboard"`
}

type ChangeTransactionInput struct {
	Date                 *time.Time `json:"date"`
	CategoryCode         *string    `json:"category_code"`
	Description          *string    `json:"description"`
	Amount               *int64     `json:"amount"`
	AccountCode          *string    `json:"account_code"`
	IsTransfer           *bool      `json:"is_transfer"`
	TransferAccountCode  *string    `json:"account_transfer"`
	ExcludeFromDashboard *bool      `json:"exclude_from_dashboard"`
}

type BulkChangeTransactionInput struct {
	IDs                  []uuid.UUID `json:"ids" binding:"required"`
	Date                 *time.Time  `json:"date"`
	CategoryCode         *string     `json:"category_code"`
	Description          *string     `json:"description"`
	Amount               *int64      `json:"amount"`
	AccountCode          *string     `json:"account_code"`
	IsTransfer           *bool       `json:"is_transfer"`
	TransferAccountCode  *string     `json:"account_transfer"`
	ExcludeFromDashboard *bool       `json:"exclude_from_dashboard"`
}

type TransactionResponse struct {
	Transactions []TransactionResponseItem        `json:"transactions"`
	Pagination   PaginationInfo                   `json:"pagination"`
	Config       userconfig.TransactionListConfig `json:"config"`
}

type TransactionResponseItem struct {
	ID                       uuid.UUID  `json:"id" gorm:"column:id"`
	CategoryCode             string     `json:"category_code" gorm:"column:category_code"`
	Description              string     `json:"description" gorm:"column:description"`
	Date                     time.Time  `json:"date" gorm:"column:date"`
	AccountCode              string     `json:"account_code" gorm:"column:account_code"`
	Amount                   int64      `json:"amount" gorm:"column:amount"`
	TransferID               *int64     `json:"transfer_id" gorm:"column:transfer_id"`
	TransferAccountCode      *string    `json:"account_transfer" gorm:"column:transfer_account_code"`
	ExcludeFromDashboard     bool       `json:"exclude_from_dashboard" gorm:"column:exclude_from_dashboard"`
	IsInvestmentMirror       bool       `json:"is_investment_operation_mirror" gorm:"column:is_investment_operation_mirror"`
	InvestmentOperationID    *uuid.UUID `json:"investment_operation_id" gorm:"column:investment_operation_id"`
	InvestmentLinkRole       *string    `json:"investment_operation_link_role" gorm:"column:investment_operation_link_role"`
	InvestmentOperationCount int        `json:"investment_operation_count" gorm:"column:investment_operation_count"`
}

type PaginationInfo struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

type CreateTransactionResponse struct {
	Transactions []*TransactionResponseItem `json:"transactions"`
}

type GetTransactionResponse struct {
	Transaction *TransactionResponseItem `json:"transaction"`
}

type BulkUpdateTransactionResponse struct {
	UpdatedCount int `json:"updated_count"`
}

const QUERY_DATA_SEPARATOR = ","
const QUERY_SORT_DESC = "-"
const QUERY_DATE_FORMAT = "2006-01-02"
const QUERY_DESCRIPTION_EXCLUDE_PREFIX = "-"

func NewHandler(service Service, configService TransactionConfigService) *Handler {
	return &Handler{
		service:       service,
		configService: configService,
	}
}

func (h *Handler) RegisterRoutes(authMw *middleware.AuthMiddleware, r gin.IRoutes) {
	r.POST("/transactions", authMw.CheckAuthMiddleware(), h.CreateTransaction)
	r.GET("/transactions", authMw.CheckAuthMiddleware(), h.GetTransactions)
	r.PATCH("/transactions/bulk", authMw.CheckAuthMiddleware(), h.UpdateTransactionsBulk)
	r.GET("/transactions/:id", authMw.CheckAuthMiddleware(), h.GetTransactionByID)
	r.PATCH("/transactions/:id", authMw.CheckAuthMiddleware(), h.UpdateTransaction)
	r.DELETE("/transactions/:id", authMw.CheckAuthMiddleware(), h.DeleteTransaction)
}

// CreateTransaction godoc
// @Summary Create transactions
// @Description Creates one or more transactions for the authenticated user
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateTransactionInput true "Transactions payload"
// @Success 201 {object} CreateTransactionResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /transactions [post]
func (h *Handler) CreateTransaction(c *gin.Context) {

	var input CreateTransactionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	req := CreateTransactionRequest{}

	for _, tx := range input.Transactions {

		singleTx := SingleTransactionRequest{
			Date:                 tx.Date,
			CategoryCode:         tx.CategoryCode,
			Description:          tx.Description,
			Amount:               tx.Amount,
			AccountCode:          tx.AccountCode,
			IsTransfer:           tx.IsTransfer,
			TransferAccountCode:  tx.TransferAccountCode,
			ExcludeFromDashboard: tx.ExcludeFromDashboard,
		}
		if tx.TransferAccountCode != nil {
			singleTx.TransferAccountCode = tx.TransferAccountCode
		}
		req.Transactions = append(req.Transactions, singleTx)
	}

	transactions, err := h.service.AddTransactions(userID, req)

	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"transactions": transactions})
}

// GetTransactions godoc
// @Summary List transactions
// @Description Returns filtered transactions with pagination metadata
// @Tags transactions
// @Produce json
// @Security BearerAuth
// @Param limit query int false "Page size" minimum(1) maximum(1000)
// @Param page query int false "Page number" minimum(1)
// @Param sort query string false "Sort field, prepend '-' for descending" Enums(DATE,AMOUNT,UPDATED,-DATE,-AMOUNT,-UPDATED)
// @Param account_code query string false "Comma-separated account codes"
// @Param category_code query string false "Comma-separated category codes"
// @Param description query string false "Space- or comma-separated description terms, supports exclusions with '-'"
// @Param operation query string false "Comma-separated operations" Enums(REVENUE,EXPENSE,TRANSFER)
// @Param min_amount query int false "Minimum absolute amount"
// @Param max_amount query int false "Maximum absolute amount"
// @Param from_date query string false "Start date (YYYY-MM-DD)"
// @Param to_date query string false "End date (YYYY-MM-DD)"
// @Success 200 {object} TransactionResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /transactions [get]
func (h *Handler) GetTransactions(c *gin.Context) {

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	filter := FilterTransactionRequest{}

	limit := c.Query("limit")
	page := c.Query("page")
	sort := c.Query("sort")
	account_code := c.Query("account_code")
	category_code := c.Query("category_code")
	description := c.Query("description")
	operation := c.Query("operation")
	min_amount := c.Query("min_amount")
	max_amount := c.Query("max_amount")
	from_date := c.Query("from_date")
	to_date := c.Query("to_date")

	if limit != "" {
		intLimit, err := strconv.Atoi(limit)
		if err != nil {
			appHttp.HandleError(c, appErr.ErrInvalidInputWithCode("transaction.filter.limit.invalid", "invalid query param limit", nil))
			return
		}
		if err := CheckLimitQueryParam(intLimit); err != nil {
			appHttp.HandleError(c, err)
			return
		}
		filter.PageSize = &intLimit
	}

	if page != "" {
		intPage, err := strconv.Atoi(page)
		if err != nil {
			appHttp.HandleError(c, appErr.ErrInvalidInputWithCode("transaction.filter.page.invalid", "invalid query param offset", nil))
			return
		}
		filter.PageNumber = &intPage
	}

	if sort != "" {
		var ok bool
		sortOrderAsc := true
		if sort, ok = strings.CutPrefix(sort, QUERY_SORT_DESC); ok {
			sortOrderAsc = false
		}
		err := CheckSortQueryParam(sort)
		if err != nil {
			appHttp.HandleError(c, err)
			return
		}
		filter.SortColumn = &sort
		filter.AscSortOrder = &sortOrderAsc
	}

	if account_code != "" {
		accounts := strings.Split(account_code, QUERY_DATA_SEPARATOR)
		err := CheckAccountQueryParam(accounts)
		if err != nil {
			appHttp.HandleError(c, err)
			return
		}
		filter.AccountCodes = accounts
	}

	if category_code != "" {
		categories := strings.Split(category_code, QUERY_DATA_SEPARATOR)
		err := CheckCategoryQueryParam(categories)
		if err != nil {
			appHttp.HandleError(c, err)
			return
		}
		filter.CategoryCodes = categories
	}

	if description != "" {
		descriptionTerms := parseDescriptionQueryTerms(description)
		err := CheckDescriptionQueryParam(descriptionTerms)
		if err != nil {
			appHttp.HandleError(c, err)
			return
		}
		filter.DescriptionTerms = descriptionTerms
	}

	if operation != "" {
		operationTypes := strings.Split(operation, QUERY_DATA_SEPARATOR)
		err := CheckOperationQueryParam(operationTypes)
		if err != nil {
			appHttp.HandleError(c, err)
			return
		}
		filter.OperationType = operationTypes
	}

	if min_amount != "" {
		intMinAmount, err := strconv.ParseInt(min_amount, 10, 64)
		if err != nil {
			appHttp.HandleError(c, appErr.ErrInvalidInputWithCode("transaction.filter.min_amount.invalid", "invalid query param min_amount", err))
			return
		}
		filter.MinAmount = &intMinAmount
	}
	if max_amount != "" {
		intMaxAmount, err := strconv.ParseInt(max_amount, 10, 64)
		if err != nil {
			appHttp.HandleError(c, appErr.ErrInvalidInputWithCode("transaction.filter.max_amount.invalid", "invalid query param max_amount", err))
			return
		}
		filter.MaxAmount = &intMaxAmount
	}
	if from_date != "" {
		parsedFromDate, err := time.Parse(QUERY_DATE_FORMAT, from_date)
		if err != nil {
			appHttp.HandleError(c, appErr.ErrInvalidInputWithCode("transaction.filter.from_date.invalid", "invalid date", err))
			return
		}
		filter.StartDate = &parsedFromDate
	}
	if to_date != "" {
		parsedToDate, err := time.Parse(QUERY_DATE_FORMAT, to_date)
		if err != nil {
			appHttp.HandleError(c, appErr.ErrInvalidInputWithCode("transaction.filter.to_date.invalid", "invalid date", err))
			return
		}
		filter.EndDate = &parsedToDate
	}
	/*
	   Example params:
	      ?limit=200
	      &page=2
	      &sort=-date
	      &account_code=accountCode1,accountCode2
	      &description=apple banana -orange //positive terms match, negative terms are excluded
	      &min_amount=100
	      &max_amount=1000
	      &from_date=2025-01-01
	      &to_date=2025-12-31
	*/
	transactions, err := h.service.GetTransactions(userID, filter)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	config, err := h.configService.GetTransactionListConfig(userID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	transactions.Config = config

	c.JSON(http.StatusOK, transactions)
}

func parseDescriptionQueryTerms(description string) []string {
	return strings.FieldsFunc(description, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}

// GetTransactionByID godoc
// @Summary Get transaction by ID
// @Description Returns a transaction by its ID
// @Tags transactions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID" format(uuid)
// @Success 200 {object} GetTransactionResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /transactions/{id} [get]
func (h *Handler) GetTransactionByID(c *gin.Context) {

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	idUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		appHttp.HandleError(c, errors.ErrInvalidTransactionID())
		return
	}

	transaction, err := h.service.GetTransactionByID(userID, idUUID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transaction": transaction,
	})

}

// UpdateTransaction godoc
// @Summary Update transaction
// @Description Updates an existing transaction
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID" format(uuid)
// @Param request body ChangeTransactionInput true "Transaction changes"
// @Success 200 {object} GetTransactionResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /transactions/{id} [patch]
func (h *Handler) UpdateTransaction(c *gin.Context) {

	var req ChangeTransactionInput

	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Errors in unmarshal")
		appHttp.HandleError(c, err)
		return
	}

	userID, ok := middleware.GetUserID(c)

	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	idUUID, err := uuid.Parse(c.Param("id"))

	if err != nil {
		appHttp.HandleError(c, errors.ErrInvalidTransactionID())
		return
	}

	updateRequest := UpdateTransactionRequest{
		Date:                 req.Date,
		AccountCode:          req.AccountCode,
		Amount:               req.Amount,
		CategoryCode:         req.CategoryCode,
		Description:          req.Description,
		IsTransfer:           req.IsTransfer,
		TransferAccountCode:  req.TransferAccountCode,
		ExcludeFromDashboard: req.ExcludeFromDashboard,
	}

	if updateRequest.IsEmpty() {
		appHttp.HandleError(c, errors.ErrInvalidInputWithMessage("no data to to update", nil))
		return
	}

	transaction, err := h.service.UpdateTransaction(userID, idUUID, updateRequest)

	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"transaction": transaction})

}

func (h *Handler) UpdateTransactionsBulk(c *gin.Context) {
	var req BulkChangeTransactionInput

	if err := c.ShouldBindJSON(&req); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	updateRequest := UpdateTransactionRequest{
		Date:                 req.Date,
		AccountCode:          req.AccountCode,
		Amount:               req.Amount,
		CategoryCode:         req.CategoryCode,
		Description:          req.Description,
		IsTransfer:           req.IsTransfer,
		TransferAccountCode:  req.TransferAccountCode,
		ExcludeFromDashboard: req.ExcludeFromDashboard,
	}

	if len(req.IDs) == 0 {
		appHttp.HandleError(c, errors.ErrInvalidInputWithMessage("no transaction ids to update", nil))
		return
	}

	if updateRequest.IsEmpty() {
		appHttp.HandleError(c, errors.ErrInvalidInputWithMessage("no data to to update", nil))
		return
	}

	updatedCount, err := h.service.UpdateTransactionsBulk(userID, req.IDs, updateRequest)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"updated_count": updatedCount,
	})
}

// DeleteTransaction godoc
// @Summary Delete transaction
// @Description Deletes a transaction
// @Tags transactions
// @Security BearerAuth
// @Param id path string true "Transaction ID" format(uuid)
// @Success 204
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /transactions/{id} [delete]
func (h *Handler) DeleteTransaction(c *gin.Context) {

	idUUID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		appHttp.HandleError(c, errors.ErrInvalidTransactionID())
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	if err := h.service.DeleteTransaction(userID, idUUID); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

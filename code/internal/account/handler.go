package account

import (
	"expense-tracker/internal/errors"
	"expense-tracker/internal/middleware"
	appHttp "expense-tracker/internal/transport/http"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

type CreateAccountInput struct {
	Name              string `json:"name" binding:"required"`
	AccountType       string `json:"type" binding:"required"`
	Currency          string `json:"currency" binding:"required"`
	AssetRole         string `json:"asset_role"`
	HideFromDashboard bool   `json:"hide_from_dashboard"`
}

type ChangeAccountInput struct {
	Name              *string `json:"name"`
	AccountType       *string `json:"type"`
	Currency          *string `json:"currency"`
	AssetRole         *string `json:"asset_role"`
	HideFromDashboard *bool   `json:"hide_from_dashboard"`
}

type ReorderAccountsInput struct {
	Codes []string `json:"codes" binding:"required"`
}

type AccountResponse struct {
	Account *Account `json:"account"`
}

type AccountListResponse struct {
	Accounts []Account `json:"accounts"`
}

func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) RegisterRoutes(authMw *middleware.AuthMiddleware, r gin.IRoutes) {
	r.POST("/accounts", authMw.CheckAuthMiddleware(), h.CreateAccount)
	r.GET("/accounts", authMw.CheckAuthMiddleware(), h.GetAccountList)
	r.PATCH("/accounts/reorder", authMw.CheckAuthMiddleware(), h.ReorderAccounts)
	r.GET("/accounts/:code", authMw.CheckAuthMiddleware(), h.GetAccountByCode)
	r.PATCH("/accounts/:code", authMw.CheckAuthMiddleware(), h.UpdateAccount)
	r.DELETE("/accounts/:code/permanent", authMw.CheckAuthMiddleware(), h.DeleteAccountPermanently)
	r.DELETE("/accounts/:code", authMw.CheckAuthMiddleware(), h.DeactivateAccount)

}

// CreateAccount godoc
// @Summary Create account
// @Description Creates a new account for the authenticated user
// @Tags accounts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateAccountInput true "Account payload"
// @Success 201 {object} AccountResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 409 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /accounts [post]
func (h *Handler) CreateAccount(c *gin.Context) {

	var req CreateAccountInput
	if err := c.ShouldBindJSON(&req); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	accType := AccountType(req.AccountType)
	if err := CheckAccountType(accType); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	account, err := h.service.AddAccount(userID, CreateAccountRequest{
		Name:              req.Name,
		Type:              accType,
		Currency:          req.Currency,
		AssetRole:         normalizeAssetRoleInput(req.AssetRole),
		HideFromDashboard: req.HideFromDashboard,
	})

	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"account": account})
}

// GetAccountList godoc
// @Summary List accounts
// @Description Returns all accounts for the authenticated user
// @Tags accounts
// @Produce json
// @Security BearerAuth
// @Success 200 {object} AccountListResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /accounts [get]
func (h *Handler) GetAccountList(c *gin.Context) {

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	accounts, err := h.service.GetAccounts(userID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accounts": accounts,
	})
}

// GetAccountByCode godoc
// @Summary Get account by code
// @Description Returns an account by its code
// @Tags accounts
// @Produce json
// @Security BearerAuth
// @Param code path string true "Account code"
// @Success 200 {object} AccountResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /accounts/{code} [get]
func (h *Handler) GetAccountByCode(c *gin.Context) {

	code := c.Param("code")
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	account, err := h.service.GetAccountByCode(userID, code)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"account": account,
	})

}

// UpdateAccount godoc
// @Summary Update account
// @Description Updates an existing account
// @Tags accounts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param code path string true "Account code"
// @Param request body ChangeAccountInput true "Account changes"
// @Success 200 {object} AccountResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /accounts/{code} [patch]
func (h *Handler) UpdateAccount(c *gin.Context) {

	var req ChangeAccountInput
	if err := c.ShouldBindJSON(&req); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	code := c.Param("code")

	var name *string
	if req.Name != nil {
		if err := CheckAccountName(*req.Name); err != nil {
			appHttp.HandleError(c, err)
			return
		}
		name = req.Name
	}
	var currency *string
	if req.Currency != nil {
		if err := CheckAccountCurrency(*req.Currency); err != nil {
			appHttp.HandleError(c, err)
			return
		}
		currency = req.Currency
	}
	var accType *AccountType
	if req.AccountType != nil {
		accTypeFromReq := AccountType(*req.AccountType)
		if err := CheckAccountType(accTypeFromReq); err != nil {
			appHttp.HandleError(c, err)
			return
		}
		accType = &accTypeFromReq
	}
	var hideFromDashboard *bool
	if req.HideFromDashboard != nil {
		hideFromDashboard = req.HideFromDashboard
	}
	var assetRole *AccountAssetRole
	if req.AssetRole != nil {
		role := AccountAssetRole(*req.AssetRole)
		if err := CheckAccountAssetRole(role); err != nil {
			appHttp.HandleError(c, err)
			return
		}
		assetRole = &role
	}
	account, err := h.service.UpdateAccount(userID, code, UpdateAccountRequest{
		Name:              name,
		Type:              accType,
		Currency:          currency,
		AssetRole:         assetRole,
		HideFromDashboard: hideFromDashboard,
	})

	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"account": account})
}

// ReorderAccounts godoc
// @Summary Reorder accounts
// @Description Reorders active accounts for the authenticated user
// @Tags accounts
// @Accept json
// @Security BearerAuth
// @Param request body ReorderAccountsInput true "Account ordering"
// @Success 204
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /accounts/reorder [patch]
func (h *Handler) ReorderAccounts(c *gin.Context) {
	var req ReorderAccountsInput
	if err := c.ShouldBindJSON(&req); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	if err := h.service.ReorderAccounts(userID, req.Codes); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// DeactivateAccount godoc
// @Summary Deactivate account
// @Description Deactivates an account
// @Tags accounts
// @Security BearerAuth
// @Param code path string true "Account code"
// @Success 204
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /accounts/{code} [delete]
func (h *Handler) DeactivateAccount(c *gin.Context) {

	code := c.Param("code")
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	err := h.service.DeactivateAccount(userID, code)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)

}

// DeleteAccountPermanently godoc
// @Summary Permanently delete account
// @Description Permanently deletes a deactivated account
// @Tags accounts
// @Security BearerAuth
// @Param code path string true "Account code"
// @Success 204
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /accounts/{code}/permanent [delete]
func (h *Handler) DeleteAccountPermanently(c *gin.Context) {
	code := c.Param("code")
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	err := h.service.DeleteAccountPermanently(userID, code)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func normalizeAssetRoleInput(value string) AccountAssetRole {
	if value == "" {
		return AccountAssetRoleNormal
	}
	return AccountAssetRole(value)
}

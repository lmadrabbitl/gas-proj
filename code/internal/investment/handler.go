package investment

import (
	"expense-tracker/internal/errors"
	"expense-tracker/internal/middleware"
	appHttp "expense-tracker/internal/transport/http"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
}

type CreateAssetInput struct {
	Code      string `json:"code" binding:"required"`
	Name      string `json:"name" binding:"required"`
	AssetType string `json:"asset_type" binding:"required"`
}

type UpdateAssetInput struct {
	Code      *string `json:"code"`
	Name      *string `json:"name"`
	CNPJ      *string `json:"cnpj"`
	AssetType *string `json:"asset_type"`
	IsActive  *bool   `json:"is_active"`
}

type CreatePortfolioInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type UpdatePortfolioInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
}

type SavePortfolioAssetInput struct {
	TargetAllocationBasisPoint int    `json:"target_allocation_basis_point"`
	MaxBuyPrice                *int64 `json:"max_buy_price"`
	SortOrder                  *int   `json:"sort_order"`
}

type ReorderPortfolioAssetsInput struct {
	Codes []string `json:"codes" binding:"required"`
}

type SuggestPortfolioInvestmentInput struct {
	InvestmentAmount int64 `json:"investment_amount" binding:"required"`
}

type CreateOperationInput struct {
	AssetCode     string    `json:"asset_code" binding:"required"`
	OperationType string    `json:"operation_type" binding:"required"`
	Date          time.Time `json:"date" binding:"required"`
	Quantity      int64     `json:"quantity" binding:"required"`
	UnitPrice     int64     `json:"unit_price" binding:"required"`
	FeeAmount     int64     `json:"fee_amount"`
	Notes         string    `json:"notes"`
}

type CreateBulkOperationsInput struct {
	Operations []CreateBulkOperationInput `json:"operations" binding:"required"`
}

type CreateBulkOperationInput struct {
	AssetCode      string    `json:"asset_code" binding:"required"`
	OperationType  string    `json:"operation_type" binding:"required"`
	Date           time.Time `json:"date" binding:"required"`
	Quantity       int64     `json:"quantity" binding:"required"`
	UnitPrice      int64     `json:"unit_price" binding:"required"`
	TotalFeeAmount int64     `json:"total_fee_amount"`
	Notes          string    `json:"notes"`
}

type UpdateOperationInput struct {
	AssetCode     *string    `json:"asset_code"`
	OperationType *string    `json:"operation_type"`
	Date          *time.Time `json:"date"`
	Quantity      *int64     `json:"quantity"`
	UnitPrice     *int64     `json:"unit_price"`
	FeeAmount     *int64     `json:"fee_amount"`
	Notes         *string    `json:"notes"`
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(authMw *middleware.AuthMiddleware, r gin.IRoutes) {
	r.POST("/investments/assets", authMw.CheckAuthMiddleware(), h.CreateAsset)
	r.GET("/investments/assets", authMw.CheckAuthMiddleware(), h.ListAssets)
	r.PATCH("/investments/assets/:code", authMw.CheckAuthMiddleware(), h.UpdateAsset)
	r.POST("/investments/assets/:code/refresh-metadata", authMw.CheckAuthMiddleware(), h.RefreshAssetMetadata)
	r.POST("/investments/assets/refresh-missing-metadata", authMw.CheckAuthMiddleware(), h.RefreshMissingAssetMetadata)
	r.POST("/investments/portfolios", authMw.CheckAuthMiddleware(), h.CreatePortfolio)
	r.GET("/investments/portfolios", authMw.CheckAuthMiddleware(), h.ListPortfolios)
	r.PATCH("/investments/portfolios/:code", authMw.CheckAuthMiddleware(), h.UpdatePortfolio)
	r.DELETE("/investments/portfolios/:code", authMw.CheckAuthMiddleware(), h.DeletePortfolio)
	r.GET("/investments/portfolios/:code/analysis", authMw.CheckAuthMiddleware(), h.AnalyzePortfolio)
	r.POST("/investments/portfolios/:code/suggestions", authMw.CheckAuthMiddleware(), h.SuggestPortfolioInvestment)
	r.PUT("/investments/portfolios/:code/assets/:assetCode", authMw.CheckAuthMiddleware(), h.SavePortfolioAsset)
	r.PATCH("/investments/portfolios/:code/assets/reorder", authMw.CheckAuthMiddleware(), h.ReorderPortfolioAssets)
	r.DELETE("/investments/portfolios/:code/assets/:assetCode", authMw.CheckAuthMiddleware(), h.DeletePortfolioAsset)
	r.POST("/investments/operations", authMw.CheckAuthMiddleware(), h.CreateOperation)
	r.POST("/investments/operations/bulk", authMw.CheckAuthMiddleware(), h.CreateOperationsBulk)
	r.GET("/investments/operations", authMw.CheckAuthMiddleware(), h.ListOperations)
	r.PATCH("/investments/operations/:id", authMw.CheckAuthMiddleware(), h.UpdateOperation)
	r.DELETE("/investments/operations/:id", authMw.CheckAuthMiddleware(), h.DeleteOperation)
	r.GET("/investments/positions", authMw.CheckAuthMiddleware(), h.ListPositions)
	r.GET("/investments/position-quotes", authMw.CheckAuthMiddleware(), h.ListPositionQuotes)
}

func (h *Handler) CreateAsset(c *gin.Context) {
	var input CreateAssetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	asset, err := h.service.CreateAsset(userID, CreateAssetRequest{
		Code:      input.Code,
		Name:      input.Name,
		AssetType: AssetType(strings.ToUpper(input.AssetType)),
	})
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"asset": asset})
}

func (h *Handler) ListAssets(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	assets, err := h.service.ListAssets(userID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"assets": assets})
}

func (h *Handler) UpdateAsset(c *gin.Context) {
	var input UpdateAssetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	req := UpdateAssetRequest{IsActive: input.IsActive, CNPJ: input.CNPJ}
	if input.Code != nil {
		req.Code = input.Code
	}
	if input.Name != nil {
		req.Name = input.Name
	}
	if input.AssetType != nil {
		assetType := AssetType(strings.ToUpper(*input.AssetType))
		req.AssetType = &assetType
	}
	asset, err := h.service.UpdateAsset(userID, c.Param("code"), req)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"asset": asset})
}

func (h *Handler) RefreshAssetMetadata(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	asset, err := h.service.RefreshAssetMetadata(userID, c.Param("code"))
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"asset": asset})
}

func (h *Handler) RefreshMissingAssetMetadata(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	updated, err := h.service.RefreshMissingAssetMetadata(userID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"updated": updated})
}

func (h *Handler) CreatePortfolio(c *gin.Context) {
	var input CreatePortfolioInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	portfolio, err := h.service.CreatePortfolio(userID, CreatePortfolioRequest{
		Name:        input.Name,
		Description: input.Description,
	})
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"portfolio": portfolio})
}

func (h *Handler) ListPortfolios(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	portfolios, err := h.service.ListPortfolios(userID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"portfolios": portfolios})
}

func (h *Handler) UpdatePortfolio(c *gin.Context) {
	var input UpdatePortfolioInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	portfolio, err := h.service.UpdatePortfolio(userID, c.Param("code"), UpdatePortfolioRequest{
		Name:        input.Name,
		Description: input.Description,
	})
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"portfolio": portfolio})
}

func (h *Handler) AnalyzePortfolio(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	analysis, err := h.service.AnalyzePortfolio(userID, c.Param("code"))
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"analysis": analysis})
}

func (h *Handler) SuggestPortfolioInvestment(c *gin.Context) {
	var input SuggestPortfolioInvestmentInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	suggestion, err := h.service.SuggestPortfolioInvestment(userID, c.Param("code"), input.InvestmentAmount)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"suggestion": suggestion})
}

func (h *Handler) DeletePortfolio(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	if err := h.service.DeletePortfolio(userID, c.Param("code")); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListPositionQuotes(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	quotes, err := h.service.ListPositionQuotes(userID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"quotes": quotes})
}

func (h *Handler) SavePortfolioAsset(c *gin.Context) {
	var input SavePortfolioAssetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	err := h.service.SavePortfolioAsset(userID, c.Param("code"), c.Param("assetCode"), SavePortfolioAssetRequest{
		TargetAllocationBasisPoint: input.TargetAllocationBasisPoint,
		MaxBuyPrice:                input.MaxBuyPrice,
		SortOrder:                  input.SortOrder,
	})
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ReorderPortfolioAssets(c *gin.Context) {
	var input ReorderPortfolioAssetsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	if err := h.service.ReorderPortfolioAssets(userID, c.Param("code"), input.Codes); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) DeletePortfolioAsset(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	if err := h.service.DeletePortfolioAsset(userID, c.Param("code"), c.Param("assetCode")); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) CreateOperation(c *gin.Context) {
	var input CreateOperationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	operation, err := h.service.CreateOperation(userID, CreateOperationRequest{
		AssetCode:     input.AssetCode,
		OperationType: OperationType(strings.ToUpper(input.OperationType)),
		Date:          input.Date,
		Quantity:      input.Quantity,
		UnitPrice:     input.UnitPrice,
		FeeAmount:     input.FeeAmount,
		Notes:         input.Notes,
	})
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"operation": operation})
}

func (h *Handler) CreateOperationsBulk(c *gin.Context) {
	var input CreateBulkOperationsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	req := CreateBulkOperationsRequest{
		Operations: make([]CreateBulkOperationRequest, 0, len(input.Operations)),
	}
	for _, item := range input.Operations {
		req.Operations = append(req.Operations, CreateBulkOperationRequest{
			AssetCode:      item.AssetCode,
			OperationType:  OperationType(strings.ToUpper(item.OperationType)),
			Date:           item.Date,
			Quantity:       item.Quantity,
			UnitPrice:      item.UnitPrice,
			TotalFeeAmount: item.TotalFeeAmount,
			Notes:          item.Notes,
		})
	}

	operations, err := h.service.CreateOperationsBulk(userID, req)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"operations": operations})
}

func (h *Handler) ListOperations(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	operations, err := h.service.ListOperations(userID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"operations": operations})
}

func (h *Handler) UpdateOperation(c *gin.Context) {
	var input UpdateOperationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	operationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		appHttp.HandleError(c, errors.ErrInvalidInputWithCode("investment.operation.id.invalid", "investment operation id is invalid", err))
		return
	}
	req := UpdateOperationRequest{
		AssetCode: input.AssetCode,
		Date:      input.Date,
		Quantity:  input.Quantity,
		UnitPrice: input.UnitPrice,
		FeeAmount: input.FeeAmount,
		Notes:     input.Notes,
	}
	if input.OperationType != nil {
		operationType := OperationType(strings.ToUpper(*input.OperationType))
		req.OperationType = &operationType
	}
	operation, err := h.service.UpdateOperation(userID, operationID, req)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"operation": operation})
}

func (h *Handler) DeleteOperation(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	operationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		appHttp.HandleError(c, errors.ErrInvalidInputWithCode("investment.operation.id.invalid", "investment operation id is invalid", err))
		return
	}
	if err := h.service.DeleteOperation(userID, operationID); err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) ListPositions(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}
	positions, err := h.service.ListPositions(userID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"positions": positions})
}

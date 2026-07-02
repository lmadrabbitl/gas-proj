package suggestion

import (
	"expense-tracker/internal/errors"
	"expense-tracker/internal/middleware"
	appHttp "expense-tracker/internal/transport/http"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
}

type CreateSuggestionInput struct {
	DescriptionContains string  `json:"description_contains" binding:"required"`
	Priority            int     `json:"priority" binding:"required"`
	EntryType           *string `json:"entry_type"`
	CategoryCode        *string `json:"category_code"`
	AccountCode         *string `json:"account_code"`
	TransferAccountCode *string `json:"transfer_account_code"`
}

type ChangeSuggestionInput struct {
	DescriptionContains *string `json:"description_contains"`
	Priority            *int    `json:"priority"`
	EntryType           *string `json:"entry_type"`
	CategoryCode        *string `json:"category_code"`
	AccountCode         *string `json:"account_code"`
	TransferAccountCode *string `json:"transfer_account_code"`
}

type SuggestionResponse struct {
	Suggestion *SuggestionResponseItem `json:"suggestion"`
}

type SuggestionListResponse struct {
	Suggestions []SuggestionResponseItem `json:"suggestions"`
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(authMw *middleware.AuthMiddleware, r gin.IRoutes) {
	r.POST("/suggestions", authMw.CheckAuthMiddleware(), h.CreateSuggestion)
	r.GET("/suggestions", authMw.CheckAuthMiddleware(), h.GetSuggestionList)
	r.GET("/suggestions/:id", authMw.CheckAuthMiddleware(), h.GetSuggestionByID)
	r.PATCH("/suggestions/:id", authMw.CheckAuthMiddleware(), h.UpdateSuggestion)
	r.DELETE("/suggestions/:id", authMw.CheckAuthMiddleware(), h.DeleteSuggestion)
}

// CreateSuggestion godoc
// @Summary Create suggestion
// @Description Creates a transaction suggestion for the authenticated user
// @Tags suggestions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateSuggestionInput true "Suggestion payload"
// @Success 201 {object} SuggestionResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /suggestions [post]
func (h *Handler) CreateSuggestion(c *gin.Context) {
	var req CreateSuggestionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	var entryType *SuggestionEntryType
	if req.EntryType != nil && *req.EntryType != "" {
		value := SuggestionEntryType(*req.EntryType)
		entryType = &value
	}

	suggestion, err := h.service.AddSuggestion(userID, CreateSuggestionRequest{
		DescriptionContains: req.DescriptionContains,
		Priority:            req.Priority,
		EntryType:           entryType,
		CategoryCode:        req.CategoryCode,
		AccountCode:         req.AccountCode,
		TransferAccountCode: req.TransferAccountCode,
	})
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"suggestion": suggestion})
}

// GetSuggestionList godoc
// @Summary List suggestions
// @Description Returns all suggestions for the authenticated user
// @Tags suggestions
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuggestionListResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /suggestions [get]
func (h *Handler) GetSuggestionList(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	suggestions, err := h.service.GetSuggestions(userID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"suggestions": suggestions})
}

// GetSuggestionByID godoc
// @Summary Get suggestion by ID
// @Description Returns a suggestion by its ID
// @Tags suggestions
// @Produce json
// @Security BearerAuth
// @Param id path string true "Suggestion ID" format(uuid)
// @Success 200 {object} SuggestionResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /suggestions/{id} [get]
func (h *Handler) GetSuggestionByID(c *gin.Context) {
	suggestionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		appHttp.HandleError(c, errors.ErrInvalidInputWithMessage("invalid suggestion id", err))
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	suggestion, err := h.service.GetSuggestionByID(userID, suggestionID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"suggestion": suggestion})
}

// UpdateSuggestion godoc
// @Summary Update suggestion
// @Description Updates an existing suggestion
// @Tags suggestions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Suggestion ID" format(uuid)
// @Param request body ChangeSuggestionInput true "Suggestion changes"
// @Success 200 {object} SuggestionResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /suggestions/{id} [patch]
func (h *Handler) UpdateSuggestion(c *gin.Context) {
	suggestionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		appHttp.HandleError(c, errors.ErrInvalidInputWithMessage("invalid suggestion id", err))
		return
	}

	var req ChangeSuggestionInput
	if err := c.ShouldBindJSON(&req); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	suggestion, err := h.service.UpdateSuggestion(userID, suggestionID, UpdateSuggestionRequest{
		DescriptionContains: req.DescriptionContains,
		Priority:            req.Priority,
		EntryType:           req.EntryType,
		CategoryCode:        req.CategoryCode,
		AccountCode:         req.AccountCode,
		TransferAccountCode: req.TransferAccountCode,
	})
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"suggestion": suggestion})
}

// DeleteSuggestion godoc
// @Summary Delete suggestion
// @Description Deletes a suggestion
// @Tags suggestions
// @Security BearerAuth
// @Param id path string true "Suggestion ID" format(uuid)
// @Success 204
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /suggestions/{id} [delete]
func (h *Handler) DeleteSuggestion(c *gin.Context) {
	suggestionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		appHttp.HandleError(c, errors.ErrInvalidInputWithMessage("invalid suggestion id", err))
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	if err := h.service.DeleteSuggestion(userID, suggestionID); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

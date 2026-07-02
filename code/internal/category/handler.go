package category

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

type CreateCategoryInput struct {
	Name         string  `json:"name" binding:"required"`
	CategoryType string  `json:"type" binding:"required"`
	Description  string  `json:"description" binding:"required"`
	ParentCode   *string `json:"parent_code"`
}

type ChangeCategoryInput struct {
	Name         *string `json:"name"`
	CategoryType *string `json:"type"`
	Description  *string `json:"description"`
	ParentCode   *string `json:"parent_code"`
}

type ReorderCategoriesInput struct {
	ParentCode *string  `json:"parent_code"`
	Codes      []string `json:"codes" binding:"required"`
}

type CategoryResponse struct {
	Category *Category `json:"category"`
}

type CategoryListResponse struct {
	Categories []Category `json:"categories"`
}

func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) RegisterRoutes(authMw *middleware.AuthMiddleware, r gin.IRoutes) {
	r.POST("/categories", authMw.CheckAuthMiddleware(), h.CreateCategory)
	r.GET("/categories", authMw.CheckAuthMiddleware(), h.GetCategoryList)
	r.PATCH("/categories/reorder", authMw.CheckAuthMiddleware(), h.ReorderCategories)
	r.GET("/categories/:code", authMw.CheckAuthMiddleware(), h.GetCategoryByCode)
	r.PATCH("/categories/:code", authMw.CheckAuthMiddleware(), h.UpdateCategory)
	r.DELETE("/categories/:code", authMw.CheckAuthMiddleware(), h.DeactivateCategory)

}

// CreateCategory godoc
// @Summary Create category
// @Description Creates a new category for the authenticated user
// @Tags categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateCategoryInput true "Category payload"
// @Success 201 {object} CategoryResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 409 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /categories [post]
func (h *Handler) CreateCategory(c *gin.Context) {

	var req CreateCategoryInput
	if err := c.ShouldBindJSON(&req); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	accType := CategoryType(req.CategoryType)
	if err := CheckCategoryType(accType); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	Category, err := h.service.AddCategory(userID, CreateCategoryRequest{
		Name:        req.Name,
		Type:        accType,
		Description: req.Description,
		ParentCode:  req.ParentCode,
	})

	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"category": Category})
}

// GetCategoryList godoc
// @Summary List categories
// @Description Returns all active categories for the authenticated user
// @Tags categories
// @Produce json
// @Security BearerAuth
// @Success 200 {object} CategoryListResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /categories [get]
func (h *Handler) GetCategoryList(c *gin.Context) {

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	includeDeactivated := c.Query("include_deactivated") == "true"
	categories, err := h.service.GetCategories(userID, includeDeactivated)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
	})
}

// GetCategoryByCode godoc
// @Summary Get category by code
// @Description Returns a category by its code
// @Tags categories
// @Produce json
// @Security BearerAuth
// @Param code path string true "Category code"
// @Success 200 {object} CategoryResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /categories/{code} [get]
func (h *Handler) GetCategoryByCode(c *gin.Context) {

	code := c.Param("code")
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	category, err := h.service.GetCategoryByCode(userID, code)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"category": category,
	})

}

// UpdateCategory godoc
// @Summary Update category
// @Description Updates an existing category
// @Tags categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param code path string true "Category code"
// @Param request body ChangeCategoryInput true "Category changes"
// @Success 200 {object} CategoryResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /categories/{code} [patch]
func (h *Handler) UpdateCategory(c *gin.Context) {

	var req ChangeCategoryInput
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
		if err := CheckCategoryName(*req.Name); err != nil {
			appHttp.HandleError(c, err)
			return
		}
		name = req.Name
	}
	var description *string
	if req.Description != nil {
		description = req.Description
	}
	var accType *CategoryType
	if req.CategoryType != nil {
		accTypeFromReq := CategoryType(*req.CategoryType)
		if err := CheckCategoryType(accTypeFromReq); err != nil {
			appHttp.HandleError(c, err)
			return
		}
		accType = &accTypeFromReq
	}
	var parentCode *string
	if req.ParentCode != nil {
		if err := CheckCategoryCode(*req.ParentCode); err != nil {
			appHttp.HandleError(c, err)
			return
		}
		parentCode = req.ParentCode
	}

	category, err := h.service.UpdateCategory(userID, code, UpdateCategoryRequest{
		Name:        name,
		Type:        accType,
		Description: description,
		ParentCode:  parentCode,
	})

	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"category": category})
}

// ReorderCategories godoc
// @Summary Reorder categories
// @Description Reorders categories within a parent group
// @Tags categories
// @Accept json
// @Security BearerAuth
// @Param request body ReorderCategoriesInput true "Category ordering"
// @Success 204
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /categories/reorder [patch]
func (h *Handler) ReorderCategories(c *gin.Context) {
	var req ReorderCategoriesInput
	if err := c.ShouldBindJSON(&req); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	if err := h.service.ReorderCategories(userID, req.ParentCode, req.Codes); err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// DeactivateCategory godoc
// @Summary Deactivate category
// @Description Deactivates a category
// @Tags categories
// @Security BearerAuth
// @Param code path string true "Category code"
// @Success 204
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 404 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /categories/{code} [delete]
func (h *Handler) DeactivateCategory(c *gin.Context) {

	code := c.Param("code")
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	err := h.service.DeactivateCategory(userID, code)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

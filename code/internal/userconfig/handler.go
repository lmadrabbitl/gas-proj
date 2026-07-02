package userconfig

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

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(authMw *middleware.AuthMiddleware, r gin.IRoutes) {
	r.GET("/users/me/config", authMw.CheckAuthMiddleware(), h.GetConfig)
	r.PATCH("/users/me/config", authMw.CheckAuthMiddleware(), h.UpdateConfig)
}

func (h *Handler) GetConfig(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	config, err := h.service.GetConfig(userID)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"config": config})
}

func (h *Handler) UpdateConfig(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		appHttp.HandleError(c, errors.ErrInvalidLoginPassword())
		return
	}

	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appHttp.HandleError(c, errors.ErrInvalidInputWithCode("request.body.invalid", "invalid request body", err))
		return
	}

	config, err := h.service.UpdateConfig(userID, req)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"config": config})
}

package user

import (
	appErr "expense-tracker/internal/errors"
	appHttp "expense-tracker/internal/transport/http"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type RegisterResponse struct {
	User UserResponse `json:"user"`
}

func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) RegisterRoutes(r gin.IRoutes) {
	r.POST("/auth/login", h.Login)
	r.POST("/auth/register", h.Register)
}

// Login godoc
// @Summary Login user
// @Description Authenticates a user and returns a JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login payload"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 401 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /auth/login [post]
func (h *Handler) Login(c *gin.Context) {

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appHttp.HandleError(c, appErr.ErrInvalidInputWithMessage("invalid request body", err))
		return
	}

	token, err := h.service.LoginUser(req.Email, req.Password)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token})
}

// Register godoc
// @Summary Register user
// @Description Creates a new user account
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Register payload"
// @Success 201 {object} RegisterResponse
// @Failure 400 {object} handlers.ErrorResponse
// @Failure 409 {object} handlers.ErrorResponse
// @Failure 500 {object} handlers.ErrorResponse
// @Router /auth/register [post]
func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		appHttp.HandleError(c, appErr.ErrInvalidInputWithMessage("invalid request body", err))
		return
	}

	user, err := h.service.CreateUser(req.Name, req.Email, req.Password)
	if err != nil {
		appHttp.HandleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"user": UserResponse{
			ID:    user.ID.String(),
			Name:  user.Name,
			Email: user.Email,
		},
	})
}

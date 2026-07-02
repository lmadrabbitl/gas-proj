package middleware

import (
	"expense-tracker/internal/user"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type contextKey = string

const userIDContextKey contextKey = "userID"
const AuthorizationHeader = "Authorization"

type AuthMiddleware struct {
	userService user.Service
}

func NewAuthMiddleware(userService user.Service) *AuthMiddleware {
	return &AuthMiddleware{
		userService: userService,
	}
}

func (a *AuthMiddleware) CheckAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		token := c.GetHeader(AuthorizationHeader)

		if token == "" {
			log.Printf("No authorization header")
			unauthorized(c)
			return
		}

		const prefix = "Bearer "

		if !strings.HasPrefix(token, prefix) {
			log.Printf("Authorization header has no bearer keyword")
			unauthorized(c)
			return
		}

		token = strings.TrimPrefix(token, prefix)

		userID, err := a.userService.ValidateToken(token)
		if err != nil {
			log.Printf("Invalid token: %s", err.Error())
			unauthorized(c)
			return
		}

		c.Set(userIDContextKey, userID)
		c.Next()
	}
}

func unauthorized(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"error": gin.H{
			"code":  "auth.unauthorized",
			"error": "unauthorized",
		},
	})
}

func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	v, ok := c.Get(userIDContextKey)
	if !ok {
		return uuid.Nil, ok
	}
	id, ok := v.(uuid.UUID)
	return id, ok
}

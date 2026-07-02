package handlers

import (
	"errors"
	appErr "expense-tracker/internal/errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func HandleError(c *gin.Context, err error) {

	var errorOut appErr.AppError

	var customErr *appErr.AppError

	if errors.As(err, &customErr) {
		errorOut = *customErr
	} else {
		errorOut = appErr.AppError{
			Status:  http.StatusInternalServerError,
			Code:    "internal_error",
			Message: "internal error",
		}
	}

	log.Printf("Error: %v", err)

	c.JSON(errorOut.Status, gin.H{
		"error": gin.H{
			"code":  errorOut.Code,
			"error": errorOut.Message,
		},
	})
}

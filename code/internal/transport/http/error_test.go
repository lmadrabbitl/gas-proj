package handlers

import (
	"encoding/json"
	stderrors "errors"
	appErr "expense-tracker/internal/errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleErrorReturnsStructuredAppErrorPayload(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	HandleError(ctx, appErr.ErrAccountNotFound())

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}

	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"error"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error.Code != "account.not_found" || payload.Error.Message != "account not found" {
		t.Fatalf("unexpected error payload: %#v", payload.Error)
	}
}

func TestHandleErrorFallsBackToInternalErrorForGenericErrors(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	HandleError(ctx, stderrors.New("boom"))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}

	var payload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"error"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error.Code != "internal_error" || payload.Error.Message != "internal error" {
		t.Fatalf("unexpected fallback payload: %#v", payload.Error)
	}
}

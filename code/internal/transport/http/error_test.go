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

func TestHandleErrorReturnsOptionalDetails(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	HandleError(ctx, appErr.ErrInvalidInputWithCode("investment.operation.sell.exceeds.position", "sell operation exceeds available quantity for asset history", nil).WithDetails(map[string]any{
		"client_row_id": "row-2",
		"asset_code":    "VALE3",
	}))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}

	var payload struct {
		Error struct {
			Code    string         `json:"code"`
			Message string         `json:"error"`
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload.Error.Code != "investment.operation.sell.exceeds.position" {
		t.Fatalf("unexpected error code: %#v", payload.Error)
	}
	if payload.Error.Details["client_row_id"] != "row-2" || payload.Error.Details["asset_code"] != "VALE3" {
		t.Fatalf("unexpected details payload: %#v", payload.Error.Details)
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

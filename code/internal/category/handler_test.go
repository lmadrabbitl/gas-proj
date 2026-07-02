package category

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appErr "expense-tracker/internal/errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type categoryServiceStub struct {
	addCategoryFn         func(userID uuid.UUID, req CreateCategoryRequest) (*Category, error)
	getCategoriesFn       func(userID uuid.UUID, includeDeactivated bool) ([]Category, error)
	getCategoryByCodeFn   func(userID uuid.UUID, code string) (*Category, error)
	getCategoriesByCodeFn func(userID uuid.UUID, codes []string) ([]Category, error)
	updateCategoryFn      func(userID uuid.UUID, code string, req UpdateCategoryRequest) (*Category, error)
	reorderCategoriesFn   func(userID uuid.UUID, parentCode *string, codes []string) error
	deactivateFn          func(userID uuid.UUID, code string) error
}

func (s *categoryServiceStub) AddCategory(userID uuid.UUID, req CreateCategoryRequest) (*Category, error) {
	return s.addCategoryFn(userID, req)
}

func (s *categoryServiceStub) GetCategories(userID uuid.UUID, includeDeactivated bool) ([]Category, error) {
	return s.getCategoriesFn(userID, includeDeactivated)
}

func (s *categoryServiceStub) GetCategoryByCode(userID uuid.UUID, code string) (*Category, error) {
	return s.getCategoryByCodeFn(userID, code)
}

func (s *categoryServiceStub) GetCategoriesByCode(userID uuid.UUID, codes []string) ([]Category, error) {
	return s.getCategoriesByCodeFn(userID, codes)
}

func (s *categoryServiceStub) UpdateCategory(userID uuid.UUID, code string, req UpdateCategoryRequest) (*Category, error) {
	return s.updateCategoryFn(userID, code, req)
}

func (s *categoryServiceStub) ReorderCategories(userID uuid.UUID, parentCode *string, codes []string) error {
	return s.reorderCategoriesFn(userID, parentCode, codes)
}

func (s *categoryServiceStub) DeactivateCategory(userID uuid.UUID, code string) error {
	return s.deactivateFn(userID, code)
}

func TestHandlerCreateCategoryRequiresUserID(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	service := &categoryServiceStub{
		addCategoryFn: func(userID uuid.UUID, req CreateCategoryRequest) (*Category, error) {
			t.Fatal("expected service not to be called without user ID")
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/categories", bytes.NewBufferString(`{"name":"Salary","type":"INCOME","description":"monthly"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).CreateCategory(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if !strings.Contains(w.Body.String(), appErr.ErrInvalidLoginPassword().Code) {
		t.Fatalf("expected invalid login error, got body: %s", w.Body.String())
	}
}

func TestHandlerCreateCategoryMapsRequestToService(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	parentCode := "income"
	var gotReq CreateCategoryRequest
	var gotUserID uuid.UUID

	service := &categoryServiceStub{
		addCategoryFn: func(callUserID uuid.UUID, req CreateCategoryRequest) (*Category, error) {
			gotUserID = callUserID
			gotReq = req
			return &Category{UserID: callUserID, Code: "salary", Name: req.Name, Type: req.Type, Description: req.Description}, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Request = httptest.NewRequest(http.MethodPost, "/categories", bytes.NewBufferString(`{"name":"Salary","type":"INCOME","description":"monthly","parent_code":"income"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).CreateCategory(c)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, w.Code)
	}
	if gotUserID != userID {
		t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
	}
	if gotReq.Type != CategoryTypeIncome {
		t.Fatalf("expected category type %q, got %q", CategoryTypeIncome, gotReq.Type)
	}
	if gotReq.ParentCode == nil || *gotReq.ParentCode != parentCode {
		t.Fatalf("expected parent code %q to be forwarded, got %+v", parentCode, gotReq.ParentCode)
	}

	var body map[string]map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected JSON response, got error: %v", err)
	}
	if _, ok := body["category"]; !ok {
		t.Fatalf("expected category response body, got %v", body)
	}
}

func TestHandlerCreateCategoryRejectsInvalidType(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	service := &categoryServiceStub{
		addCategoryFn: func(userID uuid.UUID, req CreateCategoryRequest) (*Category, error) {
			t.Fatal("expected service not to be called for invalid type")
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uuid.New())
	c.Request = httptest.NewRequest(http.MethodPost, "/categories", bytes.NewBufferString(`{"name":"Salary","type":"OTHER","description":"monthly"}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).CreateCategory(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandlerUpdateCategoryRejectsInvalidParentCode(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	service := &categoryServiceStub{
		updateCategoryFn: func(userID uuid.UUID, code string, req UpdateCategoryRequest) (*Category, error) {
			t.Fatal("expected service not to be called for invalid parent code")
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", uuid.New())
	c.Params = []gin.Param{{Key: "code", Value: "salary"}}
	c.Request = httptest.NewRequest(http.MethodPatch, "/categories/salary", bytes.NewBufferString(`{"parent_code":""}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).UpdateCategory(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	if !strings.Contains(w.Body.String(), "validation.code.is.required") {
		t.Fatalf("expected validation.code.is.required response, got body: %s", w.Body.String())
	}
}

func TestHandlerReorderCategoriesReturnsNoContent(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	parentCode := "income"
	var gotParentCode *string
	var gotCodes []string

	service := &categoryServiceStub{
		reorderCategoriesFn: func(callUserID uuid.UUID, reqParentCode *string, codes []string) error {
			if callUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, callUserID)
			}
			gotParentCode = reqParentCode
			gotCodes = append([]string(nil), codes...)
			return nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Request = httptest.NewRequest(http.MethodPatch, "/categories/reorder", bytes.NewBufferString(`{"parent_code":"income","codes":["salary","bonus"]}`))
	c.Request.Header.Set("Content-Type", "application/json")

	NewHandler(service).ReorderCategories(c)

	if c.Writer.Status() != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, c.Writer.Status())
	}
	if gotParentCode == nil || *gotParentCode != parentCode {
		t.Fatalf("expected parent code %q, got %+v", parentCode, gotParentCode)
	}
	if len(gotCodes) != 2 || gotCodes[0] != "salary" || gotCodes[1] != "bonus" {
		t.Fatalf("expected reorder codes to be forwarded, got %v", gotCodes)
	}
}

func TestHandlerGetCategoryListReturnsCategories(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	service := &categoryServiceStub{
		getCategoriesFn: func(callUserID uuid.UUID, includeDeactivated bool) ([]Category, error) {
			if callUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, callUserID)
			}
			if includeDeactivated {
				t.Fatal("expected default category list to exclude deactivated rows")
			}
			return []Category{{Code: "income"}}, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Request = httptest.NewRequest(http.MethodGet, "/categories", nil)

	NewHandler(service).GetCategoryList(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if !strings.Contains(w.Body.String(), `"categories"`) {
		t.Fatalf("expected categories payload, got %s", w.Body.String())
	}
}

func TestHandlerGetCategoryListRequiresUserID(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	service := &categoryServiceStub{
		getCategoriesFn: func(userID uuid.UUID, includeDeactivated bool) ([]Category, error) {
			t.Fatal("expected service not to be called without user ID")
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/categories", nil)

	NewHandler(service).GetCategoryList(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandlerGetCategoryByCodeReturnsCategory(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	service := &categoryServiceStub{
		getCategoryByCodeFn: func(callUserID uuid.UUID, code string) (*Category, error) {
			if callUserID != userID || code != "income" {
				t.Fatalf("unexpected args: %s %q", callUserID, code)
			}
			return &Category{Code: code}, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "code", Value: "income"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/categories/income", nil)

	NewHandler(service).GetCategoryByCode(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if !strings.Contains(w.Body.String(), `"category"`) {
		t.Fatalf("expected category payload, got %s", w.Body.String())
	}
}

func TestHandlerGetCategoryByCodeRequiresUserID(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	service := &categoryServiceStub{
		getCategoryByCodeFn: func(userID uuid.UUID, code string) (*Category, error) {
			t.Fatal("expected service not to be called without user ID")
			return nil, nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "code", Value: "income"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/categories/income", nil)

	NewHandler(service).GetCategoryByCode(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandlerUpdateCategoryRejectsInvalidBodyAndType(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	service := &categoryServiceStub{
		updateCategoryFn: func(userID uuid.UUID, code string, req UpdateCategoryRequest) (*Category, error) {
			t.Fatal("expected service not to be called for invalid update")
			return nil, nil
		},
	}

	t.Run("invalid body", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("userID", uuid.New())
		c.Params = []gin.Param{{Key: "code", Value: "salary"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/categories/salary", bytes.NewBufferString(`{"name":`))
		c.Request.Header.Set("Content-Type", "application/json")

		NewHandler(service).UpdateCategory(c)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})

	t.Run("invalid type", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("userID", uuid.New())
		c.Params = []gin.Param{{Key: "code", Value: "salary"}}
		c.Request = httptest.NewRequest(http.MethodPatch, "/categories/salary", bytes.NewBufferString(`{"type":"OTHER"}`))
		c.Request.Header.Set("Content-Type", "application/json")

		NewHandler(service).UpdateCategory(c)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})
}

func TestHandlerDeactivateCategoryReturnsNoContent(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	userID := uuid.New()
	service := &categoryServiceStub{
		deactivateFn: func(callUserID uuid.UUID, code string) error {
			if callUserID != userID || code != "income" {
				t.Fatalf("unexpected args: %s %q", callUserID, code)
			}
			return nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("userID", userID)
	c.Params = []gin.Param{{Key: "code", Value: "income"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/categories/income", nil)

	NewHandler(service).DeactivateCategory(c)

	if c.Writer.Status() != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, c.Writer.Status())
	}
}

func TestHandlerDeactivateCategoryRequiresUserID(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)

	service := &categoryServiceStub{
		deactivateFn: func(userID uuid.UUID, code string) error {
			t.Fatal("expected service not to be called without user ID")
			return nil
		},
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = []gin.Param{{Key: "code", Value: "income"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/categories/income", nil)

	NewHandler(service).DeactivateCategory(c)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

package suggestion

import (
	"bytes"
	"encoding/json"
	appErr "expense-tracker/internal/errors"
	"expense-tracker/internal/middleware"
	"expense-tracker/internal/user"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type handlerServiceStub struct {
	addFn    func(userID uuid.UUID, req CreateSuggestionRequest) (*SuggestionResponseItem, error)
	listFn   func(userID uuid.UUID) ([]SuggestionResponseItem, error)
	getFn    func(userID, suggestionID uuid.UUID) (*SuggestionResponseItem, error)
	updateFn func(userID, suggestionID uuid.UUID, req UpdateSuggestionRequest) (*SuggestionResponseItem, error)
	deleteFn func(userID, suggestionID uuid.UUID) error
}

func (s *handlerServiceStub) AddSuggestion(userID uuid.UUID, req CreateSuggestionRequest) (*SuggestionResponseItem, error) {
	return s.addFn(userID, req)
}
func (s *handlerServiceStub) GetSuggestions(userID uuid.UUID) ([]SuggestionResponseItem, error) {
	return s.listFn(userID)
}
func (s *handlerServiceStub) GetSuggestionByID(userID, suggestionID uuid.UUID) (*SuggestionResponseItem, error) {
	return s.getFn(userID, suggestionID)
}
func (s *handlerServiceStub) UpdateSuggestion(userID, suggestionID uuid.UUID, req UpdateSuggestionRequest) (*SuggestionResponseItem, error) {
	return s.updateFn(userID, suggestionID, req)
}
func (s *handlerServiceStub) DeleteSuggestion(userID, suggestionID uuid.UUID) error {
	return s.deleteFn(userID, suggestionID)
}

type authUserServiceStub struct{}

func (s *authUserServiceStub) LoginUser(email, password string) (string, error) { return "", nil }
func (s *authUserServiceStub) CreateUser(name, email, password string) (*user.User, error) {
	return nil, nil
}
func (s *authUserServiceStub) GenerateToken(userID uuid.UUID) (string, error) { return "", nil }
func (s *authUserServiceStub) ValidateToken(token string) (uuid.UUID, error) {
	if token == "ok" {
		return uuid.MustParse("11111111-1111-1111-1111-111111111111"), nil
	}
	return uuid.Nil, appErr.ErrInvalidLoginPassword()
}

func TestCreateSuggestionReturnsCreatedPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	entryType := SuggestionEntryTypeExpense
	router := gin.New()
	NewHandler(&handlerServiceStub{
		addFn: func(userID uuid.UUID, req CreateSuggestionRequest) (*SuggestionResponseItem, error) {
			if userID == uuid.Nil {
				t.Fatal("expected authenticated user ID")
			}
			return &SuggestionResponseItem{
				ID:                  uuid.MustParse("22222222-2222-2222-2222-222222222222"),
				DescriptionContains: req.DescriptionContains,
				Priority:            req.Priority,
				EntryType:           &entryType,
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			}, nil
		},
		listFn: func(userID uuid.UUID) ([]SuggestionResponseItem, error) { return nil, nil },
		getFn:  func(userID, suggestionID uuid.UUID) (*SuggestionResponseItem, error) { return nil, nil },
		updateFn: func(userID, suggestionID uuid.UUID, req UpdateSuggestionRequest) (*SuggestionResponseItem, error) {
			return nil, nil
		},
		deleteFn: func(userID, suggestionID uuid.UUID) error { return nil },
	}).RegisterRoutes(middleware.NewAuthMiddleware(&authUserServiceStub{}), router)

	req := httptest.NewRequest(http.MethodPost, "/suggestions", bytes.NewBufferString(`{"description_contains":"padaria","priority":1,"entry_type":"EXPENSE"}`))
	req.Header.Set("Authorization", "Bearer ok")
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", recorder.Code)
	}

	var payload map[string]map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload["suggestion"]["description_contains"] != "padaria" {
		t.Fatalf("unexpected payload: %v", payload)
	}
}

func TestGetSuggestionsReturnsList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	NewHandler(&handlerServiceStub{
		addFn: func(userID uuid.UUID, req CreateSuggestionRequest) (*SuggestionResponseItem, error) { return nil, nil },
		listFn: func(userID uuid.UUID) ([]SuggestionResponseItem, error) {
			return []SuggestionResponseItem{{DescriptionContains: "mercado", Priority: 1}}, nil
		},
		getFn: func(userID, suggestionID uuid.UUID) (*SuggestionResponseItem, error) { return nil, nil },
		updateFn: func(userID, suggestionID uuid.UUID, req UpdateSuggestionRequest) (*SuggestionResponseItem, error) {
			return nil, nil
		},
		deleteFn: func(userID, suggestionID uuid.UUID) error { return nil },
	}).RegisterRoutes(middleware.NewAuthMiddleware(&authUserServiceStub{}), router)

	req := httptest.NewRequest(http.MethodGet, "/suggestions", nil)
	req.Header.Set("Authorization", "Bearer ok")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("mercado")) {
		t.Fatalf("expected list payload, got %s", recorder.Body.String())
	}
}

func TestDeleteSuggestionReturnsNoContent(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	NewHandler(&handlerServiceStub{
		addFn:  func(userID uuid.UUID, req CreateSuggestionRequest) (*SuggestionResponseItem, error) { return nil, nil },
		listFn: func(userID uuid.UUID) ([]SuggestionResponseItem, error) { return nil, nil },
		getFn:  func(userID, suggestionID uuid.UUID) (*SuggestionResponseItem, error) { return nil, nil },
		updateFn: func(userID, suggestionID uuid.UUID, req UpdateSuggestionRequest) (*SuggestionResponseItem, error) {
			return nil, nil
		},
		deleteFn: func(userID, suggestionID uuid.UUID) error { return nil },
	}).RegisterRoutes(middleware.NewAuthMiddleware(&authUserServiceStub{}), router)

	req := httptest.NewRequest(http.MethodDelete, "/suggestions/33333333-3333-3333-3333-333333333333", nil)
	req.Header.Set("Authorization", "Bearer ok")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
}

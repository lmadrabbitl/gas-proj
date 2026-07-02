package user

import (
	"bytes"
	"encoding/json"
	appErr "expense-tracker/internal/errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type handlerServiceStub struct {
	createUserFn func(name, email, password string) (*User, error)
	loginUserFn  func(email, password string) (string, error)
}

func (s *handlerServiceStub) GenerateToken(userID uuid.UUID) (string, error) {
	return "", nil
}

func (s *handlerServiceStub) ValidateToken(token string) (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (s *handlerServiceStub) LoginUser(email string, password string) (string, error) {
	return s.loginUserFn(email, password)
}

func (s *handlerServiceStub) CreateUser(name, email, password string) (*User, error) {
	return s.createUserFn(name, email, password)
}

func TestRegisterReturnsSafeUserPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)

	stub := &handlerServiceStub{
		createUserFn: func(name, email, password string) (*User, error) {
			return &User{
				ID:           uuid.MustParse("11111111-1111-1111-1111-111111111111"),
				Name:         "Test User",
				Email:        "test@user.com",
				PasswordHash: "secret",
			}, nil
		},
		loginUserFn: func(email, password string) (string, error) {
			return "token", nil
		},
	}

	router := gin.New()
	NewHandler(stub).RegisterRoutes(router)

	body := bytes.NewBufferString(`{"name":"Test User","email":"test@user.com","password":"password123"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", recorder.Code)
	}

	var payload map[string]map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	userPayload := payload["user"]
	if userPayload["id"] == "" || userPayload["name"] == "" || userPayload["email"] == "" {
		t.Fatalf("expected safe user payload, got %v", userPayload)
	}

	if _, hasPassword := userPayload["PasswordHash"]; hasPassword {
		t.Fatalf("did not expect password hash in response: %v", userPayload)
	}
}

func TestRegisterRejectsInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	NewHandler(&handlerServiceStub{
		createUserFn: func(name, email, password string) (*User, error) { return nil, nil },
		loginUserFn:  func(email, password string) (string, error) { return "", nil },
	}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(`{"email":"test@user.com"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestRegisterReturnsDuplicateEmailError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	NewHandler(&handlerServiceStub{
		createUserFn: func(name, email, password string) (*User, error) {
			return nil, appErr.ErrDuplicateUser()
		},
		loginUserFn: func(email, password string) (string, error) {
			return "", nil
		},
	}).RegisterRoutes(router)

	body := bytes.NewBufferString(`{"name":"Test User","email":"test@user.com","password":"password123"}`)
	req := httptest.NewRequest(http.MethodPost, "/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestLoginReturnsToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	NewHandler(&handlerServiceStub{
		createUserFn: func(name, email, password string) (*User, error) { return nil, nil },
		loginUserFn: func(email, password string) (string, error) {
			if email != "test@user.com" || password != "password123" {
				t.Fatalf("unexpected credentials: %q %q", email, password)
			}
			return "jwt-token", nil
		},
	}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"test@user.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("jwt-token")) {
		t.Fatalf("expected token payload, got %s", recorder.Body.String())
	}
}

func TestLoginRejectsInvalidBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	NewHandler(&handlerServiceStub{
		createUserFn: func(name, email, password string) (*User, error) { return nil, nil },
		loginUserFn:  func(email, password string) (string, error) { return "", nil },
	}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"test@user.com"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestLoginReturnsServiceError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	NewHandler(&handlerServiceStub{
		createUserFn: func(name, email, password string) (*User, error) { return nil, nil },
		loginUserFn: func(email, password string) (string, error) {
			return "", appErr.ErrInvalidLoginPassword()
		},
	}).RegisterRoutes(router)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(`{"email":"test@user.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

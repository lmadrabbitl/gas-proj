package middleware

import (
	"errors"
	"expense-tracker/internal/user"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type authUserServiceStub struct {
	validateTokenFn func(token string) (uuid.UUID, error)
}

func (s *authUserServiceStub) GenerateToken(userID uuid.UUID) (string, error) {
	return "", nil
}

func (s *authUserServiceStub) ValidateToken(token string) (uuid.UUID, error) {
	return s.validateTokenFn(token)
}

func (s *authUserServiceStub) LoginUser(email string, password string) (string, error) {
	return "", nil
}

func (s *authUserServiceStub) CreateUser(name, email, password string) (*user.User, error) {
	return nil, nil
}

func TestCheckAuthMiddlewareRejectsMissingBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/protected", NewAuthMiddleware(&authUserServiceStub{
		validateTokenFn: func(token string) (uuid.UUID, error) {
			return uuid.Nil, nil
		},
	}).CheckAuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
}

func TestCheckAuthMiddlewareRejectsHeaderWithoutBearerPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validateCalled := false
	router := gin.New()
	router.GET("/protected", NewAuthMiddleware(&authUserServiceStub{
		validateTokenFn: func(token string) (uuid.UUID, error) {
			validateCalled = true
			return uuid.Nil, nil
		},
	}).CheckAuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(AuthorizationHeader, "Token abc")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
	if validateCalled {
		t.Fatal("expected token validation not to run without bearer prefix")
	}
}

func TestCheckAuthMiddlewareRejectsInvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/protected", NewAuthMiddleware(&authUserServiceStub{
		validateTokenFn: func(token string) (uuid.UUID, error) {
			if token != "bad-token" {
				t.Fatalf("expected trimmed bearer token, got %q", token)
			}
			return uuid.Nil, errors.New("boom")
		},
	}).CheckAuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(AuthorizationHeader, "Bearer bad-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
}

func TestCheckAuthMiddlewareSetsUserIDAndContinues(t *testing.T) {
	gin.SetMode(gin.TestMode)

	expectedUserID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	router := gin.New()
	router.GET("/protected", NewAuthMiddleware(&authUserServiceStub{
		validateTokenFn: func(token string) (uuid.UUID, error) {
			if token != "good-token" {
				t.Fatalf("expected trimmed bearer token, got %q", token)
			}
			return expectedUserID, nil
		},
	}).CheckAuthMiddleware(), func(c *gin.Context) {
		userID, ok := GetUserID(c)
		if !ok {
			t.Fatal("expected user ID in context")
		}
		if userID != expectedUserID {
			t.Fatalf("expected user ID %s, got %s", expectedUserID, userID)
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(AuthorizationHeader, "Bearer good-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestGetUserIDReturnsFalseForWrongType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("userID", "not-a-uuid")

	userID, ok := GetUserID(context)
	if ok {
		t.Fatal("expected GetUserID to fail for wrong type")
	}
	if userID != uuid.Nil {
		t.Fatalf("expected nil UUID, got %s", userID)
	}
}

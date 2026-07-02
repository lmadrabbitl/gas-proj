package user

import (
	appErr "expense-tracker/internal/errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type inMemoryUserRepo struct {
	users map[string]*User
}

func newInMemoryUserRepo() *inMemoryUserRepo {
	return &inMemoryUserRepo{users: map[string]*User{}}
}

func (r *inMemoryUserRepo) GetByEmail(email string) (*User, error) {
	user, ok := r.users[email]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (r *inMemoryUserRepo) CreateUser(user *User) (*User, error) {
	if _, exists := r.users[user.Email]; exists {
		return nil, appErr.ErrDuplicateUser()
	}
	r.users[user.Email] = user
	return user, nil
}

func TestCreateUserHashesPasswordAndNormalizesFields(t *testing.T) {
	repo := newInMemoryUserRepo()
	service := NewService("secret", repo)

	user, err := service.CreateUser("  Test User  ", "  TEST@User.com ", "password123")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	if user.Name != "Test User" {
		t.Fatalf("expected trimmed user name, got %q", user.Name)
	}

	if user.Email != "test@user.com" {
		t.Fatalf("expected normalized email, got %q", user.Email)
	}

	if user.PasswordHash == "password123" {
		t.Fatal("expected password hash to differ from the original password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte("password123")); err != nil {
		t.Fatalf("expected stored password hash to match original password: %v", err)
	}
}

func TestLoginUserNormalizesEmail(t *testing.T) {
	repo := newInMemoryUserRepo()
	service := NewService("secret", repo)

	createdUser, err := service.CreateUser("Test User", "test@user.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	token, err := service.LoginUser("  TEST@USER.COM ", "password123")
	if err != nil {
		t.Fatalf("LoginUser returned error: %v", err)
	}

	userID, err := service.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken returned error: %v", err)
	}

	if userID != createdUser.ID {
		t.Fatalf("expected token for user %s, got %s", createdUser.ID, userID)
	}
}

func TestLoginUserReturnsInvalidCredentialsForUnknownEmail(t *testing.T) {
	service := NewService("secret", newInMemoryUserRepo())

	_, err := service.LoginUser("missing@user.com", "password123")
	if err == nil {
		t.Fatal("expected error for missing user")
	}

	appError, ok := err.(*appErr.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}

	if appError.Code != "auth.login.invalid_credentials" {
		t.Fatalf("expected auth.login.invalid_credentials error code, got %q", appError.Code)
	}
}

func TestLoginUserReturnsInvalidCredentialsForWrongPassword(t *testing.T) {
	repo := newInMemoryUserRepo()
	service := NewService("secret", repo)

	user, err := service.CreateUser("Test User", "test@user.com", "password123")
	if err != nil {
		t.Fatalf("CreateUser returned error: %v", err)
	}

	if user.ID == uuid.Nil {
		t.Fatal("expected created user ID")
	}

	_, err = service.LoginUser("test@user.com", "not-the-password")
	if err == nil {
		t.Fatal("expected error for wrong password")
	}

	appError, ok := err.(*appErr.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}

	if appError.Code != "auth.login.invalid_credentials" {
		t.Fatalf("expected auth.login.invalid_credentials error code, got %q", appError.Code)
	}
}

func TestGenerateAndValidateTokenRoundTrip(t *testing.T) {
	service := NewService("secret", newInMemoryUserRepo())
	expectedUserID := uuid.New()

	token, err := service.GenerateToken(expectedUserID)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}

	userID, err := service.ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken returned error: %v", err)
	}

	if userID != expectedUserID {
		t.Fatalf("expected token for user %s, got %s", expectedUserID, userID)
	}
}

func TestValidateTokenRejectsTamperedToken(t *testing.T) {
	service := NewService("secret", newInMemoryUserRepo())
	userID := uuid.New()

	token, err := service.GenerateToken(userID)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}

	_, err = service.ValidateToken(token + "tampered")
	if err == nil {
		t.Fatal("expected tampered token to fail validation")
	}
}

func TestValidateTokenRejectsTokenWithoutUserIDStringClaim(t *testing.T) {
	service := NewService("secret", newInMemoryUserRepo()).(*service)

	badToken, err := jwtTokenWithNonStringUserID(service.secretkey)
	if err != nil {
		t.Fatalf("failed to build malformed token: %v", err)
	}

	_, err = service.ValidateToken(badToken)
	if err == nil {
		t.Fatal("expected malformed token to fail validation")
	}
}

func TestNormalizeNameTrimsWhitespace(t *testing.T) {
	got := normalizeName("  Test User  ")
	if got != "Test User" {
		t.Fatalf("expected trimmed name, got %q", got)
	}
}

func TestNormalizeEmailLowercasesAndTrimsWhitespace(t *testing.T) {
	got := NormalizeEmail("  TEST@User.COM ")
	if got != "test@user.com" {
		t.Fatalf("expected normalized email, got %q", got)
	}
}

func TestCheckUserPasswordRejectsTooLongPassword(t *testing.T) {
	password := strings.Repeat("a", 73)
	err := CheckUserPassword(password)
	if err == nil {
		t.Fatal("expected long password to be rejected")
	}
}

func jwtTokenWithNonStringUserID(secret []byte) (string, error) {
	claims := jwt.MapClaims{
		userIDClaimsKey:     123,
		expirationClaimsKey: time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

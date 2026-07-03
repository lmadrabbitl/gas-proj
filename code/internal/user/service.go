package user

import (
	custom_errors "expense-tracker/internal/errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const userIDClaimsKey = "user_id"
const expirationClaimsKey = "exp"

type Service interface {
	GenerateToken(userID uuid.UUID) (string, error)
	ValidateToken(token string) (uuid.UUID, error)
	LoginUser(email string, password string) (string, error)
	CreateUser(name, email, password string) (*User, error)
}

type service struct {
	secretkey []byte
	repo      Repository
	db        *gorm.DB
	bootstrap Bootstrapper
}

func NewService(secret string, repo Repository, db *gorm.DB, bootstrap Bootstrapper) Service {
	return &service{
		secretkey: []byte(secret),
		repo:      repo,
		db:        db,
		bootstrap: bootstrap,
	}
}

func (s *service) GenerateToken(userID uuid.UUID) (string, error) {

	claims := jwt.MapClaims{
		userIDClaimsKey:     userID.String(),
		expirationClaimsKey: time.Now().Add(time.Hour * 500).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretkey)
}

func (s *service) ValidateToken(token string) (uuid.UUID, error) {

	parsedToken, err := jwt.Parse(token, func(jwtToken *jwt.Token) (interface{}, error) {
		return s.secretkey, nil
	})

	if err != nil || !parsedToken.Valid {
		if err != nil {
			log.Printf("Error: %s", err.Error())
		}
		return uuid.Nil, fmt.Errorf("Invalid token1.")
	}
	claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, fmt.Errorf("Invalid token2")
	}

	userUUIDStr, ok := claims[userIDClaimsKey].(string)
	if !ok {
		return uuid.Nil, fmt.Errorf("Invalid token3")
	}

	userUUID, err := uuid.Parse(userUUIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("Invalid token4")
	}

	return userUUID, nil

}

func (s *service) LoginUser(email, password string) (string, error) {
	email = NormalizeEmail(email)

	if err := CheckUserEmail(email); err != nil {
		return "", err
	}
	if err := CheckUserPassword(password); err != nil {
		return "", err
	}

	user, err := s.repo.GetByEmail(email)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", custom_errors.ErrInvalidLoginPassword()
		}
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", custom_errors.ErrInvalidLoginPassword()
	}

	token, err := s.GenerateToken(user.ID)
	if err != nil {
		return "", custom_errors.ErrTokenGeneration()
	}
	return token, nil
}

func (s *service) CreateUser(name, email, password string) (*User, error) {
	name = normalizeName(name)
	email = NormalizeEmail(email)

	if err := CheckUserName(name); err != nil {
		return nil, err
	}

	if err := CheckUserEmail(email); err != nil {
		return nil, err
	}
	if err := CheckUserPassword(password); err != nil {
		return nil, err
	}

	passwordHash, err := getPasswordHash(password)
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:           uuid.New(),
		Name:         name,
		Email:        email,
		PasswordHash: passwordHash,
	}

	if s.db != nil && s.bootstrap != nil {
		err = s.db.Transaction(func(tx *gorm.DB) error {
			created, createErr := s.repo.CreateUserWithDB(tx, user)
			if createErr != nil {
				return createErr
			}
			user = created
			return s.bootstrap.Bootstrap(tx, user.ID)
		})
		if err != nil {
			return nil, err
		}
		return user, nil
	}

	user, err = s.repo.CreateUser(user)
	if err != nil {
		return nil, err
	}
	return user, nil
}
func getPasswordHash(password string) (string, error) {

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func normalizeName(name string) string {
	return strings.TrimSpace(name)
}

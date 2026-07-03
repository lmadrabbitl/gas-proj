package user

import (
	"errors"
	appErr "expense-tracker/internal/errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type Repository interface {
	GetByEmail(string) (*User, error)
	CreateUser(user *User) (*User, error)
	CreateUserWithDB(db *gorm.DB, user *User) (*User, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) GetByEmail(email string) (*User, error) {
	var user *User
	if err := r.db.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

func (r *repository) CreateUser(user *User) (*User, error) {
	return r.CreateUserWithDB(r.db, user)
}

func (r *repository) CreateUserWithDB(db *gorm.DB, user *User) (*User, error) {
	if err := db.Create(user).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, appErr.ErrDuplicateUser()
		}
		return nil, err
	}
	return user, nil
}

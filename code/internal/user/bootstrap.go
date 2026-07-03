package user

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Bootstrapper interface {
	Bootstrap(tx *gorm.DB, userID uuid.UUID) error
}

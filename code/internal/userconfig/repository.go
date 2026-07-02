package userconfig

import (
	"errors"
	appErr "expense-tracker/internal/errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	GetByUserID(userID uuid.UUID) (*UserConfig, error)
	Upsert(config *UserConfig) (*UserConfig, error)
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (repo *repository) GetByUserID(userID uuid.UUID) (*UserConfig, error) {
	var config UserConfig
	err := repo.db.Where("user_id = ?", userID).First(&config).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (repo *repository) Upsert(config *UserConfig) (*UserConfig, error) {
	if err := repo.db.
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"language", "settings", "updated_at"}),
		}).
		Create(config).Error; err != nil {
		return nil, appErr.ErrInvalidInputWithCode(
			"user_config.persist.failed",
			"failed to persist user config",
			err,
		)
	}
	return config, nil
}

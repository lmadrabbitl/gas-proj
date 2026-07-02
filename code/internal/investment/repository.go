package investment

import (
	"errors"
	appErr "expense-tracker/internal/errors"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	CreateAsset(asset *Asset) (*Asset, error)
	ListAssets(userID uuid.UUID) ([]Asset, error)
	GetAssetByCode(userID uuid.UUID, code string) (*Asset, error)
	UpdateAsset(userID uuid.UUID, code string, update *UpdateAsset) (*Asset, error)
	UpdateAssetMetadata(userID uuid.UUID, code string, update *UpdateAssetMetadata) (*Asset, error)
	CreatePortfolio(portfolio *Portfolio) (*Portfolio, error)
	ListPortfolios(userID uuid.UUID) ([]Portfolio, error)
	GetPortfolioByCode(userID uuid.UUID, code string) (*Portfolio, error)
	UpdatePortfolio(userID uuid.UUID, code string, update *UpdatePortfolio) (*Portfolio, error)
	DeletePortfolio(userID uuid.UUID, code string) error
	GetNextPortfolioSortOrder(userID uuid.UUID) (int, error)
	ListPortfolioAssets(userID uuid.UUID) ([]PortfolioAssetRow, error)
	GetPortfolioAsset(userID, portfolioID, assetID uuid.UUID) (*PortfolioAsset, error)
	GetNextPortfolioAssetSortOrder(userID, portfolioID uuid.UUID) (int, error)
	UpsertPortfolioAsset(membership *PortfolioAsset) (*PortfolioAsset, error)
	DeletePortfolioAsset(userID, portfolioID, assetID uuid.UUID) error
	ReorderPortfolioAssets(userID, portfolioID uuid.UUID, codes []string) error
	CreateOperation(db *gorm.DB, operation *Operation) (*Operation, error)
	ListOperations(userID uuid.UUID) ([]OperationRow, error)
	GetOperationByID(userID, operationID uuid.UUID) (*Operation, error)
	UpdateOperation(db *gorm.DB, userID, operationID uuid.UUID, update *UpdateOperationModel) (*Operation, error)
	DeleteOperation(db *gorm.DB, userID, operationID uuid.UUID) error
	ListAssetOperations(db *gorm.DB, userID, assetID uuid.UUID) ([]Operation, error)
	UpsertPosition(db *gorm.DB, position *Position) error
	DeletePosition(db *gorm.DB, userID, assetID uuid.UUID) error
	ListPositions(userID uuid.UUID) ([]PositionRow, error)
	ListAssetQuoteCaches(assetCodes []string) ([]AssetQuoteCache, error)
	UpsertAssetQuoteCache(cache *AssetQuoteCache) error
	DB() *gorm.DB
}

type UpdateAsset struct {
	Code              *string
	Name              *string
	CNPJ              *string
	AssetType         *AssetType
	IsActive          *bool
	MetadataSource    *string
	MetadataUpdatedAt *time.Time
}

type UpdateAssetMetadata struct {
	Name              *string
	CNPJ              *string
	MetadataSource    *string
	MetadataUpdatedAt *time.Time
}

type UpdatePortfolio struct {
	Name        *string
	Description *string
}

type UpdateOperationModel struct {
	AssetID       *uuid.UUID
	OperationType *OperationType
	Date          *time.Time
	Quantity      *int64
	UnitPrice     *int64
	FeeAmount     *int64
	GrossAmount   *int64
	NetAmount     *int64
	Notes         *string
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (repo *repository) DB() *gorm.DB {
	return repo.db
}

func (repo *repository) useDB(db *gorm.DB) *gorm.DB {
	if db != nil {
		return db
	}
	return repo.db
}

func (repo *repository) CreateAsset(asset *Asset) (*Asset, error) {
	if err := repo.db.Create(asset).Error; err != nil {
		return nil, mapCreateError(err, "investment.asset.duplicate", "there is already one asset with that code for this user")
	}
	return asset, nil
}

func (repo *repository) ListAssets(userID uuid.UUID) ([]Asset, error) {
	var assets []Asset
	err := repo.db.Where("user_id = ?", userID).Order("is_active DESC").Order("code ASC").Find(&assets).Error
	return assets, err
}

func (repo *repository) GetAssetByCode(userID uuid.UUID, code string) (*Asset, error) {
	var asset Asset
	err := repo.db.Where("user_id = ? AND code = ?", userID, code).First(&asset).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrInvalidInputWithCode("investment.asset.not_found", "investment asset not found", nil)
		}
		return nil, err
	}
	return &asset, nil
}

func (repo *repository) UpdateAsset(userID uuid.UUID, code string, update *UpdateAsset) (*Asset, error) {
	updates := map[string]interface{}{}
	if update.Code != nil {
		updates["code"] = *update.Code
	}
	if update.Name != nil {
		updates["name"] = *update.Name
	}
	if update.CNPJ != nil {
		cnpj := strings.TrimSpace(*update.CNPJ)
		if cnpj == "" {
			updates["cnpj"] = nil
		} else {
			updates["cnpj"] = onlyDigits(cnpj)
		}
	}
	if update.AssetType != nil {
		updates["asset_type"] = *update.AssetType
	}
	if update.IsActive != nil {
		updates["is_active"] = *update.IsActive
	}
	if update.MetadataSource != nil {
		updates["metadata_source"] = *update.MetadataSource
	}
	if update.MetadataUpdatedAt != nil {
		updates["metadata_updated_at"] = *update.MetadataUpdatedAt
	}

	var asset Asset
	result := repo.db.Model(&Asset{}).
		Where("user_id = ? AND code = ?", userID, code).
		Clauses(clause.Returning{}).
		Updates(updates).
		Scan(&asset)
	if result.Error != nil {
		return nil, mapCreateError(result.Error, "investment.asset.duplicate", "there is already one asset with that code for this user")
	}
	if result.RowsAffected == 0 {
		return nil, appErr.ErrInvalidInputWithCode("investment.asset.not_found", "investment asset not found", nil)
	}
	return &asset, nil
}

func (repo *repository) UpdateAssetMetadata(userID uuid.UUID, code string, update *UpdateAssetMetadata) (*Asset, error) {
	updates := map[string]interface{}{}
	if update.Name != nil {
		updates["name"] = *update.Name
	}
	if update.CNPJ != nil {
		updates["cnpj"] = *update.CNPJ
	}
	if update.MetadataSource != nil {
		updates["metadata_source"] = *update.MetadataSource
	}
	if update.MetadataUpdatedAt != nil {
		updates["metadata_updated_at"] = *update.MetadataUpdatedAt
	}

	var asset Asset
	result := repo.db.Model(&Asset{}).
		Where("user_id = ? AND code = ?", userID, code).
		Clauses(clause.Returning{}).
		Updates(updates).
		Scan(&asset)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, appErr.ErrInvalidInputWithCode("investment.asset.not_found", "investment asset not found", nil)
	}
	return &asset, nil
}

func (repo *repository) CreatePortfolio(portfolio *Portfolio) (*Portfolio, error) {
	if err := repo.db.Create(portfolio).Error; err != nil {
		return nil, mapCreateError(err, "investment.portfolio.duplicate", "there is already one portfolio with that code for this user")
	}
	return portfolio, nil
}

func (repo *repository) ListPortfolios(userID uuid.UUID) ([]Portfolio, error) {
	var portfolios []Portfolio
	err := repo.db.Where("user_id = ?", userID).Order("sort_order ASC").Order("created_at ASC").Find(&portfolios).Error
	return portfolios, err
}

func (repo *repository) GetPortfolioByCode(userID uuid.UUID, code string) (*Portfolio, error) {
	var portfolio Portfolio
	err := repo.db.Where("user_id = ? AND code = ?", userID, code).First(&portfolio).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrInvalidInputWithCode("investment.portfolio.not_found", "investment portfolio not found", nil)
		}
		return nil, err
	}
	return &portfolio, nil
}

func (repo *repository) UpdatePortfolio(userID uuid.UUID, code string, update *UpdatePortfolio) (*Portfolio, error) {
	updates := map[string]interface{}{}
	if update.Name != nil {
		updates["name"] = *update.Name
	}
	if update.Description != nil {
		updates["description"] = *update.Description
	}
	var portfolio Portfolio
	result := repo.db.Model(&Portfolio{}).
		Where("user_id = ? AND code = ?", userID, code).
		Clauses(clause.Returning{}).
		Updates(updates).
		Scan(&portfolio)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, appErr.ErrInvalidInputWithCode("investment.portfolio.not_found", "investment portfolio not found", nil)
	}
	return &portfolio, nil
}

func (repo *repository) DeletePortfolio(userID uuid.UUID, code string) error {
	result := repo.db.Where("user_id = ? AND code = ?", userID, code).Delete(&Portfolio{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return appErr.ErrInvalidInputWithCode("investment.portfolio.not_found", "investment portfolio not found", nil)
	}
	return nil
}

func (repo *repository) GetNextPortfolioSortOrder(userID uuid.UUID) (int, error) {
	var next int
	err := repo.db.Raw(`
		SELECT COALESCE(MAX(sort_order), 0) + 1
		FROM investment_portfolios
		WHERE user_id = ?
	`, userID).Scan(&next).Error
	return next, err
}

func (repo *repository) ListPortfolioAssets(userID uuid.UUID) ([]PortfolioAssetRow, error) {
	var rows []PortfolioAssetRow
	err := repo.db.Raw(`
		SELECT
			p.code AS portfolio_code,
			p.name AS portfolio_name,
			p.description AS portfolio_description,
			p.sort_order AS portfolio_sort_order,
			a.code AS asset_code,
			a.name AS asset_name,
			a.asset_type AS asset_type,
			pa.target_allocation_bps,
			pa.max_buy_price,
			pa.sort_order
		FROM investment_portfolios p
		JOIN investment_portfolio_assets pa
			ON pa.portfolio_id = p.id
			AND pa.user_id = p.user_id
		JOIN investment_assets a
			ON a.id = pa.asset_id
			AND a.user_id = pa.user_id
		WHERE p.user_id = ?
		ORDER BY p.sort_order ASC, p.created_at ASC, pa.sort_order ASC, a.code ASC
	`, userID).Scan(&rows).Error
	return rows, err
}

func (repo *repository) GetPortfolioAsset(userID, portfolioID, assetID uuid.UUID) (*PortfolioAsset, error) {
	var membership PortfolioAsset
	err := repo.db.Where("user_id = ? AND portfolio_id = ? AND asset_id = ?", userID, portfolioID, assetID).First(&membership).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &membership, nil
}

func (repo *repository) GetNextPortfolioAssetSortOrder(userID, portfolioID uuid.UUID) (int, error) {
	var next int
	err := repo.db.Raw(`
		SELECT COALESCE(MAX(sort_order), 0) + 1
		FROM investment_portfolio_assets
		WHERE user_id = ? AND portfolio_id = ?
	`, userID, portfolioID).Scan(&next).Error
	return next, err
}

func (repo *repository) UpsertPortfolioAsset(membership *PortfolioAsset) (*PortfolioAsset, error) {
	err := repo.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "portfolio_id"},
			{Name: "asset_id"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"target_allocation_bps": membership.TargetAllocationBPS,
			"max_buy_price":         membership.MaxBuyPrice,
			"sort_order":            membership.SortOrder,
			"updated_at":            time.Now(),
		}),
	}).Create(membership).Error
	if err != nil {
		return nil, err
	}
	return membership, nil
}

func (repo *repository) DeletePortfolioAsset(userID, portfolioID, assetID uuid.UUID) error {
	result := repo.db.Where("user_id = ? AND portfolio_id = ? AND asset_id = ?", userID, portfolioID, assetID).Delete(&PortfolioAsset{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return appErr.ErrInvalidInputWithCode("investment.portfolio.asset.not_found", "investment portfolio asset not found", nil)
	}
	return nil
}

func (repo *repository) ReorderPortfolioAssets(userID, portfolioID uuid.UUID, codes []string) error {
	return repo.db.Transaction(func(tx *gorm.DB) error {
		for index, code := range codes {
			result := tx.Model(&PortfolioAsset{}).
				Where(`
					user_id = ?
					AND portfolio_id = ?
					AND asset_id IN (
						SELECT id
						FROM investment_assets
						WHERE user_id = ?
						AND code = ?
					)
				`, userID, portfolioID, userID, code).
				Update("sort_order", index+1)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return appErr.ErrInvalidInputWithCode("investment.portfolio.asset.not_found", "investment portfolio asset not found", nil)
			}
		}
		return nil
	})
}

func (repo *repository) CreateOperation(db *gorm.DB, operation *Operation) (*Operation, error) {
	if err := repo.useDB(db).Create(operation).Error; err != nil {
		return nil, err
	}
	return operation, nil
}

func (repo *repository) ListOperations(userID uuid.UUID) ([]OperationRow, error) {
	rows := make([]OperationRow, 0)
	err := repo.db.Raw(`
		SELECT
			o.id,
			a.code AS asset_code,
			a.name AS asset_name,
			a.asset_type AS asset_type,
			o.operation_type,
			o.date,
			o.quantity,
			o.unit_price,
			o.fee_amount,
			o.gross_amount,
			o.net_amount,
			o.notes,
			o.created_at,
			o.updated_at
		FROM investment_operations o
		JOIN investment_assets a ON a.id = o.asset_id
		WHERE o.user_id = ?
		ORDER BY o.date DESC, o.created_at DESC, o.id DESC
	`, userID).Scan(&rows).Error
	return rows, err
}

func (repo *repository) GetOperationByID(userID, operationID uuid.UUID) (*Operation, error) {
	var operation Operation
	err := repo.db.Where("user_id = ? AND id = ?", userID, operationID).First(&operation).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrInvalidInputWithCode("investment.operation.not_found", "investment operation not found", nil)
		}
		return nil, err
	}
	return &operation, nil
}

func (repo *repository) UpdateOperation(db *gorm.DB, userID, operationID uuid.UUID, update *UpdateOperationModel) (*Operation, error) {
	updates := map[string]interface{}{}
	if update.AssetID != nil {
		updates["asset_id"] = *update.AssetID
	}
	if update.OperationType != nil {
		updates["operation_type"] = *update.OperationType
	}
	if update.Date != nil {
		updates["date"] = *update.Date
	}
	if update.Quantity != nil {
		updates["quantity"] = *update.Quantity
	}
	if update.UnitPrice != nil {
		updates["unit_price"] = *update.UnitPrice
	}
	if update.FeeAmount != nil {
		updates["fee_amount"] = *update.FeeAmount
	}
	if update.GrossAmount != nil {
		updates["gross_amount"] = *update.GrossAmount
	}
	if update.NetAmount != nil {
		updates["net_amount"] = *update.NetAmount
	}
	if update.Notes != nil {
		updates["notes"] = *update.Notes
	}

	var operation Operation
	result := repo.useDB(db).Model(&Operation{}).
		Where("user_id = ? AND id = ?", userID, operationID).
		Clauses(clause.Returning{}).
		Updates(updates).
		Scan(&operation)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, appErr.ErrInvalidInputWithCode("investment.operation.not_found", "investment operation not found", nil)
	}
	return &operation, nil
}

func (repo *repository) DeleteOperation(db *gorm.DB, userID, operationID uuid.UUID) error {
	result := repo.useDB(db).Where("user_id = ? AND id = ?", userID, operationID).Delete(&Operation{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return appErr.ErrInvalidInputWithCode("investment.operation.not_found", "investment operation not found", nil)
	}
	return nil
}

func (repo *repository) ListAssetOperations(db *gorm.DB, userID, assetID uuid.UUID) ([]Operation, error) {
	var operations []Operation
	err := repo.useDB(db).
		Where("user_id = ? AND asset_id = ?", userID, assetID).
		Order("date ASC").
		Order("created_at ASC").
		Order("id ASC").
		Find(&operations).Error
	return operations, err
}

func (repo *repository) UpsertPosition(db *gorm.DB, position *Position) error {
	return repo.useDB(db).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "user_id"},
			{Name: "asset_id"},
		},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"current_quantity":     position.CurrentQuantity,
			"average_price":        position.AveragePrice,
			"total_cost_basis":     position.TotalCostBasis,
			"realized_pnl":         position.RealizedPNL,
			"last_recalculated_at": position.LastRecalculatedAt,
			"updated_at":           time.Now(),
		}),
	}).Create(position).Error
}

func (repo *repository) DeletePosition(db *gorm.DB, userID, assetID uuid.UUID) error {
	return repo.useDB(db).Where("user_id = ? AND asset_id = ?", userID, assetID).Delete(&Position{}).Error
}

func (repo *repository) ListPositions(userID uuid.UUID) ([]PositionRow, error) {
	rows := make([]PositionRow, 0)
	err := repo.db.Raw(`
		SELECT
			a.code AS asset_code,
			a.name AS asset_name,
			a.asset_type AS asset_type,
			p.current_quantity,
			p.average_price,
			p.total_cost_basis,
			p.realized_pnl,
			p.last_recalculated_at AS last_recalculated
		FROM investment_positions p
		JOIN investment_assets a ON a.id = p.asset_id
		WHERE p.user_id = ?
		AND p.current_quantity > 0
		ORDER BY a.code ASC
	`, userID).Scan(&rows).Error
	return rows, err
}

func (repo *repository) ListAssetQuoteCaches(assetCodes []string) ([]AssetQuoteCache, error) {
	if len(assetCodes) == 0 {
		return []AssetQuoteCache{}, nil
	}
	var rows []AssetQuoteCache
	err := repo.db.Where("asset_code IN ?", assetCodes).Find(&rows).Error
	return rows, err
}

func (repo *repository) UpsertAssetQuoteCache(cache *AssetQuoteCache) error {
	return repo.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "asset_code"}},
		DoUpdates: clause.Assignments(map[string]any{
			"current_price":    cache.CurrentPrice,
			"quote_updated_at": cache.QuoteUpdatedAt,
			"source":           cache.Source,
			"fetched_at":       cache.FetchedAt,
			"updated_at":       cache.UpdatedAt,
		}),
	}).Create(cache).Error
}

func mapCreateError(err error, code, message string) error {
	var pgErr *pgconn.PgError
	log.Printf("Error in creation: %s", err.Error())
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23503":
			return appErr.ErrUserNotFound()
		case "23505":
			return appErr.ErrInvalidInputWithCode(code, message, nil)
		}
	}
	return err
}

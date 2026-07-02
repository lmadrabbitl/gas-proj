package investment

import (
	"context"
	"errors"
	appErr "expense-tracker/internal/errors"
	"expense-tracker/internal/slugutil"
	"expense-tracker/internal/transaction"
	"expense-tracker/internal/userconfig"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	investmentQuoteCacheTTL          = 30 * time.Minute
	maxSuggestedInvestmentSearchStep = int64(1_000_000_00)
)

type Service interface {
	CreateAsset(userID uuid.UUID, req CreateAssetRequest) (*Asset, error)
	ListAssets(userID uuid.UUID) ([]Asset, error)
	UpdateAsset(userID uuid.UUID, code string, req UpdateAssetRequest) (*Asset, error)
	RefreshAssetMetadata(userID uuid.UUID, code string) (*Asset, error)
	RefreshMissingAssetMetadata(userID uuid.UUID) (int, error)
	CreatePortfolio(userID uuid.UUID, req CreatePortfolioRequest) (*Portfolio, error)
	ListPortfolios(userID uuid.UUID) ([]PortfolioResponse, error)
	UpdatePortfolio(userID uuid.UUID, code string, req UpdatePortfolioRequest) (*Portfolio, error)
	DeletePortfolio(userID uuid.UUID, code string) error
	SavePortfolioAsset(userID uuid.UUID, portfolioCode, assetCode string, req SavePortfolioAssetRequest) error
	DeletePortfolioAsset(userID uuid.UUID, portfolioCode, assetCode string) error
	ReorderPortfolioAssets(userID uuid.UUID, portfolioCode string, assetCodes []string) error
	CreateOperation(userID uuid.UUID, req CreateOperationRequest) (*OperationRow, error)
	CreateOperationsBulk(userID uuid.UUID, req CreateBulkOperationsRequest) ([]OperationRow, error)
	ListOperations(userID uuid.UUID) ([]OperationRow, error)
	UpdateOperation(userID uuid.UUID, operationID uuid.UUID, req UpdateOperationRequest) (*OperationRow, error)
	DeleteOperation(userID uuid.UUID, operationID uuid.UUID) error
	ListPositions(userID uuid.UUID) ([]PositionRow, error)
	ListPositionQuotes(userID uuid.UUID) ([]PositionQuoteRow, error)
	AnalyzePortfolio(userID uuid.UUID, portfolioCode string) (*PortfolioAnalysisResponse, error)
	SuggestPortfolioInvestment(userID uuid.UUID, portfolioCode string, investmentAmount int64) (*PortfolioSuggestionResponse, error)
}

type service struct {
	repo                  Repository
	quoteProvider         QuoteProvider
	assetMetadataProvider AssetMetadataProvider
	userConfigService     UserConfigService
	transactionReader     TransactionReader
}

type UserConfigService interface {
	GetConfig(userID uuid.UUID) (*userconfig.Config, error)
}

type TransactionReader interface {
	ListVisibleByCategoryIDs(userID uuid.UUID, categoryIDs []uuid.UUID) ([]transaction.TransactionCategoryMatchRow, error)
}

type investmentIncomeAsset struct {
	Code string
	Name string
	Type AssetType
}

type PortfolioResponse struct {
	Code        string                   `json:"code"`
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	SortOrder   int                      `json:"sort_order"`
	Assets      []PortfolioAssetResponse `json:"assets"`
}

type PortfolioAssetResponse struct {
	AssetCode                  string    `json:"asset_code"`
	AssetName                  string    `json:"asset_name"`
	AssetType                  AssetType `json:"asset_type"`
	TargetAllocationBasisPoint int       `json:"target_allocation_basis_point"`
	MaxBuyPrice                *int64    `json:"max_buy_price"`
	SortOrder                  int       `json:"sort_order"`
}

type CreateAssetRequest struct {
	Code      string
	Name      string
	AssetType AssetType
}

type UpdateAssetRequest struct {
	Code      *string
	Name      *string
	CNPJ      *string
	AssetType *AssetType
	IsActive  *bool
}

type CreatePortfolioRequest struct {
	Name        string
	Description string
}

type UpdatePortfolioRequest struct {
	Name        *string
	Description *string
}

type SavePortfolioAssetRequest struct {
	TargetAllocationBasisPoint int
	MaxBuyPrice                *int64
	SortOrder                  *int
}

type CreateOperationRequest struct {
	AssetCode     string
	OperationType OperationType
	Date          time.Time
	Quantity      int64
	UnitPrice     int64
	FeeAmount     int64
	Notes         string
}

type CreateBulkOperationsRequest struct {
	Operations []CreateBulkOperationRequest
}

type CreateBulkOperationRequest struct {
	AssetCode      string
	OperationType  OperationType
	Date           time.Time
	Quantity       int64
	UnitPrice      int64
	TotalFeeAmount int64
	Notes          string
}

type UpdateOperationRequest struct {
	AssetCode     *string
	OperationType *OperationType
	Date          *time.Time
	Quantity      *int64
	UnitPrice     *int64
	FeeAmount     *int64
	Notes         *string
}

func NewService(repo Repository, quoteProvider QuoteProvider, assetMetadataProvider AssetMetadataProvider, userConfigService UserConfigService, transactionReader TransactionReader) Service {
	return &service{repo: repo, quoteProvider: quoteProvider, assetMetadataProvider: assetMetadataProvider, userConfigService: userConfigService, transactionReader: transactionReader}
}

func (s *service) CreateAsset(userID uuid.UUID, req CreateAssetRequest) (*Asset, error) {
	req.Code = normalizeAssetCode(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	if err := CheckAssetCode(req.Code); err != nil {
		return nil, err
	}
	if err := CheckAssetName(req.Name); err != nil {
		return nil, err
	}
	if err := CheckAssetType(req.AssetType); err != nil {
		return nil, err
	}

	return s.repo.CreateAsset(&Asset{
		ID:        uuid.New(),
		UserID:    userID,
		Code:      req.Code,
		Name:      req.Name,
		AssetType: req.AssetType,
		IsActive:  true,
	})
}

func (s *service) ListAssets(userID uuid.UUID) ([]Asset, error) {
	assets, err := s.repo.ListAssets(userID)
	if err != nil {
		return nil, err
	}
	s.refreshMissingAssetMetadata(userID, assets)
	return assets, nil
}

func (s *service) UpdateAsset(userID uuid.UUID, code string, req UpdateAssetRequest) (*Asset, error) {
	if err := CheckAssetCode(code); err != nil {
		return nil, err
	}
	update := &UpdateAsset{}
	if req.Code != nil {
		normalized := normalizeAssetCode(*req.Code)
		if err := CheckAssetCode(normalized); err != nil {
			return nil, err
		}
		update.Code = &normalized
	}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if err := CheckAssetName(trimmed); err != nil {
			return nil, err
		}
		update.Name = &trimmed
	}
	if req.CNPJ != nil {
		trimmed := strings.TrimSpace(*req.CNPJ)
		update.CNPJ = &trimmed
	}
	if req.AssetType != nil {
		if err := CheckAssetType(*req.AssetType); err != nil {
			return nil, err
		}
		update.AssetType = req.AssetType
	}
	if req.IsActive != nil {
		update.IsActive = req.IsActive
	}
	if update.Name != nil || update.CNPJ != nil {
		source := "manual"
		update.MetadataSource = &source
		now := time.Now().UTC()
		update.MetadataUpdatedAt = &now
	}
	if update.Code == nil && update.Name == nil && update.CNPJ == nil && update.AssetType == nil && update.IsActive == nil {
		return nil, appErr.ErrInvalidInputWithMessage("at least one asset field must be provided", nil)
	}
	return s.repo.UpdateAsset(userID, normalizeAssetCode(code), update)
}

func (s *service) RefreshAssetMetadata(userID uuid.UUID, code string) (*Asset, error) {
	normalizedCode := normalizeAssetCode(code)
	if err := CheckAssetCode(normalizedCode); err != nil {
		return nil, err
	}
	asset, err := s.repo.GetAssetByCode(userID, normalizedCode)
	if err != nil {
		return nil, err
	}
	return s.refreshAssetMetadata(userID, *asset, true)
}

func (s *service) RefreshMissingAssetMetadata(userID uuid.UUID) (int, error) {
	assets, err := s.repo.ListAssets(userID)
	if err != nil {
		return 0, err
	}
	updated := 0
	for index := range assets {
		asset := assets[index]
		if !assetNeedsMetadataRefresh(asset) {
			continue
		}
		if _, err := s.refreshAssetMetadata(userID, asset, false); err != nil {
			log.Printf("investment metadata unavailable for %s: %v", asset.Code, err)
			continue
		}
		updated++
	}
	return updated, nil
}

func (s *service) CreatePortfolio(userID uuid.UUID, req CreatePortfolioRequest) (*Portfolio, error) {
	name := strings.TrimSpace(req.Name)
	if err := CheckPortfolioName(name); err != nil {
		return nil, err
	}
	portfolios, err := s.repo.ListPortfolios(userID)
	if err != nil {
		return nil, err
	}
	existingCodes := make(map[string]struct{}, len(portfolios))
	for _, portfolio := range portfolios {
		existingCodes[portfolio.Code] = struct{}{}
	}
	sortOrder, err := s.repo.GetNextPortfolioSortOrder(userID)
	if err != nil {
		return nil, err
	}
	return s.repo.CreatePortfolio(&Portfolio{
		ID:          uuid.New(),
		UserID:      userID,
		Code:        slugutil.GenerateUnique(name, "portfolio", existingCodes),
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		SortOrder:   sortOrder,
	})
}

func (s *service) ListPortfolios(userID uuid.UUID) ([]PortfolioResponse, error) {
	portfolios, err := s.repo.ListPortfolios(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.repo.ListPortfolioAssets(userID)
	if err != nil {
		return nil, err
	}
	byCode := make(map[string]*PortfolioResponse, len(portfolios))
	out := make([]PortfolioResponse, 0, len(portfolios))
	for _, portfolio := range portfolios {
		item := PortfolioResponse{
			Code:        portfolio.Code,
			Name:        portfolio.Name,
			Description: portfolio.Description,
			SortOrder:   portfolio.SortOrder,
			Assets:      []PortfolioAssetResponse{},
		}
		out = append(out, item)
		byCode[portfolio.Code] = &out[len(out)-1]
	}
	for _, row := range rows {
		item := byCode[row.PortfolioCode]
		if item == nil {
			continue
		}
		item.Assets = append(item.Assets, PortfolioAssetResponse{
			AssetCode:                  row.AssetCode,
			AssetName:                  row.AssetName,
			AssetType:                  row.AssetType,
			TargetAllocationBasisPoint: row.TargetAllocationBPS,
			MaxBuyPrice:                row.MaxBuyPrice,
			SortOrder:                  row.SortOrder,
		})
	}
	return out, nil
}

func (s *service) UpdatePortfolio(userID uuid.UUID, code string, req UpdatePortfolioRequest) (*Portfolio, error) {
	if err := CheckPortfolioCode(code); err != nil {
		return nil, err
	}
	update := &UpdatePortfolio{}
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		if err := CheckPortfolioName(trimmed); err != nil {
			return nil, err
		}
		update.Name = &trimmed
	}
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		update.Description = &trimmed
	}
	if update.Name == nil && update.Description == nil {
		return nil, appErr.ErrInvalidInputWithMessage("at least one portfolio field must be provided", nil)
	}
	return s.repo.UpdatePortfolio(userID, strings.ToLower(code), update)
}

func (s *service) DeletePortfolio(userID uuid.UUID, code string) error {
	if err := CheckPortfolioCode(code); err != nil {
		return err
	}
	return s.repo.DeletePortfolio(userID, strings.ToLower(code))
}

func (s *service) SavePortfolioAsset(userID uuid.UUID, portfolioCode, assetCode string, req SavePortfolioAssetRequest) error {
	if err := CheckPortfolioCode(portfolioCode); err != nil {
		return err
	}
	if err := CheckAssetCode(assetCode); err != nil {
		return err
	}
	if err := CheckTargetAllocationBPS(req.TargetAllocationBasisPoint); err != nil {
		return err
	}
	if req.MaxBuyPrice != nil {
		if err := CheckMoney("max buy price", *req.MaxBuyPrice, true); err != nil {
			return err
		}
	}

	portfolio, err := s.repo.GetPortfolioByCode(userID, strings.ToLower(portfolioCode))
	if err != nil {
		return err
	}
	asset, err := s.repo.GetAssetByCode(userID, normalizeAssetCode(assetCode))
	if err != nil {
		return err
	}
	existing, err := s.repo.GetPortfolioAsset(userID, portfolio.ID, asset.ID)
	if err != nil {
		return err
	}
	sortOrder := 0
	if req.SortOrder != nil {
		if err := CheckPortfolioAssetSortOrder(*req.SortOrder); err != nil {
			return err
		}
		sortOrder = *req.SortOrder
	} else if existing != nil {
		sortOrder = existing.SortOrder
	} else {
		sortOrder, err = s.repo.GetNextPortfolioAssetSortOrder(userID, portfolio.ID)
		if err != nil {
			return err
		}
	}
	id := uuid.New()
	if existing != nil {
		id = existing.ID
	}
	_, err = s.repo.UpsertPortfolioAsset(&PortfolioAsset{
		ID:                  id,
		UserID:              userID,
		PortfolioID:         portfolio.ID,
		AssetID:             asset.ID,
		TargetAllocationBPS: req.TargetAllocationBasisPoint,
		MaxBuyPrice:         req.MaxBuyPrice,
		SortOrder:           sortOrder,
	})
	return err
}

func (s *service) DeletePortfolioAsset(userID uuid.UUID, portfolioCode, assetCode string) error {
	portfolio, err := s.repo.GetPortfolioByCode(userID, strings.ToLower(portfolioCode))
	if err != nil {
		return err
	}
	asset, err := s.repo.GetAssetByCode(userID, normalizeAssetCode(assetCode))
	if err != nil {
		return err
	}
	return s.repo.DeletePortfolioAsset(userID, portfolio.ID, asset.ID)
}

func (s *service) ReorderPortfolioAssets(userID uuid.UUID, portfolioCode string, assetCodes []string) error {
	if err := CheckPortfolioCode(portfolioCode); err != nil {
		return err
	}
	if len(assetCodes) == 0 {
		return appErr.ErrInvalidInputWithMessage("portfolio asset order must include all assets exactly once", nil)
	}

	portfolio, err := s.repo.GetPortfolioByCode(userID, strings.ToLower(portfolioCode))
	if err != nil {
		return err
	}

	normalizedCodes := make([]string, 0, len(assetCodes))
	seenCodes := make(map[string]struct{}, len(assetCodes))
	for _, code := range assetCodes {
		normalized := normalizeAssetCode(code)
		if err := CheckAssetCode(normalized); err != nil {
			return err
		}
		if _, exists := seenCodes[normalized]; exists {
			return appErr.ErrInvalidInputWithMessage("portfolio asset order must include all assets exactly once", nil)
		}
		seenCodes[normalized] = struct{}{}
		normalizedCodes = append(normalizedCodes, normalized)
	}

	rows, err := s.repo.ListPortfolioAssets(userID)
	if err != nil {
		return err
	}
	existingCodes := make([]string, 0)
	for _, row := range rows {
		if row.PortfolioCode != portfolio.Code {
			continue
		}
		existingCodes = append(existingCodes, row.AssetCode)
	}
	if len(existingCodes) != len(normalizedCodes) {
		return appErr.ErrInvalidInputWithMessage("portfolio asset order must include all assets exactly once", nil)
	}
	for _, code := range existingCodes {
		if _, exists := seenCodes[code]; !exists {
			return appErr.ErrInvalidInputWithMessage("portfolio asset order must include all assets exactly once", nil)
		}
	}

	return s.repo.ReorderPortfolioAssets(userID, portfolio.ID, normalizedCodes)
}

func (s *service) CreateOperation(userID uuid.UUID, req CreateOperationRequest) (*OperationRow, error) {
	req.AssetCode = normalizeAssetCode(req.AssetCode)
	req.Notes = strings.TrimSpace(req.Notes)
	if err := s.validateCreateOperation(req); err != nil {
		return nil, err
	}
	asset, err := s.repo.GetAssetByCode(userID, req.AssetCode)
	if err != nil {
		return nil, err
	}
	grossAmount := req.Quantity * req.UnitPrice
	netAmount := computeNetAmount(req.OperationType, grossAmount, req.FeeAmount)

	var created *Operation
	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		created, err = s.repo.CreateOperation(tx, &Operation{
			ID:            uuid.New(),
			UserID:        userID,
			AssetID:       asset.ID,
			OperationType: req.OperationType,
			Date:          req.Date,
			Quantity:      req.Quantity,
			UnitPrice:     req.UnitPrice,
			FeeAmount:     req.FeeAmount,
			GrossAmount:   grossAmount,
			NetAmount:     netAmount,
			Notes:         req.Notes,
		})
		if err != nil {
			return err
		}
		return s.rebuildPosition(tx, userID, asset.ID)
	})
	if err != nil {
		return nil, err
	}
	return s.operationToRow(userID, created)
}

func (s *service) CreateOperationsBulk(userID uuid.UUID, req CreateBulkOperationsRequest) ([]OperationRow, error) {
	if len(req.Operations) == 0 {
		return nil, appErr.ErrInvalidInputWithMessage("at least one operation is required", nil)
	}

	prepared := make([]preparedBulkOperation, 0, len(req.Operations))
	assetsByCode := make(map[string]*Asset, len(req.Operations))
	for _, item := range req.Operations {
		normalized := CreateOperationRequest{
			AssetCode:     normalizeAssetCode(item.AssetCode),
			OperationType: item.OperationType,
			Date:          item.Date,
			Quantity:      item.Quantity,
			UnitPrice:     item.UnitPrice,
			Notes:         strings.TrimSpace(item.Notes),
		}
		if err := s.validateCreateOperation(normalized); err != nil {
			return nil, err
		}
		if err := CheckMoney("total fee amount", item.TotalFeeAmount, true); err != nil {
			return nil, err
		}

		asset, ok := assetsByCode[normalized.AssetCode]
		if !ok {
			foundAsset, err := s.getOrCreateAssetForBulk(userID, normalized.AssetCode)
			if err != nil {
				return nil, err
			}
			asset = foundAsset
			assetsByCode[normalized.AssetCode] = asset
		}

		prepared = append(prepared, preparedBulkOperation{
			asset:          asset,
			assetCode:      normalized.AssetCode,
			operationType:  normalized.OperationType,
			date:           normalized.Date,
			quantity:       normalized.Quantity,
			unitPrice:      normalized.UnitPrice,
			totalFeeAmount: item.TotalFeeAmount,
			notes:          normalized.Notes,
			grossAmount:    normalized.Quantity * normalized.UnitPrice,
		})
	}

	if err := allocateBulkFees(prepared); err != nil {
		return nil, err
	}

	createdIDs := make([]uuid.UUID, 0, len(prepared))
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		affectedAssets := make([]uuid.UUID, 0, len(prepared))
		for _, item := range prepared {
			netAmount := computeNetAmount(item.operationType, item.grossAmount, item.allocatedFee)
			created, err := s.repo.CreateOperation(tx, &Operation{
				ID:            uuid.New(),
				UserID:        userID,
				AssetID:       item.asset.ID,
				OperationType: item.operationType,
				Date:          item.date,
				Quantity:      item.quantity,
				UnitPrice:     item.unitPrice,
				FeeAmount:     item.allocatedFee,
				GrossAmount:   item.grossAmount,
				NetAmount:     netAmount,
				Notes:         item.notes,
			})
			if err != nil {
				return err
			}
			createdIDs = append(createdIDs, created.ID)
			affectedAssets = append(affectedAssets, item.asset.ID)
		}

		for _, assetID := range uniqueUUIDs(affectedAssets) {
			if err := s.rebuildPosition(tx, userID, assetID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	rows, err := s.repo.ListOperations(userID)
	if err != nil {
		return nil, err
	}
	createdSet := make(map[uuid.UUID]struct{}, len(createdIDs))
	for _, id := range createdIDs {
		createdSet[id] = struct{}{}
	}
	result := make([]OperationRow, 0, len(createdIDs))
	for _, row := range rows {
		if _, ok := createdSet[row.ID]; ok {
			result = append(result, row)
		}
	}
	return result, nil
}

func (s *service) ListOperations(userID uuid.UUID) ([]OperationRow, error) {
	rows, err := s.repo.ListOperations(userID)
	if err != nil {
		return nil, err
	}
	s.refreshMissingAssetMetadataForUser(userID)
	return rows, nil
}

func (s *service) UpdateOperation(userID uuid.UUID, operationID uuid.UUID, req UpdateOperationRequest) (*OperationRow, error) {
	current, err := s.repo.GetOperationByID(userID, operationID)
	if err != nil {
		return nil, err
	}

	finalAssetID := current.AssetID
	finalOperationType := current.OperationType
	finalDate := current.Date
	finalQuantity := current.Quantity
	finalUnitPrice := current.UnitPrice
	finalFeeAmount := current.FeeAmount
	finalNotes := current.Notes

	update := &UpdateOperationModel{}
	if req.AssetCode != nil {
		normalized := normalizeAssetCode(*req.AssetCode)
		if err := CheckAssetCode(normalized); err != nil {
			return nil, err
		}
		asset, err := s.repo.GetAssetByCode(userID, normalized)
		if err != nil {
			return nil, err
		}
		finalAssetID = asset.ID
		update.AssetID = &asset.ID
	}
	if req.OperationType != nil {
		if err := CheckOperationType(*req.OperationType); err != nil {
			return nil, err
		}
		finalOperationType = *req.OperationType
		update.OperationType = req.OperationType
	}
	if req.Date != nil {
		finalDate = *req.Date
		update.Date = req.Date
	}
	if req.Quantity != nil {
		if err := CheckQuantity(*req.Quantity); err != nil {
			return nil, err
		}
		finalQuantity = *req.Quantity
		update.Quantity = req.Quantity
	}
	if req.UnitPrice != nil {
		if err := CheckMoney("unit price", *req.UnitPrice, true); err != nil {
			return nil, err
		}
		finalUnitPrice = *req.UnitPrice
		update.UnitPrice = req.UnitPrice
	}
	if req.FeeAmount != nil {
		if err := CheckMoney("fee amount", *req.FeeAmount, true); err != nil {
			return nil, err
		}
		finalFeeAmount = *req.FeeAmount
		update.FeeAmount = req.FeeAmount
	}
	if req.Notes != nil {
		trimmed := strings.TrimSpace(*req.Notes)
		finalNotes = trimmed
		update.Notes = &trimmed
	}
	if update.AssetID == nil && update.OperationType == nil && update.Date == nil && update.Quantity == nil &&
		update.UnitPrice == nil && update.FeeAmount == nil && update.Notes == nil {
		return nil, appErr.ErrInvalidInputWithMessage("at least one operation field must be provided", nil)
	}

	grossAmount := finalQuantity * finalUnitPrice
	netAmount := computeNetAmount(finalOperationType, grossAmount, finalFeeAmount)
	update.GrossAmount = &grossAmount
	update.NetAmount = &netAmount

	affectedAssets := []uuid.UUID{current.AssetID}
	if finalAssetID != current.AssetID {
		affectedAssets = append(affectedAssets, finalAssetID)
	}

	var updated *Operation
	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		updated, err = s.repo.UpdateOperation(tx, userID, operationID, update)
		if err != nil {
			return err
		}
		for _, assetID := range uniqueUUIDs(affectedAssets) {
			if err := s.rebuildPosition(tx, userID, assetID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	_ = finalDate
	_ = finalNotes
	return s.operationToRow(userID, updated)
}

func (s *service) DeleteOperation(userID uuid.UUID, operationID uuid.UUID) error {
	current, err := s.repo.GetOperationByID(userID, operationID)
	if err != nil {
		return err
	}
	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := s.repo.DeleteOperation(tx, userID, operationID); err != nil {
			return err
		}
		return s.rebuildPosition(tx, userID, current.AssetID)
	})
}

func (s *service) ListPositions(userID uuid.UUID) ([]PositionRow, error) {
	rows, err := s.repo.ListPositions(userID)
	if err != nil {
		return nil, err
	}

	portfolioAssets, err := s.repo.ListPortfolioAssets(userID)
	if err != nil {
		return nil, err
	}
	portfolioNamesByAssetCode := make(map[string][]string)
	seenPortfolioNamesByAssetCode := make(map[string]map[string]struct{})
	for _, row := range portfolioAssets {
		if _, ok := seenPortfolioNamesByAssetCode[row.AssetCode]; !ok {
			seenPortfolioNamesByAssetCode[row.AssetCode] = make(map[string]struct{})
		}
		if _, ok := seenPortfolioNamesByAssetCode[row.AssetCode][row.PortfolioName]; ok {
			continue
		}
		seenPortfolioNamesByAssetCode[row.AssetCode][row.PortfolioName] = struct{}{}
		portfolioNamesByAssetCode[row.AssetCode] = append(portfolioNamesByAssetCode[row.AssetCode], row.PortfolioName)
	}

	watchedCategoryIDs := s.watchedInvestmentCategoryIDs(userID)
	if len(rows) > 0 && len(watchedCategoryIDs) > 0 {
		assets := make([]investmentIncomeAsset, 0, len(rows))
		for _, row := range rows {
			assets = append(assets, investmentIncomeAsset{
				Code: row.AssetCode,
				Name: row.AssetName,
				Type: row.AssetType,
			})
		}

		incomeSummary := s.buildIncomeSummary(userID, assets, watchedCategoryIDs)
		incomeByCode := make(map[string]PortfolioIncomeAssetRow, len(incomeSummary.Rows))
		for _, incomeRow := range incomeSummary.Rows {
			incomeByCode[incomeRow.AssetCode] = incomeRow
		}

		for index := range rows {
			rows[index].PortfolioNames = append([]string{}, portfolioNamesByAssetCode[rows[index].AssetCode]...)
			rows[index].MatchedDividends = incomeByCode[rows[index].AssetCode].Amount
		}
	} else {
		for index := range rows {
			rows[index].PortfolioNames = append([]string{}, portfolioNamesByAssetCode[rows[index].AssetCode]...)
		}
	}

	s.refreshMissingAssetMetadataForUser(userID)
	return rows, nil
}

func (s *service) ListPositionQuotes(userID uuid.UUID) ([]PositionQuoteRow, error) {
	rows, err := s.repo.ListPositions(userID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || s.quoteProvider == nil {
		return []PositionQuoteRow{}, nil
	}

	tickers := make([]string, 0, len(rows))
	for _, row := range rows {
		tickers = append(tickers, row.AssetCode)
	}
	quotesByCode, err := s.fetchQuotesByCode(tickers, false)
	if err != nil {
		return nil, err
	}
	return orderedPositionQuotes(rows, quotesByCode), nil
}

func orderedPositionQuotes(positionRows []PositionRow, quotesByCode map[string]PositionQuoteRow) []PositionQuoteRow {
	out := make([]PositionQuoteRow, 0, len(quotesByCode))
	for _, row := range positionRows {
		quote, ok := quotesByCode[row.AssetCode]
		if !ok {
			continue
		}
		out = append(out, quote)
	}
	return out
}

func (s *service) AnalyzePortfolio(userID uuid.UUID, portfolioCode string) (*PortfolioAnalysisResponse, error) {
	if err := CheckPortfolioCode(portfolioCode); err != nil {
		return nil, err
	}
	portfolio, selectedAssets, normalizedTargets, positionsByCode, quotesByCode, totalCurrentValue, totalCostBasis, err := s.preparePortfolioSnapshot(userID, portfolioCode)
	if err != nil {
		return nil, err
	}
	toleranceBPS := userconfig.DefaultInvestmentRebalanceToleranceBPS
	suggestionStrategy := userconfig.DefaultInvestmentSuggestionStrategy
	watchedCategoryIDs := []uuid.UUID{}
	if s.userConfigService != nil {
		if config, configErr := s.userConfigService.GetConfig(userID); configErr == nil && config != nil {
			toleranceBPS = config.Settings.Investments.Portfolios.RebalanceToleranceBPS
			suggestionStrategy = config.Settings.Investments.Portfolios.SuggestionStrategy
			watchedCategoryIDs = config.Settings.Investments.Integration.WatchedCategoryIDs
		}
	}

	response := &PortfolioAnalysisResponse{
		PortfolioCode:                   portfolio.Code,
		PortfolioName:                   portfolio.Name,
		PortfolioDescription:            portfolio.Description,
		RebalanceToleranceBasisPoint:    toleranceBPS,
		Rows:                            make([]PortfolioAnalysisRow, 0, len(selectedAssets)),
		IncomeSummary:                   PortfolioIncomeSummary{Rows: []PortfolioIncomeAssetRow{}},
		TargetAllocationBasisPointTotal: 0,
	}
	for _, asset := range selectedAssets {
		response.TargetAllocationBasisPointTotal += normalizedTargets[asset.AssetCode]
	}

	totalUnrealized := totalCurrentValue - totalCostBasis
	response.TotalCurrentValue = totalCurrentValue
	response.TotalCostBasis = totalCostBasis
	response.TotalUnrealizedPNLAmount = totalUnrealized
	if totalCostBasis > 0 {
		pnlBPS := ratioToBasisPoints(totalUnrealized, totalCostBasis)
		response.TotalUnrealizedPNLBasisPoint = &pnlBPS
	}

	for _, asset := range selectedAssets {
		position := positionsByCode[asset.AssetCode]
		quote := quotesByCode[asset.AssetCode]
		currentValue := position.CurrentQuantity * quote.CurrentPrice
		currentAllocation := 0
		if totalCurrentValue > 0 {
			currentAllocation = ratioToBasisPoints(currentValue, totalCurrentValue)
		}
		row := PortfolioAnalysisRow{
			AssetCode:                   asset.AssetCode,
			AssetName:                   asset.AssetName,
			AssetType:                   asset.AssetType,
			CurrentQuantity:             position.CurrentQuantity,
			AveragePrice:                position.AveragePrice,
			TotalCostBasis:              position.TotalCostBasis,
			CurrentPrice:                quote.CurrentPrice,
			QuoteUpdatedAt:              quote.QuoteUpdatedAt,
			CurrentValue:                currentValue,
			CurrentAllocationBasisPoint: currentAllocation,
			TargetAllocationBasisPoint:  normalizedTargets[asset.AssetCode],
			AllocationDriftBasisPoint:   currentAllocation - normalizedTargets[asset.AssetCode],
			MaxBuyPrice:                 asset.MaxBuyPrice,
			BlockedByMaxBuyPrice:        asset.MaxBuyPrice != nil && quote.CurrentPrice > *asset.MaxBuyPrice,
			UnrealizedPNLAmount:         currentValue - position.TotalCostBasis,
		}
		if totalCurrentValue > 0 && currentAllocation < normalizedTargets[asset.AssetCode] {
			targetValue := basisPointsOfAmount(totalCurrentValue, normalizedTargets[asset.AssetCode])
			if targetValue > currentValue {
				row.BuyOnlyGapAmount = targetValue - currentValue
			}
		}
		if position.TotalCostBasis > 0 {
			pnlBPS := ratioToBasisPoints(row.UnrealizedPNLAmount, position.TotalCostBasis)
			row.UnrealizedPNLBasisPoint = &pnlBPS
		}
		response.Rows = append(response.Rows, row)
	}
	response.MinimumSuggestedInvestment = minimumSuggestedInvestment(
		selectedAssets,
		normalizedTargets,
		positionsByCode,
		quotesByCode,
		totalCurrentValue,
		toleranceBPS,
		suggestionStrategy,
	)
	response.IncomeSummary = s.buildPortfolioIncomeSummary(userID, selectedAssets, watchedCategoryIDs)

	return response, nil
}

func (s *service) SuggestPortfolioInvestment(userID uuid.UUID, portfolioCode string, investmentAmount int64) (*PortfolioSuggestionResponse, error) {
	if err := CheckPortfolioCode(portfolioCode); err != nil {
		return nil, err
	}
	if err := CheckMoney("investment amount", investmentAmount, false); err != nil {
		return nil, err
	}

	portfolio, selectedAssets, normalizedTargets, positionsByCode, quotesByCode, totalCurrentValue, _, err := s.preparePortfolioSnapshot(userID, portfolioCode)
	if err != nil {
		return nil, err
	}

	suggestionStrategy := userconfig.DefaultInvestmentSuggestionStrategy
	if s.userConfigService != nil {
		if config, configErr := s.userConfigService.GetConfig(userID); configErr == nil && config != nil {
			suggestionStrategy = config.Settings.Investments.Portfolios.SuggestionStrategy
		}
	}

	plan := buildPortfolioSuggestion(selectedAssets, normalizedTargets, positionsByCode, quotesByCode, totalCurrentValue, investmentAmount, suggestionStrategy)
	return &PortfolioSuggestionResponse{
		PortfolioCode:                   portfolio.Code,
		PortfolioName:                   portfolio.Name,
		PortfolioDescription:            portfolio.Description,
		InvestmentAmount:                investmentAmount,
		PlannedSpend:                    plan.plannedSpend,
		CashRemainder:                   plan.cashRemainder,
		TargetAllocationBasisPointTotal: plan.targetTotal,
		Rows:                            plan.rows,
	}, nil
}

type portfolioSuggestionPlan struct {
	rows          []PortfolioSuggestionRow
	plannedSpend  int64
	cashRemainder int64
	targetTotal   int
}

func (s *service) preparePortfolioSnapshot(
	userID uuid.UUID,
	portfolioCode string,
) (*Portfolio, []PortfolioAssetResponse, map[string]int, map[string]PositionRow, map[string]PositionQuoteRow, int64, int64, error) {
	portfolio, err := s.repo.GetPortfolioByCode(userID, strings.ToLower(portfolioCode))
	if err != nil {
		return nil, nil, nil, nil, nil, 0, 0, err
	}
	portfolioRows, err := s.repo.ListPortfolioAssets(userID)
	if err != nil {
		return nil, nil, nil, nil, nil, 0, 0, err
	}
	selectedAssets := make([]PortfolioAssetResponse, 0)
	for _, row := range portfolioRows {
		if row.PortfolioCode != portfolio.Code {
			continue
		}
		selectedAssets = append(selectedAssets, PortfolioAssetResponse{
			AssetCode:                  row.AssetCode,
			AssetName:                  row.AssetName,
			AssetType:                  row.AssetType,
			TargetAllocationBasisPoint: row.TargetAllocationBPS,
			MaxBuyPrice:                row.MaxBuyPrice,
			SortOrder:                  row.SortOrder,
		})
	}
	normalizedTargets := normalizeTargetAllocations(selectedAssets)
	if len(selectedAssets) == 0 {
		return portfolio, selectedAssets, normalizedTargets, map[string]PositionRow{}, map[string]PositionQuoteRow{}, 0, 0, nil
	}

	positions, err := s.repo.ListPositions(userID)
	if err != nil {
		return nil, nil, nil, nil, nil, 0, 0, err
	}
	positionsByCode := make(map[string]PositionRow, len(positions))
	for _, row := range positions {
		positionsByCode[row.AssetCode] = row
	}

	tickers := make([]string, 0, len(selectedAssets))
	for _, asset := range selectedAssets {
		tickers = append(tickers, asset.AssetCode)
	}
	quotesByCode, err := s.fetchQuotesByCode(tickers, true)
	if err != nil {
		return nil, nil, nil, nil, nil, 0, 0, err
	}

	totalCurrentValue := int64(0)
	totalCostBasis := int64(0)
	for _, asset := range selectedAssets {
		quote, ok := quotesByCode[asset.AssetCode]
		if !ok {
			return nil, nil, nil, nil, nil, 0, 0, appErr.ErrInvalidInputWithCode(
				"investment.portfolio.analysis.quote_unavailable",
				"investment portfolio analysis requires quotes for all portfolio assets",
				nil,
			)
		}
		position := positionsByCode[asset.AssetCode]
		totalCurrentValue += position.CurrentQuantity * quote.CurrentPrice
		totalCostBasis += position.TotalCostBasis
	}

	return portfolio, selectedAssets, normalizedTargets, positionsByCode, quotesByCode, totalCurrentValue, totalCostBasis, nil
}

func (s *service) fetchQuotesByCode(tickers []string, allowStaleCache bool) (map[string]PositionQuoteRow, error) {
	if len(tickers) == 0 {
		return map[string]PositionQuoteRow{}, nil
	}

	now := time.Now().UTC()
	cachedRows, err := s.repo.ListAssetQuoteCaches(tickers)
	if err != nil {
		return nil, err
	}

	cachedByCode := make(map[string]AssetQuoteCache, len(cachedRows))
	outByCode := make(map[string]PositionQuoteRow, len(tickers))
	missingTickers := make([]string, 0, len(tickers))
	for _, row := range cachedRows {
		cachedByCode[row.AssetCode] = row
		if now.Sub(row.FetchedAt) < investmentQuoteCacheTTL {
			outByCode[row.AssetCode] = PositionQuoteRow{
				AssetCode:      row.AssetCode,
				CurrentPrice:   row.CurrentPrice,
				QuoteUpdatedAt: row.QuoteUpdatedAt,
			}
		}
	}

	for _, ticker := range tickers {
		if _, ok := outByCode[ticker]; ok {
			continue
		}
		missingTickers = append(missingTickers, ticker)
	}

	if len(missingTickers) > 0 && s.quoteProvider != nil {
		quotesCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		quotes, fetchErr := s.quoteProvider.FetchQuotes(quotesCtx, missingTickers)
		if fetchErr != nil {
			log.Printf("investment quotes unavailable: %v", fetchErr)
		}
		for ticker, quote := range quotes {
			outByCode[ticker] = PositionQuoteRow{
				AssetCode:      ticker,
				CurrentPrice:   quote.CurrentPrice,
				QuoteUpdatedAt: quote.Timestamp,
			}
			cache := &AssetQuoteCache{
				ID:             uuid.New(),
				AssetCode:      ticker,
				CurrentPrice:   quote.CurrentPrice,
				QuoteUpdatedAt: quote.Timestamp,
				Source:         "quote-provider",
				FetchedAt:      now,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if existing, ok := cachedByCode[ticker]; ok {
				cache.ID = existing.ID
				cache.CreatedAt = existing.CreatedAt
			}
			if err := s.repo.UpsertAssetQuoteCache(cache); err != nil {
				log.Printf("investment quote cache update failed for %s: %v", ticker, err)
			}
		}
	}

	if allowStaleCache {
		for _, ticker := range tickers {
			if _, ok := outByCode[ticker]; ok {
				continue
			}
			if cached, ok := cachedByCode[ticker]; ok {
				outByCode[ticker] = PositionQuoteRow{
					AssetCode:      cached.AssetCode,
					CurrentPrice:   cached.CurrentPrice,
					QuoteUpdatedAt: cached.QuoteUpdatedAt,
				}
			}
		}
	}

	return outByCode, nil
}

func ratioToBasisPoints(numerator, denominator int64) int {
	if denominator == 0 {
		return 0
	}
	return int(math.Round(float64(numerator) * 10000 / float64(denominator)))
}

func basisPointsOfAmount(amount int64, bps int) int64 {
	return int64(math.Round(float64(amount) * float64(bps) / 10000))
}

func buildPortfolioSuggestion(
	assets []PortfolioAssetResponse,
	normalizedTargets map[string]int,
	positionsByCode map[string]PositionRow,
	quotesByCode map[string]PositionQuoteRow,
	totalCurrentValue int64,
	investmentAmount int64,
	strategy userconfig.InvestmentSuggestionStrategy,
) portfolioSuggestionPlan {
	switch strategy {
	case userconfig.InvestmentSuggestionStrategyProportionalGap:
		return buildPortfolioSuggestionProportionalGap(assets, normalizedTargets, positionsByCode, quotesByCode, totalCurrentValue, investmentAmount)
	default:
		return buildPortfolioSuggestionBestNextShare(assets, normalizedTargets, positionsByCode, quotesByCode, totalCurrentValue, investmentAmount)
	}
}

func buildPortfolioSuggestionProportionalGap(
	assets []PortfolioAssetResponse,
	normalizedTargets map[string]int,
	positionsByCode map[string]PositionRow,
	quotesByCode map[string]PositionQuoteRow,
	totalCurrentValue int64,
	investmentAmount int64,
) portfolioSuggestionPlan {
	pendingShares := make(map[string]int64, len(assets))
	currentValues := make(map[string]int64, len(assets))
	blockedByMaxPrice := make(map[string]bool, len(assets))
	gapValues := make(map[string]int64, len(assets))
	for _, asset := range assets {
		quote := quotesByCode[asset.AssetCode]
		position := positionsByCode[asset.AssetCode]
		currentValues[asset.AssetCode] = position.CurrentQuantity * quote.CurrentPrice
		blockedByMaxPrice[asset.AssetCode] = asset.MaxBuyPrice != nil && quote.CurrentPrice > *asset.MaxBuyPrice
	}

	finalTotal := totalCurrentValue + investmentAmount
	totalPositiveGap := int64(0)
	for _, asset := range assets {
		targetValue := basisPointsOfAmount(finalTotal, normalizedTargets[asset.AssetCode])
		gapValue := targetValue - currentValues[asset.AssetCode]
		if gapValue < 0 || blockedByMaxPrice[asset.AssetCode] {
			gapValue = 0
		}
		gapValues[asset.AssetCode] = gapValue
		totalPositiveGap += gapValue
	}

	remaining := investmentAmount
	if totalPositiveGap > 0 {
		for _, asset := range assets {
			quote := quotesByCode[asset.AssetCode]
			gapValue := gapValues[asset.AssetCode]
			if gapValue <= 0 || quote.CurrentPrice <= 0 {
				continue
			}
			plannedSpend := int64(math.Floor(float64(investmentAmount) * float64(gapValue) / float64(totalPositiveGap)))
			if plannedSpend > gapValue {
				plannedSpend = gapValue
			}
			buyShares := plannedSpend / quote.CurrentPrice
			if buyShares <= 0 {
				continue
			}
			pendingShares[asset.AssetCode] = buyShares
			remaining -= buyShares * quote.CurrentPrice
		}
	}

	for {
		bestTicker := ""
		bestResidualGap := int64(0)
		bestPrice := int64(0)

		for _, asset := range assets {
			quote := quotesByCode[asset.AssetCode]
			if quote.CurrentPrice <= 0 || quote.CurrentPrice > remaining || blockedByMaxPrice[asset.AssetCode] {
				continue
			}
			projectedValue := currentValues[asset.AssetCode] + pendingShares[asset.AssetCode]*quote.CurrentPrice
			targetValue := basisPointsOfAmount(finalTotal, normalizedTargets[asset.AssetCode])
			residualGap := targetValue - projectedValue
			if residualGap <= 0 {
				continue
			}
			if residualGap > bestResidualGap || (residualGap == bestResidualGap && (bestTicker == "" || quote.CurrentPrice < bestPrice)) {
				bestTicker = asset.AssetCode
				bestResidualGap = residualGap
				bestPrice = quote.CurrentPrice
			}
		}

		if bestTicker == "" {
			break
		}

		pendingShares[bestTicker]++
		remaining -= quotesByCode[bestTicker].CurrentPrice
	}

	return finalizePortfolioSuggestionPlan(assets, normalizedTargets, quotesByCode, totalCurrentValue, investmentAmount, currentValues, blockedByMaxPrice, pendingShares)
}

func buildPortfolioSuggestionBestNextShare(
	assets []PortfolioAssetResponse,
	normalizedTargets map[string]int,
	positionsByCode map[string]PositionRow,
	quotesByCode map[string]PositionQuoteRow,
	totalCurrentValue int64,
	investmentAmount int64,
) portfolioSuggestionPlan {
	pendingShares := make(map[string]int64, len(assets))
	currentValues := make(map[string]int64, len(assets))
	blockedByMaxPrice := make(map[string]bool, len(assets))
	finalTotal := totalCurrentValue + investmentAmount
	for _, asset := range assets {
		quote := quotesByCode[asset.AssetCode]
		position := positionsByCode[asset.AssetCode]
		currentValues[asset.AssetCode] = position.CurrentQuantity * quote.CurrentPrice
		blockedByMaxPrice[asset.AssetCode] = asset.MaxBuyPrice != nil && quote.CurrentPrice > *asset.MaxBuyPrice
	}

	remaining := investmentAmount
	for {
		currentScore := portfolioDeviationScore(assets, normalizedTargets, currentValues, quotesByCode, finalTotal, pendingShares)
		bestTicker := ""
		bestScore := currentScore
		bestImprovement := int64(0)
		bestPrice := int64(0)

		for _, asset := range assets {
			quote := quotesByCode[asset.AssetCode]
			if quote.CurrentPrice <= 0 || quote.CurrentPrice > remaining || blockedByMaxPrice[asset.AssetCode] {
				continue
			}

			candidateShares := cloneSharePlan(pendingShares)
			candidateShares[asset.AssetCode]++
			score := portfolioDeviationScore(assets, normalizedTargets, currentValues, quotesByCode, finalTotal, candidateShares)
			improvement := currentScore - score
			if improvement < 0 {
				continue
			}

			if bestTicker == "" ||
				score < bestScore ||
				(score == bestScore && improvement > bestImprovement) ||
				(score == bestScore && improvement == bestImprovement && quote.CurrentPrice < bestPrice) ||
				(score == bestScore && improvement == bestImprovement && quote.CurrentPrice == bestPrice && asset.AssetCode < bestTicker) {
				bestTicker = asset.AssetCode
				bestScore = score
				bestImprovement = improvement
				bestPrice = quote.CurrentPrice
			}
		}

		if bestTicker == "" {
			break
		}

		pendingShares[bestTicker]++
		remaining -= quotesByCode[bestTicker].CurrentPrice
	}

	return finalizePortfolioSuggestionPlan(assets, normalizedTargets, quotesByCode, totalCurrentValue, investmentAmount, currentValues, blockedByMaxPrice, pendingShares)
}

func finalizePortfolioSuggestionPlan(
	assets []PortfolioAssetResponse,
	normalizedTargets map[string]int,
	quotesByCode map[string]PositionQuoteRow,
	totalCurrentValue int64,
	investmentAmount int64,
	currentValues map[string]int64,
	blockedByMaxPrice map[string]bool,
	pendingShares map[string]int64,
) portfolioSuggestionPlan {
	rows := make([]PortfolioSuggestionRow, 0, len(assets))
	plannedSpend := int64(0)
	targetTotal := 0
	finalTotal := totalCurrentValue + investmentAmount
	for _, asset := range assets {
		quote := quotesByCode[asset.AssetCode]
		currentValue := currentValues[asset.AssetCode]
		buyShares := pendingShares[asset.AssetCode]
		spend := buyShares * quote.CurrentPrice
		plannedSpend += spend
		currentAllocation := 0
		if totalCurrentValue > 0 {
			currentAllocation = ratioToBasisPoints(currentValue, totalCurrentValue)
		}
		projectedAllocation := 0
		if finalTotal > 0 {
			projectedAllocation = ratioToBasisPoints(currentValue+spend, finalTotal)
		}

		rows = append(rows, PortfolioSuggestionRow{
			AssetCode:                     asset.AssetCode,
			AssetName:                     asset.AssetName,
			AssetType:                     asset.AssetType,
			CurrentPrice:                  quote.CurrentPrice,
			CurrentAllocationBasisPoint:   currentAllocation,
			TargetAllocationBasisPoint:    normalizedTargets[asset.AssetCode],
			ProjectedAllocationBasisPoint: projectedAllocation,
			MaxBuyPrice:                   asset.MaxBuyPrice,
			BlockedByMaxBuyPrice:          blockedByMaxPrice[asset.AssetCode],
			BuyShares:                     buyShares,
			PlannedSpend:                  spend,
		})
		targetTotal += normalizedTargets[asset.AssetCode]
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].PlannedSpend == rows[j].PlannedSpend {
			return rows[i].AssetCode < rows[j].AssetCode
		}
		return rows[i].PlannedSpend > rows[j].PlannedSpend
	})

	return portfolioSuggestionPlan{
		rows:          rows,
		plannedSpend:  plannedSpend,
		cashRemainder: investmentAmount - plannedSpend,
		targetTotal:   targetTotal,
	}
}

func portfolioDeviationScore(
	assets []PortfolioAssetResponse,
	normalizedTargets map[string]int,
	currentValues map[string]int64,
	quotesByCode map[string]PositionQuoteRow,
	finalTotal int64,
	pendingShares map[string]int64,
) int64 {
	score := int64(0)
	for _, asset := range assets {
		targetValue := basisPointsOfAmount(finalTotal, normalizedTargets[asset.AssetCode])
		projectedValue := currentValues[asset.AssetCode] + pendingShares[asset.AssetCode]*quotesByCode[asset.AssetCode].CurrentPrice
		score += absInt64(targetValue - projectedValue)
	}
	return score
}

func cloneSharePlan(source map[string]int64) map[string]int64 {
	cloned := make(map[string]int64, len(source)+1)
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func minimumSuggestedInvestment(
	assets []PortfolioAssetResponse,
	normalizedTargets map[string]int,
	positionsByCode map[string]PositionRow,
	quotesByCode map[string]PositionQuoteRow,
	totalCurrentValue int64,
	toleranceBPS int,
	strategy userconfig.InvestmentSuggestionStrategy,
) *int64 {
	if len(assets) == 0 {
		return nil
	}

	needsRebalance := false
	low := int64(0)
	high := int64(0)
	for _, asset := range assets {
		quote := quotesByCode[asset.AssetCode]
		currentValue := positionsByCode[asset.AssetCode].CurrentQuantity * quote.CurrentPrice
		currentAllocation := 0
		if totalCurrentValue > 0 {
			currentAllocation = ratioToBasisPoints(currentValue, totalCurrentValue)
		}
		isBlocked := asset.MaxBuyPrice != nil && quote.CurrentPrice > *asset.MaxBuyPrice
		if currentAllocation+toleranceBPS < normalizedTargets[asset.AssetCode] && !isBlocked {
			needsRebalance = true
			if quote.CurrentPrice > low {
				low = quote.CurrentPrice
			}
			high += quote.CurrentPrice
		}
	}
	if !needsRebalance {
		zero := int64(0)
		return &zero
	}
	if high <= 0 {
		return nil
	}

	for satisfiesRebalanceTolerance(assets, normalizedTargets, positionsByCode, quotesByCode, totalCurrentValue, high, toleranceBPS, strategy) == false {
		if high > maxSuggestedInvestmentSearchStep {
			return nil
		}
		high *= 2
		if high <= 0 {
			return nil
		}
	}

	for low < high {
		mid := low + (high-low)/2
		if satisfiesRebalanceTolerance(assets, normalizedTargets, positionsByCode, quotesByCode, totalCurrentValue, mid, toleranceBPS, strategy) {
			high = mid
		} else {
			low = mid + 1
		}
	}

	result := low
	return &result
}

func satisfiesRebalanceTolerance(
	assets []PortfolioAssetResponse,
	normalizedTargets map[string]int,
	positionsByCode map[string]PositionRow,
	quotesByCode map[string]PositionQuoteRow,
	totalCurrentValue int64,
	investmentAmount int64,
	toleranceBPS int,
	strategy userconfig.InvestmentSuggestionStrategy,
) bool {
	plan := buildPortfolioSuggestion(assets, normalizedTargets, positionsByCode, quotesByCode, totalCurrentValue, investmentAmount, strategy)
	rowsByCode := make(map[string]PortfolioSuggestionRow, len(plan.rows))
	for _, row := range plan.rows {
		rowsByCode[row.AssetCode] = row
	}
	for _, asset := range assets {
		row := rowsByCode[asset.AssetCode]
		if row.BlockedByMaxBuyPrice {
			continue
		}
		if row.ProjectedAllocationBasisPoint+toleranceBPS < row.TargetAllocationBasisPoint {
			return false
		}
	}
	return true
}

func normalizeTargetAllocations(assets []PortfolioAssetResponse) map[string]int {
	type remainderRow struct {
		assetCode string
		remainder float64
		sortOrder int
	}

	out := make(map[string]int, len(assets))
	totalRaw := 0
	for _, asset := range assets {
		totalRaw += asset.TargetAllocationBasisPoint
	}
	if totalRaw <= 0 {
		for _, asset := range assets {
			out[asset.AssetCode] = 0
		}
		return out
	}

	remainders := make([]remainderRow, 0, len(assets))
	assigned := 0
	for _, asset := range assets {
		normalized := float64(asset.TargetAllocationBasisPoint) * 10000 / float64(totalRaw)
		base := int(math.Floor(normalized))
		out[asset.AssetCode] = base
		assigned += base
		remainders = append(remainders, remainderRow{
			assetCode: asset.AssetCode,
			remainder: normalized - float64(base),
			sortOrder: asset.SortOrder,
		})
	}

	sort.Slice(remainders, func(i, j int) bool {
		if remainders[i].remainder == remainders[j].remainder {
			if remainders[i].sortOrder == remainders[j].sortOrder {
				return remainders[i].assetCode < remainders[j].assetCode
			}
			return remainders[i].sortOrder < remainders[j].sortOrder
		}
		return remainders[i].remainder > remainders[j].remainder
	})

	for i := 0; i < 10000-assigned && i < len(remainders); i++ {
		out[remainders[i].assetCode]++
	}

	return out
}

func (s *service) buildPortfolioIncomeSummary(
	userID uuid.UUID,
	assets []PortfolioAssetResponse,
	watchedCategoryIDs []uuid.UUID,
) PortfolioIncomeSummary {
	selectedIncomeAssets := make([]investmentIncomeAsset, 0, len(assets))
	selectedAssetCodes := make(map[string]struct{}, len(assets))
	for _, asset := range assets {
		selectedIncomeAssets = append(selectedIncomeAssets, investmentIncomeAsset{
			Code: asset.AssetCode,
			Name: asset.AssetName,
			Type: asset.AssetType,
		})
		selectedAssetCodes[asset.AssetCode] = struct{}{}
	}

	allAssets, err := s.repo.ListAssets(userID)
	if err != nil {
		log.Printf("investment assets unavailable for income summary: %v", err)
		return s.buildIncomeSummary(userID, selectedIncomeAssets, watchedCategoryIDs)
	}

	allIncomeAssets := make([]investmentIncomeAsset, 0, len(allAssets))
	for _, asset := range allAssets {
		allIncomeAssets = append(allIncomeAssets, investmentIncomeAsset{
			Code: asset.Code,
			Name: asset.Name,
			Type: asset.AssetType,
		})
	}

	globalSummary := s.buildIncomeSummary(userID, allIncomeAssets, watchedCategoryIDs)
	filteredRows := make([]PortfolioIncomeAssetRow, 0, len(globalSummary.Rows))
	matchedDividendsTotal := int64(0)
	matchedTransactionsCount := 0
	for _, row := range globalSummary.Rows {
		if _, ok := selectedAssetCodes[row.AssetCode]; !ok {
			continue
		}
		filteredRows = append(filteredRows, row)
		matchedDividendsTotal += row.Amount
		matchedTransactionsCount += row.TransactionCount
	}

	globalSummary.Rows = filteredRows
	globalSummary.MatchedDividendsTotal = matchedDividendsTotal
	globalSummary.MatchedTransactionsCount = matchedTransactionsCount
	return globalSummary
}

func (s *service) buildIncomeSummary(
	userID uuid.UUID,
	assets []investmentIncomeAsset,
	watchedCategoryIDs []uuid.UUID,
) PortfolioIncomeSummary {
	summary := PortfolioIncomeSummary{
		Rows: []PortfolioIncomeAssetRow{},
	}
	if s.transactionReader == nil || len(assets) == 0 || len(watchedCategoryIDs) == 0 {
		return summary
	}

	transactions, err := s.transactionReader.ListVisibleByCategoryIDs(userID, watchedCategoryIDs)
	if err != nil {
		log.Printf("investment income summary unavailable: %v", err)
		return summary
	}

	assetsByCode := make(map[string]investmentIncomeAsset, len(assets))
	for _, asset := range assets {
		assetsByCode[asset.Code] = asset
	}

	type aggregate struct {
		asset            investmentIncomeAsset
		amount           int64
		transactionCount int
	}
	aggregates := make(map[string]*aggregate, len(assets))
	for _, tx := range transactions {
		matches := matchedAssetCodesFromDescription(tx.Description, assetsByCode)
		switch len(matches) {
		case 0:
			summary.UnmatchedTransactionsCount++
			continue
		case 1:
			assetCode := matches[0]
			item := aggregates[assetCode]
			if item == nil {
				asset := assetsByCode[assetCode]
				item = &aggregate{asset: asset}
				aggregates[assetCode] = item
			}
			item.amount += tx.Amount
			item.transactionCount++
			summary.MatchedDividendsTotal += tx.Amount
			summary.MatchedTransactionsCount++
		default:
			for _, assetCode := range matches {
				item := aggregates[assetCode]
				if item == nil {
					asset := assetsByCode[assetCode]
					item = &aggregate{asset: asset}
					aggregates[assetCode] = item
				}
				item.amount += tx.Amount
				item.transactionCount++
				summary.MatchedDividendsTotal += tx.Amount
				summary.MatchedTransactionsCount++
			}
			summary.AmbiguousTransactionsCount++
		}
	}

	for _, asset := range assets {
		item := aggregates[asset.Code]
		if item == nil {
			continue
		}
		summary.Rows = append(summary.Rows, PortfolioIncomeAssetRow{
			AssetCode:        item.asset.Code,
			AssetName:        item.asset.Name,
			AssetType:        item.asset.Type,
			Amount:           item.amount,
			TransactionCount: item.transactionCount,
		})
	}

	sort.Slice(summary.Rows, func(i, j int) bool {
		if summary.Rows[i].Amount == summary.Rows[j].Amount {
			return summary.Rows[i].AssetCode < summary.Rows[j].AssetCode
		}
		return summary.Rows[i].Amount > summary.Rows[j].Amount
	})

	return summary
}

func (s *service) watchedInvestmentCategoryIDs(userID uuid.UUID) []uuid.UUID {
	if s.userConfigService == nil {
		return nil
	}

	config, err := s.userConfigService.GetConfig(userID)
	if err != nil {
		log.Printf("investment user config unavailable: %v", err)
		return nil
	}
	if config == nil {
		return nil
	}

	return config.Settings.Investments.Integration.WatchedCategoryIDs
}

func matchedAssetCodesFromDescription(description string, assetsByCode map[string]investmentIncomeAsset) []string {
	if len(assetsByCode) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	matches := make([]string, 0, 1)
	for _, token := range tokenizeAssetDescription(description) {
		if _, ok := assetsByCode[token]; !ok {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		matches = append(matches, token)
	}
	sort.Strings(matches)
	return matches
}

func tokenizeAssetDescription(description string) []string {
	upper := strings.ToUpper(strings.TrimSpace(description))
	if upper == "" {
		return nil
	}
	return strings.FieldsFunc(upper, func(r rune) bool {
		return (r < 'A' || r > 'Z') && (r < '0' || r > '9')
	})
}

func (s *service) validateCreateOperation(req CreateOperationRequest) error {
	if err := CheckAssetCode(req.AssetCode); err != nil {
		return err
	}
	if err := CheckOperationType(req.OperationType); err != nil {
		return err
	}
	if err := CheckQuantity(req.Quantity); err != nil {
		return err
	}
	if err := CheckMoney("unit price", req.UnitPrice, true); err != nil {
		return err
	}
	if err := CheckMoney("fee amount", req.FeeAmount, true); err != nil {
		return err
	}
	return nil
}

func (s *service) rebuildPosition(tx *gorm.DB, userID, assetID uuid.UUID) error {
	operations, err := s.repo.ListAssetOperations(tx, userID, assetID)
	if err != nil {
		return err
	}
	if len(operations) == 0 {
		return s.repo.DeletePosition(tx, userID, assetID)
	}

	var currentQuantity int64
	var totalCostBasis int64
	var realizedPNL int64

	for _, operation := range operations {
		switch operation.OperationType {
		case OperationTypeBuy, OperationTypeBonification:
			currentQuantity += operation.Quantity
			totalCostBasis += operation.NetAmount
		case OperationTypeSell:
			if operation.Quantity > currentQuantity {
				return appErr.ErrInvalidInputWithCode(
					"investment.operation.sell.exceeds.position",
					"sell operation exceeds available quantity for asset history",
					nil,
				)
			}
			sellCostBasis := divideRounded(totalCostBasis*operation.Quantity, currentQuantity)
			realizedPNL += operation.NetAmount - sellCostBasis
			currentQuantity -= operation.Quantity
			totalCostBasis -= sellCostBasis
			if currentQuantity == 0 {
				totalCostBasis = 0
			}
		}
	}

	averagePrice := int64(0)
	if currentQuantity > 0 {
		averagePrice = divideRounded(totalCostBasis, currentQuantity)
	}

	return s.repo.UpsertPosition(tx, &Position{
		ID:                 uuid.New(),
		UserID:             userID,
		AssetID:            assetID,
		CurrentQuantity:    currentQuantity,
		AveragePrice:       averagePrice,
		TotalCostBasis:     totalCostBasis,
		RealizedPNL:        realizedPNL,
		LastRecalculatedAt: time.Now(),
	})
}

func (s *service) operationToRow(userID uuid.UUID, operation *Operation) (*OperationRow, error) {
	rows, err := s.repo.ListOperations(userID)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.ID == operation.ID {
			result := row
			return &result, nil
		}
	}
	return nil, appErr.ErrInvalidInputWithCode("investment.operation.not_found", "investment operation not found", nil)
}

func normalizeAssetCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func computeNetAmount(operationType OperationType, grossAmount, feeAmount int64) int64 {
	if operationType == OperationTypeSell {
		return grossAmount - feeAmount
	}
	if operationType == OperationTypeBuy || operationType == OperationTypeBonification {
		return grossAmount + feeAmount
	}
	return grossAmount
}

func divideRounded(numerator, denominator int64) int64 {
	if denominator == 0 {
		return 0
	}
	return (numerator + (denominator / 2)) / denominator
}

func uniqueUUIDs(input []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(input))
	out := make([]uuid.UUID, 0, len(input))
	for _, id := range input {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

type preparedBulkOperation struct {
	asset          *Asset
	assetCode      string
	operationType  OperationType
	date           time.Time
	quantity       int64
	unitPrice      int64
	totalFeeAmount int64
	allocatedFee   int64
	notes          string
	grossAmount    int64
}

func allocateBulkFees(items []preparedBulkOperation) error {
	byGroup := make(map[string][]int)
	for index, item := range items {
		dateKey := item.date.UTC().Format("2006-01-02")
		groupKey := dateKey + ":" + strconv.FormatInt(item.totalFeeAmount, 10)
		byGroup[groupKey] = append(byGroup[groupKey], index)
	}

	for _, indexes := range byGroup {
		dayFee := int64(-1)
		totalGross := int64(0)
		for _, index := range indexes {
			totalGross += items[index].grossAmount
			if dayFee == -1 {
				dayFee = items[index].totalFeeAmount
				continue
			}
		}
		if dayFee <= 0 {
			for _, index := range indexes {
				items[index].allocatedFee = 0
			}
			continue
		}
		remaining := dayFee
		for i, index := range indexes {
			if i == len(indexes)-1 {
				items[index].allocatedFee = remaining
				break
			}
			allocated := divideRounded(dayFee*items[index].grossAmount, totalGross)
			if allocated > remaining {
				allocated = remaining
			}
			items[index].allocatedFee = allocated
			remaining -= allocated
		}
	}
	return nil
}

func (s *service) getOrCreateAssetForBulk(userID uuid.UUID, code string) (*Asset, error) {
	asset, err := s.repo.GetAssetByCode(userID, code)
	if err == nil {
		s.refreshAssetMetadataAsync(userID, asset)
		return asset, nil
	}

	var inputErr *appErr.AppError
	if !errors.As(err, &inputErr) || inputErr.Code != "investment.asset.not_found" {
		return nil, err
	}

	assetType := inferAssetTypeFromCode(code)
	created, err := s.repo.CreateAsset(&Asset{
		ID:        uuid.New(),
		UserID:    userID,
		Code:      code,
		Name:      code,
		AssetType: assetType,
		IsActive:  true,
	})
	if err != nil {
		return nil, err
	}
	s.refreshAssetMetadataAsync(userID, created)
	return created, nil
}

func (s *service) refreshAssetMetadataAsync(userID uuid.UUID, asset *Asset) {
	if s.assetMetadataProvider == nil || asset == nil {
		return
	}
	if !assetNeedsMetadataRefresh(*asset) {
		return
	}

	go func() {
		if _, err := s.refreshAssetMetadata(userID, *asset, false); err != nil {
			log.Printf("investment metadata unavailable for %s: %v", asset.Code, err)
		}
	}()
}

func (s *service) refreshMissingAssetMetadataForUser(userID uuid.UUID) {
	assets, err := s.repo.ListAssets(userID)
	if err != nil {
		log.Printf("investment metadata refresh skipped while listing assets for %s: %v", userID, err)
		return
	}
	s.refreshMissingAssetMetadata(userID, assets)
}

func (s *service) refreshMissingAssetMetadata(userID uuid.UUID, assets []Asset) {
	for index := range assets {
		asset := assets[index]
		s.refreshAssetMetadataAsync(userID, &asset)
	}
}

func assetNeedsMetadataRefresh(asset Asset) bool {
	return asset.CNPJ == nil || strings.TrimSpace(asset.Name) == "" || asset.Name == asset.Code
}

func (s *service) refreshAssetMetadata(userID uuid.UUID, asset Asset, force bool) (*Asset, error) {
	if s.assetMetadataProvider == nil {
		return &asset, nil
	}
	if !force && !assetNeedsMetadataRefresh(asset) {
		return &asset, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	metadata, err := s.assetMetadataProvider.FetchAssetMetadata(ctx, asset.Code, asset.AssetType)
	if err != nil {
		return nil, err
	}
	if metadata == nil {
		return &asset, nil
	}

	update := &UpdateAssetMetadata{
		Name:              &metadata.Name,
		MetadataSource:    &metadata.Source,
		MetadataUpdatedAt: &metadata.UpdatedAt,
	}
	if metadata.CNPJ != nil {
		update.CNPJ = metadata.CNPJ
	}
	return s.repo.UpdateAssetMetadata(userID, asset.Code, update)
}

func inferAssetTypeFromCode(code string) AssetType {
	if strings.HasSuffix(code, "11") {
		return AssetTypeFII
	}
	return AssetTypeStock
}

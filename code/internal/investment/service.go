package investment

import (
	"context"
	"errors"
	"expense-tracker/internal/account"
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
	investmentMirrorCategoryCode     = "aporteretirada"
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
	PreviewImportOperations(userID uuid.UUID, req ImportOperationsRequest) (*PreviewImportOperationsResponse, error)
	ImportOperations(userID uuid.UUID, req ImportOperationsRequest) (*ImportOperationsResponse, error)
	ListOperations(userID uuid.UUID) ([]OperationRow, error)
	CreateOperationMirror(userID, operationID uuid.UUID, req CreateOperationMirrorRequest) (*OperationRow, error)
	CreateOperationMirrorsBulk(userID uuid.UUID, req CreateOperationMirrorsBulkRequest) ([]OperationRow, error)
	UpdateOperation(userID uuid.UUID, operationID uuid.UUID, req UpdateOperationRequest) (*OperationRow, error)
	DeleteOperation(userID uuid.UUID, operationID uuid.UUID) error
	DeleteOperationsBulk(userID uuid.UUID, req DeleteOperationsBulkRequest) error
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
	AssetCode             string
	BrokerageAccountCode  string
	InvestmentAccountCode string
	OperationType         OperationType
	Date                  time.Time
	Quantity              int64
	UnitPrice             int64
	FeeAmount             int64
	Notes                 string
}

type CreateBulkOperationsRequest struct {
	Operations []CreateBulkOperationRequest
}

type ImportOperationsRequest struct {
	Operations                 []ImportOperationRequest
	CreateMirroredTransactions bool
	MirroredTransactions       []MirroredTransactionDraftRequest
}

type ImportOperationRequest struct {
	ClientRowID           string
	AssetCode             string
	BrokerageAccountCode  string
	InvestmentAccountCode string
	OperationType         OperationType
	Date                  time.Time
	Quantity              int64
	UnitPrice             int64
	TotalFeeAmount        int64
	Notes                 string
}

type MirroredTransactionDraftRequest struct {
	ClientRowID                     string
	SourceAccountCode               string
	DestinationAccountCode          string
	TransactionID                   *uuid.UUID
	RealizedPNLTransactionID        *uuid.UUID
	BonificationIncomeTransactionID *uuid.UUID
}

type ImportOperationsResponse struct {
	Operations                  []OperationRow
	MirroringEnabled            bool
	MirroredTransactionsCreated int
}

type PreviewImportOperationsResponse struct {
	PositionPreviewRows []PositionPreviewRow `json:"position_preview_rows"`
	MirrorPreviewRows   []MirrorPreviewRow   `json:"mirror_preview_rows"`
}

type MirrorPreviewExtraType string

const (
	MirrorPreviewExtraTypeNone               MirrorPreviewExtraType = "NONE"
	MirrorPreviewExtraTypeRealizedPNL        MirrorPreviewExtraType = "REALIZED_PNL"
	MirrorPreviewExtraTypeBonificationIncome MirrorPreviewExtraType = "BONIFICATION_INCOME"
)

type MirrorPreviewRow struct {
	ClientRowID            string                 `json:"client_row_id"`
	GroupKey               string                 `json:"group_key"`
	OperationType          OperationType          `json:"operation_type"`
	BrokerageAccountCode   string                 `json:"brokerage_account_code"`
	InvestmentAccountCode  string                 `json:"investment_account_code"`
	Date                   time.Time              `json:"date"`
	Description            string                 `json:"description"`
	TransferAmount         int64                  `json:"transfer_amount"`
	ExtraAmount            int64                  `json:"extra_amount"`
	ExtraType              MirrorPreviewExtraType `json:"extra_type"`
	SourceAccountCode      string                 `json:"source_account_code"`
	DestinationAccountCode string                 `json:"destination_account_code"`
}

type CreateOperationMirrorRequest struct {
	SourceAccountCode               string
	DestinationAccountCode          string
	TransactionID                   *uuid.UUID
	RealizedPNLTransactionID        *uuid.UUID
	BonificationIncomeTransactionID *uuid.UUID
}

type CreateOperationMirrorsBulkRequest struct {
	Items []CreateOperationMirrorBulkItemRequest
}

type CreateOperationMirrorBulkItemRequest struct {
	OperationID                     uuid.UUID
	SourceAccountCode               string
	DestinationAccountCode          string
	TransactionID                   *uuid.UUID
	RealizedPNLTransactionID        *uuid.UUID
	BonificationIncomeTransactionID *uuid.UUID
}

type DeleteOperationsBulkRequest struct {
	OperationIDs []uuid.UUID
}

type CreateBulkOperationRequest struct {
	AssetCode             string
	BrokerageAccountCode  string
	InvestmentAccountCode string
	OperationType         OperationType
	Date                  time.Time
	Quantity              int64
	UnitPrice             int64
	TotalFeeAmount        int64
	Notes                 string
}

type UpdateOperationRequest struct {
	AssetCode             *string
	BrokerageAccountCode  *string
	InvestmentAccountCode *string
	OperationType         *OperationType
	Date                  *time.Time
	Quantity              *int64
	UnitPrice             *int64
	FeeAmount             *int64
	Notes                 *string
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
	req.BrokerageAccountCode = normalizeAccountCode(req.BrokerageAccountCode)
	req.InvestmentAccountCode = normalizeAccountCode(req.InvestmentAccountCode)
	req.Notes = strings.TrimSpace(req.Notes)
	if err := s.validateCreateOperation(req); err != nil {
		return nil, err
	}
	asset, err := s.repo.GetAssetByCode(userID, req.AssetCode)
	if err != nil {
		return nil, err
	}
	var created *Operation
	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		brokerageAccount, err := resolveActiveBrokerageAccountByCode(tx, userID, req.BrokerageAccountCode)
		if err != nil {
			return err
		}
		investmentAccount, err := resolveActiveInvestmentAccountByCode(tx, userID, req.InvestmentAccountCode)
		if err != nil {
			return err
		}
		created, err = s.repo.CreateOperation(tx, &Operation{
			ID:                     uuid.New(),
			UserID:                 userID,
			AssetID:                asset.ID,
			BrokerageAccountID:     &brokerageAccount.ID,
			InvestmentAccountID:    &investmentAccount.ID,
			OperationType:          req.OperationType,
			Date:                   req.Date,
			Quantity:               req.Quantity,
			UnitPrice:              req.UnitPrice,
			FeeAmount:              0,
			OriginalTotalFeeAmount: req.FeeAmount,
			GrossAmount:            req.Quantity * req.UnitPrice,
			NetAmount:              computeNetAmount(req.OperationType, req.Quantity*req.UnitPrice, 0),
			Notes:                  req.Notes,
		})
		if err != nil {
			return err
		}
		if err := s.reallocateOperationFeesByDate(tx, userID, req.Date, &brokerageAccount.ID, req.FeeAmount); err != nil {
			return err
		}
		affectedAssetIDs, err := s.collectAssetIDsForFeeGroups(tx, userID, []feeGroupReference{{
			Date:               req.Date,
			BrokerageAccountID: &brokerageAccount.ID,
		}})
		if err != nil {
			return err
		}
		return s.syncMirroredTransactionsForAssets(tx, userID, affectedAssetIDs)
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
	brokerageAccountsByCode := make(map[string]*dbAccountRow, len(req.Operations))
	investmentAccountsByCode := make(map[string]*dbAccountRow, len(req.Operations))
	for _, item := range req.Operations {
		normalized := CreateOperationRequest{
			AssetCode:             normalizeAssetCode(item.AssetCode),
			BrokerageAccountCode:  normalizeAccountCode(item.BrokerageAccountCode),
			InvestmentAccountCode: normalizeAccountCode(item.InvestmentAccountCode),
			OperationType:         item.OperationType,
			Date:                  item.Date,
			Quantity:              item.Quantity,
			UnitPrice:             item.UnitPrice,
			Notes:                 strings.TrimSpace(item.Notes),
		}
		if err := s.validateCreateOperation(normalized); err != nil {
			return nil, err
		}
		if err := CheckMoney("total fee amount", item.TotalFeeAmount, true); err != nil {
			return nil, err
		}
		if err := CheckOperationFeeAmount(normalized.OperationType, item.TotalFeeAmount); err != nil {
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

		brokerageAccount, ok := brokerageAccountsByCode[normalized.BrokerageAccountCode]
		if !ok {
			brokerageAccount = &dbAccountRow{Code: normalized.BrokerageAccountCode}
			brokerageAccountsByCode[normalized.BrokerageAccountCode] = brokerageAccount
		}
		investmentAccount, ok := investmentAccountsByCode[normalized.InvestmentAccountCode]
		if !ok {
			investmentAccount = &dbAccountRow{Code: normalized.InvestmentAccountCode}
			investmentAccountsByCode[normalized.InvestmentAccountCode] = investmentAccount
		}

		prepared = append(prepared, preparedBulkOperation{
			asset:             asset,
			assetCode:         normalized.AssetCode,
			brokerageAccount:  brokerageAccount,
			investmentAccount: investmentAccount,
			operationType:     normalized.OperationType,
			date:              normalized.Date,
			quantity:          normalized.Quantity,
			unitPrice:         normalized.UnitPrice,
			totalFeeAmount:    item.TotalFeeAmount,
			notes:             normalized.Notes,
			grossAmount:       normalized.Quantity * normalized.UnitPrice,
		})
	}

	if err := ensureConsistentTotalFeeByDate(prepared); err != nil {
		return nil, err
	}

	createdIDs := make([]uuid.UUID, 0, len(prepared))
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := loadBrokerageAccountsByCode(tx, userID, brokerageAccountsByCode); err != nil {
			return err
		}
		if err := loadInvestmentAccountsByCode(tx, userID, investmentAccountsByCode); err != nil {
			return err
		}
		affectedGroups := make(map[string]preparedBulkOperation)
		for _, item := range prepared {
			created, err := s.repo.CreateOperation(tx, &Operation{
				ID:                     uuid.New(),
				UserID:                 userID,
				AssetID:                item.asset.ID,
				BrokerageAccountID:     &item.brokerageAccount.ID,
				InvestmentAccountID:    &item.investmentAccount.ID,
				OperationType:          item.operationType,
				Date:                   item.date,
				Quantity:               item.quantity,
				UnitPrice:              item.unitPrice,
				FeeAmount:              0,
				OriginalTotalFeeAmount: item.totalFeeAmount,
				GrossAmount:            item.grossAmount,
				NetAmount:              computeNetAmount(item.operationType, item.grossAmount, 0),
				Notes:                  item.notes,
			})
			if err != nil {
				return err
			}
			createdIDs = append(createdIDs, created.ID)
			affectedGroups[feeGroupKey(item.date, &item.brokerageAccount.ID)] = item
		}

		for _, item := range affectedGroups {
			totalFee := totalFeeAmountForDateAndBrokerage(prepared, item.date, &item.brokerageAccount.ID)
			if err := s.reallocateOperationFeesByDate(tx, userID, item.date, &item.brokerageAccount.ID, totalFee); err != nil {
				return err
			}
		}
		affectedAssetIDs, err := s.collectAssetIDsForFeeGroups(tx, userID, feeGroupReferencesFromPreparedOperations(affectedGroups))
		if err != nil {
			return err
		}
		return s.syncMirroredTransactionsForAssets(tx, userID, affectedAssetIDs)
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

func (s *service) PreviewImportOperations(userID uuid.UUID, req ImportOperationsRequest) (*PreviewImportOperationsResponse, error) {
	if len(req.Operations) == 0 {
		return nil, appErr.ErrInvalidInputWithMessage("at least one operation is required", nil)
	}

	prepared, _, _, _, err := s.prepareImportedOperations(userID, req)
	if err != nil {
		return nil, err
	}

	rows, err := s.previewPositionRowsForImportedOperations(userID, prepared)
	if err != nil {
		return nil, err
	}
	mirrorRows, err := s.previewMirrorRowsForImportedOperations(userID, prepared)
	if err != nil {
		return nil, err
	}

	return &PreviewImportOperationsResponse{
		PositionPreviewRows: rows,
		MirrorPreviewRows:   mirrorRows,
	}, nil
}

func (s *service) ImportOperations(userID uuid.UUID, req ImportOperationsRequest) (*ImportOperationsResponse, error) {
	if len(req.Operations) == 0 {
		return nil, appErr.ErrInvalidInputWithMessage("at least one operation is required", nil)
	}
	prepared, bulkItems, brokerageAccountsByCode, investmentAccountsByCode, err := s.prepareImportedOperations(userID, req)
	if err != nil {
		return nil, err
	}

	result := &ImportOperationsResponse{
		MirroringEnabled: req.CreateMirroredTransactions,
	}
	createdIDs := make([]uuid.UUID, 0, len(prepared))
	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if err := loadBrokerageAccountsByCode(tx, userID, brokerageAccountsByCode); err != nil {
			return err
		}
		if err := loadInvestmentAccountsByCode(tx, userID, investmentAccountsByCode); err != nil {
			return err
		}
		affectedAccountCodes := make(map[string]struct{})
		affectedGroups := make(map[string]preparedBulkOperation)
		createdByClientRowID := make(map[string]uuid.UUID, len(prepared))
		categoryID := uuid.Nil
		var categoryErr error
		if req.CreateMirroredTransactions {
			categoryID, categoryErr = resolveActiveLeafCategoryID(tx, userID, investmentMirrorCategoryCode)
			if categoryErr != nil {
				return categoryErr
			}
		}

		for _, item := range prepared {
			created, err := s.repo.CreateOperation(tx, &Operation{
				ID:                     uuid.New(),
				UserID:                 userID,
				AssetID:                item.asset.ID,
				BrokerageAccountID:     &item.brokerageAccount.ID,
				InvestmentAccountID:    &item.investmentAccount.ID,
				OperationType:          item.operationType,
				Date:                   item.date,
				Quantity:               item.quantity,
				UnitPrice:              item.unitPrice,
				FeeAmount:              0,
				OriginalTotalFeeAmount: item.totalFeeAmount,
				GrossAmount:            item.grossAmount,
				NetAmount:              computeNetAmount(item.operationType, item.grossAmount, 0),
				Notes:                  item.notes,
			})
			if err != nil {
				return err
			}
			createdIDs = append(createdIDs, created.ID)
			createdByClientRowID[item.clientRowID] = created.ID
			affectedGroups[feeGroupKey(item.date, &item.brokerageAccount.ID)] = item.preparedBulkOperation
		}

		for _, item := range affectedGroups {
			totalFee := totalFeeAmountForDateAndBrokerage(bulkItems, item.date, &item.brokerageAccount.ID)
			if err := s.reallocateOperationFeesByDate(tx, userID, item.date, &item.brokerageAccount.ID, totalFee); err != nil {
				return err
			}
		}

		if req.CreateMirroredTransactions {
			groupedItems := make(map[string][]preparedImportedOperation)
			for _, item := range prepared {
				if item.mirrorDraft == nil {
					continue
				}
				groupKey := cashMovementGroupKeyForPreparedOperation(item.preparedBulkOperation)
				groupedItems[groupKey] = append(groupedItems[groupKey], item)
			}

			for _, groupItems := range groupedItems {
				operations := make([]Operation, 0, len(groupItems))
				for _, item := range groupItems {
					operationID := createdByClientRowID[item.clientRowID]
					operation, err := loadOperationByID(tx, userID, operationID)
					if err != nil {
						return err
					}
					operations = append(operations, *operation)
				}

				summary, err := s.buildMirrorCashMovementSummary(tx, userID, operations, groupItems[0].assetCode)
				if err != nil {
					return err
				}
				draft := *groupItems[0].mirrorDraft
				var visibleTx *transaction.Transaction
				var hiddenTx *transaction.Transaction
				var realizedPnlTx *transaction.Transaction
				gainCategoryID := uuid.Nil
				lossCategoryID := uuid.Nil
				if summary.OperationType == OperationTypeSell {
					gainCategoryID, lossCategoryID, err = s.resolveSellMirrorCategoryIDs(tx, userID)
					if err != nil {
						return err
					}
				}
				bonificationIncomeCategoryID := uuid.Nil
				if summary.OperationType == OperationTypeBonification {
					bonificationIncomeCategoryID, err = s.resolveBonificationIncomeCategoryID(tx, userID)
					if err != nil {
						return err
					}
				}
				if draft.TransactionID != nil {
					visibleTx, hiddenTx, err = attachExistingMirrorTransferPair(tx, userID, categoryID, summary, *draft.TransactionID)
				} else {
					visibleTx, hiddenTx, err = createMirrorTransferPair(tx, userID, categoryID, summary, draft)
				}
				if err != nil {
					return err
				}
				if summary.OperationType == OperationTypeSell {
					if draft.RealizedPNLTransactionID != nil {
						realizedPnlTx, err = attachExistingRealizedPNLTransaction(tx, userID, gainCategoryID, lossCategoryID, summary, *draft.RealizedPNLTransactionID)
					} else {
						realizedPnlTx, err = createRealizedPNLTransaction(tx, userID, gainCategoryID, lossCategoryID, summary)
					}
					if err != nil {
						return err
					}
				}
				var bonificationIncomeTx *transaction.Transaction
				if summary.OperationType == OperationTypeBonification {
					if draft.BonificationIncomeTransactionID != nil {
						bonificationIncomeTx, err = attachExistingBonificationIncomeTransaction(tx, userID, bonificationIncomeCategoryID, summary, *draft.BonificationIncomeTransactionID)
					} else {
						bonificationIncomeTx, err = createBonificationIncomeTransaction(tx, userID, bonificationIncomeCategoryID, summary)
					}
					if err != nil {
						return err
					}
				}
				if err := s.repo.CreateOperationTransactionLinks(tx, createOperationTransactionLinksForGroup(userID, operations, map[OperationTransactionLinkRole]*transaction.Transaction{
					OperationTransactionLinkRoleVisibleTransfer:    visibleTx,
					OperationTransactionLinkRoleHiddenTransfer:     hiddenTx,
					OperationTransactionLinkRoleRealizedPNL:        realizedPnlTx,
					OperationTransactionLinkRoleBonificationIncome: bonificationIncomeTx,
				})); err != nil {
					return err
				}
				result.MirroredTransactionsCreated += 2
				if realizedPnlTx != nil {
					result.MirroredTransactionsCreated++
				}
				if bonificationIncomeTx != nil {
					result.MirroredTransactionsCreated++
				}
				if draft.TransactionID == nil {
					affectedAccountCodes[draft.SourceAccountCode] = struct{}{}
					affectedAccountCodes[draft.DestinationAccountCode] = struct{}{}
				}
				extraAccountIDs := make([]uuid.UUID, 0, 2)
				if realizedPnlTx != nil {
					extraAccountIDs = append(extraAccountIDs, realizedPnlTx.AccountID)
				}
				if bonificationIncomeTx != nil {
					extraAccountIDs = append(extraAccountIDs, bonificationIncomeTx.AccountID)
				}
				if len(extraAccountIDs) > 0 {
					codes, err := resolveAccountCodesByIDs(tx, userID, extraAccountIDs)
					if err != nil {
						return err
					}
					for _, code := range codes {
						affectedAccountCodes[code] = struct{}{}
					}
				}
			}
		}

		if len(affectedAccountCodes) > 0 {
			codes := make([]string, 0, len(affectedAccountCodes))
			for code := range affectedAccountCodes {
				codes = append(codes, code)
			}
			if err := refreshAccountBalances(tx, userID, codes); err != nil {
				return err
			}
		}
		affectedAssetIDs, err := s.collectAssetIDsForFeeGroups(tx, userID, feeGroupReferencesFromPreparedOperations(affectedGroups))
		if err != nil {
			return err
		}
		return s.syncMirroredTransactionsForAssets(tx, userID, affectedAssetIDs)
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
	result.Operations = make([]OperationRow, 0, len(createdIDs))
	for _, row := range rows {
		if _, ok := createdSet[row.ID]; ok {
			result.Operations = append(result.Operations, row)
		}
	}

	return result, nil
}

func (s *service) ListOperations(userID uuid.UUID) ([]OperationRow, error) {
	rows, err := s.repo.ListOperations(userID)
	if err != nil {
		return nil, err
	}
	annotateOperationCashMovementGroups(rows)
	s.refreshMissingAssetMetadataForUser(userID)
	return rows, nil
}

func (s *service) CreateOperationMirror(userID, operationID uuid.UUID, req CreateOperationMirrorRequest) (*OperationRow, error) {
	operation, err := s.repo.GetOperationByID(userID, operationID)
	if err != nil {
		return nil, err
	}
	if !isMirrorableOperationType(operation.OperationType) {
		return nil, appErr.ErrInvalidInputWithMessage("only buy, sell, and bonification operations can be mirrored", nil)
	}

	if req.TransactionID == nil {
		req.SourceAccountCode = strings.TrimSpace(req.SourceAccountCode)
		req.DestinationAccountCode = strings.TrimSpace(req.DestinationAccountCode)
		if req.SourceAccountCode == "" || req.DestinationAccountCode == "" {
			return nil, appErr.ErrInvalidInputWithMessage("mirror accounts are required", nil)
		}
	}

	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		categoryID, err := resolveActiveLeafCategoryID(tx, userID, investmentMirrorCategoryCode)
		if err != nil {
			return err
		}
		assetCode, err := resolveAssetCodeByID(tx, userID, operation.AssetID)
		if err != nil {
			return err
		}
		groupOperations, err := s.listOperationsInCashMovementGroup(tx, userID, operation, assetCode)
		if err != nil {
			return err
		}
		groupLinks, err := listOperationTransactionLinksForOperations(tx, userID, operationIDsFromOperations(groupOperations))
		if err != nil {
			return err
		}
		if len(groupLinks) > 0 {
			return appErr.ErrInvalidInputWithCode("investment.operation.mirror.already_linked", "investment operation already has linked mirror transactions", nil)
		}
		summary, err := s.buildMirrorCashMovementSummary(tx, userID, groupOperations, assetCode)
		if err != nil {
			return err
		}
		gainCategoryID := uuid.Nil
		lossCategoryID := uuid.Nil
		if summary.OperationType == OperationTypeSell {
			gainCategoryID, lossCategoryID, err = s.resolveSellMirrorCategoryIDs(tx, userID)
			if err != nil {
				return err
			}
		}
		bonificationIncomeCategoryID := uuid.Nil
		if summary.OperationType == OperationTypeBonification {
			bonificationIncomeCategoryID, err = s.resolveBonificationIncomeCategoryID(tx, userID)
			if err != nil {
				return err
			}
		}

		var visibleTx *transaction.Transaction
		var hiddenTx *transaction.Transaction
		var realizedPnlTx *transaction.Transaction
		var bonificationIncomeTx *transaction.Transaction
		if req.TransactionID != nil {
			visibleTx, hiddenTx, err = attachExistingMirrorTransferPair(tx, userID, categoryID, summary, *req.TransactionID)
			if err != nil {
				return err
			}
		} else {
			visibleTx, hiddenTx, err = createMirrorTransferPair(tx, userID, categoryID, summary, MirroredTransactionDraftRequest{
				SourceAccountCode:      req.SourceAccountCode,
				DestinationAccountCode: req.DestinationAccountCode,
			})
			if err != nil {
				return err
			}
		}

		if summary.OperationType == OperationTypeSell {
			if req.RealizedPNLTransactionID != nil {
				realizedPnlTx, err = attachExistingRealizedPNLTransaction(tx, userID, gainCategoryID, lossCategoryID, summary, *req.RealizedPNLTransactionID)
			} else {
				realizedPnlTx, err = createRealizedPNLTransaction(tx, userID, gainCategoryID, lossCategoryID, summary)
			}
			if err != nil {
				return err
			}
		}
		if summary.OperationType == OperationTypeBonification {
			if req.BonificationIncomeTransactionID != nil {
				bonificationIncomeTx, err = attachExistingBonificationIncomeTransaction(tx, userID, bonificationIncomeCategoryID, summary, *req.BonificationIncomeTransactionID)
			} else {
				bonificationIncomeTx, err = createBonificationIncomeTransaction(tx, userID, bonificationIncomeCategoryID, summary)
			}
			if err != nil {
				return err
			}
		}

		if err := s.repo.CreateOperationTransactionLinks(tx, createOperationTransactionLinksForGroup(userID, groupOperations, map[OperationTransactionLinkRole]*transaction.Transaction{
			OperationTransactionLinkRoleVisibleTransfer:    visibleTx,
			OperationTransactionLinkRoleHiddenTransfer:     hiddenTx,
			OperationTransactionLinkRoleRealizedPNL:        realizedPnlTx,
			OperationTransactionLinkRoleBonificationIncome: bonificationIncomeTx,
		})); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return s.operationToRow(userID, operation)
}

func (s *service) CreateOperationMirrorsBulk(userID uuid.UUID, req CreateOperationMirrorsBulkRequest) ([]OperationRow, error) {
	if len(req.Items) == 0 {
		return nil, appErr.ErrInvalidInputWithMessage("at least one mirror item is required", nil)
	}

	updatedIDs := make([]uuid.UUID, 0, len(req.Items))
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		categoryID, err := resolveActiveLeafCategoryID(tx, userID, investmentMirrorCategoryCode)
		if err != nil {
			return err
		}

		processedGroups := make(map[string]struct{}, len(req.Items))
		for _, item := range req.Items {
			operation, err := s.repo.GetOperationByID(userID, item.OperationID)
			if err != nil {
				return err
			}
			if !isMirrorableOperationType(operation.OperationType) {
				return appErr.ErrInvalidInputWithMessage("only buy, sell, and bonification operations can be mirrored", nil)
			}

			assetCode, err := resolveAssetCodeByID(tx, userID, operation.AssetID)
			if err != nil {
				return err
			}
			groupKey := cashMovementGroupKeyForOperation(operation, assetCode)
			if _, ok := processedGroups[groupKey]; ok {
				continue
			}
			groupOperations, err := s.listOperationsInCashMovementGroup(tx, userID, operation, assetCode)
			if err != nil {
				return err
			}
			groupLinks, err := listOperationTransactionLinksForOperations(tx, userID, operationIDsFromOperations(groupOperations))
			if err != nil {
				return err
			}
			if len(groupLinks) > 0 {
				return appErr.ErrInvalidInputWithCode("investment.operation.mirror.already_linked", "investment operation already has linked mirror transactions", nil)
			}
			summary, err := s.buildMirrorCashMovementSummary(tx, userID, groupOperations, assetCode)
			if err != nil {
				return err
			}
			gainCategoryID := uuid.Nil
			lossCategoryID := uuid.Nil
			if summary.OperationType == OperationTypeSell {
				gainCategoryID, lossCategoryID, err = s.resolveSellMirrorCategoryIDs(tx, userID)
				if err != nil {
					return err
				}
			}
			bonificationIncomeCategoryID := uuid.Nil
			if summary.OperationType == OperationTypeBonification {
				bonificationIncomeCategoryID, err = s.resolveBonificationIncomeCategoryID(tx, userID)
				if err != nil {
					return err
				}
			}

			var visibleTx *transaction.Transaction
			var hiddenTx *transaction.Transaction
			var realizedPnlTx *transaction.Transaction
			var bonificationIncomeTx *transaction.Transaction
			if item.TransactionID != nil {
				visibleTx, hiddenTx, err = attachExistingMirrorTransferPair(tx, userID, categoryID, summary, *item.TransactionID)
				if err != nil {
					return err
				}
			} else {
				item.SourceAccountCode = strings.TrimSpace(item.SourceAccountCode)
				item.DestinationAccountCode = strings.TrimSpace(item.DestinationAccountCode)
				if item.SourceAccountCode == "" || item.DestinationAccountCode == "" {
					return appErr.ErrInvalidInputWithMessage("mirror accounts are required", nil)
				}
				visibleTx, hiddenTx, err = createMirrorTransferPair(tx, userID, categoryID, summary, MirroredTransactionDraftRequest{
					SourceAccountCode:      item.SourceAccountCode,
					DestinationAccountCode: item.DestinationAccountCode,
				})
				if err != nil {
					return err
				}
			}

			if summary.OperationType == OperationTypeSell {
				if item.RealizedPNLTransactionID != nil {
					realizedPnlTx, err = attachExistingRealizedPNLTransaction(tx, userID, gainCategoryID, lossCategoryID, summary, *item.RealizedPNLTransactionID)
				} else {
					realizedPnlTx, err = createRealizedPNLTransaction(tx, userID, gainCategoryID, lossCategoryID, summary)
				}
				if err != nil {
					return err
				}
			}
			if summary.OperationType == OperationTypeBonification {
				if item.BonificationIncomeTransactionID != nil {
					bonificationIncomeTx, err = attachExistingBonificationIncomeTransaction(tx, userID, bonificationIncomeCategoryID, summary, *item.BonificationIncomeTransactionID)
				} else {
					bonificationIncomeTx, err = createBonificationIncomeTransaction(tx, userID, bonificationIncomeCategoryID, summary)
				}
				if err != nil {
					return err
				}
			}

			if err := s.repo.CreateOperationTransactionLinks(tx, createOperationTransactionLinksForGroup(userID, groupOperations, map[OperationTransactionLinkRole]*transaction.Transaction{
				OperationTransactionLinkRoleVisibleTransfer:    visibleTx,
				OperationTransactionLinkRoleHiddenTransfer:     hiddenTx,
				OperationTransactionLinkRoleRealizedPNL:        realizedPnlTx,
				OperationTransactionLinkRoleBonificationIncome: bonificationIncomeTx,
			})); err != nil {
				return err
			}
			updatedIDs = append(updatedIDs, operationIDsFromOperations(groupOperations)...)
			processedGroups[groupKey] = struct{}{}
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
	updatedSet := make(map[uuid.UUID]struct{}, len(updatedIDs))
	for _, id := range updatedIDs {
		updatedSet[id] = struct{}{}
	}
	result := make([]OperationRow, 0, len(updatedIDs))
	for _, row := range rows {
		if _, ok := updatedSet[row.ID]; ok {
			result = append(result, row)
		}
	}
	return result, nil
}

func (s *service) UpdateOperation(userID uuid.UUID, operationID uuid.UUID, req UpdateOperationRequest) (*OperationRow, error) {
	current, err := s.repo.GetOperationByID(userID, operationID)
	if err != nil {
		return nil, err
	}

	finalAssetID := current.AssetID
	finalBrokerageAccountID := current.BrokerageAccountID
	finalInvestmentAccountID := current.InvestmentAccountID
	finalOperationType := current.OperationType
	finalDate := current.Date
	finalQuantity := current.Quantity
	finalUnitPrice := current.UnitPrice
	finalTotalFeeAmount := current.OriginalTotalFeeAmount
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
	if req.BrokerageAccountCode != nil {
		normalized := normalizeAccountCode(*req.BrokerageAccountCode)
		brokerageAccount, err := resolveActiveBrokerageAccountByCode(s.repo.DB(), userID, normalized)
		if err != nil {
			return nil, err
		}
		finalBrokerageAccountID = &brokerageAccount.ID
		update.BrokerageAccountID = &brokerageAccount.ID
	}
	if req.InvestmentAccountCode != nil {
		normalized := normalizeAccountCode(*req.InvestmentAccountCode)
		investmentAccount, err := resolveActiveInvestmentAccountByCode(s.repo.DB(), userID, normalized)
		if err != nil {
			return nil, err
		}
		finalInvestmentAccountID = &investmentAccount.ID
		update.InvestmentAccountID = &investmentAccount.ID
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
		finalTotalFeeAmount = *req.FeeAmount
		update.OriginalTotalFeeAmount = req.FeeAmount
	}
	if err := CheckOperationFeeAmount(finalOperationType, finalTotalFeeAmount); err != nil {
		return nil, err
	}
	if req.Notes != nil {
		trimmed := strings.TrimSpace(*req.Notes)
		finalNotes = trimmed
		update.Notes = &trimmed
	}
	if update.AssetID == nil && update.BrokerageAccountID == nil && update.InvestmentAccountID == nil && update.OperationType == nil && update.Date == nil && update.Quantity == nil &&
		update.UnitPrice == nil && update.OriginalTotalFeeAmount == nil && update.Notes == nil {
		return nil, appErr.ErrInvalidInputWithMessage("at least one operation field must be provided", nil)
	}

	currentAssetCode, err := resolveAssetCodeByID(s.repo.DB(), userID, current.AssetID)
	if err != nil {
		return nil, err
	}
	finalAssetCode := currentAssetCode
	if finalAssetID != current.AssetID {
		finalAssetCode, err = resolveAssetCodeByID(s.repo.DB(), userID, finalAssetID)
		if err != nil {
			return nil, err
		}
	}
	currentGroupKey := cashMovementGroupKeyForOperation(current, currentAssetCode)
	finalGroupKey := cashMovementGroupKeyForOperation(&Operation{
		Date:                finalDate,
		BrokerageAccountID:  finalBrokerageAccountID,
		InvestmentAccountID: finalInvestmentAccountID,
		OperationType:       finalOperationType,
	}, finalAssetCode)
	if currentGroupKey != finalGroupKey {
		links, err := s.repo.ListOperationTransactionLinks(s.repo.DB(), userID, operationID)
		if err != nil {
			return nil, err
		}
		if len(links) > 0 {
			return nil, appErr.ErrInvalidInputWithMessage("linked mirrored operations cannot move to a different cash movement group", nil)
		}
	}

	grossAmount := finalQuantity * finalUnitPrice
	update.GrossAmount = &grossAmount
	provisionalNetAmount := computeNetAmount(finalOperationType, grossAmount, 0)
	update.NetAmount = &provisionalNetAmount

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
		groupChanged := feeGroupKey(current.Date, current.BrokerageAccountID) != feeGroupKey(finalDate, finalBrokerageAccountID)
		if groupChanged {
			oldTotalFee, err := s.totalFeeAmountForExistingDate(tx, userID, current.Date, current.BrokerageAccountID)
			if err != nil {
				return err
			}
			if err := s.reallocateOperationFeesByDate(tx, userID, current.Date, current.BrokerageAccountID, oldTotalFee); err != nil {
				return err
			}
			if req.FeeAmount == nil {
				existingNewDateFee, err := s.totalFeeAmountForExistingDate(tx, userID, finalDate, finalBrokerageAccountID)
				if err != nil {
					return err
				}
				if existingNewDateFee > 0 {
					finalTotalFeeAmount = existingNewDateFee
				}
			}
			if finalTotalFeeAmount != updated.OriginalTotalFeeAmount {
				updated, err = s.repo.UpdateOperation(tx, userID, operationID, &UpdateOperationModel{
					OriginalTotalFeeAmount: &finalTotalFeeAmount,
				})
				if err != nil {
					return err
				}
			}
		}
		if err := s.reallocateOperationFeesByDate(tx, userID, finalDate, finalBrokerageAccountID, finalTotalFeeAmount); err != nil {
			return err
		}
		affectedAssetIDs, err := s.collectAssetIDsForFeeGroups(tx, userID, []feeGroupReference{
			{
				Date:               current.Date,
				BrokerageAccountID: current.BrokerageAccountID,
			},
			{
				Date:               finalDate,
				BrokerageAccountID: finalBrokerageAccountID,
			},
		})
		if err != nil {
			return err
		}
		affectedAssets = append(affectedAssets, affectedAssetIDs...)
		if err := s.syncMirroredTransactionsForAssets(tx, userID, affectedAssets); err != nil {
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
		if err := s.deleteMirroredTransactionsForOperation(tx, userID, operationID); err != nil {
			return err
		}
		if err := s.repo.DeleteOperation(tx, userID, operationID); err != nil {
			return err
		}
		oldTotalFee, err := s.totalFeeAmountForExistingDate(tx, userID, current.Date, current.BrokerageAccountID)
		if err != nil {
			return err
		}
		if err := s.reallocateOperationFeesByDate(tx, userID, current.Date, current.BrokerageAccountID, oldTotalFee); err != nil {
			return err
		}
		affectedAssetIDs, err := s.collectAssetIDsForFeeGroups(tx, userID, []feeGroupReference{{
			Date:               current.Date,
			BrokerageAccountID: current.BrokerageAccountID,
		}})
		if err != nil {
			return err
		}
		affectedAssetIDs = append(affectedAssetIDs, current.AssetID)
		if err := s.syncMirroredTransactionsForAssets(tx, userID, affectedAssetIDs); err != nil {
			return err
		}
		return s.rebuildPositions(tx, userID, affectedAssetIDs)
	})
}

func (s *service) DeleteOperationsBulk(userID uuid.UUID, req DeleteOperationsBulkRequest) error {
	operationIDs := uniqueUUIDs(req.OperationIDs)
	if len(operationIDs) == 0 {
		return appErr.ErrInvalidInputWithMessage("at least one operation id is required", nil)
	}

	operations := make([]Operation, 0, len(operationIDs))
	affectedFeeGroups := make([]feeGroupReference, 0, len(operationIDs))
	affectedAssetIDs := make([]uuid.UUID, 0, len(operationIDs))
	for _, operationID := range operationIDs {
		current, err := s.repo.GetOperationByID(userID, operationID)
		if err != nil {
			return err
		}
		operations = append(operations, *current)
		affectedFeeGroups = append(affectedFeeGroups, feeGroupReference{
			Date:               current.Date,
			BrokerageAccountID: current.BrokerageAccountID,
		})
		affectedAssetIDs = append(affectedAssetIDs, current.AssetID)
	}

	return s.repo.DB().Transaction(func(tx *gorm.DB) error {
		for _, operation := range operations {
			if err := s.deleteMirroredTransactionsForOperation(tx, userID, operation.ID); err != nil {
				return err
			}
		}
		if err := s.repo.DeleteOperations(tx, userID, operationIDs); err != nil {
			return err
		}

		for _, group := range uniqueFeeGroupReferences(affectedFeeGroups) {
			totalFeeAmount, err := s.totalFeeAmountForExistingDate(tx, userID, group.Date, group.BrokerageAccountID)
			if err != nil {
				return err
			}
			if err := s.reallocateOperationFeesByDate(tx, userID, group.Date, group.BrokerageAccountID, totalFeeAmount); err != nil {
				return err
			}
		}

		groupAssetIDs, err := s.collectAssetIDsForFeeGroups(tx, userID, affectedFeeGroups)
		if err != nil {
			return err
		}
		affectedAssetIDs = append(affectedAssetIDs, groupAssetIDs...)
		if err := s.syncMirroredTransactionsForAssets(tx, userID, affectedAssetIDs); err != nil {
			return err
		}
		return s.rebuildPositions(tx, userID, affectedAssetIDs)
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
	if req.BrokerageAccountCode == "" {
		return appErr.ErrInvalidInputWithMessage("brokerage account is required", nil)
	}
	if req.InvestmentAccountCode == "" {
		return appErr.ErrInvalidInputWithMessage("investment account is required", nil)
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
	if err := CheckOperationFeeAmount(req.OperationType, req.FeeAmount); err != nil {
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
	sortOperationsForHistory(operations)

	var currentQuantity int64
	var totalCostBasis int64
	var realizedPNL int64

	for _, operation := range operations {
		switch operation.OperationType {
		case OperationTypeBuy, OperationTypeBonification:
			currentQuantity += operation.Quantity
			totalCostBasis += operation.NetAmount
		case OperationTypeAmortization:
			if operation.NetAmount > totalCostBasis {
				return appErr.ErrInvalidInputWithCode(
					"investment.operation.amortization.exceeds.position",
					"amortization exceeds available cost basis for asset history",
					nil,
				)
			}
			totalCostBasis -= operation.NetAmount
			if totalCostBasis < 0 {
				totalCostBasis = 0
			}
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

func (s *service) rebuildPositions(tx *gorm.DB, userID uuid.UUID, assetIDs []uuid.UUID) error {
	for _, assetID := range uniqueUUIDs(assetIDs) {
		if err := s.rebuildPosition(tx, userID, assetID); err != nil {
			return err
		}
	}
	return nil
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
	if operationType == OperationTypeAmortization {
		return grossAmount
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
	asset             *Asset
	assetCode         string
	brokerageAccount  *dbAccountRow
	investmentAccount *dbAccountRow
	operationType     OperationType
	date              time.Time
	quantity          int64
	unitPrice         int64
	totalFeeAmount    int64
	allocatedFee      int64
	notes             string
	grossAmount       int64
}

type preparedImportedOperation struct {
	preparedBulkOperation
	clientRowID string
	mirrorDraft *MirroredTransactionDraftRequest
}

type mirrorCashMovementSummary struct {
	OperationIDs        []uuid.UUID
	AssetCode           string
	OperationType       OperationType
	Date                time.Time
	Quantity            int64
	NetAmount           int64
	BrokerageAccountID  *uuid.UUID
	InvestmentAccountID *uuid.UUID
	ReleasedCostBasis   int64
	RealizedPNL         int64
}

func (s mirrorCashMovementSummary) previewExtraType() MirrorPreviewExtraType {
	switch s.OperationType {
	case OperationTypeSell:
		return MirrorPreviewExtraTypeRealizedPNL
	case OperationTypeBonification:
		return MirrorPreviewExtraTypeBonificationIncome
	default:
		return MirrorPreviewExtraTypeNone
	}
}

func (s mirrorCashMovementSummary) previewExtraAmount() int64 {
	switch s.OperationType {
	case OperationTypeSell:
		return s.RealizedPNL
	case OperationTypeBonification:
		return s.NetAmount
	default:
		return 0
	}
}

type previewTimelineOperation struct {
	date          time.Time
	createdAt     time.Time
	id            string
	clientRowID   string
	assetCode     string
	operationType OperationType
	quantity      int64
	netAmount     int64
}

type previewPositionState struct {
	CurrentQuantity int64
	TotalCostBasis  int64
}

func (s *service) prepareImportedOperations(
	userID uuid.UUID,
	req ImportOperationsRequest,
) ([]preparedImportedOperation, []preparedBulkOperation, map[string]*dbAccountRow, map[string]*dbAccountRow, error) {
	mirrorDraftsByRowID := make(map[string]MirroredTransactionDraftRequest, len(req.MirroredTransactions))
	for _, draft := range req.MirroredTransactions {
		rowID := strings.TrimSpace(draft.ClientRowID)
		if rowID == "" {
			return nil, nil, nil, nil, appErr.ErrInvalidInputWithMessage("mirrored transaction client row id is required", nil)
		}
		if _, exists := mirrorDraftsByRowID[rowID]; exists {
			return nil, nil, nil, nil, appErr.ErrInvalidInputWithMessage("duplicate mirrored transaction client row id", nil)
		}
		mirrorDraftsByRowID[rowID] = MirroredTransactionDraftRequest{
			ClientRowID:                     rowID,
			SourceAccountCode:               strings.TrimSpace(draft.SourceAccountCode),
			DestinationAccountCode:          strings.TrimSpace(draft.DestinationAccountCode),
			TransactionID:                   draft.TransactionID,
			RealizedPNLTransactionID:        draft.RealizedPNLTransactionID,
			BonificationIncomeTransactionID: draft.BonificationIncomeTransactionID,
		}
	}

	prepared := make([]preparedImportedOperation, 0, len(req.Operations))
	assetsByCode := make(map[string]*Asset, len(req.Operations))
	brokerageAccountsByCode := make(map[string]*dbAccountRow, len(req.Operations))
	investmentAccountsByCode := make(map[string]*dbAccountRow, len(req.Operations))
	for _, item := range req.Operations {
		clientRowID := strings.TrimSpace(item.ClientRowID)
		if clientRowID == "" {
			return nil, nil, nil, nil, appErr.ErrInvalidInputWithMessage("client row id is required", nil)
		}

		normalized := CreateOperationRequest{
			AssetCode:             normalizeAssetCode(item.AssetCode),
			BrokerageAccountCode:  normalizeAccountCode(item.BrokerageAccountCode),
			InvestmentAccountCode: normalizeAccountCode(item.InvestmentAccountCode),
			OperationType:         item.OperationType,
			Date:                  item.Date,
			Quantity:              item.Quantity,
			UnitPrice:             item.UnitPrice,
			Notes:                 strings.TrimSpace(item.Notes),
		}
		if err := s.validateCreateOperation(normalized); err != nil {
			return nil, nil, nil, nil, err
		}
		if err := CheckMoney("total fee amount", item.TotalFeeAmount, true); err != nil {
			return nil, nil, nil, nil, err
		}

		asset, ok := assetsByCode[normalized.AssetCode]
		if !ok {
			foundAsset, err := s.getOrCreateAssetForBulk(userID, normalized.AssetCode)
			if err != nil {
				return nil, nil, nil, nil, err
			}
			asset = foundAsset
			assetsByCode[normalized.AssetCode] = asset
		}

		brokerageAccount, ok := brokerageAccountsByCode[normalized.BrokerageAccountCode]
		if !ok {
			brokerageAccount = &dbAccountRow{Code: normalized.BrokerageAccountCode}
			brokerageAccountsByCode[normalized.BrokerageAccountCode] = brokerageAccount
		}
		investmentAccount, ok := investmentAccountsByCode[normalized.InvestmentAccountCode]
		if !ok {
			investmentAccount = &dbAccountRow{Code: normalized.InvestmentAccountCode}
			investmentAccountsByCode[normalized.InvestmentAccountCode] = investmentAccount
		}

		preparedItem := preparedImportedOperation{
			preparedBulkOperation: preparedBulkOperation{
				asset:             asset,
				assetCode:         normalized.AssetCode,
				brokerageAccount:  brokerageAccount,
				investmentAccount: investmentAccount,
				operationType:     normalized.OperationType,
				date:              normalized.Date,
				quantity:          normalized.Quantity,
				unitPrice:         normalized.UnitPrice,
				totalFeeAmount:    item.TotalFeeAmount,
				notes:             normalized.Notes,
				grossAmount:       normalized.Quantity * normalized.UnitPrice,
			},
			clientRowID: clientRowID,
		}

		prepared = append(prepared, preparedItem)
	}

	bulkItems := make([]preparedBulkOperation, 0, len(prepared))
	for _, item := range prepared {
		bulkItems = append(bulkItems, item.preparedBulkOperation)
	}
	if err := ensureConsistentTotalFeeByDate(bulkItems); err != nil {
		return nil, nil, nil, nil, err
	}
	if req.CreateMirroredTransactions {
		if err := assignMirrorDraftsToPreparedImportedOperations(prepared, mirrorDraftsByRowID); err != nil {
			return nil, nil, nil, nil, err
		}
	}

	return prepared, bulkItems, brokerageAccountsByCode, investmentAccountsByCode, nil
}

func cashMovementGroupKeyForPreparedOperation(item preparedBulkOperation) string {
	accountKey := "none"
	if item.brokerageAccount != nil && strings.TrimSpace(item.brokerageAccount.Code) != "" {
		accountKey = strings.ToLower(strings.TrimSpace(item.brokerageAccount.Code))
	}
	investmentAccountKey := "none"
	if item.investmentAccount != nil && strings.TrimSpace(item.investmentAccount.Code) != "" {
		investmentAccountKey = strings.ToLower(strings.TrimSpace(item.investmentAccount.Code))
	}
	return strings.Join([]string{
		item.date.UTC().Format("2006-01-02"),
		accountKey,
		investmentAccountKey,
		strings.ToUpper(strings.TrimSpace(item.assetCode)),
		string(item.operationType),
	}, "|")
}

func hasMultiOperationCashMovementGroup(items []preparedBulkOperation) bool {
	counts := make(map[string]int, len(items))
	for _, item := range items {
		counts[cashMovementGroupKeyForPreparedOperation(item)]++
	}
	for _, count := range counts {
		if count > 1 {
			return true
		}
	}
	return false
}

func mirroredTransactionDraftKey(draft MirroredTransactionDraftRequest) string {
	transactionID := ""
	if draft.TransactionID != nil {
		transactionID = draft.TransactionID.String()
	}
	realizedPNLTransactionID := ""
	if draft.RealizedPNLTransactionID != nil {
		realizedPNLTransactionID = draft.RealizedPNLTransactionID.String()
	}
	bonificationIncomeTransactionID := ""
	if draft.BonificationIncomeTransactionID != nil {
		bonificationIncomeTransactionID = draft.BonificationIncomeTransactionID.String()
	}
	return strings.Join([]string{
		strings.ToLower(strings.TrimSpace(draft.SourceAccountCode)),
		strings.ToLower(strings.TrimSpace(draft.DestinationAccountCode)),
		transactionID,
		realizedPNLTransactionID,
		bonificationIncomeTransactionID,
	}, "|")
}

func validateMirroredTransactionDraft(draft MirroredTransactionDraftRequest) error {
	if draft.TransactionID != nil {
		return nil
	}
	if draft.SourceAccountCode == "" || draft.DestinationAccountCode == "" {
		return appErr.ErrInvalidInputWithMessage("mirrored transaction accounts are required", nil)
	}
	if strings.EqualFold(draft.SourceAccountCode, draft.DestinationAccountCode) {
		return appErr.ErrInvalidInputWithMessage("mirrored transaction accounts must differ", nil)
	}
	return nil
}

func assignMirrorDraftsToPreparedImportedOperations(prepared []preparedImportedOperation, mirrorDraftsByRowID map[string]MirroredTransactionDraftRequest) error {
	groups := make(map[string][]int)
	for index, item := range prepared {
		if !isMirrorableOperationType(item.operationType) {
			continue
		}
		key := cashMovementGroupKeyForPreparedOperation(item.preparedBulkOperation)
		groups[key] = append(groups[key], index)
	}

	for _, indexes := range groups {
		var selected *MirroredTransactionDraftRequest
		var selectedKey string
		for _, index := range indexes {
			draft, ok := mirrorDraftsByRowID[prepared[index].clientRowID]
			if !ok {
				continue
			}
			currentKey := mirroredTransactionDraftKey(draft)
			if selected == nil {
				candidate := draft
				selected = &candidate
				selectedKey = currentKey
				continue
			}
			if currentKey != selectedKey {
				return appErr.ErrInvalidInputWithMessage("grouped cash movement requires a single mirrored transaction draft", nil)
			}
		}
		if selected == nil {
			return appErr.ErrInvalidInputWithMessage("missing mirrored transaction draft for operation row", nil)
		}
		if err := validateMirroredTransactionDraft(*selected); err != nil {
			return err
		}
		draftCopy := *selected
		for _, index := range indexes {
			prepared[index].mirrorDraft = &draftCopy
		}
	}

	return nil
}

func ensureConsistentTotalFeeByDate(items []preparedBulkOperation) error {
	byGroup := make(map[string][]int)
	for index, item := range items {
		if item.operationType == OperationTypeBonification {
			continue
		}
		groupKey := feeGroupKey(item.date, &item.brokerageAccount.ID)
		byGroup[groupKey] = append(byGroup[groupKey], index)
	}

	for _, indexes := range byGroup {
		dayFee := int64(-1)
		for _, index := range indexes {
			if dayFee == -1 {
				dayFee = items[index].totalFeeAmount
				continue
			}
			if items[index].totalFeeAmount != dayFee {
				return appErr.ErrInvalidInputWithMessage("all operations on the same day and brokerage account must use the same total fee amount", nil)
			}
		}
	}
	return nil
}

func (s *service) previewPositionRowsForImportedOperations(userID uuid.UUID, prepared []preparedImportedOperation) ([]PositionPreviewRow, error) {
	if len(prepared) == 0 {
		return []PositionPreviewRow{}, nil
	}

	allocatedFees := allocatePreparedImportedOperationFees(prepared)
	preparedByAsset := make(map[uuid.UUID][]preparedImportedOperation)
	for _, item := range prepared {
		preparedByAsset[item.asset.ID] = append(preparedByAsset[item.asset.ID], item)
	}

	rows := make([]PositionPreviewRow, 0, len(preparedByAsset))
	for assetID, assetPrepared := range preparedByAsset {
		existingOperations, err := s.repo.ListAssetOperations(nil, userID, assetID)
		if err != nil {
			return nil, err
		}

		currentState, projectedState, err := previewPositionStatesForAsset(existingOperations, assetPrepared, allocatedFees)
		if err != nil {
			return nil, err
		}

		asset := assetPrepared[0].asset
		row := PositionPreviewRow{
			AssetCode:             asset.Code,
			AssetName:             asset.Name,
			CurrentQuantity:       currentState.CurrentQuantity,
			DraftChange:           projectedState.CurrentQuantity - currentState.CurrentQuantity,
			ProjectedQuantity:     projectedState.CurrentQuantity,
			CurrentAveragePrice:   averagePriceFromState(currentState),
			ProjectedAveragePrice: averagePriceFromState(projectedState),
		}
		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].AssetCode < rows[j].AssetCode
	})

	return rows, nil
}

func (s *service) previewMirrorRowsForImportedOperations(userID uuid.UUID, prepared []preparedImportedOperation) ([]MirrorPreviewRow, error) {
	if len(prepared) == 0 {
		return []MirrorPreviewRow{}, nil
	}

	existingRows, err := s.repo.ListOperations(userID)
	if err != nil {
		return nil, err
	}

	allocatedFees := allocatePreparedImportedOperationFees(prepared)
	grouped := make(map[string][]preparedImportedOperation)
	for _, item := range prepared {
		if !isMirrorableOperationType(item.operationType) {
			continue
		}
		key := cashMovementGroupKeyForPreparedOperation(item.preparedBulkOperation)
		grouped[key] = append(grouped[key], item)
	}

	rows := make([]MirrorPreviewRow, 0, len(grouped))
	for groupKey, groupItems := range grouped {
		first := groupItems[0]
		summary := mirrorCashMovementSummary{
			AssetCode:           first.assetCode,
			OperationType:       first.operationType,
			Date:                first.date,
			BrokerageAccountID:  &first.brokerageAccount.ID,
			InvestmentAccountID: &first.investmentAccount.ID,
		}
		for _, item := range groupItems {
			summary.Quantity += item.quantity
			summary.NetAmount += computeNetAmount(item.operationType, item.grossAmount, allocatedFees[item.clientRowID])
		}
		if summary.OperationType == OperationTypeSell {
			sellSummary, err := s.previewMirrorSellSummaryForImportedOperations(existingRows, first.assetCode, first.investmentAccount.Code, prepared, groupItems, allocatedFees)
			if err != nil {
				return nil, err
			}
			summary.ReleasedCostBasis = sellSummary.ReleasedCostBasis
			summary.RealizedPNL = sellSummary.RealizedPNL
		}

		rows = append(rows, MirrorPreviewRow{
			ClientRowID:            first.clientRowID,
			GroupKey:               groupKey,
			OperationType:          summary.OperationType,
			BrokerageAccountCode:   first.brokerageAccount.Code,
			InvestmentAccountCode:  first.investmentAccount.Code,
			Date:                   summary.Date,
			Description:            mirrorTransactionDescription(summary.OperationType, summary.Quantity, summary.AssetCode),
			TransferAmount:         mirrorTransferAmount(summary),
			ExtraAmount:            summary.previewExtraAmount(),
			ExtraType:              summary.previewExtraType(),
			SourceAccountCode:      defaultMirrorSourceAccountCode(summary, first.brokerageAccount.Code, first.investmentAccount.Code),
			DestinationAccountCode: defaultMirrorDestinationAccountCode(summary, first.brokerageAccount.Code, first.investmentAccount.Code),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if sameOperationDate(rows[i].Date, rows[j].Date) {
			return rows[i].Description < rows[j].Description
		}
		return rows[i].Date.Before(rows[j].Date)
	})
	return rows, nil
}

func (s *service) previewMirrorSellSummaryForImportedOperations(
	existingRows []OperationRow,
	assetCode string,
	investmentAccountCode string,
	allPrepared []preparedImportedOperation,
	groupItems []preparedImportedOperation,
	allocatedFees map[string]int64,
) (mirrorCashMovementSummary, error) {
	groupClientRowIDs := make(map[string]struct{}, len(groupItems))
	timeline := make([]previewTimelineOperation, 0, len(existingRows)+len(allPrepared))
	for _, item := range groupItems {
		groupClientRowIDs[item.clientRowID] = struct{}{}
	}

	for _, operation := range existingRows {
		if operation.AssetCode != assetCode {
			continue
		}
		if operation.InvestmentAccountCode == nil || *operation.InvestmentAccountCode != investmentAccountCode {
			continue
		}
		timeline = append(timeline, previewTimelineOperation{
			date:          operation.Date,
			createdAt:     operation.CreatedAt,
			id:            operation.ID.String(),
			operationType: operation.OperationType,
			quantity:      operation.Quantity,
			netAmount:     operation.NetAmount,
		})
	}
	for index, item := range allPrepared {
		if item.assetCode != assetCode || item.investmentAccount.Code != investmentAccountCode {
			continue
		}
		timeline = append(timeline, previewTimelineOperation{
			date:          item.date,
			createdAt:     time.Time{},
			id:            "draft-sell-" + strconv.Itoa(index) + "-" + item.clientRowID,
			clientRowID:   item.clientRowID,
			assetCode:     item.assetCode,
			operationType: item.operationType,
			quantity:      item.quantity,
			netAmount:     computeNetAmount(item.operationType, item.grossAmount, allocatedFees[item.clientRowID]),
		})
	}
	sortPreviewTimelineOperations(timeline)

	summary := mirrorCashMovementSummary{OperationType: OperationTypeSell}
	var currentQuantity int64
	var totalCostBasis int64
	for _, operation := range timeline {
		switch operation.operationType {
		case OperationTypeBuy, OperationTypeBonification:
			currentQuantity += operation.quantity
			totalCostBasis += operation.netAmount
		case OperationTypeAmortization:
			if operation.netAmount > totalCostBasis {
				return mirrorCashMovementSummary{}, appErr.ErrInvalidInputWithCode(
					"investment.operation.amortization.exceeds.position",
					"amortization exceeds available cost basis for asset history",
					nil,
				)
			}
			totalCostBasis -= operation.netAmount
			if totalCostBasis < 0 {
				totalCostBasis = 0
			}
		case OperationTypeSell:
			if operation.quantity > currentQuantity {
				err := appErr.ErrInvalidInputWithCode(
					"investment.operation.sell.exceeds.position",
					"sell operation exceeds available quantity for asset history",
					nil,
				)
				if operation.clientRowID != "" {
					err = err.WithDetails(map[string]any{
						"client_row_id":      operation.clientRowID,
						"asset_code":         operation.assetCode,
						"attempted_quantity": operation.quantity,
						"available_quantity": currentQuantity,
					})
				}
				return mirrorCashMovementSummary{}, err
			}
			sellCostBasis := divideRounded(totalCostBasis*operation.quantity, currentQuantity)
			if _, ok := groupClientRowIDs[operation.clientRowID]; ok {
				summary.ReleasedCostBasis += sellCostBasis
				summary.RealizedPNL += operation.netAmount - sellCostBasis
			}
			currentQuantity -= operation.quantity
			totalCostBasis -= sellCostBasis
			if currentQuantity == 0 {
				totalCostBasis = 0
			}
		}
	}
	return summary, nil
}

func allocatePreparedImportedOperationFees(items []preparedImportedOperation) map[string]int64 {
	allocatedByRowID := make(map[string]int64, len(items))
	byGroup := make(map[string][]int)
	for index, item := range items {
		if item.operationType == OperationTypeBonification {
			continue
		}
		groupKey := feeGroupKey(item.date, &item.brokerageAccount.ID)
		byGroup[groupKey] = append(byGroup[groupKey], index)
	}

	for _, indexes := range byGroup {
		totalFeeAmount := items[indexes[0]].totalFeeAmount
		totalGross := int64(0)
		for _, index := range indexes {
			totalGross += items[index].grossAmount
		}
		remaining := totalFeeAmount
		for order, index := range indexes {
			allocated := int64(0)
			if totalGross > 0 {
				if order == len(indexes)-1 {
					allocated = remaining
				} else {
					allocated = divideRounded(totalFeeAmount*items[index].grossAmount, totalGross)
					if allocated > remaining {
						allocated = remaining
					}
				}
			}
			allocatedByRowID[items[index].clientRowID] = allocated
			remaining -= allocated
		}
	}

	return allocatedByRowID
}

func previewPositionStatesForAsset(
	existingOperations []Operation,
	prepared []preparedImportedOperation,
	allocatedFees map[string]int64,
) (previewPositionState, previewPositionState, error) {
	sortOperationsForHistory(existingOperations)
	timeline := make([]previewTimelineOperation, 0, len(existingOperations)+len(prepared))
	for _, operation := range existingOperations {
		timeline = append(timeline, previewTimelineOperation{
			date:          operation.Date,
			createdAt:     operation.CreatedAt,
			id:            operation.ID.String(),
			operationType: operation.OperationType,
			quantity:      operation.Quantity,
			netAmount:     operation.NetAmount,
		})
	}
	for index, item := range prepared {
		allocatedFee := allocatedFees[item.clientRowID]
		timeline = append(timeline, previewTimelineOperation{
			date:          item.date,
			createdAt:     time.Time{},
			id:            "draft-" + strconv.Itoa(index) + "-" + item.clientRowID,
			clientRowID:   item.clientRowID,
			assetCode:     item.assetCode,
			operationType: item.operationType,
			quantity:      item.quantity,
			netAmount:     computeNetAmount(item.operationType, item.grossAmount, allocatedFee),
		})
	}

	sortPreviewTimelineOperations(timeline)

	currentState := previewPositionState{}
	for _, operation := range existingOperations {
		var err error
		currentState, err = applyPreviewTimelineOperation(currentState, previewTimelineOperation{
			operationType: operation.OperationType,
			quantity:      operation.Quantity,
			netAmount:     operation.NetAmount,
		})
		if err != nil {
			return previewPositionState{}, previewPositionState{}, err
		}
	}

	projectedState := previewPositionState{}
	for _, operation := range timeline {
		var err error
		projectedState, err = applyPreviewTimelineOperation(projectedState, operation)
		if err != nil {
			return previewPositionState{}, previewPositionState{}, err
		}
	}

	return currentState, projectedState, nil
}

func applyPreviewTimelineOperation(current previewPositionState, operation previewTimelineOperation) (previewPositionState, error) {
	switch operation.operationType {
	case OperationTypeBuy, OperationTypeBonification:
		current.CurrentQuantity += operation.quantity
		current.TotalCostBasis += operation.netAmount
	case OperationTypeAmortization:
		if operation.netAmount > current.TotalCostBasis {
			err := appErr.ErrInvalidInputWithCode(
				"investment.operation.amortization.exceeds.position",
				"amortization exceeds available cost basis for asset history",
				nil,
			)
			if operation.clientRowID != "" {
				err = err.WithDetails(map[string]any{
					"client_row_id": operation.clientRowID,
					"asset_code":    operation.assetCode,
				})
			}
			return previewPositionState{}, err
		}
		current.TotalCostBasis -= operation.netAmount
		if current.TotalCostBasis < 0 {
			current.TotalCostBasis = 0
		}
	case OperationTypeSell:
		if operation.quantity > current.CurrentQuantity {
			err := appErr.ErrInvalidInputWithCode(
				"investment.operation.sell.exceeds.position",
				"sell operation exceeds available quantity for asset history",
				nil,
			)
			if operation.clientRowID != "" {
				err = err.WithDetails(map[string]any{
					"client_row_id":      operation.clientRowID,
					"asset_code":         operation.assetCode,
					"attempted_quantity": operation.quantity,
					"available_quantity": current.CurrentQuantity,
				})
			}
			return previewPositionState{}, err
		}
		sellCostBasis := divideRounded(current.TotalCostBasis*operation.quantity, current.CurrentQuantity)
		current.CurrentQuantity -= operation.quantity
		current.TotalCostBasis -= sellCostBasis
		if current.CurrentQuantity == 0 {
			current.TotalCostBasis = 0
		}
	}
	return current, nil
}

func averagePriceFromState(state previewPositionState) int64 {
	if state.CurrentQuantity <= 0 {
		return 0
	}
	return divideRounded(state.TotalCostBasis, state.CurrentQuantity)
}

func totalFeeAmountForDateAndBrokerage(items []preparedBulkOperation, date time.Time, brokerageAccountID *uuid.UUID) int64 {
	targetGroup := feeGroupKey(date, brokerageAccountID)
	for _, item := range items {
		if item.operationType == OperationTypeBonification {
			continue
		}
		if feeGroupKey(item.date, &item.brokerageAccount.ID) == targetGroup {
			return item.totalFeeAmount
		}
	}
	return 0
}

func sameOperationDate(left, right time.Time) bool {
	return left.UTC().Format("2006-01-02") == right.UTC().Format("2006-01-02")
}

func operationHistoryPriority(operationType OperationType) int {
	if operationType == OperationTypeSell {
		return 1
	}
	return 0
}

func sortOperationsForHistory(operations []Operation) {
	sort.Slice(operations, func(i, j int) bool {
		left := operations[i]
		right := operations[j]
		if sameOperationDate(left.Date, right.Date) {
			leftPriority := operationHistoryPriority(left.OperationType)
			rightPriority := operationHistoryPriority(right.OperationType)
			if leftPriority != rightPriority {
				return leftPriority < rightPriority
			}
			if !left.CreatedAt.Equal(right.CreatedAt) {
				return left.CreatedAt.Before(right.CreatedAt)
			}
			return left.ID.String() < right.ID.String()
		}
		return left.Date.Before(right.Date)
	})
}

func sortPreviewTimelineOperations(timeline []previewTimelineOperation) {
	sort.Slice(timeline, func(i, j int) bool {
		left := timeline[i]
		right := timeline[j]
		if sameOperationDate(left.date, right.date) {
			leftPriority := operationHistoryPriority(left.operationType)
			rightPriority := operationHistoryPriority(right.operationType)
			if leftPriority != rightPriority {
				return leftPriority < rightPriority
			}
			if left.createdAt.Equal(right.createdAt) {
				return left.id < right.id
			}
			if left.createdAt.IsZero() != right.createdAt.IsZero() {
				return !left.createdAt.IsZero()
			}
			return left.createdAt.Before(right.createdAt)
		}
		return left.date.Before(right.date)
	})
}

func cashMovementGroupKeyForRow(row OperationRow) string {
	accountKey := "none"
	if row.BrokerageAccountCode != nil && strings.TrimSpace(*row.BrokerageAccountCode) != "" {
		accountKey = strings.ToLower(strings.TrimSpace(*row.BrokerageAccountCode))
	}
	investmentAccountKey := "none"
	if row.InvestmentAccountCode != nil && strings.TrimSpace(*row.InvestmentAccountCode) != "" {
		investmentAccountKey = strings.ToLower(strings.TrimSpace(*row.InvestmentAccountCode))
	}
	return strings.Join([]string{
		row.Date.UTC().Format("2006-01-02"),
		accountKey,
		investmentAccountKey,
		strings.ToUpper(strings.TrimSpace(row.AssetCode)),
		string(row.OperationType),
	}, "|")
}

func annotateOperationCashMovementGroups(rows []OperationRow) {
	type aggregate struct {
		size     int
		quantity int64
		gross    int64
		net      int64
	}

	aggregates := make(map[string]aggregate, len(rows))
	for _, row := range rows {
		key := cashMovementGroupKeyForRow(row)
		current := aggregates[key]
		current.size++
		current.quantity += row.Quantity
		current.gross += row.GrossAmount
		current.net += row.NetAmount
		aggregates[key] = current
	}

	for index := range rows {
		key := cashMovementGroupKeyForRow(rows[index])
		current := aggregates[key]
		rows[index].CashMovementGroupKey = key
		rows[index].CashMovementGroupSize = current.size
		rows[index].CashMovementGroupQty = current.quantity
		rows[index].CashMovementGroupGross = current.gross
		rows[index].CashMovementGroupNet = current.net
	}
}

func cashMovementGroupKeyForOperation(operation *Operation, assetCode string) string {
	accountKey := "none"
	if operation != nil && operation.BrokerageAccountID != nil {
		accountKey = operation.BrokerageAccountID.String()
	}
	investmentAccountKey := "none"
	if operation != nil && operation.InvestmentAccountID != nil {
		investmentAccountKey = operation.InvestmentAccountID.String()
	}
	return strings.Join([]string{
		operation.Date.UTC().Format("2006-01-02"),
		accountKey,
		investmentAccountKey,
		strings.ToUpper(strings.TrimSpace(assetCode)),
		string(operation.OperationType),
	}, "|")
}

func summarizeOperationCashMovementGroup(operations []Operation, assetCode string) mirrorCashMovementSummary {
	summary := mirrorCashMovementSummary{
		OperationIDs: make([]uuid.UUID, 0, len(operations)),
		AssetCode:    strings.ToUpper(strings.TrimSpace(assetCode)),
	}
	if len(operations) == 0 {
		return summary
	}

	summary.OperationType = operations[0].OperationType
	summary.Date = operations[0].Date
	summary.BrokerageAccountID = operations[0].BrokerageAccountID
	summary.InvestmentAccountID = operations[0].InvestmentAccountID
	for _, operation := range operations {
		summary.OperationIDs = append(summary.OperationIDs, operation.ID)
		summary.Quantity += operation.Quantity
		summary.NetAmount += operation.NetAmount
	}
	return summary
}

func (s *service) buildMirrorCashMovementSummary(tx *gorm.DB, userID uuid.UUID, operations []Operation, assetCode string) (mirrorCashMovementSummary, error) {
	summary := summarizeOperationCashMovementGroup(operations, assetCode)
	if len(operations) == 0 || summary.OperationType != OperationTypeSell {
		return summary, nil
	}

	assetOperations, err := s.repo.ListAssetOperations(tx, userID, operations[0].AssetID)
	if err != nil {
		return mirrorCashMovementSummary{}, err
	}
	sortOperationsForHistory(assetOperations)

	groupOperationIDs := make(map[uuid.UUID]struct{}, len(summary.OperationIDs))
	for _, operationID := range summary.OperationIDs {
		groupOperationIDs[operationID] = struct{}{}
	}
	var investmentAccountID *uuid.UUID
	if operations[0].InvestmentAccountID != nil {
		id := *operations[0].InvestmentAccountID
		investmentAccountID = &id
	}

	var currentQuantity int64
	var totalCostBasis int64
	for _, operation := range assetOperations {
		if investmentAccountID != nil {
			if operation.InvestmentAccountID == nil || *operation.InvestmentAccountID != *investmentAccountID {
				continue
			}
		}
		switch operation.OperationType {
		case OperationTypeBuy, OperationTypeBonification:
			currentQuantity += operation.Quantity
			totalCostBasis += operation.NetAmount
		case OperationTypeAmortization:
			if operation.NetAmount > totalCostBasis {
				return mirrorCashMovementSummary{}, appErr.ErrInvalidInputWithCode(
					"investment.operation.amortization.exceeds.position",
					"amortization exceeds available cost basis for asset history",
					nil,
				)
			}
			totalCostBasis -= operation.NetAmount
			if totalCostBasis < 0 {
				totalCostBasis = 0
			}
		case OperationTypeSell:
			if operation.Quantity > currentQuantity {
				return mirrorCashMovementSummary{}, appErr.ErrInvalidInputWithCode(
					"investment.operation.sell.exceeds.position",
					"sell operation exceeds available quantity for asset history",
					nil,
				)
			}
			sellCostBasis := divideRounded(totalCostBasis*operation.Quantity, currentQuantity)
			if _, ok := groupOperationIDs[operation.ID]; ok {
				summary.ReleasedCostBasis += sellCostBasis
				summary.RealizedPNL += operation.NetAmount - sellCostBasis
			}
			currentQuantity -= operation.Quantity
			totalCostBasis -= sellCostBasis
			if currentQuantity == 0 {
				totalCostBasis = 0
			}
		}
	}

	return summary, nil
}

func feeGroupKey(date time.Time, brokerageAccountID *uuid.UUID) string {
	accountKey := "none"
	if brokerageAccountID != nil {
		accountKey = brokerageAccountID.String()
	}
	return date.UTC().Format("2006-01-02") + "|" + accountKey
}

type feeGroupReference struct {
	Date               time.Time
	BrokerageAccountID *uuid.UUID
}

func feeGroupReferencesFromPreparedOperations(groups map[string]preparedBulkOperation) []feeGroupReference {
	refs := make([]feeGroupReference, 0, len(groups))
	for _, item := range groups {
		refs = append(refs, feeGroupReference{
			Date:               item.date,
			BrokerageAccountID: &item.brokerageAccount.ID,
		})
	}
	return refs
}

func uniqueFeeGroupReferences(groups []feeGroupReference) []feeGroupReference {
	unique := make([]feeGroupReference, 0, len(groups))
	seen := make(map[string]struct{}, len(groups))
	for _, group := range groups {
		key := feeGroupKey(group.Date, group.BrokerageAccountID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, group)
	}
	return unique
}

func (s *service) collectAssetIDsForFeeGroups(tx *gorm.DB, userID uuid.UUID, groups []feeGroupReference) ([]uuid.UUID, error) {
	assetIDs := make([]uuid.UUID, 0, len(groups))
	seenGroups := make(map[string]struct{}, len(groups))
	seenAssets := make(map[uuid.UUID]struct{})
	for _, group := range groups {
		groupKey := feeGroupKey(group.Date, group.BrokerageAccountID)
		if _, ok := seenGroups[groupKey]; ok {
			continue
		}
		seenGroups[groupKey] = struct{}{}

		operations, err := s.repo.ListOperationsByDate(tx, userID, group.Date)
		if err != nil {
			return nil, err
		}
		for _, operation := range operations {
			if feeGroupKey(operation.Date, operation.BrokerageAccountID) != groupKey {
				continue
			}
			if _, ok := seenAssets[operation.AssetID]; ok {
				continue
			}
			seenAssets[operation.AssetID] = struct{}{}
			assetIDs = append(assetIDs, operation.AssetID)
		}
	}
	return assetIDs, nil
}

func (s *service) syncMirroredTransactionsForAssets(tx *gorm.DB, userID uuid.UUID, assetIDs []uuid.UUID) error {
	for _, assetID := range uniqueUUIDs(assetIDs) {
		operations, err := s.repo.ListAssetOperations(tx, userID, assetID)
		if err != nil {
			return err
		}
		if len(operations) == 0 {
			continue
		}
		assetCode, err := resolveAssetCodeByID(tx, userID, assetID)
		if err != nil {
			return err
		}
		seenGroups := make(map[string]struct{}, len(operations))
		for _, operation := range operations {
			groupKey := cashMovementGroupKeyForOperation(&operation, assetCode)
			if _, ok := seenGroups[groupKey]; ok {
				continue
			}
			seenGroups[groupKey] = struct{}{}
			if err := s.syncMirroredTransactionsForOperation(tx, userID, &operation); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *service) resolveOperationCashMovementGroupSize(tx *gorm.DB, userID uuid.UUID, operation *Operation, assetCode string) (int, error) {
	groupOperations, err := s.listOperationsInCashMovementGroup(tx, userID, operation, assetCode)
	if err != nil {
		return 0, err
	}
	return len(groupOperations), nil
}

func (s *service) listOperationsInCashMovementGroup(tx *gorm.DB, userID uuid.UUID, operation *Operation, assetCode string) ([]Operation, error) {
	operations, err := s.repo.ListOperationsByDate(tx, userID, operation.Date)
	if err != nil {
		return nil, err
	}

	targetKey := cashMovementGroupKeyForOperation(operation, assetCode)
	groupOperations := make([]Operation, 0, len(operations))
	for _, candidate := range operations {
		candidateAssetCode, err := resolveAssetCodeByID(tx, userID, candidate.AssetID)
		if err != nil {
			return nil, err
		}
		if cashMovementGroupKeyForOperation(&candidate, candidateAssetCode) == targetKey {
			groupOperations = append(groupOperations, candidate)
		}
	}
	return groupOperations, nil
}

func (s *service) totalFeeAmountForExistingDate(tx *gorm.DB, userID uuid.UUID, date time.Time, brokerageAccountID *uuid.UUID) (int64, error) {
	operations, err := s.repo.ListOperationsByDate(tx, userID, date)
	if err != nil {
		return 0, err
	}
	targetGroup := feeGroupKey(date, brokerageAccountID)
	for _, operation := range operations {
		if operation.OperationType == OperationTypeBonification {
			continue
		}
		if feeGroupKey(operation.Date, operation.BrokerageAccountID) != targetGroup {
			continue
		}
		return operation.OriginalTotalFeeAmount, nil
	}
	return 0, nil
}

func (s *service) reallocateOperationFeesByDate(tx *gorm.DB, userID uuid.UUID, date time.Time, brokerageAccountID *uuid.UUID, totalFeeAmount int64) error {
	operations, err := s.repo.ListOperationsByDate(tx, userID, date)
	if err != nil {
		return err
	}

	reallocatable := make([]preparedBulkOperation, 0, len(operations))
	for _, operation := range operations {
		if operation.OperationType == OperationTypeBonification {
			continue
		}
		if feeGroupKey(operation.Date, operation.BrokerageAccountID) != feeGroupKey(date, brokerageAccountID) {
			continue
		}
		reallocatable = append(reallocatable, preparedBulkOperation{
			date:           operation.Date,
			operationType:  operation.OperationType,
			quantity:       operation.Quantity,
			unitPrice:      operation.UnitPrice,
			totalFeeAmount: totalFeeAmount,
			grossAmount:    operation.GrossAmount,
		})
	}

	if len(reallocatable) > 0 {
		totalGross := int64(0)
		for _, item := range reallocatable {
			totalGross += item.grossAmount
		}
		remaining := totalFeeAmount
		for index := range reallocatable {
			if index == len(reallocatable)-1 {
				reallocatable[index].allocatedFee = remaining
				break
			}
			allocated := int64(0)
			if totalGross > 0 && totalFeeAmount > 0 {
				allocated = divideRounded(totalFeeAmount*reallocatable[index].grossAmount, totalGross)
			}
			if allocated > remaining {
				allocated = remaining
			}
			reallocatable[index].allocatedFee = allocated
			remaining -= allocated
		}
	}

	reallocIndex := 0
	affectedAssets := make([]uuid.UUID, 0, len(operations))
	for _, operation := range operations {
		affectedAssets = append(affectedAssets, operation.AssetID)
		if feeGroupKey(operation.Date, operation.BrokerageAccountID) != feeGroupKey(date, brokerageAccountID) {
			continue
		}
		if operation.OperationType == OperationTypeBonification {
			if operation.OriginalTotalFeeAmount != 0 || operation.FeeAmount != 0 {
				zero := int64(0)
				netAmount := computeNetAmount(operation.OperationType, operation.GrossAmount, 0)
				if _, err := s.repo.UpdateOperation(tx, userID, operation.ID, &UpdateOperationModel{
					FeeAmount:              &zero,
					OriginalTotalFeeAmount: &zero,
					NetAmount:              &netAmount,
				}); err != nil {
					return err
				}
			}
			continue
		}

		allocatedFee := reallocatable[reallocIndex].allocatedFee
		netAmount := computeNetAmount(operation.OperationType, operation.GrossAmount, allocatedFee)
		if _, err := s.repo.UpdateOperation(tx, userID, operation.ID, &UpdateOperationModel{
			FeeAmount:              &allocatedFee,
			OriginalTotalFeeAmount: &totalFeeAmount,
			NetAmount:              &netAmount,
		}); err != nil {
			return err
		}
		operation.FeeAmount = allocatedFee
		operation.OriginalTotalFeeAmount = totalFeeAmount
		operation.NetAmount = netAmount
		if err := s.syncMirroredTransactionsForOperation(tx, userID, &operation); err != nil {
			return err
		}
		reallocIndex++
	}

	for _, assetID := range uniqueUUIDs(affectedAssets) {
		if err := s.rebuildPosition(tx, userID, assetID); err != nil {
			return err
		}
	}
	return nil
}

type dbAccountRow struct {
	ID        uuid.UUID                `gorm:"column:id"`
	Code      string                   `gorm:"column:code"`
	AssetRole account.AccountAssetRole `gorm:"column:asset_role"`
}

func resolveActiveLeafCategoryID(tx *gorm.DB, userID uuid.UUID, code string) (uuid.UUID, error) {
	var row struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	err := tx.Table("categories").
		Select("id").
		Where("user_id = ? AND code = ? AND type = ? AND deactivated_at IS NULL AND parent_id IS NOT NULL", userID, strings.ToLower(code), "MOVEMENT").
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, appErr.ErrInvalidInputWithMessage("required investment movement category not found", nil)
		}
		return uuid.Nil, err
	}
	return row.ID, nil
}

func resolveActiveAccountByCode(tx *gorm.DB, userID uuid.UUID, code string) (*dbAccountRow, error) {
	var row dbAccountRow
	err := tx.Table("accounts").
		Select("id, code, asset_role").
		Where("user_id = ? AND code = ? AND deactivated_at IS NULL", userID, strings.ToLower(strings.TrimSpace(code))).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, appErr.ErrAccountNotFound()
		}
		return nil, err
	}
	return &row, nil
}

func resolveActiveBrokerageAccountByCode(tx *gorm.DB, userID uuid.UUID, code string) (*dbAccountRow, error) {
	row, err := resolveActiveAccountByCode(tx, userID, code)
	if err != nil {
		return nil, err
	}
	if row.AssetRole != account.AccountAssetRoleBrokerage {
		return nil, appErr.ErrInvalidInputWithMessage("investment operations require an active investment account", nil)
	}
	return row, nil
}

func resolveActiveInvestmentAccountByCode(tx *gorm.DB, userID uuid.UUID, code string) (*dbAccountRow, error) {
	row, err := resolveActiveAccountByCode(tx, userID, code)
	if err != nil {
		return nil, err
	}
	if row.AssetRole != account.AccountAssetRoleInvestment {
		return nil, appErr.ErrInvalidInputWithMessage("investment operations require an active investment account bucket", nil)
	}
	return row, nil
}

func loadBrokerageAccountsByCode(tx *gorm.DB, userID uuid.UUID, accountsByCode map[string]*dbAccountRow) error {
	for code, row := range accountsByCode {
		resolved, err := resolveActiveBrokerageAccountByCode(tx, userID, code)
		if err != nil {
			return err
		}
		row.ID = resolved.ID
		row.Code = resolved.Code
		row.AssetRole = resolved.AssetRole
	}
	return nil
}

func loadInvestmentAccountsByCode(tx *gorm.DB, userID uuid.UUID, accountsByCode map[string]*dbAccountRow) error {
	for code, row := range accountsByCode {
		resolved, err := resolveActiveInvestmentAccountByCode(tx, userID, code)
		if err != nil {
			return err
		}
		row.ID = resolved.ID
		row.Code = resolved.Code
		row.AssetRole = resolved.AssetRole
	}
	return nil
}

func normalizeAccountCode(code string) string {
	return strings.ToLower(strings.TrimSpace(code))
}

func resolveAssetCodeByID(tx *gorm.DB, userID, assetID uuid.UUID) (string, error) {
	type assetRow struct {
		Code string `gorm:"column:code"`
	}
	var row assetRow
	err := tx.Table("investment_assets").
		Select("code").
		Where("user_id = ? AND id = ?", userID, assetID).
		First(&row).Error
	if err != nil {
		return "", err
	}
	return row.Code, nil
}

func loadOperationByID(tx *gorm.DB, userID, operationID uuid.UUID) (*Operation, error) {
	var operation Operation
	if err := tx.Where("user_id = ? AND id = ?", userID, operationID).First(&operation).Error; err != nil {
		return nil, err
	}
	return &operation, nil
}

func loadTransactionByID(tx *gorm.DB, userID, transactionID uuid.UUID) (*transaction.Transaction, error) {
	var row transaction.Transaction
	err := tx.Where("user_id = ? AND id = ?", userID, transactionID).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

func deleteTransactionsByIDs(tx *gorm.DB, userID uuid.UUID, transactionIDs []uuid.UUID) error {
	if len(transactionIDs) == 0 {
		return nil
	}
	return tx.Where("user_id = ? AND id IN ?", userID, transactionIDs).Delete(&transaction.Transaction{}).Error
}

func resolveAccountCodesByIDs(tx *gorm.DB, userID uuid.UUID, accountIDs []uuid.UUID) ([]string, error) {
	uniqueIDs := uniqueUUIDs(accountIDs)
	if len(uniqueIDs) == 0 {
		return []string{}, nil
	}

	rows := make([]dbAccountRow, 0, len(uniqueIDs))
	if err := tx.Table("accounts").
		Select("id, code").
		Where("user_id = ? AND id IN ?", userID, uniqueIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	codes := make([]string, 0, len(rows))
	for _, row := range rows {
		codes = append(codes, row.Code)
	}
	return codes, nil
}

func loadTransferPairByVisibleTransactionID(tx *gorm.DB, userID, transactionID uuid.UUID) (*transaction.Transaction, *transaction.Transaction, error) {
	visibleTx, err := loadTransactionByID(tx, userID, transactionID)
	if err != nil {
		return nil, nil, err
	}
	if visibleTx.TransferID == nil {
		return nil, nil, appErr.ErrInvalidInputWithCode("investment.operation.mirror.requires_transfer", "selected transaction must be a transfer", nil)
	}
	if visibleTx.IsVisible != nil && !*visibleTx.IsVisible {
		return nil, nil, appErr.ErrInvalidInputWithCode("investment.operation.mirror.requires_visible_transfer", "selected transaction must be the visible transfer row", nil)
	}

	var pair []transaction.Transaction
	if err := tx.Where("user_id = ? AND transfer_id = ?", userID, *visibleTx.TransferID).Order("created_at ASC").Find(&pair).Error; err != nil {
		return nil, nil, err
	}
	if len(pair) != 2 {
		return nil, nil, appErr.ErrInvalidInputWithCode("transaction.transfer.invalid_pair", "linked transfer pair is inconsistent", nil)
	}

	first := pair[0]
	second := pair[1]
	if first.IsVisible != nil && !*first.IsVisible {
		first, second = second, first
	}
	if first.IsVisible != nil && !*first.IsVisible {
		return nil, nil, appErr.ErrInvalidInputWithCode("investment.operation.mirror.requires_visible_transfer", "selected transaction must be the visible transfer row", nil)
	}
	return &first, &second, nil
}

func ensureTransactionsAreUnlinked(tx *gorm.DB, userID uuid.UUID, transactionIDs []uuid.UUID) error {
	var count int64
	if err := tx.Table("investment_operation_transaction_links").
		Where("user_id = ? AND transaction_id IN ?", userID, uniqueUUIDs(transactionIDs)).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return appErr.ErrInvalidInputWithCode("investment.operation.mirror.transaction_already_linked", "selected transaction is already linked to another investment operation", nil)
	}
	return nil
}

func resolveActiveLeafCategoryIDByIDAndType(tx *gorm.DB, userID uuid.UUID, categoryID uuid.UUID, categoryType string) (uuid.UUID, error) {
	var row struct {
		ID uuid.UUID `gorm:"column:id"`
	}
	err := tx.Table("categories").
		Select("id").
		Where(
			"user_id = ? AND id = ? AND type = ? AND deactivated_at IS NULL AND parent_id IS NOT NULL",
			userID,
			categoryID,
			categoryType,
		).
		First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, appErr.ErrInvalidInputWithCode(
				"investment.operation.mirror.sell_categories_required",
				"sell mirror categories must be configured with active leaf categories",
				nil,
			)
		}
		return uuid.Nil, err
	}
	return row.ID, nil
}

func (s *service) resolveSellMirrorCategoryIDs(tx *gorm.DB, userID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	if s.userConfigService == nil {
		return uuid.Nil, uuid.Nil, appErr.ErrInvalidInputWithCode(
			"investment.operation.mirror.sell_categories_required",
			"sell mirror categories are required",
			nil,
		)
	}

	config, err := s.userConfigService.GetConfig(userID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	if config == nil ||
		config.Settings.Investments.Integration.SellGainCategoryID == nil ||
		config.Settings.Investments.Integration.SellLossCategoryID == nil {
		return uuid.Nil, uuid.Nil, appErr.ErrInvalidInputWithCode(
			"investment.operation.mirror.sell_categories_required",
			"sell mirror categories are required",
			nil,
		)
	}

	gainCategoryID, err := resolveActiveLeafCategoryIDByIDAndType(tx, userID, *config.Settings.Investments.Integration.SellGainCategoryID, "INCOME")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	lossCategoryID, err := resolveActiveLeafCategoryIDByIDAndType(tx, userID, *config.Settings.Investments.Integration.SellLossCategoryID, "EXPENSE")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return gainCategoryID, lossCategoryID, nil
}

func (s *service) resolveBonificationIncomeCategoryID(tx *gorm.DB, userID uuid.UUID) (uuid.UUID, error) {
	if s.userConfigService == nil {
		return uuid.Nil, appErr.ErrInvalidInputWithCode(
			"investment.operation.mirror.bonification_income_category_required",
			"bonification income category is required",
			nil,
		)
	}

	config, err := s.userConfigService.GetConfig(userID)
	if err != nil {
		return uuid.Nil, err
	}
	if config == nil || config.Settings.Investments.Integration.BonificationIncomeCategoryID == nil {
		return uuid.Nil, appErr.ErrInvalidInputWithCode(
			"investment.operation.mirror.bonification_income_category_required",
			"bonification income category is required",
			nil,
		)
	}

	return resolveActiveLeafCategoryIDByIDAndType(tx, userID, *config.Settings.Investments.Integration.BonificationIncomeCategoryID, "INCOME")
}

func listOperationTransactionLinksForOperations(tx *gorm.DB, userID uuid.UUID, operationIDs []uuid.UUID) ([]OperationTransactionLink, error) {
	if len(operationIDs) == 0 {
		return []OperationTransactionLink{}, nil
	}

	var links []OperationTransactionLink
	if err := tx.Where("user_id = ? AND investment_operation_id IN ?", userID, uniqueUUIDs(operationIDs)).
		Find(&links).Error; err != nil {
		return nil, err
	}
	return links, nil
}

func operationIDsFromOperations(operations []Operation) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(operations))
	for _, operation := range operations {
		ids = append(ids, operation.ID)
	}
	return ids
}

func createOperationTransactionLinksForGroup(userID uuid.UUID, operations []Operation, transactionsByRole map[OperationTransactionLinkRole]*transaction.Transaction) []*OperationTransactionLink {
	if len(transactionsByRole) == 0 {
		return []*OperationTransactionLink{}
	}
	links := make([]*OperationTransactionLink, 0, len(operations)*len(transactionsByRole))
	for _, operation := range operations {
		for role, row := range transactionsByRole {
			if row == nil {
				continue
			}
			links = append(links, &OperationTransactionLink{
				ID:                    uuid.New(),
				UserID:                userID,
				InvestmentOperationID: operation.ID,
				TransactionID:         row.ID,
				Role:                  role,
			})
		}
	}
	return links
}

func mirrorTransferAmount(summary mirrorCashMovementSummary) int64 {
	if summary.OperationType == OperationTypeSell {
		return summary.ReleasedCostBasis
	}
	return summary.NetAmount
}

func defaultMirrorSourceAccountCode(summary mirrorCashMovementSummary, brokerageAccountCode string, investmentAccountCode string) string {
	if summary.OperationType == OperationTypeSell || summary.OperationType == OperationTypeAmortization {
		return investmentAccountCode
	}
	return brokerageAccountCode
}

func defaultMirrorDestinationAccountCode(summary mirrorCashMovementSummary, brokerageAccountCode string, investmentAccountCode string) string {
	if summary.OperationType == OperationTypeSell || summary.OperationType == OperationTypeAmortization {
		return brokerageAccountCode
	}
	return investmentAccountCode
}

func createMirrorTransferPair(tx *gorm.DB, userID, categoryID uuid.UUID, summary mirrorCashMovementSummary, draft MirroredTransactionDraftRequest) (*transaction.Transaction, *transaction.Transaction, error) {
	sourceAccount, err := resolveActiveAccountByCode(tx, userID, draft.SourceAccountCode)
	if err != nil {
		return nil, nil, err
	}
	destinationAccount, err := resolveActiveAccountByCode(tx, userID, draft.DestinationAccountCode)
	if err != nil {
		return nil, nil, err
	}
	if sourceAccount.ID == destinationAccount.ID {
		return nil, nil, appErr.ErrInvalidInputWithMessage("account can't be the same as account transfer", nil)
	}

	transferID, err := nextTransferID(tx)
	if err != nil {
		return nil, nil, err
	}

	description := mirrorTransactionDescription(summary.OperationType, summary.Quantity, summary.AssetCode)
	amount := mirrorTransferAmount(summary)
	negativeAmount := -amount
	visible := &transaction.Transaction{
		ID:                   uuid.New(),
		UserID:               userID,
		CategoryID:           categoryID,
		Description:          description,
		Date:                 summary.Date,
		AccountID:            sourceAccount.ID,
		Amount:               negativeAmount,
		TransferID:           &transferID,
		TransferAccountID:    &destinationAccount.ID,
		IsVisible:            transaction.BoolPtr(true),
		ExcludeFromDashboard: false,
	}
	hidden := &transaction.Transaction{
		ID:                   uuid.New(),
		UserID:               userID,
		CategoryID:           categoryID,
		Description:          description,
		Date:                 summary.Date,
		AccountID:            destinationAccount.ID,
		Amount:               amount,
		TransferID:           &transferID,
		TransferAccountID:    &sourceAccount.ID,
		IsVisible:            transaction.BoolPtr(false),
		ExcludeFromDashboard: false,
	}
	if err := tx.Create(visible).Error; err != nil {
		return nil, nil, err
	}
	if err := tx.Create(hidden).Error; err != nil {
		return nil, nil, err
	}
	return visible, hidden, nil
}

func createRealizedPNLTransaction(
	tx *gorm.DB,
	userID uuid.UUID,
	gainCategoryID uuid.UUID,
	lossCategoryID uuid.UUID,
	summary mirrorCashMovementSummary,
) (*transaction.Transaction, error) {
	if summary.BrokerageAccountID == nil || *summary.BrokerageAccountID == uuid.Nil {
		return nil, appErr.ErrInvalidInputWithMessage("brokerage account is required for mirrored sell pnl", nil)
	}

	categoryID := gainCategoryID
	if summary.RealizedPNL < 0 {
		categoryID = lossCategoryID
	}

	row := &transaction.Transaction{
		ID:                   uuid.New(),
		UserID:               userID,
		CategoryID:           categoryID,
		Description:          realizedPNLTransactionDescription(summary.Quantity, summary.AssetCode),
		Date:                 summary.Date,
		AccountID:            *summary.BrokerageAccountID,
		Amount:               summary.RealizedPNL,
		IsVisible:            transaction.BoolPtr(true),
		ExcludeFromDashboard: false,
	}
	if err := tx.Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func createBonificationIncomeTransaction(
	tx *gorm.DB,
	userID uuid.UUID,
	categoryID uuid.UUID,
	summary mirrorCashMovementSummary,
) (*transaction.Transaction, error) {
	if summary.BrokerageAccountID == nil || *summary.BrokerageAccountID == uuid.Nil {
		return nil, appErr.ErrInvalidInputWithMessage("brokerage account is required for mirrored bonification income", nil)
	}

	row := &transaction.Transaction{
		ID:                   uuid.New(),
		UserID:               userID,
		CategoryID:           categoryID,
		Description:          bonificationIncomeTransactionDescription(summary.Quantity, summary.AssetCode),
		Date:                 summary.Date,
		AccountID:            *summary.BrokerageAccountID,
		Amount:               summary.NetAmount,
		IsVisible:            transaction.BoolPtr(true),
		ExcludeFromDashboard: false,
	}
	if err := tx.Create(row).Error; err != nil {
		return nil, err
	}
	return row, nil
}

func attachExistingRealizedPNLTransaction(
	tx *gorm.DB,
	userID uuid.UUID,
	gainCategoryID uuid.UUID,
	lossCategoryID uuid.UUID,
	summary mirrorCashMovementSummary,
	transactionID uuid.UUID,
) (*transaction.Transaction, error) {
	row, err := loadTransactionByID(tx, userID, transactionID)
	if err != nil {
		return nil, err
	}
	if row.TransferID != nil {
		return nil, appErr.ErrInvalidInputWithCode("investment.operation.mirror.realized_pnl_requires_non_transfer", "selected realized pnl transaction cannot be a transfer", nil)
	}
	if err := ensureTransactionsAreUnlinked(tx, userID, []uuid.UUID{row.ID}); err != nil {
		return nil, err
	}

	categoryID := gainCategoryID
	if summary.RealizedPNL < 0 {
		categoryID = lossCategoryID
	}
	if row.CategoryID != categoryID {
		return nil, appErr.ErrInvalidInputWithCode(
			"investment.operation.mirror.realized_pnl_category_mismatch",
			"selected realized pnl transaction must already use the configured category",
			nil,
		)
	}
	if summary.BrokerageAccountID == nil || *summary.BrokerageAccountID == uuid.Nil {
		return nil, appErr.ErrInvalidInputWithMessage("brokerage account is required for mirrored sell pnl", nil)
	}

	if err := tx.Model(&transaction.Transaction{}).
		Where("user_id = ? AND id = ?", userID, row.ID).
		Updates(map[string]any{
			"category_id": categoryID,
			"description": realizedPNLTransactionDescription(summary.Quantity, summary.AssetCode),
			"date":        summary.Date,
			"account_id":  *summary.BrokerageAccountID,
			"amount":      summary.RealizedPNL,
		}).Error; err != nil {
		return nil, err
	}

	accountIDsToRefresh := accountIDsForRefreshFromTransaction(row)
	accountIDsToRefresh = append(accountIDsToRefresh, *summary.BrokerageAccountID)
	accountCodes, err := resolveAccountCodesByIDs(tx, userID, accountIDsToRefresh)
	if err != nil {
		return nil, err
	}
	if err := refreshAccountBalances(tx, userID, accountCodes); err != nil {
		return nil, err
	}

	row.CategoryID = categoryID
	row.Description = realizedPNLTransactionDescription(summary.Quantity, summary.AssetCode)
	row.Date = summary.Date
	row.AccountID = *summary.BrokerageAccountID
	row.Amount = summary.RealizedPNL
	return row, nil
}

func attachExistingBonificationIncomeTransaction(
	tx *gorm.DB,
	userID uuid.UUID,
	categoryID uuid.UUID,
	summary mirrorCashMovementSummary,
	transactionID uuid.UUID,
) (*transaction.Transaction, error) {
	row, err := loadTransactionByID(tx, userID, transactionID)
	if err != nil {
		return nil, err
	}
	if row.TransferID != nil {
		return nil, appErr.ErrInvalidInputWithCode("investment.operation.mirror.bonification_income_requires_non_transfer", "selected bonification income transaction cannot be a transfer", nil)
	}
	if err := ensureTransactionsAreUnlinked(tx, userID, []uuid.UUID{row.ID}); err != nil {
		return nil, err
	}
	if row.CategoryID != categoryID {
		return nil, appErr.ErrInvalidInputWithCode(
			"investment.operation.mirror.bonification_income_category_mismatch",
			"selected bonification income transaction must already use the configured category",
			nil,
		)
	}
	if summary.BrokerageAccountID == nil || *summary.BrokerageAccountID == uuid.Nil {
		return nil, appErr.ErrInvalidInputWithMessage("brokerage account is required for mirrored bonification income", nil)
	}

	if err := tx.Model(&transaction.Transaction{}).
		Where("user_id = ? AND id = ?", userID, row.ID).
		Updates(map[string]any{
			"category_id": categoryID,
			"description": bonificationIncomeTransactionDescription(summary.Quantity, summary.AssetCode),
			"date":        summary.Date,
			"account_id":  *summary.BrokerageAccountID,
			"amount":      summary.NetAmount,
		}).Error; err != nil {
		return nil, err
	}

	row.CategoryID = categoryID
	row.Description = bonificationIncomeTransactionDescription(summary.Quantity, summary.AssetCode)
	row.Date = summary.Date
	row.AccountID = *summary.BrokerageAccountID
	row.Amount = summary.NetAmount
	return row, nil
}

func attachExistingMirrorTransferPair(tx *gorm.DB, userID, categoryID uuid.UUID, summary mirrorCashMovementSummary, transactionID uuid.UUID) (*transaction.Transaction, *transaction.Transaction, error) {
	visibleTx, hiddenTx, err := loadTransferPairByVisibleTransactionID(tx, userID, transactionID)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureTransactionsAreUnlinked(tx, userID, []uuid.UUID{visibleTx.ID, hiddenTx.ID}); err != nil {
		return nil, nil, err
	}

	description := mirrorTransactionDescription(summary.OperationType, summary.Quantity, summary.AssetCode)
	transferAmount := mirrorTransferAmount(summary)
	if err := tx.Model(&transaction.Transaction{}).
		Where("user_id = ? AND id = ?", userID, visibleTx.ID).
		Updates(map[string]any{
			"category_id": categoryID,
			"description": description,
			"date":        summary.Date,
			"amount":      -transferAmount,
		}).Error; err != nil {
		return nil, nil, err
	}
	if err := tx.Model(&transaction.Transaction{}).
		Where("user_id = ? AND id = ?", userID, hiddenTx.ID).
		Updates(map[string]any{
			"category_id": categoryID,
			"description": description,
			"date":        summary.Date,
			"amount":      transferAmount,
		}).Error; err != nil {
		return nil, nil, err
	}

	accountCodes, err := resolveAccountCodesByIDs(tx, userID, accountIDsForRefreshFromTransaction(visibleTx))
	if err != nil {
		return nil, nil, err
	}
	hiddenAccountCodes, err := resolveAccountCodesByIDs(tx, userID, accountIDsForRefreshFromTransaction(hiddenTx))
	if err != nil {
		return nil, nil, err
	}
	if err := refreshAccountBalances(tx, userID, append(accountCodes, hiddenAccountCodes...)); err != nil {
		return nil, nil, err
	}

	visibleTx.CategoryID = categoryID
	visibleTx.Description = description
	visibleTx.Date = summary.Date
	visibleTx.Amount = -transferAmount
	hiddenTx.CategoryID = categoryID
	hiddenTx.Description = description
	hiddenTx.Date = summary.Date
	hiddenTx.Amount = transferAmount
	return visibleTx, hiddenTx, nil
}

func collectTransactionRefreshMetadata(tx *gorm.DB, userID uuid.UUID, transactionIDs []uuid.UUID) ([]uuid.UUID, error) {
	accountIDsToRefresh := make([]uuid.UUID, 0, len(transactionIDs)*2)
	for _, transactionID := range uniqueUUIDs(transactionIDs) {
		row, err := loadTransactionByID(tx, userID, transactionID)
		if err != nil {
			return nil, err
		}
		accountIDsToRefresh = append(accountIDsToRefresh, accountIDsForRefreshFromTransaction(row)...)
	}
	return accountIDsToRefresh, nil
}

func groupTransactionIDsByRole(links []OperationTransactionLink) map[uuid.UUID]OperationTransactionLinkRole {
	roles := make(map[uuid.UUID]OperationTransactionLinkRole, len(links))
	for _, link := range links {
		if _, ok := roles[link.TransactionID]; !ok {
			roles[link.TransactionID] = link.Role
		}
	}
	return roles
}

func filterOperationsExcludingID(operations []Operation, excludedID uuid.UUID) []Operation {
	filtered := make([]Operation, 0, len(operations))
	for _, operation := range operations {
		if operation.ID == excludedID {
			continue
		}
		filtered = append(filtered, operation)
	}
	return filtered
}

func (s *service) syncMirroredTransactionsForOperations(tx *gorm.DB, userID uuid.UUID, operations []Operation, assetCode string) error {
	if len(operations) == 0 {
		return nil
	}

	links, err := listOperationTransactionLinksForOperations(tx, userID, operationIDsFromOperations(operations))
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}

	if !isMirrorableOperationType(operations[0].OperationType) {
		transactionIDs := make([]uuid.UUID, 0, len(links))
		for _, link := range links {
			transactionIDs = append(transactionIDs, link.TransactionID)
		}
		return s.deleteMirroredTransactionsForOperationIDs(tx, userID, operationIDsFromOperations(operations), uniqueUUIDs(transactionIDs))
	}

	categoryID, err := resolveActiveLeafCategoryID(tx, userID, investmentMirrorCategoryCode)
	if err != nil {
		return err
	}
	summary, err := s.buildMirrorCashMovementSummary(tx, userID, operations, assetCode)
	if err != nil {
		return err
	}
	description := mirrorTransactionDescription(summary.OperationType, summary.Quantity, summary.AssetCode)
	accountIDsToRefresh, err := collectTransactionRefreshMetadata(tx, userID, mapKeysUUIDRole(groupTransactionIDsByRole(links)))
	if err != nil {
		return err
	}
	gainCategoryID := uuid.Nil
	lossCategoryID := uuid.Nil
	if summary.OperationType == OperationTypeSell {
		gainCategoryID, lossCategoryID, err = s.resolveSellMirrorCategoryIDs(tx, userID)
		if err != nil {
			return err
		}
	}

	for transactionID, role := range groupTransactionIDsByRole(links) {
		updates := map[string]any{
			"date": summary.Date,
		}
		switch role {
		case OperationTransactionLinkRoleVisibleTransfer:
			updates["category_id"] = categoryID
			updates["description"] = description
			updates["amount"] = -mirrorTransferAmount(summary)
		case OperationTransactionLinkRoleHiddenTransfer:
			updates["category_id"] = categoryID
			updates["description"] = description
			updates["amount"] = mirrorTransferAmount(summary)
		case OperationTransactionLinkRoleRealizedPNL:
			pnlCategoryID := gainCategoryID
			if summary.RealizedPNL < 0 {
				pnlCategoryID = lossCategoryID
			}
			updates["category_id"] = pnlCategoryID
			updates["description"] = realizedPNLTransactionDescription(summary.Quantity, summary.AssetCode)
			updates["amount"] = summary.RealizedPNL
			if summary.BrokerageAccountID != nil {
				updates["account_id"] = *summary.BrokerageAccountID
			}
		case OperationTransactionLinkRoleBonificationIncome:
			bonificationIncomeCategoryID, categoryErr := s.resolveBonificationIncomeCategoryID(tx, userID)
			if categoryErr != nil {
				return categoryErr
			}
			updates["category_id"] = bonificationIncomeCategoryID
			updates["description"] = bonificationIncomeTransactionDescription(summary.Quantity, summary.AssetCode)
			updates["amount"] = summary.NetAmount
			if summary.BrokerageAccountID != nil {
				updates["account_id"] = *summary.BrokerageAccountID
			}
		default:
			updates["category_id"] = categoryID
			updates["description"] = description
			updates["amount"] = mirrorTransferAmount(summary)
		}
		if err := tx.Model(&transaction.Transaction{}).
			Where("user_id = ? AND id = ?", userID, transactionID).
			Updates(updates).Error; err != nil {
			return err
		}
	}

	accountCodesToRefresh, err := resolveAccountCodesByIDs(tx, userID, accountIDsToRefresh)
	if err != nil {
		return err
	}
	return refreshAccountBalances(tx, userID, accountCodesToRefresh)
}

func mapKeysUUIDRole(values map[uuid.UUID]OperationTransactionLinkRole) []uuid.UUID {
	keys := make([]uuid.UUID, 0, len(values))
	for id := range values {
		keys = append(keys, id)
	}
	return keys
}

func (s *service) syncMirroredTransactionsForOperation(tx *gorm.DB, userID uuid.UUID, operation *Operation) error {
	assetCode, err := resolveAssetCodeByID(tx, userID, operation.AssetID)
	if err != nil {
		return err
	}
	groupOperations, err := s.listOperationsInCashMovementGroup(tx, userID, operation, assetCode)
	if err != nil {
		return err
	}
	return s.syncMirroredTransactionsForOperations(tx, userID, groupOperations, assetCode)
}

func (s *service) deleteMirroredTransactionsForOperationIDs(tx *gorm.DB, userID uuid.UUID, operationIDs []uuid.UUID, transactionIDs []uuid.UUID) error {
	if len(operationIDs) > 0 {
		if err := tx.Where("user_id = ? AND investment_operation_id IN ?", userID, uniqueUUIDs(operationIDs)).
			Delete(&OperationTransactionLink{}).Error; err != nil {
			return err
		}
	}
	if len(transactionIDs) == 0 {
		return nil
	}

	accountIDsToRefresh, err := collectTransactionRefreshMetadata(tx, userID, transactionIDs)
	if err != nil {
		return err
	}
	if err := deleteTransactionsByIDs(tx, userID, uniqueUUIDs(transactionIDs)); err != nil {
		return err
	}
	accountCodesToRefresh, err := resolveAccountCodesByIDs(tx, userID, accountIDsToRefresh)
	if err != nil {
		return err
	}
	return refreshAccountBalances(tx, userID, accountCodesToRefresh)
}

func (s *service) deleteMirroredTransactionsForOperation(tx *gorm.DB, userID, operationID uuid.UUID) error {
	operation, err := s.repo.GetOperationByID(userID, operationID)
	if err != nil {
		return err
	}
	assetCode, err := resolveAssetCodeByID(tx, userID, operation.AssetID)
	if err != nil {
		return err
	}
	groupOperations, err := s.listOperationsInCashMovementGroup(tx, userID, operation, assetCode)
	if err != nil {
		return err
	}
	links, err := s.repo.ListOperationTransactionLinks(tx, userID, operationID)
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}

	transactionIDs := make([]uuid.UUID, 0, len(links))
	for _, link := range links {
		transactionIDs = append(transactionIDs, link.TransactionID)
	}
	if err := s.repo.DeleteOperationTransactionLinks(tx, userID, operationID); err != nil {
		return err
	}

	remainingOperations := filterOperationsExcludingID(groupOperations, operationID)
	remainingLinks, err := listOperationTransactionLinksForOperations(tx, userID, operationIDsFromOperations(remainingOperations))
	if err != nil {
		return err
	}
	if len(remainingLinks) == 0 {
		return s.deleteMirroredTransactionsForOperationIDs(tx, userID, nil, transactionIDs)
	}
	return s.syncMirroredTransactionsForOperations(tx, userID, remainingOperations, assetCode)
}

func accountIDsForRefreshFromTransaction(row *transaction.Transaction) []uuid.UUID {
	out := []uuid.UUID{}
	if row.AccountID != uuid.Nil {
		out = append(out, row.AccountID)
	}
	if row.TransferAccountID != nil && *row.TransferAccountID != uuid.Nil {
		out = append(out, *row.TransferAccountID)
	}
	return out
}

func mirrorTransactionDescription(operationType OperationType, quantity int64, assetCode string) string {
	switch operationType {
	case OperationTypeSell:
		return "VENDA DE " + strconv.FormatInt(quantity, 10) + " " + strings.ToUpper(strings.TrimSpace(assetCode))
	case OperationTypeBonification:
		return "BONIFICAÇÃO DE " + strconv.FormatInt(quantity, 10) + " " + strings.ToUpper(strings.TrimSpace(assetCode))
	case OperationTypeAmortization:
		return "AMORTIZAÇÃO DE " + strconv.FormatInt(quantity, 10) + " " + strings.ToUpper(strings.TrimSpace(assetCode))
	default:
		return "COMPRA DE " + strconv.FormatInt(quantity, 10) + " " + strings.ToUpper(strings.TrimSpace(assetCode))
	}
}

func realizedPNLTransactionDescription(quantity int64, assetCode string) string {
	return "RESULTADO DE VENDA DE " + strconv.FormatInt(quantity, 10) + " " + strings.ToUpper(strings.TrimSpace(assetCode))
}

func bonificationIncomeTransactionDescription(quantity int64, assetCode string) string {
	return "BONIFICAÇÃO DE " + strconv.FormatInt(quantity, 10) + " " + strings.ToUpper(strings.TrimSpace(assetCode))
}

func isMirrorableOperationType(operationType OperationType) bool {
	return operationType == OperationTypeBuy || operationType == OperationTypeSell || operationType == OperationTypeBonification || operationType == OperationTypeAmortization
}

func nextTransferID(tx *gorm.DB) (int64, error) {
	var transferID int64
	if err := tx.Raw("SELECT nextval('transfer_id_seq')").Scan(&transferID).Error; err != nil {
		return 0, err
	}
	return transferID, nil
}

func refreshAccountBalances(tx *gorm.DB, userID uuid.UUID, accountCodes []string) error {
	normalized := make([]string, 0, len(accountCodes))
	seen := make(map[string]struct{}, len(accountCodes))
	for _, code := range accountCodes {
		lower := strings.ToLower(strings.TrimSpace(code))
		if lower == "" {
			continue
		}
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		normalized = append(normalized, lower)
	}

	for _, code := range normalized {
		var account dbAccountRow
		if err := tx.Table("accounts").Select("id, code").Where("user_id = ? AND code = ?", userID, code).First(&account).Error; err != nil {
			return err
		}
		var balance int64
		if err := tx.Table("transactions").Select("COALESCE(SUM(amount), 0)").Where("user_id = ? AND account_id = ?", userID, account.ID).Scan(&balance).Error; err != nil {
			return err
		}
		if err := tx.Table("accounts").Where("user_id = ? AND id = ?", userID, account.ID).Update("balance", balance).Error; err != nil {
			return err
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

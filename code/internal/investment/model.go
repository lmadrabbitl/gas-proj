package investment

import (
	"time"

	"github.com/google/uuid"
)

type AssetType string

const (
	AssetTypeStock AssetType = "STOCK"
	AssetTypeFII   AssetType = "FII"
	AssetTypeETF   AssetType = "ETF"
)

type OperationType string

const (
	OperationTypeBuy          OperationType = "BUY"
	OperationTypeSell         OperationType = "SELL"
	OperationTypeBonification OperationType = "BONIFICATION"
)

type OperationTransactionLinkRole string

const (
	OperationTransactionLinkRoleVisibleTransfer OperationTransactionLinkRole = "VISIBLE_TRANSFER"
	OperationTransactionLinkRoleHiddenTransfer  OperationTransactionLinkRole = "HIDDEN_TRANSFER"
)

type Asset struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	UserID            uuid.UUID  `gorm:"type:uuid;not null" json:"user_id"`
	Code              string     `gorm:"type:varchar(20);not null" json:"code"`
	Name              string     `gorm:"type:text;not null" json:"name"`
	CNPJ              *string    `gorm:"column:cnpj;type:varchar(14)" json:"cnpj"`
	AssetType         AssetType  `gorm:"column:asset_type;type:text;not null" json:"asset_type"`
	MetadataSource    *string    `gorm:"column:metadata_source;type:text" json:"metadata_source"`
	MetadataUpdatedAt *time.Time `gorm:"column:metadata_updated_at;type:timestamptz" json:"metadata_updated_at"`
	IsActive          bool       `gorm:"type:boolean;not null;default:true" json:"is_active"`
	CreatedAt         time.Time  `gorm:"type:timestamptz;not null" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"type:timestamptz;not null" json:"updated_at"`
}

func (Asset) TableName() string {
	return "investment_assets"
}

type Operation struct {
	ID                     uuid.UUID     `gorm:"type:uuid;primaryKey"`
	UserID                 uuid.UUID     `gorm:"type:uuid;not null"`
	AssetID                uuid.UUID     `gorm:"type:uuid;not null"`
	BrokerageAccountID     *uuid.UUID    `gorm:"column:brokerage_account_id;type:uuid"`
	InvestmentAccountID    *uuid.UUID    `gorm:"column:investment_account_id;type:uuid"`
	OperationType          OperationType `gorm:"column:operation_type;type:text;not null"`
	Date                   time.Time     `gorm:"type:date;not null"`
	Quantity               int64         `gorm:"type:bigint;not null"`
	UnitPrice              int64         `gorm:"column:unit_price;type:bigint;not null"`
	FeeAmount              int64         `gorm:"column:fee_amount;type:bigint;not null"`
	OriginalTotalFeeAmount int64         `gorm:"column:original_total_fee_amount;type:bigint;not null;default:0"`
	GrossAmount            int64         `gorm:"column:gross_amount;type:bigint;not null"`
	NetAmount              int64         `gorm:"column:net_amount;type:bigint;not null"`
	Notes                  string        `gorm:"type:text;not null"`
	CreatedAt              time.Time     `gorm:"type:timestamptz;not null"`
	UpdatedAt              time.Time     `gorm:"type:timestamptz;not null"`
}

func (Operation) TableName() string {
	return "investment_operations"
}

type OperationTransactionLink struct {
	ID                    uuid.UUID                    `gorm:"type:uuid;primaryKey"`
	UserID                uuid.UUID                    `gorm:"type:uuid;not null"`
	InvestmentOperationID uuid.UUID                    `gorm:"column:investment_operation_id;type:uuid;not null"`
	TransactionID         uuid.UUID                    `gorm:"column:transaction_id;type:uuid;not null"`
	Role                  OperationTransactionLinkRole `gorm:"type:text;not null"`
	CreatedAt             time.Time                    `gorm:"type:timestamptz;not null"`
	UpdatedAt             time.Time                    `gorm:"type:timestamptz;not null"`
}

func (OperationTransactionLink) TableName() string {
	return "investment_operation_transaction_links"
}

type Position struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID             uuid.UUID `gorm:"type:uuid;not null"`
	AssetID            uuid.UUID `gorm:"type:uuid;not null"`
	CurrentQuantity    int64     `gorm:"column:current_quantity;type:bigint;not null"`
	AveragePrice       int64     `gorm:"column:average_price;type:bigint;not null"`
	TotalCostBasis     int64     `gorm:"column:total_cost_basis;type:bigint;not null"`
	RealizedPNL        int64     `gorm:"column:realized_pnl;type:bigint;not null"`
	LastRecalculatedAt time.Time `gorm:"column:last_recalculated_at;type:timestamptz;not null"`
	CreatedAt          time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt          time.Time `gorm:"type:timestamptz;not null"`
}

func (Position) TableName() string {
	return "investment_positions"
}

type AssetQuoteCache struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	AssetCode      string    `gorm:"column:asset_code;type:varchar(20);not null"`
	CurrentPrice   int64     `gorm:"column:current_price;type:bigint;not null"`
	QuoteUpdatedAt time.Time `gorm:"column:quote_updated_at;type:timestamptz;not null"`
	Source         string    `gorm:"type:text;not null"`
	FetchedAt      time.Time `gorm:"column:fetched_at;type:timestamptz;not null"`
	CreatedAt      time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt      time.Time `gorm:"type:timestamptz;not null"`
}

func (AssetQuoteCache) TableName() string {
	return "investment_asset_quotes"
}

type Portfolio struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null" json:"user_id"`
	Code        string    `gorm:"type:varchar(50);not null" json:"code"`
	Name        string    `gorm:"type:text;not null" json:"name"`
	Description string    `gorm:"type:text;not null" json:"description"`
	SortOrder   int       `gorm:"column:sort_order;type:integer;not null" json:"sort_order"`
	CreatedAt   time.Time `gorm:"type:timestamptz;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"type:timestamptz;not null" json:"updated_at"`
}

func (Portfolio) TableName() string {
	return "investment_portfolios"
}

type PortfolioAsset struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID              uuid.UUID `gorm:"type:uuid;not null"`
	PortfolioID         uuid.UUID `gorm:"column:portfolio_id;type:uuid;not null"`
	AssetID             uuid.UUID `gorm:"column:asset_id;type:uuid;not null"`
	TargetAllocationBPS int       `gorm:"column:target_allocation_bps;type:integer;not null"`
	MaxBuyPrice         *int64    `gorm:"column:max_buy_price;type:bigint"`
	SortOrder           int       `gorm:"column:sort_order;type:integer;not null"`
	CreatedAt           time.Time `gorm:"type:timestamptz;not null"`
	UpdatedAt           time.Time `gorm:"type:timestamptz;not null"`
}

func (PortfolioAsset) TableName() string {
	return "investment_portfolio_assets"
}

type PortfolioAssetRow struct {
	PortfolioCode        string
	PortfolioName        string
	PortfolioDescription string
	PortfolioSortOrder   int
	AssetCode            string
	AssetName            string
	AssetType            AssetType
	TargetAllocationBPS  int
	MaxBuyPrice          *int64
	SortOrder            int
}

type OperationRow struct {
	ID                     uuid.UUID     `json:"id" gorm:"column:id"`
	AssetCode              string        `json:"asset_code" gorm:"column:asset_code"`
	AssetName              string        `json:"asset_name" gorm:"column:asset_name"`
	AssetType              AssetType     `json:"asset_type" gorm:"column:asset_type"`
	BrokerageAccountCode   *string       `json:"brokerage_account_code" gorm:"column:brokerage_account_code"`
	InvestmentAccountCode  *string       `json:"investment_account_code" gorm:"column:investment_account_code"`
	HasLinkedMirror        bool          `json:"has_linked_mirror" gorm:"column:has_linked_mirror"`
	OperationType          OperationType `json:"operation_type" gorm:"column:operation_type"`
	Date                   time.Time     `json:"date" gorm:"column:date"`
	Quantity               int64         `json:"quantity" gorm:"column:quantity"`
	UnitPrice              int64         `json:"unit_price" gorm:"column:unit_price"`
	FeeAmount              int64         `json:"fee_amount" gorm:"column:fee_amount"`
	OriginalTotalFeeAmount int64         `json:"original_total_fee_amount" gorm:"column:original_total_fee_amount"`
	GrossAmount            int64         `json:"gross_amount" gorm:"column:gross_amount"`
	NetAmount              int64         `json:"net_amount" gorm:"column:net_amount"`
	Notes                  string        `json:"notes" gorm:"column:notes"`
	CreatedAt              time.Time     `json:"created_at" gorm:"column:created_at"`
	UpdatedAt              time.Time     `json:"updated_at" gorm:"column:updated_at"`
}

type PositionRow struct {
	AssetCode        string     `json:"asset_code"`
	AssetName        string     `json:"asset_name"`
	AssetType        AssetType  `json:"asset_type"`
	PortfolioNames   []string   `json:"portfolio_names"`
	CurrentQuantity  int64      `json:"current_quantity"`
	AveragePrice     int64      `json:"average_price"`
	TotalCostBasis   int64      `json:"total_cost_basis"`
	RealizedPNL      int64      `json:"realized_pnl"`
	MatchedDividends int64      `json:"matched_dividends_total"`
	CurrentPrice     *int64     `json:"current_price,omitempty"`
	QuoteUpdatedAt   *time.Time `json:"quote_updated_at,omitempty"`
	LastRecalculated time.Time  `json:"last_recalculated"`
}

type PositionQuoteRow struct {
	AssetCode      string    `json:"asset_code"`
	CurrentPrice   int64     `json:"current_price"`
	QuoteUpdatedAt time.Time `json:"quote_updated_at"`
}

type PortfolioAnalysisRow struct {
	AssetCode                   string    `json:"asset_code"`
	AssetName                   string    `json:"asset_name"`
	AssetType                   AssetType `json:"asset_type"`
	CurrentQuantity             int64     `json:"current_quantity"`
	AveragePrice                int64     `json:"average_price"`
	TotalCostBasis              int64     `json:"total_cost_basis"`
	CurrentPrice                int64     `json:"current_price"`
	QuoteUpdatedAt              time.Time `json:"quote_updated_at"`
	CurrentValue                int64     `json:"current_value"`
	CurrentAllocationBasisPoint int       `json:"current_allocation_basis_point"`
	TargetAllocationBasisPoint  int       `json:"target_allocation_basis_point"`
	AllocationDriftBasisPoint   int       `json:"allocation_drift_basis_point"`
	BuyOnlyGapAmount            int64     `json:"buy_only_gap_amount"`
	MaxBuyPrice                 *int64    `json:"max_buy_price"`
	BlockedByMaxBuyPrice        bool      `json:"blocked_by_max_buy_price"`
	UnrealizedPNLAmount         int64     `json:"unrealized_pnl_amount"`
	UnrealizedPNLBasisPoint     *int      `json:"unrealized_pnl_basis_point,omitempty"`
}

type PortfolioAnalysisResponse struct {
	PortfolioCode                   string                 `json:"portfolio_code"`
	PortfolioName                   string                 `json:"portfolio_name"`
	PortfolioDescription            string                 `json:"portfolio_description"`
	TargetAllocationBasisPointTotal int                    `json:"target_allocation_basis_point_total"`
	RebalanceToleranceBasisPoint    int                    `json:"rebalance_tolerance_basis_point"`
	MinimumSuggestedInvestment      *int64                 `json:"minimum_suggested_investment,omitempty"`
	IncomeSummary                   PortfolioIncomeSummary `json:"income_summary"`
	TotalCurrentValue               int64                  `json:"total_current_value"`
	TotalCostBasis                  int64                  `json:"total_cost_basis"`
	TotalUnrealizedPNLAmount        int64                  `json:"total_unrealized_pnl_amount"`
	TotalUnrealizedPNLBasisPoint    *int                   `json:"total_unrealized_pnl_basis_point,omitempty"`
	Rows                            []PortfolioAnalysisRow `json:"rows"`
}

type PortfolioIncomeSummary struct {
	MatchedDividendsTotal      int64                     `json:"matched_dividends_total"`
	MatchedTransactionsCount   int                       `json:"matched_transactions_count"`
	UnmatchedTransactionsCount int                       `json:"unmatched_transactions_count"`
	AmbiguousTransactionsCount int                       `json:"ambiguous_transactions_count"`
	Rows                       []PortfolioIncomeAssetRow `json:"rows"`
}

type PortfolioIncomeAssetRow struct {
	AssetCode        string    `json:"asset_code"`
	AssetName        string    `json:"asset_name"`
	AssetType        AssetType `json:"asset_type"`
	Amount           int64     `json:"amount"`
	TransactionCount int       `json:"transaction_count"`
}

type PortfolioSuggestionRow struct {
	AssetCode                     string    `json:"asset_code"`
	AssetName                     string    `json:"asset_name"`
	AssetType                     AssetType `json:"asset_type"`
	CurrentPrice                  int64     `json:"current_price"`
	CurrentAllocationBasisPoint   int       `json:"current_allocation_basis_point"`
	TargetAllocationBasisPoint    int       `json:"target_allocation_basis_point"`
	ProjectedAllocationBasisPoint int       `json:"projected_allocation_basis_point"`
	MaxBuyPrice                   *int64    `json:"max_buy_price"`
	BlockedByMaxBuyPrice          bool      `json:"blocked_by_max_buy_price"`
	BuyShares                     int64     `json:"buy_shares"`
	PlannedSpend                  int64     `json:"planned_spend"`
}

type PortfolioSuggestionResponse struct {
	PortfolioCode                   string                   `json:"portfolio_code"`
	PortfolioName                   string                   `json:"portfolio_name"`
	PortfolioDescription            string                   `json:"portfolio_description"`
	InvestmentAmount                int64                    `json:"investment_amount"`
	PlannedSpend                    int64                    `json:"planned_spend"`
	CashRemainder                   int64                    `json:"cash_remainder"`
	TargetAllocationBasisPointTotal int                      `json:"target_allocation_basis_point_total"`
	Rows                            []PortfolioSuggestionRow `json:"rows"`
}

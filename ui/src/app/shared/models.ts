export interface LoginResponse {
  token: string;
}

export interface UserSummary {
  id: string;
  name: string;
  email: string;
}

export interface RegisterPayload {
  name: string;
  email: string;
  password: string;
}

export interface RegisterResponse {
  user: UserSummary;
}

export interface ApiError {
  error?:
    | string
    | {
        code?: string;
        error?: string;
        details?: Record<string, unknown>;
      };
}

export interface TransactionListConfig {
  page_size: number;
  show_total: boolean;
}

export interface ReportsConfig {
  show_empty_categories: boolean;
}

export type InvestmentSuggestionStrategy = 'BEST_NEXT_SHARE' | 'PROPORTIONAL_GAP';

export interface InvestmentPortfoliosConfig {
  rebalance_tolerance_basis_point: number;
  suggestion_strategy: InvestmentSuggestionStrategy;
}

export interface InvestmentsConfig {
  portfolios: InvestmentPortfoliosConfig;
  integration: InvestmentIntegrationConfig;
}

export interface InvestmentIntegrationConfig {
  watched_category_ids: string[];
  sell_gain_category_id?: string | null;
  sell_loss_category_id?: string | null;
  bonification_income_category_id?: string | null;
}

export interface UIConfig {
  hide_amounts: boolean;
}

export interface UserConfigSettings {
  transactions: {
    list: TransactionListConfig;
  };
  reports: ReportsConfig;
  investments: InvestmentsConfig;
  ui: UIConfig;
}

export interface UserConfig {
  language: string;
  settings: UserConfigSettings;
}

export type AccountType = 'ASSET' | 'LIABILITY';
export type AccountAssetRole = 'NORMAL' | 'BROKERAGE' | 'INVESTMENT';
export type CategoryType = 'INCOME' | 'EXPENSE' | 'MOVEMENT';
export type InvestmentAssetType = 'STOCK' | 'FII' | 'ETF';
export type InvestmentOperationType = 'BUY' | 'SELL' | 'BONIFICATION' | 'AMORTIZATION';

export interface Account {
  ID: string;
  UserID: string;
  Code: string;
  Name: string;
  Type: AccountType;
  Balance: number;
  Currency: string;
  asset_role: AccountAssetRole;
  hide_from_dashboard: boolean;
  SortOrder?: number | null;
  CreatedAt: string;
  UpdatedAt: string;
  DeactivatedAt?: string | null;
}

export interface Category {
  ID: string;
  UserID: string;
  ParentID?: string | null;
  Code: string;
  Name: string;
  Type: CategoryType;
  Description: string;
  SortOrder?: number | null;
  CreatedAt: string;
  UpdatedAt: string;
  DeactivatedAt?: string | null;
  SubCategories?: Category[];
}

export interface InvestmentAsset {
  id: string;
  user_id: string;
  code: string;
  name: string;
  cnpj?: string | null;
  asset_type: InvestmentAssetType;
  metadata_source?: string | null;
  metadata_updated_at?: string | null;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface InvestmentOperation {
  id: string;
  asset_code: string;
  asset_name: string;
  asset_type: InvestmentAssetType;
  brokerage_account_code?: string | null;
  investment_account_code?: string | null;
  cash_movement_group_key?: string;
  cash_movement_group_size?: number;
  cash_movement_group_gross_amount?: number;
  cash_movement_group_net_amount?: number;
  cash_movement_group_quantity?: number;
  has_linked_mirror?: boolean;
  operation_type: InvestmentOperationType;
  date: string;
  quantity: number;
  unit_price: number;
  fee_amount: number;
  original_total_fee_amount: number;
  gross_amount: number;
  net_amount: number;
  notes: string;
  created_at: string;
  updated_at: string;
}

export interface InvestmentPosition {
  asset_code: string;
  asset_name: string;
  asset_type: InvestmentAssetType;
  portfolio_names: string[];
  current_quantity: number;
  average_price: number;
  total_cost_basis: number;
  realized_pnl: number;
  matched_dividends_total: number;
  current_price?: number | null;
  quote_updated_at?: string | null;
  last_recalculated: string;
}

export interface InvestmentPositionPreviewRow {
  asset_code: string;
  asset_name: string;
  current_quantity: number;
  draft_change: number;
  projected_quantity: number;
  current_average_price: number;
  projected_average_price: number;
}

export interface InvestmentPositionQuote {
  asset_code: string;
  current_price: number;
  quote_updated_at: string;
}

export interface InvestmentPortfolioAsset {
  asset_code: string;
  asset_name: string;
  asset_type: InvestmentAssetType;
  target_allocation_basis_point: number;
  max_buy_price?: number | null;
  sort_order: number;
}

export interface InvestmentPortfolio {
  code: string;
  name: string;
  description: string;
  sort_order: number;
  assets: InvestmentPortfolioAsset[];
}

export interface InvestmentPortfolioAnalysisRow {
  asset_code: string;
  asset_name: string;
  asset_type: InvestmentAssetType;
  current_quantity: number;
  average_price: number;
  total_cost_basis: number;
  current_price: number;
  quote_updated_at: string;
  current_value: number;
  current_allocation_basis_point: number;
  target_allocation_basis_point: number;
  allocation_drift_basis_point: number;
  buy_only_gap_amount: number;
  max_buy_price?: number | null;
  blocked_by_max_buy_price: boolean;
  unrealized_pnl_amount: number;
  unrealized_pnl_basis_point?: number | null;
}

export interface InvestmentPortfolioAnalysis {
  portfolio_code: string;
  portfolio_name: string;
  portfolio_description: string;
  target_allocation_basis_point_total: number;
  rebalance_tolerance_basis_point: number;
  minimum_suggested_investment?: number | null;
  income_summary: InvestmentPortfolioIncomeSummary;
  total_current_value: number;
  total_cost_basis: number;
  total_unrealized_pnl_amount: number;
  total_unrealized_pnl_basis_point?: number | null;
  rows: InvestmentPortfolioAnalysisRow[];
}

export interface InvestmentPortfolioIncomeSummaryRow {
  asset_code: string;
  asset_name: string;
  asset_type: InvestmentAssetType;
  amount: number;
  transaction_count: number;
}

export interface InvestmentPortfolioIncomeSummary {
  matched_dividends_total: number;
  matched_transactions_count: number;
  unmatched_transactions_count: number;
  ambiguous_transactions_count: number;
  rows: InvestmentPortfolioIncomeSummaryRow[];
}

export interface InvestmentPortfolioSuggestionRow {
  asset_code: string;
  asset_name: string;
  asset_type: InvestmentAssetType;
  current_price: number;
  current_allocation_basis_point: number;
  target_allocation_basis_point: number;
  projected_allocation_basis_point: number;
  max_buy_price?: number | null;
  blocked_by_max_buy_price: boolean;
  buy_shares: number;
  planned_spend: number;
}

export interface InvestmentPortfolioSuggestion {
  portfolio_code: string;
  portfolio_name: string;
  portfolio_description: string;
  investment_amount: number;
  planned_spend: number;
  cash_remainder: number;
  target_allocation_basis_point_total: number;
  rows: InvestmentPortfolioSuggestionRow[];
}

export interface Transaction {
  id: string;
  category_code: string;
  description: string;
  date: string;
  account_code: string;
  amount: number;
  transfer_id?: number | null;
  account_transfer?: string | null;
  exclude_from_dashboard?: boolean;
  is_investment_operation_mirror?: boolean;
  investment_operation_id?: string | null;
  investment_operation_link_role?: string | null;
  investment_operation_count?: number;
}

export interface Pagination {
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
}

export interface TransactionResponse {
  transactions: Transaction[];
  pagination: Pagination;
  config: TransactionListConfig;
}

export interface AccountBalance {
  AccountCode: string;
  Balance: number;
}

export interface ReportTopItem {
  description: string;
  amount: number;
}

export interface CategoryYearlyBalance {
  code: string;
  monthly_data: number[];
  subcategories?: CategoryYearlyBalance[];
  top_items_by_month?: Array<ReportTopItem[] | null>;
}

export interface DashboardReport {
  year: number;
  month: number;
  balances: AccountBalance[];
  yearly: CategoryYearlyBalance[];
  recent_transactions: Transaction[];
  top_expenses: Transaction[];
}

export interface CreateAccountPayload {
  name: string;
  type: AccountType;
  currency: string;
  asset_role: AccountAssetRole;
  hide_from_dashboard: boolean;
}

export interface UpdateAccountPayload {
  name?: string;
  type?: AccountType;
  currency?: string;
  asset_role?: AccountAssetRole;
  hide_from_dashboard?: boolean;
}

export interface CreateCategoryPayload {
  name: string;
  type: CategoryType;
  description: string;
  parent_code?: string | null;
}

export interface UpdateCategoryPayload {
  name?: string;
  type?: CategoryType;
  description?: string;
  parent_code?: string | null;
}

export interface CreateInvestmentAssetPayload {
  code: string;
  name: string;
  asset_type: InvestmentAssetType;
}

export interface UpdateInvestmentAssetPayload {
  code?: string;
  name?: string;
  cnpj?: string;
  asset_type?: InvestmentAssetType;
  is_active?: boolean;
}

export interface CreateInvestmentPortfolioPayload {
  name: string;
  description: string;
}

export interface UpdateInvestmentPortfolioPayload {
  name?: string;
  description?: string;
}

export interface SaveInvestmentPortfolioAssetPayload {
  target_allocation_basis_point: number;
  max_buy_price?: number | null;
  sort_order?: number;
}

export interface CreateInvestmentOperationPayload {
  asset_code: string;
  brokerage_account_code: string;
  investment_account_code: string;
  operation_type: InvestmentOperationType;
  date: string;
  quantity: number;
  unit_price: number;
  fee_amount: number;
  notes: string;
}

export interface CreateBulkInvestmentOperationPayload {
  operations: Array<{
    asset_code: string;
    brokerage_account_code: string;
    investment_account_code: string;
    operation_type: InvestmentOperationType;
    date: string;
    quantity: number;
    unit_price: number;
    total_fee_amount: number;
    notes: string;
  }>;
}

export interface ImportInvestmentOperationsPayload {
  operations: Array<{
    client_row_id: string;
    asset_code: string;
    brokerage_account_code: string;
    investment_account_code: string;
    operation_type: InvestmentOperationType;
    date: string;
    quantity: number;
    unit_price: number;
    total_fee_amount: number;
    notes: string;
  }>;
  create_mirrored_transactions: boolean;
  mirrored_transactions: Array<{
    client_row_id: string;
    source_account_code: string;
    destination_account_code: string;
    transaction_id?: string | null;
    realized_pnl_transaction_id?: string | null;
  }>;
}

export interface ImportInvestmentOperationsResponse {
  operations: InvestmentOperation[];
  mirroring_enabled: boolean;
  mirrored_transactions_created: number;
}

export interface PreviewImportInvestmentOperationsResponse {
  position_preview_rows: InvestmentPositionPreviewRow[];
  mirror_preview_rows?: InvestmentMirrorPreviewRow[];
}

export type InvestmentMirrorExtraType = 'NONE' | 'REALIZED_PNL' | 'BONIFICATION_INCOME';

export interface InvestmentMirrorPreviewRow {
  client_row_id: string;
  group_key: string;
  operation_type: InvestmentOperationType;
  brokerage_account_code: string;
  investment_account_code: string;
  date: string;
  description: string;
  transfer_amount: number;
  extra_amount: number;
  extra_type: InvestmentMirrorExtraType;
  source_account_code: string;
  destination_account_code: string;
}

export interface UpdateInvestmentOperationPayload {
  asset_code?: string;
  brokerage_account_code?: string;
  investment_account_code?: string;
  operation_type?: InvestmentOperationType;
  date?: string;
  quantity?: number;
  unit_price?: number;
  fee_amount?: number;
  notes?: string;
}

export interface CreateInvestmentOperationMirrorPayload {
  source_account_code?: string;
  destination_account_code?: string;
  transaction_id?: string | null;
  realized_pnl_transaction_id?: string | null;
  bonification_income_transaction_id?: string | null;
}

export interface CreateInvestmentOperationMirrorsBulkPayload {
  items: Array<{
    operation_id: string;
    source_account_code?: string;
    destination_account_code?: string;
    transaction_id?: string | null;
    realized_pnl_transaction_id?: string | null;
  }>;
}

export interface DeleteInvestmentOperationsBulkPayload {
  operation_ids: string[];
}

export interface TransactionPayload {
  date: string;
  category_code: string;
  description: string;
  amount: number;
  account_code: string;
  is_transfer: boolean;
  account_transfer?: string | null;
  exclude_from_dashboard: boolean;
}

export type TransactionUpdatePayload = Partial<TransactionPayload>;

export interface BulkTransactionUpdatePayload {
  ids: string[];
  date?: string;
  category_code?: string;
  description?: string;
  amount?: number;
  account_code?: string;
  is_transfer?: boolean;
  account_transfer?: string | null;
  exclude_from_dashboard?: boolean;
}

export type SuggestionEntryType = 'REVENUE' | 'EXPENSE' | 'TRANSFER';

export interface Suggestion {
  id: string;
  description_contains: string;
  priority: number;
  entry_type?: SuggestionEntryType | null;
  category_code?: string | null;
  account_code?: string | null;
  transfer_account_code?: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateSuggestionPayload {
  description_contains: string;
  priority: number;
  entry_type?: SuggestionEntryType | '' | null;
  category_code?: string | '' | null;
  account_code?: string | '' | null;
  transfer_account_code?: string | '' | null;
}

export interface UpdateSuggestionPayload {
  description_contains?: string;
  priority?: number;
  entry_type?: SuggestionEntryType | '' | null;
  category_code?: string | '' | null;
  account_code?: string | '' | null;
  transfer_account_code?: string | '' | null;
}

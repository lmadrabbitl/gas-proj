package transaction

import (
	"errors"
	appErr "expense-tracker/internal/errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository interface {
	CreateSingle(db *gorm.DB, userID uuid.UUID, transaction *Transaction) (*TransactionResponseItem, error)
	CreateMany(db *gorm.DB, userID uuid.UUID, transaction []*Transaction) ([]*TransactionResponseItem, error)
	GetByID(db *gorm.DB, userID, transactionID uuid.UUID) (*Transaction, error)
	GetByIDs(db *gorm.DB, userID uuid.UUID, transactionIDs []uuid.UUID) ([]*Transaction, error)
	GetDTOByID(db *gorm.DB, userID, transactionID uuid.UUID) (*TransactionResponseItem, error)
	GetDTOByIDs(db *gorm.DB, userID uuid.UUID, transactionIDs []uuid.UUID) ([]*TransactionResponseItem, error)
	GetByTransferID(db *gorm.DB, userID uuid.UUID, transferID int64) ([]Transaction, error)
	GetByUser(db *gorm.DB, userID uuid.UUID, filter *FilterTransactionQuery, showAll bool) (*TransactionResponse, error)
	ListVisibleByCategoryIDs(userID uuid.UUID, categoryIDs []uuid.UUID) ([]TransactionCategoryMatchRow, error)
	Update(db *gorm.DB, userID, transactionID uuid.UUID, transaction *UpdateTransaction) (*TransactionResponseItem, error)
	Delete(db *gorm.DB, userID, transactionID uuid.UUID) error
	GetNextTransferID(db *gorm.DB) (int64, error)
	CalculateAccountBalance(db *gorm.DB, userID uuid.UUID, accCode string) (int64, error)
}

type UpdateTransaction struct {
	Date                 *time.Time
	CategoryID           *uuid.UUID
	Description          *string
	Amount               *int64
	AccountID            *uuid.UUID
	TransferAccountID    *uuid.UUID
	TransferID           *int64
	IsVisible            *bool
	ExcludeFromDashboard *bool
}

type OperationTransaction string

const CreditOperation OperationTransaction = "credit"
const DebitOperation OperationTransaction = "debit"
const TransferOperation OperationTransaction = "transfer"
const InvestmentOperation OperationTransaction = "investment"

func (t OperationTransaction) IsValid() bool {
	switch t {
	case CreditOperation, DebitOperation, TransferOperation, InvestmentOperation:
		return true
	default:
		return false
	}
}

func NewOperationTransaction(op string) (OperationTransaction, bool) {
	switch strings.ToLower(op) {
	case string(CreditOperation):
		return CreditOperation, true
	case string(DebitOperation):
		return DebitOperation, true
	case string(TransferOperation):
		return TransferOperation, true
	case string(InvestmentOperation):
		return InvestmentOperation, true
	default:
		return "", false
	}
}

type FilterTransactionQuery struct {
	PageNumber    int
	PageSize      int
	SortColumn    TransactionSortField
	AscSortOrder  bool
	OperationType []OperationTransaction
	AccountIDs    []uuid.UUID
	CategoryIDs   []uuid.UUID
	Description   []string
	MinAmount     *int64
	MaxAmount     *int64
	StartDate     *time.Time
	EndDate       *time.Time
}

type TransactionCategoryMatchRow struct {
	ID          uuid.UUID
	CategoryID  uuid.UUID
	Description string
	Date        time.Time
	Amount      int64
}

type repository struct {
	db *gorm.DB
}

var UpdateToNullInt64 int64 = -1
var UpdateToNullUUID uuid.UUID = uuid.Nil

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (repo *repository) CreateSingle(db *gorm.DB, userID uuid.UUID, transaction *Transaction) (*TransactionResponseItem, error) {

	if db == nil {
		db = repo.db
	}
	db.Begin()
	if err := db.Create(transaction).Error; err != nil {
		return nil, mapPGError(err)
	}

	return repo.GetDTOByID(db, userID, transaction.ID)
}

func (repo *repository) CreateMany(db *gorm.DB, userID uuid.UUID, transactions []*Transaction) ([]*TransactionResponseItem, error) {

	if err := repo.db.Select("*").Create(transactions).Error; err != nil {
		return nil, mapPGError(err)
	}

	idList := make([]uuid.UUID, len(transactions))
	for _, tx := range transactions {
		idList = append(idList, tx.ID)
	}

	return repo.GetDTOByIDs(db, userID, idList)
}

func (repo *repository) GetDTOByID(db *gorm.DB, userID, transactionID uuid.UUID) (*TransactionResponseItem, error) {
	if db == nil {
		db = repo.db
	}
	transactionIds := []uuid.UUID{transactionID}
	transactionResponses, err := repo.GetDTOByIDs(db, userID, transactionIds)
	if err != nil {
		return nil, err
	}
	if len(transactionResponses) == 0 {
		return nil, appErr.ErrInvalidTransactionID()
	}

	return transactionResponses[0], nil
}

func (repo *repository) GetDTOByIDs(db *gorm.DB, userID uuid.UUID, transactionIDs []uuid.UUID) ([]*TransactionResponseItem, error) {
	if db == nil {
		db = repo.db
	}
	var transactions []*TransactionResponseItem

	err := repo.baseQueryWithCodes(db).
		Where("t.user_id = ? and t.id IN ?", userID, transactionIDs).Find(&transactions).Error
	if err != nil {
		return nil, mapPGError(err)
	}

	return transactions, nil
}

func (repo *repository) GetByID(db *gorm.DB, userID, transactionID uuid.UUID) (*Transaction, error) {
	if db == nil {
		db = repo.db
	}
	transactionIds := []uuid.UUID{transactionID}
	transactionResponses, err := repo.GetByIDs(db, userID, transactionIds)
	if err != nil {
		return nil, err
	}
	if len(transactionResponses) == 0 {
		return nil, appErr.ErrInvalidTransactionID()
	}

	return transactionResponses[0], nil
}

func (repo *repository) GetByIDs(db *gorm.DB, userID uuid.UUID, transactionIDs []uuid.UUID) ([]*Transaction, error) {
	if db == nil {
		db = repo.db
	}
	var transactions []*Transaction

	err := db.Where("user_id = ? and id IN ?", userID, transactionIDs).Find(&transactions).Error
	if err != nil {
		return nil, mapPGError(err)
	}

	return transactions, nil
}

func (repo *repository) GetByTransferID(db *gorm.DB, userID uuid.UUID, transferID int64) ([]Transaction, error) {

	if db == nil {
		db = repo.db
	}

	var transactions []Transaction

	err := db.Where("user_id = ? and transfer_id = ?", userID, transferID).Find(&transactions).Error
	if err != nil {
		return nil, err
	}

	if len(transactions) != 2 {
		return nil, appErr.ErrInvalidNumberOfTransferTransactions()
	}

	return transactions, nil
}

func (repo *repository) GetByUser(db *gorm.DB, userID uuid.UUID, filter *FilterTransactionQuery, showHidden bool) (*TransactionResponse, error) {
	if db == nil {
		db = repo.db
	}
	var transactions []TransactionResponseItem

	db = repo.baseQueryWithCodes(db).
		Where("t.user_id = ?", userID)
	if !showHidden {
		db = db.Where("is_visible = TRUE")
	}
	db, err := applyQueryFilter(db, filter)
	if err != nil {
		return nil, err
	}
	db = applyQuerySort(db, filter.SortColumn, filter.AscSortOrder)

	//before pagination query count
	var total int64
	countQuery := db.Session(&gorm.Session{}) //duplicating to avoid issues
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, err
	}
	pageNumber, pageSize := normalizePagination(filter.PageNumber, filter.PageSize)
	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)

	db = applyQueryPagination(db, pageNumber, pageSize)

	if err := db.Find(&transactions).Error; err != nil {
		return nil, err
	}

	transactionResponse := &TransactionResponse{
		Transactions: transactions,
		Pagination: PaginationInfo{
			Page:       pageNumber,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	return transactionResponse, nil
}

func (repo *repository) ListVisibleByCategoryIDs(userID uuid.UUID, categoryIDs []uuid.UUID) ([]TransactionCategoryMatchRow, error) {
	if len(categoryIDs) == 0 {
		return []TransactionCategoryMatchRow{}, nil
	}

	var rows []TransactionCategoryMatchRow
	err := repo.db.Model(&Transaction{}).
		Select("id, category_id, description, date, amount").
		Where("user_id = ? AND category_id IN ? AND is_visible = TRUE", userID, categoryIDs).
		Order("date DESC, created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, mapPGError(err)
	}
	return rows, nil
}

func (repo *repository) Update(db *gorm.DB, userID, transactionID uuid.UUID, transaction *UpdateTransaction) (*TransactionResponseItem, error) {
	if db == nil {
		db = repo.db
	}
	var updated Transaction

	updates := make(map[string]any)

	if transaction.Date != nil {
		updates["date"] = *transaction.Date
	}
	if transaction.CategoryID != nil {
		updates["category_id"] = *transaction.CategoryID
	}
	if transaction.Description != nil {
		updates["description"] = *transaction.Description
	}
	if transaction.Amount != nil {
		updates["amount"] = *transaction.Amount
	}
	if transaction.AccountID != nil {
		updates["account_id"] = *transaction.AccountID
	}
	if transaction.TransferAccountID != nil {
		if *transaction.TransferAccountID == UpdateToNullUUID {
			//TransferAccountID needs to change to null
			updates["transfer_account_id"] = nil
		} else {
			updates["transfer_account_id"] = *transaction.TransferAccountID
		}
	}
	if transaction.IsVisible != nil {
		updates["is_visible"] = *transaction.IsVisible
	}
	if transaction.ExcludeFromDashboard != nil {
		updates["exclude_from_dashboard"] = *transaction.ExcludeFromDashboard
	}
	if transaction.TransferID != nil {
		if *transaction.TransferID == UpdateToNullInt64 {
			//transferID needs to change to null
			updates["transfer_id"] = nil
		} else {
			updates["transfer_id"] = *transaction.TransferID
		}
	}

	if len(updates) == 0 {
		return nil, fmt.Errorf("%w: no fields to update", appErr.ErrInvalidInput())
	}

	result := db.Model(&Transaction{}).
		Where("user_id = ? and id = ?", userID, transactionID).
		Clauses(clause.Returning{}).
		Updates(updates).Scan(&updated)
	if result.Error != nil {
		return nil, mapPGError(result.Error)
	}

	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return repo.GetDTOByID(db, userID, updated.ID)

}

func (repo *repository) GetNextTransferID(db *gorm.DB) (int64, error) {
	if db == nil {
		db = repo.db
	}
	var transferID int64

	err := db.Raw("SELECT nextval('transfer_id_seq')").Scan(&transferID).Error
	if err != nil {
		return -1, err
	}
	return transferID, nil
}

func (repo *repository) Delete(db *gorm.DB, userID, transactionID uuid.UUID) error {
	if db == nil {
		db = repo.db
	}
	result := db.
		Where("user_id = ? AND id = ?", userID, transactionID).
		Delete(&Transaction{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return appErr.ErrTransactionNotFound()
	}

	return nil
}

func (repo *repository) CalculateAccountBalance(db *gorm.DB, userID uuid.UUID, accCode string) (int64, error) {
	if db == nil {
		db = repo.db
	}

	var total int64
	err := db.Model(&Transaction{}).
		Select("SUM(transactions.amount)").
		Joins("JOIN accounts ON accounts.id = transactions.account_id").
		Where("accounts.code = ? AND transactions.user_id = ?", accCode, userID).
		Scan(&total).Error

	if err != nil {
		return 0, err
	}

	return total, nil
}

func (repo *repository) baseQueryWithCodes(db *gorm.DB) *gorm.DB {
	if db == nil {
		db = repo.db
	}

	return db.Table("transactions t").
		Select(`  
			t.id, 
			c.code as category_code, 
			t.description, 
			t.date, 
			a.code as account_code, 
			t.amount,
			t.transfer_id, 
			a2.code as transfer_account_code,
			t.exclude_from_dashboard,
			CASE WHEN iotl.transaction_id IS NOT NULL THEN TRUE ELSE FALSE END AS is_investment_operation_mirror,
			iotl.investment_operation_id,
			iotl.role AS investment_operation_link_role,
			COALESCE(iotl.investment_operation_count, 0) AS investment_operation_count
		`).
		Joins("JOIN accounts a ON a.id = t.account_id").
		Joins("JOIN categories c ON c.id = t.category_id").
		Joins("LEFT JOIN accounts a2 ON a2.id = t.transfer_account_id").
		Joins(`LEFT JOIN (
			SELECT
				user_id,
				transaction_id,
				MIN(investment_operation_id::text)::uuid AS investment_operation_id,
				MIN(role) AS role,
				COUNT(DISTINCT investment_operation_id) AS investment_operation_count
			FROM investment_operation_transaction_links
			GROUP BY user_id, transaction_id
		) iotl ON iotl.transaction_id = t.id AND iotl.user_id = t.user_id`)
}

func mapPGError(err error) error {

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return appErr.ErrTransactionNotFound()
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch pgErr.Code {
	case "23503":
		switch pgErr.ConstraintName {
		case "transactions_user_id_fkey":
			return appErr.ErrUserNotFound().WithErr(err)
		case "transactions_account_id_fkey":
			return appErr.ErrAccountNotFound().WithErr(err)
		case "transactions_category_id_fkey":
			return appErr.ErrCategoryNotFound().WithErr(err)
		}
		return err
	}
	return err
}

func applyQueryFilter(db *gorm.DB, filter *FilterTransactionQuery) (*gorm.DB, error) {

	if len(filter.AccountIDs) > 0 {
		db = applyAccountFilter(db, filter.AccountIDs)
	}

	if len(filter.CategoryIDs) > 0 {
		db = db.Where("t.category_id IN ?", filter.CategoryIDs)
	}

	if filter.StartDate != nil {
		db = db.Where("t.date >= ?", *filter.StartDate)
	}

	if filter.EndDate != nil {
		db = db.Where("t.date <= ?", *filter.EndDate)
	}

	if filter.MinAmount != nil {
		db = db.Where("abs(t.amount) >= ?", *filter.MinAmount)
	}

	if filter.MaxAmount != nil {
		db = db.Where("abs(t.amount) <= ?", *filter.MaxAmount)
	}

	if len(filter.OperationType) > 0 {
		var err error
		db, err = applyOperationFilter(db, filter.OperationType)
		if err != nil {
			var custErr *appErr.AppError
			if errors.As(err, &custErr) {
				return nil, err
			} else {
				return nil, appErr.ErrInvalidInputWithMessage("error when adding operation filter", err)
			}
		}
	}

	if len(filter.Description) > 0 {
		var err error
		db, err = applyDescriptionFilter(db, filter.Description)
		if err != nil {
			var custErr *appErr.AppError
			if errors.As(err, &custErr) {
				return nil, err
			} else {
				return nil, appErr.ErrInvalidInputWithMessage("error when adding operation filter", err)
			}
		}
	}

	return db, nil
}

func applyQueryPagination(db *gorm.DB, pageNumber, pageSize int) *gorm.DB {
	pageNumber, pageSize = normalizePagination(pageNumber, pageSize)

	offset := (pageNumber - 1) * pageSize
	limit := pageSize

	return db.Offset(offset).Limit(limit)
}

func normalizePagination(pageNumber, pageSize int) (int, int) {
	if pageNumber <= 0 {
		pageNumber = 1
	}

	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > MaxTransactionPageSize {
		pageSize = MaxTransactionPageSize
	}

	return pageNumber, pageSize
}

func applyQuerySort(db *gorm.DB, sort TransactionSortField, asc bool) *gorm.DB {

	if !sort.IsValid() {
		asc = false
	}

	sortColumn := mapSortField(sort) //defaults to date if not valid

	db = db.Order(clause.OrderByColumn{
		Column: clause.Column{
			Raw:  true,
			Name: sortColumn,
		}, Desc: !asc})

	db = db.Order("t.created_at DESC") //backup sort

	return db
}

func applyOperationFilter(db *gorm.DB, operations []OperationTransaction) (*gorm.DB, error) {
	conditions := buildOperationConditions(operations)
	if len(conditions) == 0 {
		return nil, appErr.ErrInvalidInputWithMessage(
			fmt.Sprintf("invalid operation type: %v", operations), nil)
	}

	db = db.Where("(" + strings.Join(conditions, " OR ") + ")")
	return db, nil
}

func buildOperationConditions(operations []OperationTransaction) []string {
	conditions := make([]string, 0, len(operations))
	for _, op := range operations {
		switch op {
		case CreditOperation:
			conditions = append(conditions, "t.amount > 0 AND t.transfer_id IS NULL")
		case DebitOperation:
			conditions = append(conditions, "t.amount < 0 AND t.transfer_id IS NULL")
		case TransferOperation:
			conditions = append(conditions, "t.transfer_id IS NOT NULL")
		case InvestmentOperation:
			conditions = append(conditions, "iotl.transaction_id IS NOT NULL")
		}
	}

	return conditions
}

func applyDescriptionFilter(db *gorm.DB, descriptions []string) (*gorm.DB, error) {
	includeTerms, excludeTerms := splitDescriptionTerms(descriptions)
	if len(includeTerms) == 0 && len(excludeTerms) == 0 {
		return db, nil //all descriptions are empty
	}

	if len(includeTerms) > 0 {
		var whereClause *gorm.DB
		const whereQuery = "t.description ILIKE ?"

		for _, desc := range includeTerms {
			if whereClause == nil {
				whereClause = db.Where(whereQuery, "%"+desc+"%")
			} else {
				whereClause = whereClause.Or(whereQuery, "%"+desc+"%")
			}
		}

		db = db.Where(whereClause)
	}

	for _, desc := range excludeTerms {
		db = db.Where("t.description NOT ILIKE ?", "%"+desc+"%")
	}

	return db, nil
}

func splitDescriptionTerms(descriptions []string) ([]string, []string) {
	includeTerms := make([]string, 0, len(descriptions))
	excludeTerms := make([]string, 0, len(descriptions))

	for _, desc := range descriptions {
		descTrimmed := strings.TrimSpace(desc)
		if descTrimmed == "" {
			continue
		}

		if normalized, isExcluded := strings.CutPrefix(descTrimmed, QUERY_DESCRIPTION_EXCLUDE_PREFIX); isExcluded {
			normalized = strings.TrimSpace(normalized)
			if normalized != "" {
				excludeTerms = append(excludeTerms, normalized)
			}
			continue
		}

		includeTerms = append(includeTerms, descTrimmed)
	}

	return includeTerms, excludeTerms
}

func applyAccountFilter(db *gorm.DB, accounts []uuid.UUID) *gorm.DB {
	db = db.Where("t.account_id IN ? OR t.transfer_account_id IN ?", accounts, accounts)
	return db
}

func mapSortField(field TransactionSortField) string {
	switch field {
	case SortByTransactionDate:
		return "t.date"
	case SortByAmount:
		return "ABS(t.amount)"
	case SortByUpdatedDate:
		return "t.updated_at"
	default:
		return "t.date" //default sort
	}
}

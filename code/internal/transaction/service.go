package transaction

import (
	"database/sql"
	"expense-tracker/internal/account"
	"expense-tracker/internal/category"
	"expense-tracker/internal/errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service interface {
	AddTransactions(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error)
	GetTransactions(userID uuid.UUID, filter FilterTransactionRequest) (*TransactionResponse, error)
	GetTransactionByID(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error)
	UpdateTransaction(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error)
	UpdateTransactionsBulk(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error)
	DeleteTransaction(userID uuid.UUID, transactionID uuid.UUID) error
	DeleteTransactionsBulk(userID uuid.UUID, transactionIDs []uuid.UUID) error
}

type AccountService interface {
	GetAccountByCode(userID uuid.UUID, code string) (*account.Account, error)
	GetAccountsByCode(userID uuid.UUID, codes []string) ([]account.Account, error)
	GetAccountsByID(userID uuid.UUID, id []uuid.UUID) ([]account.Account, error)
	UpdateBalance(db *gorm.DB, userID uuid.UUID, code string, newBalance int64) error
}

type CategoryReader interface {
	GetCategoryByCode(userID uuid.UUID, code string) (*category.Category, error)
	GetCategoriesByCode(userID uuid.UUID, codes []string) ([]category.Category, error)
}

type DBTransactionCreator interface {
	Begin(opts ...*sql.TxOptions) *gorm.DB
}

type transactionService struct {
	repo       Repository
	accService AccountService
	catReader  CategoryReader
	dbTx       DBTransactionCreator
}

type transactionPair struct {
	t1         *Transaction
	t2         *Transaction
	isTransfer bool
}

type updatePair struct {
	updatedT1      *UpdateTransaction
	updatedT2      *UpdateTransaction
	createdT2      *Transaction
	hasToCreateT2  bool
	hasToDeleteT2  bool
	isInvertedSign bool
}

type TransactionSortField string

const (
	SortByTransactionDate TransactionSortField = "DATE"
	SortByAmount          TransactionSortField = "AMOUNT"
	SortByUpdatedDate     TransactionSortField = "UPDATED"
)

func (t TransactionSortField) IsValid() bool {
	switch t {
	case SortByTransactionDate, SortByAmount, SortByUpdatedDate:
		return true
	default:
		return false
	}
}

func NewTransactionSortField(sort string) TransactionSortField {
	switch strings.ToUpper(sort) {
	case string(SortByAmount):
		return SortByAmount
	case string(SortByTransactionDate):
		return SortByTransactionDate
	case string(SortByUpdatedDate):
		return SortByUpdatedDate
	default:
		return SortByUpdatedDate
	}
}

type CreateTransactionRequest struct {
	Transactions []SingleTransactionRequest
}

type SingleTransactionRequest struct {
	Date                 time.Time
	CategoryCode         string
	Description          string
	Amount               int64
	AccountCode          string
	IsTransfer           bool
	TransferAccountCode  *string
	ExcludeFromDashboard bool
}

type UpdateTransactionRequest struct {
	Date                 *time.Time
	CategoryCode         *string
	Description          *string
	Amount               *int64
	AccountCode          *string
	IsTransfer           *bool
	TransferAccountCode  *string
	ExcludeFromDashboard *bool
}

func (r UpdateTransactionRequest) IsEmpty() bool {
	return r.Date == nil &&
		r.CategoryCode == nil &&
		r.Description == nil &&
		r.Amount == nil &&
		r.AccountCode == nil &&
		r.IsTransfer == nil &&
		r.TransferAccountCode == nil &&
		r.ExcludeFromDashboard == nil
}

type FilterTransactionRequest struct {
	PageNumber       *int
	PageSize         *int
	SortColumn       *string
	AscSortOrder     *bool
	OperationType    []string
	AccountCodes     []string
	CategoryCodes    []string
	DescriptionTerms []string
	MinAmount        *int64
	MaxAmount        *int64
	StartDate        *time.Time
	EndDate          *time.Time
}

func NewService(repo Repository, accService AccountService, catReader CategoryReader, dbTx DBTransactionCreator) Service {
	return &transactionService{
		repo:       repo,
		accService: accService,
		catReader:  catReader,
		dbTx:       dbTx,
	}
}

func (serv *transactionService) beginDBTransaction() *gorm.DB {
	if serv.dbTx == nil {
		return nil
	}
	return serv.dbTx.Begin()
}

func (serv *transactionService) AddTransactions(userID uuid.UUID, req CreateTransactionRequest) ([]*TransactionResponseItem, error) {

	// creating small cache just for this bulk transaction
	var catCache = map[string]*category.Category{}
	var accCache = map[string]*account.Account{}

	//starting db transaction
	dbTransaction := serv.beginDBTransaction()
	shouldCommit := false
	defer func() {
		if dbTransaction == nil {
			return
		}
		if !shouldCommit {
			dbTransaction.Rollback()
		} else if err := dbTransaction.Commit().Error; err != nil {
			err = fmt.Errorf("commit failed: %w", err)
		}
	}()

	transactions := make([]*Transaction, 0, len(req.Transactions)*2)
	for _, t := range req.Transactions {
		description := normalizeDescriptionForStorage(t.Description)
		transaction := &Transaction{
			ID:                   uuid.New(),
			UserID:               userID,
			Description:          description,
			Date:                 t.Date,
			IsVisible:            BoolPtr(true),
			ExcludeFromDashboard: t.ExcludeFromDashboard,
		}

		//check cache first
		var acc *account.Account
		var ok bool
		var err error
		if acc, ok = accCache[userID.String()+t.AccountCode]; !ok {
			if acc, err = serv.accService.GetAccountByCode(userID, t.AccountCode); err != nil {
				return nil, err
			}
			accCache[userID.String()+t.AccountCode] = acc
		}
		transaction.AccountID = acc.ID

		var cat *category.Category
		if cat, ok = catCache[userID.String()+t.CategoryCode]; !ok {
			if cat, err = serv.catReader.GetCategoryByCode(userID, t.CategoryCode); err != nil {
				return nil, err
			}
			catCache[userID.String()+t.CategoryCode] = cat
		}

		if cat.ParentID == nil {
			return nil, errors.ErrInvalidInputWithMessage("can't create transaction on parent category", nil)
		}
		transaction.CategoryID = cat.ID

		if t.IsTransfer {
			transaction.ExcludeFromDashboard = false
			if t.TransferAccountCode == nil {
				return nil, errors.ErrInvalidInputWithMessage("Transfers need transfer_account_id", nil)
			}
			var transferAcc *account.Account
			if transferAcc, ok = accCache[userID.String()+*t.TransferAccountCode]; !ok {
				if transferAcc, err = serv.accService.GetAccountByCode(userID, *t.TransferAccountCode); err != nil {
					return nil, err
				}
				accCache[userID.String()+*t.TransferAccountCode] = transferAcc
			}
			transaction.TransferAccountID = &transferAcc.ID

			transferID, err := serv.repo.GetNextTransferID(dbTransaction)
			if err != nil {
				return nil, err
			}
			transaction.TransferID = &transferID

			amount := t.Amount
			if amount > 0 {
				amount = -amount
			} //negative for visible row of transaction
			transaction.Amount = amount

			//inserting transaction1
			transactions = append(transactions, transaction)

			//creating transaction2
			transaction2 := &Transaction{
				ID:                   uuid.New(),
				UserID:               userID,
				Description:          description,
				Date:                 t.Date,
				IsVisible:            BoolPtr(false),
				CategoryID:           cat.ID,
				AccountID:            transferAcc.ID,
				TransferID:           &transferID,
				Amount:               -amount,
				TransferAccountID:    &acc.ID,
				ExcludeFromDashboard: false,
			}
			//inserting transaction2
			transactions = append(transactions, transaction2)

		} else {
			transaction.Amount = t.Amount
			transactions = append(transactions, transaction)
		}

	}

	transactionsResp, err := serv.repo.CreateMany(dbTransaction, userID, transactions)
	if err != nil {
		return nil, err
	}

	accountsToUpdate := getAccountsToUpdate(transactionsResp)
	if err := serv.updateAllAccountBalances(dbTransaction, userID, accountsToUpdate); err != nil {
		return nil, err
	}
	shouldCommit = true
	return transactionsResp, nil

}

func (serv *transactionService) GetTransactions(userID uuid.UUID, filterReq FilterTransactionRequest) (*TransactionResponse, error) {

	filterQuery := defaultFilterTransactionQuery()

	if filterReq.StartDate != nil {
		filterQuery.StartDate = filterReq.StartDate
	}

	if filterReq.EndDate != nil {
		filterQuery.EndDate = filterReq.EndDate

		if filterQuery.StartDate != nil {
			if filterQuery.StartDate.After(*filterQuery.EndDate) {
				return nil, errors.ErrInvalidInputWithMessage("start date can't be after end date", nil)
			}
		}
	}

	//for min and max amount, only work with positive numbers
	if filterReq.MinAmount != nil {
		if *filterReq.MinAmount < 0 {
			invertedValue := -(*filterReq.MinAmount)
			filterQuery.MinAmount = &invertedValue
		} else {
			filterQuery.MinAmount = filterReq.MinAmount
		}
	}

	if filterReq.MaxAmount != nil {
		if *filterReq.MaxAmount < 0 {
			invertedValue := -(*filterReq.MaxAmount)
			filterQuery.MaxAmount = &invertedValue
		} else {
			filterQuery.MaxAmount = filterReq.MaxAmount
		}

		if filterQuery.MinAmount != nil {
			if *filterQuery.MinAmount > *filterQuery.MaxAmount {
				return nil, errors.ErrInvalidInputWithMessage("min amount can't be higher than max amount", nil)
			}
		}
	}

	if len(filterReq.AccountCodes) > 0 {
		accounts, err := serv.accService.GetAccountsByCode(userID, filterReq.AccountCodes)
		if err != nil {
			return nil, err
		}
		accountIDs := make([]uuid.UUID, 0, len(accounts))
		for _, acc := range accounts {
			accountIDs = append(accountIDs, acc.ID)
		}
		filterQuery.AccountIDs = accountIDs
	}

	if len(filterReq.CategoryCodes) > 0 {
		categories, err := serv.catReader.GetCategoriesByCode(userID, filterReq.CategoryCodes)
		if err != nil {
			return nil, err
		}
		categoryIDs := make([]uuid.UUID, 0, len(categories))
		for _, cat := range categories {
			categoryIDs = append(categoryIDs, cat.ID)
		}
		filterQuery.CategoryIDs = categoryIDs
	}

	if len(filterReq.DescriptionTerms) > 0 {
		filterQuery.Description = filterReq.DescriptionTerms
	}

	if len(filterReq.OperationType) > 0 {
		operationTransactions := make([]OperationTransaction, 0, len(filterReq.OperationType))
		for _, opStr := range filterReq.OperationType {
			op, ok := NewOperationTransaction(opStr)
			if !ok {
				return nil, errors.ErrInvalidInputWithCode("transaction.filter.operation.invalid", "invalid operation type", nil)
			}
			operationTransactions = append(operationTransactions, op)
		}
		filterQuery.OperationType = operationTransactions
	}

	if filterReq.PageNumber != nil {
		filterQuery.PageNumber = *filterReq.PageNumber
	}

	if filterReq.PageSize != nil {
		filterQuery.PageSize = *filterReq.PageSize
	}

	if filterReq.SortColumn != nil {
		filterQuery.SortColumn = NewTransactionSortField(*filterReq.SortColumn)
	}

	if filterReq.AscSortOrder != nil {
		filterQuery.AscSortOrder = *filterReq.AscSortOrder
	}

	transactions, err := serv.repo.GetByUser(nil, userID, filterQuery, false)
	if err != nil {
		return nil, err
	}

	return transactions, nil
}

func (serv *transactionService) GetTransactionByID(userID uuid.UUID, transactionID uuid.UUID) (*TransactionResponseItem, error) {
	return serv.repo.GetDTOByID(nil, userID, transactionID)
}

func (serv *transactionService) UpdateTransaction(userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest) (*TransactionResponseItem, error) {
	if err := serv.ensureMirrorTransactionUpdateAllowed(userID, transactionID, req); err != nil {
		return nil, err
	}
	dbTransaction := serv.beginDBTransaction()
	shouldCommit := false
	accountSet := map[uuid.UUID]struct{}{}
	defer func() {
		if dbTransaction == nil {
			return
		}
		if !shouldCommit {
			dbTransaction.Rollback()
		} else if err := dbTransaction.Commit().Error; err != nil {
			err = fmt.Errorf("commit failed: %w", err)
		}
	}()

	updatedTransaction, err := serv.updateTransactionWithDB(dbTransaction, userID, transactionID, req, accountSet)
	if err != nil {
		return nil, err
	}

	accCodeList, err := serv.getAccountCodesFromIds(userID, mapKeysUUID(accountSet))
	if err != nil {
		return nil, err
	}

	if err := serv.updateAllAccountBalances(dbTransaction, userID, accCodeList); err != nil {
		return nil, err
	}
	shouldCommit = true
	return updatedTransaction, nil
}

func (serv *transactionService) UpdateTransactionsBulk(userID uuid.UUID, transactionIDs []uuid.UUID, req UpdateTransactionRequest) (int, error) {
	if len(transactionIDs) == 0 {
		return 0, errors.ErrInvalidInputWithMessage("no transaction ids to update", nil)
	}
	if req.IsEmpty() {
		return 0, errors.ErrInvalidInputWithMessage("no data to to update", nil)
	}

	dbTransaction := serv.beginDBTransaction()
	shouldCommit := false
	accountSet := map[uuid.UUID]struct{}{}
	uniqueIDs := uniqueTransactionIDs(transactionIDs)
	var bulkTransferState *bool

	defer func() {
		if dbTransaction == nil {
			return
		}
		if !shouldCommit {
			dbTransaction.Rollback()
		} else if err := dbTransaction.Commit().Error; err != nil {
			err = fmt.Errorf("commit failed: %w", err)
		}
	}()

	for _, transactionID := range uniqueIDs {
		if err := serv.ensureMirrorTransactionUpdateAllowed(userID, transactionID, req); err != nil {
			return 0, err
		}
		pair, err := serv.getTransactionPairWithDB(dbTransaction, userID, transactionID)
		if err != nil {
			return 0, err
		}
		if pair.t1.IsVisible != nil && !*pair.t1.IsVisible {
			return 0, errors.ErrInvalidInputWithMessage("can't bulk update hidden transfer row", nil)
		}

		if bulkTransferState == nil {
			value := pair.isTransfer
			bulkTransferState = &value
		} else if *bulkTransferState != pair.isTransfer {
			return 0, errors.ErrInvalidInputWithMessage("can't bulk update mixed transfer and non-transfer transactions", nil)
		}

		if _, err := serv.updateTransactionPair(dbTransaction, userID, pair, req, accountSet); err != nil {
			return 0, err
		}
	}

	accCodeList, err := serv.getAccountCodesFromIds(userID, mapKeysUUID(accountSet))
	if err != nil {
		return 0, err
	}
	if err := serv.updateAllAccountBalances(dbTransaction, userID, accCodeList); err != nil {
		return 0, err
	}

	shouldCommit = true
	return len(uniqueIDs), nil
}

func (serv *transactionService) updateTransactionWithDB(db *gorm.DB, userID uuid.UUID, transactionID uuid.UUID, req UpdateTransactionRequest, accountSet map[uuid.UUID]struct{}) (*TransactionResponseItem, error) {
	pair, err := serv.getTransactionPairWithDB(db, userID, transactionID)
	if err != nil {
		return nil, err
	}
	return serv.updateTransactionPair(db, userID, pair, req, accountSet)
}

func (serv *transactionService) updateTransactionPair(db *gorm.DB, userID uuid.UUID, oldTransactionPair *transactionPair, req UpdateTransactionRequest, accountSet map[uuid.UUID]struct{}) (*TransactionResponseItem, error) {
	if err := CheckUpdateRequest(req, *oldTransactionPair); err != nil {
		return nil, err
	}

	pair := updatePair{
		updatedT1: &UpdateTransaction{},
		updatedT2: &UpdateTransaction{},
	}

	isFinalTransfer := oldTransactionPair.isTransfer
	if req.IsTransfer != nil {
		isFinalTransfer = *req.IsTransfer
	}

	if req.Date != nil {
		pair.updatedT1.Date = req.Date
		if isFinalTransfer {
			pair.updatedT2.Date = req.Date
		}
	}
	if req.Description != nil {
		normalizedDescription := normalizeDescriptionForStorage(*req.Description)
		pair.updatedT1.Description = &normalizedDescription
		if isFinalTransfer {
			pair.updatedT2.Description = &normalizedDescription
		}
	}
	if req.CategoryCode != nil {
		category, err := serv.catReader.GetCategoryByCode(userID, *req.CategoryCode)
		if err != nil {
			return nil, err
		}
		catID := &category.ID
		pair.updatedT1.CategoryID = catID
		if isFinalTransfer {
			pair.updatedT2.CategoryID = catID
		}
	}

	if req.Amount != nil {
		if isFinalTransfer {
			positiveAmount := *req.Amount
			if positiveAmount < 0 {
				positiveAmount = -positiveAmount
			}
			negativeAmount := -positiveAmount
			pair.updatedT1.Amount = &negativeAmount
			pair.updatedT2.Amount = &positiveAmount
		} else {
			pair.updatedT1.Amount = req.Amount
		}
	}

	if req.ExcludeFromDashboard != nil && !isFinalTransfer {
		pair.updatedT1.ExcludeFromDashboard = req.ExcludeFromDashboard
	}

	if req.AccountCode != nil {
		account, err := serv.accService.GetAccountByCode(userID, *req.AccountCode)
		if err != nil {
			return nil, err
		}
		newAccountID := &account.ID
		if isFinalTransfer {
			pair.updatedT1.AccountID = newAccountID
			pair.updatedT2.TransferAccountID = newAccountID
		} else {
			pair.updatedT1.AccountID = newAccountID
		}
	}

	if req.TransferAccountCode != nil && isFinalTransfer {
		transferAccount, err := serv.accService.GetAccountByCode(userID, *req.TransferAccountCode)
		if err != nil {
			return nil, err
		}
		newTransferAccountID := &transferAccount.ID
		pair.updatedT1.TransferAccountID = newTransferAccountID
		pair.updatedT2.AccountID = newTransferAccountID
	}

	if req.IsTransfer != nil {
		if oldTransactionPair.t1.TransferID != nil && !*req.IsTransfer {
			pair.updatedT1.TransferID = &UpdateToNullInt64
			pair.hasToDeleteT2 = true
			pair.updatedT1.TransferAccountID = &UpdateToNullUUID
		} else if oldTransactionPair.t1.TransferID == nil && *req.IsTransfer {
			pair.hasToCreateT2 = true
			newTransferID, err := serv.repo.GetNextTransferID(db)
			if err != nil {
				return nil, err
			}
			pair.updatedT1.TransferID = &newTransferID
			pair.updatedT2.TransferID = &newTransferID
			transaction2Visibility := false
			pair.updatedT2.IsVisible = &transaction2Visibility
			if pair.updatedT2.Amount == nil {
				var positiveAmount int64
				if pair.updatedT1.Amount == nil {
					positiveAmount = oldTransactionPair.t1.Amount
				} else {
					positiveAmount = *pair.updatedT1.Amount
				}
				if positiveAmount < 0 {
					positiveAmount = -positiveAmount
				}
				negativeAmount := -positiveAmount
				pair.updatedT1.Amount = &negativeAmount
				pair.updatedT2.Amount = &positiveAmount
			}
		}
	}

	if isFinalTransfer {
		excludeFromDashboard := false
		pair.updatedT1.ExcludeFromDashboard = &excludeFromDashboard
		pair.updatedT2.ExcludeFromDashboard = &excludeFromDashboard
		acc1 := pair.updatedT1.AccountID
		if acc1 == nil {
			acc1 = &oldTransactionPair.t1.AccountID
		}
		acc2 := pair.updatedT1.TransferAccountID
		if acc2 == nil {
			acc2 = oldTransactionPair.t1.TransferAccountID
		}
		if acc1 != nil && acc2 != nil && *acc1 == *acc2 {
			return nil, errors.ErrInvalidInputWithMessage("account can't be the same as account transfer", nil)
		}
	}

	if pair.hasToCreateT2 {
		pair.createdT2 = &Transaction{
			ID:                uuid.New(),
			UserID:            userID,
			CategoryID:        *coalesce(pair.updatedT2.CategoryID, &oldTransactionPair.t1.CategoryID),
			Description:       *coalesce(pair.updatedT2.Description, &oldTransactionPair.t1.Description),
			Date:              *coalesce(pair.updatedT2.Date, &oldTransactionPair.t1.Date),
			AccountID:         *coalesce(pair.updatedT2.AccountID, pair.updatedT1.TransferAccountID, oldTransactionPair.t1.TransferAccountID),
			TransferID:        coalesce(pair.updatedT2.TransferID, pair.updatedT1.TransferID, oldTransactionPair.t1.TransferID),
			Amount:            *pair.updatedT2.Amount,
			TransferAccountID: coalesce(pair.updatedT2.TransferAccountID, pair.updatedT1.AccountID, &oldTransactionPair.t1.AccountID),
			IsVisible:         BoolPtr(false),
		}
	}

	if pair.hasToCreateT2 {
		if _, err := serv.repo.CreateSingle(db, userID, pair.createdT2); err != nil {
			return nil, err
		}
		accountSet[pair.createdT2.AccountID] = struct{}{}
		if pair.createdT2.TransferAccountID != nil {
			accountSet[*pair.createdT2.TransferAccountID] = struct{}{}
		}
	} else if pair.hasToDeleteT2 {
		if err := serv.repo.Delete(db, userID, oldTransactionPair.t2.ID); err != nil {
			return nil, err
		}
		accountSet[oldTransactionPair.t2.AccountID] = struct{}{}
		if oldTransactionPair.t2.TransferAccountID != nil {
			accountSet[*oldTransactionPair.t2.TransferAccountID] = struct{}{}
		}
	} else if oldTransactionPair.isTransfer {
		if _, err := serv.repo.Update(db, userID, oldTransactionPair.t2.ID, pair.updatedT2); err != nil {
			return nil, err
		}
		accountSet[oldTransactionPair.t2.AccountID] = struct{}{}
		if oldTransactionPair.t2.TransferAccountID != nil {
			accountSet[*oldTransactionPair.t2.TransferAccountID] = struct{}{}
		}
		if pair.updatedT2.AccountID != nil {
			accountSet[*pair.updatedT2.AccountID] = struct{}{}
		}
		if pair.updatedT2.TransferAccountID != nil {
			accountSet[*pair.updatedT2.TransferAccountID] = struct{}{}
		}
	}

	updatedTransaction1, err := serv.repo.Update(db, userID, oldTransactionPair.t1.ID, pair.updatedT1)
	if err != nil {
		return nil, err
	}
	accountSet[oldTransactionPair.t1.AccountID] = struct{}{}
	if oldTransactionPair.t1.TransferAccountID != nil {
		accountSet[*oldTransactionPair.t1.TransferAccountID] = struct{}{}
	}
	if pair.updatedT1.AccountID != nil {
		accountSet[*pair.updatedT1.AccountID] = struct{}{}
	}
	if pair.updatedT1.TransferAccountID != nil {
		accountSet[*pair.updatedT1.TransferAccountID] = struct{}{}
	}

	return updatedTransaction1, nil
}

func (serv *transactionService) DeleteTransaction(userID uuid.UUID, transactionID uuid.UUID) error {
	dto, err := serv.repo.GetDTOByID(nil, userID, transactionID)
	if err != nil {
		return err
	}
	if dto.IsInvestmentMirror {
		return errors.ErrInvalidInputWithCode("transaction.linked_investment_operation.read_only", "linked mirrored transactions cannot be deleted directly", nil)
	}

	//create transaction and check if transfer
	transactionPair, err := serv.getTransactionPair(userID, transactionID)
	if err != nil {
		return err
	}

	dbTransaction := serv.beginDBTransaction()
	shouldCommit := false
	accountSet := map[uuid.UUID]any{}
	defer func() {
		if dbTransaction == nil {
			return
		}
		if !shouldCommit {
			dbTransaction.Rollback()
		} else if err := dbTransaction.Commit().Error; err != nil {
			err = fmt.Errorf("commit failed: %w", err)
		}
	}()

	accountSet[transactionPair.t1.AccountID] = struct{}{}
	if transactionPair.t1.TransferAccountID != nil {
		accountSet[*transactionPair.t1.TransferAccountID] = struct{}{}
	}

	if transactionPair.isTransfer {
		//start db transaction to delete both rows
		accountSet[transactionPair.t2.AccountID] = struct{}{}
		if transactionPair.t2.TransferAccountID != nil {
			accountSet[*transactionPair.t2.TransferAccountID] = struct{}{}
		}
		if err := serv.repo.Delete(dbTransaction, userID, transactionPair.t1.ID); err != nil {
			return err
		}
		if err := serv.repo.Delete(dbTransaction, userID, transactionPair.t2.ID); err != nil {
			return err
		}

	} else {
		err := serv.repo.Delete(dbTransaction, userID, transactionID)
		if err != nil {
			return err
		}
	}

	var accIDList []uuid.UUID
	for accID := range accountSet {
		accIDList = append(accIDList, accID)
	}

	accCodeList, err := serv.getAccountCodesFromIds(userID, accIDList)
	if err != nil {
		return err
	}

	if err := serv.updateAllAccountBalances(dbTransaction, userID, accCodeList); err != nil {
		return err
	}

	shouldCommit = true
	return nil
}

func (serv *transactionService) DeleteTransactionsBulk(userID uuid.UUID, transactionIDs []uuid.UUID) error {
	uniqueIDs := uniqueTransactionIDs(transactionIDs)
	if len(uniqueIDs) == 0 {
		return errors.ErrInvalidInputWithMessage("at least one transaction id is required", nil)
	}

	dbTransaction := serv.beginDBTransaction()
	shouldCommit := false
	deleteIDSet := map[uuid.UUID]struct{}{}
	accountSet := map[uuid.UUID]struct{}{}
	defer func() {
		if dbTransaction == nil {
			return
		}
		if !shouldCommit {
			dbTransaction.Rollback()
		} else if err := dbTransaction.Commit().Error; err != nil {
			err = fmt.Errorf("commit failed: %w", err)
		}
	}()

	for _, transactionID := range uniqueIDs {
		dto, err := serv.repo.GetDTOByID(dbTransaction, userID, transactionID)
		if err != nil {
			return err
		}
		if dto.IsInvestmentMirror {
			return errors.ErrInvalidInputWithCode("transaction.linked_investment_operation.read_only", "linked mirrored transactions cannot be deleted directly", nil)
		}

		transactionPair, err := serv.getTransactionPairWithDB(dbTransaction, userID, transactionID)
		if err != nil {
			return err
		}

		accountSet[transactionPair.t1.AccountID] = struct{}{}
		if transactionPair.t1.TransferAccountID != nil {
			accountSet[*transactionPair.t1.TransferAccountID] = struct{}{}
		}
		deleteIDSet[transactionPair.t1.ID] = struct{}{}

		if transactionPair.isTransfer {
			accountSet[transactionPair.t2.AccountID] = struct{}{}
			if transactionPair.t2.TransferAccountID != nil {
				accountSet[*transactionPair.t2.TransferAccountID] = struct{}{}
			}
			deleteIDSet[transactionPair.t2.ID] = struct{}{}
		}
	}

	for deleteID := range deleteIDSet {
		if err := serv.repo.Delete(dbTransaction, userID, deleteID); err != nil {
			return err
		}
	}

	accCodeList, err := serv.getAccountCodesFromIds(userID, mapKeysUUID(accountSet))
	if err != nil {
		return err
	}

	if err := serv.updateAllAccountBalances(dbTransaction, userID, accCodeList); err != nil {
		return err
	}

	shouldCommit = true
	return nil
}

func (serv *transactionService) ensureMirrorTransactionUpdateAllowed(userID, transactionID uuid.UUID, req UpdateTransactionRequest) error {
	dto, err := serv.repo.GetDTOByID(nil, userID, transactionID)
	if err != nil {
		return err
	}
	if !dto.IsInvestmentMirror {
		return nil
	}
	if req.Date != nil ||
		req.CategoryCode != nil ||
		req.Description != nil ||
		req.Amount != nil ||
		req.AccountCode != nil ||
		req.IsTransfer != nil ||
		req.TransferAccountCode != nil ||
		req.ExcludeFromDashboard != nil {
		return errors.ErrInvalidInputWithCode("transaction.linked_investment_operation.protected_fields", "linked mirrored transaction fields are protected by the investment operation", nil)
	}
	return nil
}

func (serv *transactionService) updateAllAccountBalances(db *gorm.DB, userID uuid.UUID, accountsToUpdate []string) error {

	for _, acc := range accountsToUpdate {
		//get balance
		accBal, err := serv.repo.CalculateAccountBalance(db, userID, acc)
		if err != nil {
			return err
		}
		//update balance
		if err := serv.accService.UpdateBalance(db, userID, acc, accBal); err != nil {
			return err
		}
	}

	return nil
}

func coalesce[T any](values ...*T) *T {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func (serv *transactionService) getTransactionPair(userID, transactionID uuid.UUID) (*transactionPair, error) {
	return serv.getTransactionPairWithDB(nil, userID, transactionID)
}

func (serv *transactionService) getTransactionPairWithDB(db *gorm.DB, userID, transactionID uuid.UUID) (*transactionPair, error) {
	// get original transaction first: tx1
	oldTransaction1, err := serv.repo.GetByID(db, userID, transactionID)
	if err != nil {
		return nil, err
	}

	// if transfer, get the other row: tx2
	var oldTransaction2 *Transaction

	if oldTransaction1.TransferID != nil {
		transferTransactions, err := serv.repo.GetByTransferID(db, userID, *oldTransaction1.TransferID)
		if err != nil {
			return nil, err
		}
		if transferTransactions[0].ID == oldTransaction1.ID {
			oldTransaction2 = &transferTransactions[1]
		} else {
			oldTransaction2 = &transferTransactions[0]
		}
	}

	return &transactionPair{
		t1:         oldTransaction1,
		t2:         oldTransaction2,
		isTransfer: (oldTransaction2 != nil),
	}, nil
}

func defaultFilterTransactionQuery() *FilterTransactionQuery {
	return &FilterTransactionQuery{
		PageNumber:   1,
		PageSize:     50,
		SortColumn:   SortByTransactionDate,
		AscSortOrder: false,
	}
}

func uniqueTransactionIDs(ids []uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]struct{}{}
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return unique
}

func mapKeysUUID(values map[uuid.UUID]struct{}) []uuid.UUID {
	keys := make([]uuid.UUID, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func (serv *transactionService) getAccountCodesFromIds(userID uuid.UUID, accIDs []uuid.UUID) ([]string, error) {

	accounts, err := serv.accService.GetAccountsByID(userID, accIDs)
	if err != nil {
		return nil, err
	}

	var accCodeList []string
	for _, acc := range accounts {
		accCodeList = append(accCodeList, acc.Code)
	}

	return accCodeList, nil
}

func getAccountsToUpdate(transactions []*TransactionResponseItem) []string {
	//adding all to same map to remove duplicates
	accountCodeSet := map[string]any{}
	for _, tx := range transactions {
		accountCodeSet[tx.AccountCode] = struct{}{}
		if tx.TransferAccountCode != nil {
			accountCodeSet[*tx.TransferAccountCode] = struct{}{}
		}
	}

	var accountCodeSlice []string
	for accountCode, _ := range accountCodeSet {
		accountCodeSlice = append(accountCodeSlice, accountCode)
	}
	return accountCodeSlice
}

func BoolPtr(b bool) *bool {
	return &b
}

func normalizeDescriptionForStorage(description string) string {
	return strings.ToUpper(description)
}

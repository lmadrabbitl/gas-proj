package reports

import (
	"expense-tracker/internal/account"
	"expense-tracker/internal/errors"
	"expense-tracker/internal/transaction"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Repository interface {
	GetYearlyReport(userID uuid.UUID, year int) ([]YearlyReportRow, error)
	GetMonthlyReportRange(userID uuid.UUID, startDate, endDate time.Time) ([]YearlyReportRow, error)
	GetDashboardMonthlyReportRange(userID uuid.UUID, startDate, endDate time.Time) ([]YearlyReportRow, error)
	GetTopReportItems(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]YearlyReportTopItemRow, error)
	GetBalanceByAccountCode(userID uuid.UUID, code string) (*int64, error)
	GetAllBalances(userID uuid.UUID) ([]*AccountBalance, error)
	GetRecentExpenseTransactions(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]*transaction.TransactionResponseItem, error)
	GetTopExpenseTransactions(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]*transaction.TransactionResponseItem, error)
}

type AccountBalance struct {
	AccountCode string
	Balance     int64
}

type YearlyReportRow struct {
	Year            int     `gorm:"column:year"`
	Month           int     `gorm:"column:month"`
	Parent          string  `gorm:"column:parent"`
	Child           *string `gorm:"column:child"` // nullable (parent totals)
	Total           int64   `gorm:"column:total"`
	IsParent        bool    `gorm:"column:is_parent"`
	ParentType      string  `gorm:"column:parent_type"`
	ParentSortOrder *int    `gorm:"column:parent_sort_order"`
	ChildSortOrder  *int    `gorm:"column:child_sort_order"`
}

type YearlyReportTopItemRow struct {
	Year        int    `gorm:"column:year"`
	Month       int    `gorm:"column:month"`
	Child       string `gorm:"column:child"`
	Description string `gorm:"column:description"`
	Amount      int64  `gorm:"column:amount"`
	Rank        int    `gorm:"column:rank"`
}

type repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (repo *repository) GetYearlyReport(userID uuid.UUID, year int) ([]YearlyReportRow, error) {
	startDate := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(1, 0, 0)
	return repo.GetMonthlyReportRange(userID, startDate, endDate)
}

func (repo *repository) GetMonthlyReportRange(userID uuid.UUID, startDate, endDate time.Time) ([]YearlyReportRow, error) {
	return repo.getMonthlyReportRange(userID, startDate, endDate, false)
}

func (repo *repository) GetDashboardMonthlyReportRange(userID uuid.UUID, startDate, endDate time.Time) ([]YearlyReportRow, error) {
	return repo.getMonthlyReportRange(userID, startDate, endDate, true)
}

func (repo *repository) getMonthlyReportRange(userID uuid.UUID, startDate, endDate time.Time, excludeFromDashboard bool) ([]YearlyReportRow, error) {
	var rows []YearlyReportRow

	excludeClause := ""
	if excludeFromDashboard {
		excludeClause = "AND t.exclude_from_dashboard = FALSE"
	}

	query := `
		WITH months AS (
			SELECT generate_series(
				date_trunc('month', ?::timestamp),
				date_trunc('month', ?::timestamp) - interval '1 month',
				interval '1 month'
			) AS month_start
		),
		parents AS (
			SELECT id, code, type, sort_order
			FROM categories
			WHERE parent_id IS NULL
			AND user_id = ?
			AND deactivated_at IS NULL
		)

		SELECT *
		FROM (
			SELECT
				EXTRACT(YEAR FROM m.month_start)  AS year,
				EXTRACT(MONTH FROM m.month_start) AS month,
				p.code AS parent,
				NULL::text AS child,
				COALESCE(SUM(t.amount), 0) AS total,
				TRUE AS is_parent,
				p.type AS parent_type,
				p.sort_order AS parent_sort_order,
				NULL::integer AS child_sort_order
			FROM parents p
			CROSS JOIN months m
			LEFT JOIN transactions t
				ON date_trunc('month', t.date) = m.month_start
				` + excludeClause + `
				AND (
					t.category_id = p.id
					OR EXISTS (
						SELECT 1
						FROM categories c
						WHERE c.parent_id = p.id
						AND c.id = t.category_id
					)
				)
			GROUP BY year, month, p.code, p.type, p.sort_order

			UNION ALL

			SELECT
				EXTRACT(YEAR FROM m.month_start)  AS year,
				EXTRACT(MONTH FROM m.month_start) AS month,
				p.code AS parent,
				c.code AS child,
				COALESCE(SUM(t.amount), 0) AS total,
				FALSE AS is_parent,
				p.type AS parent_type,
				p.sort_order AS parent_sort_order,
				c.sort_order AS child_sort_order
			FROM parents p
			JOIN categories c
				ON c.parent_id = p.id
				AND c.deactivated_at IS NULL
			CROSS JOIN months m
			LEFT JOIN transactions t
				ON t.category_id = c.id
				AND date_trunc('month', t.date) = m.month_start
				` + excludeClause + `
			GROUP BY year, month, p.code, c.code, p.type, p.sort_order, c.sort_order
		)
		report_rows

		ORDER BY
			CASE parent_type
				WHEN 'INCOME' THEN 0
				WHEN 'EXPENSE' THEN 1
				ELSE 2
			END,
			parent_sort_order ASC NULLS LAST,
			year,
			month,
			parent,
			is_parent DESC,
			child_sort_order ASC NULLS LAST,
			child NULLS FIRST;
		`

	err := repo.db.Raw(query, startDate, endDate, userID).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (repo *repository) GetTopReportItems(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]YearlyReportTopItemRow, error) {
	var rows []YearlyReportTopItemRow

	query := `
		WITH ranked_items AS (
			SELECT
				EXTRACT(YEAR FROM t.date)::integer AS year,
				EXTRACT(MONTH FROM t.date)::integer AS month,
				c.code AS child,
				t.description,
				t.amount,
				ROW_NUMBER() OVER (
					PARTITION BY EXTRACT(YEAR FROM t.date), EXTRACT(MONTH FROM t.date), c.code
					ORDER BY ABS(t.amount) DESC, t.date DESC, t.updated_at DESC, t.id
				) AS rank
			FROM transactions t
			JOIN categories c
				ON c.id = t.category_id
				AND c.parent_id IS NOT NULL
				AND c.user_id = ?
				AND c.deactivated_at IS NULL
			JOIN categories p
				ON p.id = c.parent_id
				AND p.user_id = ?
				AND p.deactivated_at IS NULL
			WHERE t.user_id = ?
			AND t.is_visible = TRUE
			AND t.date >= ?
			AND t.date < ?
		)
		SELECT year, month, child, description, amount, rank
		FROM ranked_items
		WHERE rank <= ?
		ORDER BY child, year, month, rank;
	`

	err := repo.db.Raw(query, userID, userID, userID, startDate, endDate, limit).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	return rows, nil
}

func (repo *repository) GetBalanceByAccountCode(userID uuid.UUID, code string) (*int64, error) {

	var account account.Account
	if err := repo.db.Where("user_id = ? and code = ?", userID, code).First(&account).Error; err != nil {
		if strings.Contains(err.Error(), "record not found") {
			return nil, errors.ErrAccountNotFound()
		}
		return nil, err
	}

	return &account.Balance, nil
}

func (repo *repository) GetAllBalances(userID uuid.UUID) ([]*AccountBalance, error) {
	var accounts []account.Account
	if err := repo.db.Where("user_id = ? and deactivated_at is null and hide_from_dashboard = false", userID).Find(&accounts).Error; err != nil {
		return nil, err
	}

	balances := make([]*AccountBalance, 0, len(accounts))
	for _, acc := range accounts {
		balances = append(balances, &AccountBalance{
			AccountCode: acc.Code,
			Balance:     acc.Balance,
		})
	}
	return balances, nil
}

func (repo *repository) GetRecentExpenseTransactions(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]*transaction.TransactionResponseItem, error) {
	return repo.getExpenseTransactions(userID, startDate, endDate, limit, []string{"t.date DESC", "t.updated_at DESC"})
}

func (repo *repository) GetTopExpenseTransactions(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]*transaction.TransactionResponseItem, error) {
	return repo.getExpenseTransactions(userID, startDate, endDate, limit, []string{"t.amount ASC", "t.date DESC", "t.updated_at DESC"})
}

func (repo *repository) getExpenseTransactions(userID uuid.UUID, startDate, endDate time.Time, limit int, orderClauses []string) ([]*transaction.TransactionResponseItem, error) {
	var transactions []*transaction.TransactionResponseItem

	query := repo.db.Table("transactions t").
		Select(`
			t.id,
			c.code as category_code,
			t.description,
			t.date,
			a.code as account_code,
			t.amount,
			t.transfer_id,
			a2.code as transfer_account_code
		`).
		Joins("JOIN accounts a ON a.id = t.account_id").
		Joins("JOIN categories c ON c.id = t.category_id").
		Joins("LEFT JOIN accounts a2 ON a2.id = t.transfer_account_id").
		Where("t.user_id = ? AND t.is_visible = TRUE", userID).
		Where("t.exclude_from_dashboard = FALSE").
		Where("t.transfer_id IS NULL AND t.amount < 0").
		Where("t.date >= ? AND t.date < ?", startDate, endDate)

	for _, orderClause := range orderClauses {
		query = query.Order(orderClause)
	}

	if err := query.Limit(limit).Find(&transactions).Error; err != nil {
		return nil, err
	}

	return transactions, nil
}

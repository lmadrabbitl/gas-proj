package reports

import (
	"expense-tracker/internal/transaction"
	"time"

	"github.com/google/uuid"
)

type Service interface {
	GetReport(userID uuid.UUID, year int) ([]*CategoryYearlyBalance, error)
	GetBalanceByAccountCode(userID uuid.UUID, code string) (*AccountBalance, error)
	GetAllBalances(userID uuid.UUID) ([]*AccountBalance, error)
	GetDashboard(userID uuid.UUID, year, month int) (*DashboardData, error)
}

type service struct {
	repo Repository
}

type CategoryYearlyBalance struct {
	CategoryCode    string                   `json:"code"`
	Monthly         []int64                  `json:"monthly_data"`
	Children        []*CategoryYearlyBalance `json:"subcategories"`
	TopItemsByMonth [][]*ReportTopItem       `json:"top_items_by_month,omitempty"`
}

type ReportTopItem struct {
	Description string `json:"description"`
	Amount      int64  `json:"amount"`
}

type DashboardData struct {
	Year               int                                    `json:"year"`
	Month              int                                    `json:"month"`
	Balances           []*AccountBalance                      `json:"balances"`
	Yearly             []*CategoryYearlyBalance               `json:"yearly"`
	RecentTransactions []*transaction.TransactionResponseItem `json:"recent_transactions"`
	TopExpenses        []*transaction.TransactionResponseItem `json:"top_expenses"`
}

func NewService(repo Repository) Service {
	return &service{
		repo: repo,
	}
}

func (serv *service) GetReport(userID uuid.UUID, year int) ([]*CategoryYearlyBalance, error) {
	startDate := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(1, 0, 0)
	return serv.getReportRange(userID, startDate, endDate)
}

func (serv *service) GetBalanceByAccountCode(userID uuid.UUID, code string) (*AccountBalance, error) {

	balance, err := serv.repo.GetBalanceByAccountCode(userID, code)
	if err != nil {
		return nil, err
	}

	return &AccountBalance{
		AccountCode: code,
		Balance:     *balance,
	}, nil

}

func (serv *service) GetAllBalances(userID uuid.UUID) ([]*AccountBalance, error) {

	balances, err := serv.repo.GetAllBalances(userID)
	if err != nil {
		return nil, err
	}

	return balances, nil

}

func (serv *service) GetDashboard(userID uuid.UUID, year, month int) (*DashboardData, error) {
	endDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
	startDate := endDate.AddDate(0, -12, 0)

	timeline, err := serv.getDashboardReportRange(userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	balances, err := serv.GetAllBalances(userID)
	if err != nil {
		return nil, err
	}

	selectedMonthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	selectedMonthEnd := selectedMonthStart.AddDate(0, 1, 0)

	recentTransactions, err := serv.repo.GetRecentExpenseTransactions(userID, selectedMonthStart, selectedMonthEnd, 6)
	if err != nil {
		return nil, err
	}

	topExpenses, err := serv.repo.GetTopExpenseTransactions(userID, selectedMonthStart, selectedMonthEnd, 5)
	if err != nil {
		return nil, err
	}

	return &DashboardData{
		Year:               year,
		Month:              month,
		Balances:           balances,
		Yearly:             timeline,
		RecentTransactions: recentTransactions,
		TopExpenses:        topExpenses,
	}, nil
}

func (serv *service) getReportRange(userID uuid.UUID, startDate, endDate time.Time) ([]*CategoryYearlyBalance, error) {
	reportRows, err := serv.repo.GetMonthlyReportRange(userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	topItemRows, err := serv.repo.GetTopReportItems(userID, startDate, endDate, 5)
	if err != nil {
		return nil, err
	}

	categoryMap := map[string]*CategoryYearlyBalance{}
	childToParentMap := map[string]string{}
	categoryOrder := make([]string, 0)

	ensureCategory := func(code string) *CategoryYearlyBalance {
		category := categoryMap[code]
		if category != nil {
			return category
		}

		category = &CategoryYearlyBalance{
			CategoryCode: code,
			Monthly:      make([]int64, 12),
		}
		categoryMap[code] = category
		categoryOrder = append(categoryOrder, code)
		return category
	}

	for _, row := range reportRows {
		monthStart := time.Date(row.Year, time.Month(row.Month), 1, 0, 0, 0, 0, time.UTC)
		monthIndex := monthDiff(startDate, monthStart)
		if monthIndex < 0 || monthIndex >= 12 {
			continue
		}

		var mapKey string
		if row.IsParent {
			mapKey = row.Parent
		} else {
			mapKey = *row.Child
			childToParentMap[*row.Child] = row.Parent
		}

		ensureCategory(mapKey).Monthly[monthIndex] = row.Total
	}

	for _, row := range topItemRows {
		monthStart := time.Date(row.Year, time.Month(row.Month), 1, 0, 0, 0, 0, time.UTC)
		monthIndex := monthDiff(startDate, monthStart)
		if monthIndex < 0 || monthIndex >= 12 {
			continue
		}

		category := ensureCategory(row.Child)
		if category.TopItemsByMonth == nil {
			category.TopItemsByMonth = make([][]*ReportTopItem, 12)
		}
		category.TopItemsByMonth[monthIndex] = append(category.TopItemsByMonth[monthIndex], &ReportTopItem{
			Description: row.Description,
			Amount:      row.Amount,
		})
	}

	var allCategoriesBalance []*CategoryYearlyBalance
	for _, key := range categoryOrder {
		allCategoriesBalance = append(allCategoriesBalance, categoryMap[key])
	}

	return getNestedCategories(childToParentMap, allCategoriesBalance)
}

func (serv *service) getDashboardReportRange(userID uuid.UUID, startDate, endDate time.Time) ([]*CategoryYearlyBalance, error) {
	reportRows, err := serv.repo.GetDashboardMonthlyReportRange(userID, startDate, endDate)
	if err != nil {
		return nil, err
	}

	categoryMap := map[string]*CategoryYearlyBalance{}
	childToParentMap := map[string]string{}
	categoryOrder := make([]string, 0)

	ensureCategory := func(code string) *CategoryYearlyBalance {
		category := categoryMap[code]
		if category != nil {
			return category
		}

		category = &CategoryYearlyBalance{
			CategoryCode: code,
			Monthly:      make([]int64, 12),
		}
		categoryMap[code] = category
		categoryOrder = append(categoryOrder, code)
		return category
	}

	for _, row := range reportRows {
		monthStart := time.Date(row.Year, time.Month(row.Month), 1, 0, 0, 0, 0, time.UTC)
		monthIndex := monthDiff(startDate, monthStart)
		if monthIndex < 0 || monthIndex >= 12 {
			continue
		}

		var mapKey string
		if row.IsParent {
			mapKey = row.Parent
		} else {
			mapKey = *row.Child
			childToParentMap[*row.Child] = row.Parent
		}

		ensureCategory(mapKey).Monthly[monthIndex] = row.Total
	}

	var allCategoriesBalance []*CategoryYearlyBalance
	for _, key := range categoryOrder {
		allCategoriesBalance = append(allCategoriesBalance, categoryMap[key])
	}

	return getNestedCategories(childToParentMap, allCategoriesBalance)
}

func monthDiff(startDate, targetDate time.Time) int {
	startYear, startMonth, _ := startDate.Date()
	targetYear, targetMonth, _ := targetDate.Date()
	return ((targetYear - startYear) * 12) + int(targetMonth-startMonth)
}

func getNestedCategories(childToParentMap map[string]string, categoryBalances []*CategoryYearlyBalance) ([]*CategoryYearlyBalance, error) {

	categoryMap := map[string]*CategoryYearlyBalance{}
	parentOrder := make([]string, 0)

	//first pass - parents
	for _, cat := range categoryBalances {
		if _, ok := childToParentMap[cat.CategoryCode]; !ok {
			categoryMap[cat.CategoryCode] = cat
			parentOrder = append(parentOrder, cat.CategoryCode)
		}
	}

	//second pass - children
	for _, cat := range categoryBalances {
		if parent, ok := childToParentMap[cat.CategoryCode]; ok {
			if parentCat, ok := categoryMap[parent]; ok {
				parentCat.Children = append(parentCat.Children, cat)
			}
		}
	}

	//final - return parent category array
	parentCategoryArray := []*CategoryYearlyBalance{}
	for _, parentCode := range parentOrder {
		parentCat, ok := categoryMap[parentCode]
		if !ok {
			continue
		}
		parentCategoryArray = append(parentCategoryArray, parentCat)
	}

	return parentCategoryArray, nil
}

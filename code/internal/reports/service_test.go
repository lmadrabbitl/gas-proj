package reports

import (
	"expense-tracker/internal/transaction"
	"testing"
	"time"

	"github.com/google/uuid"
)

type repositoryStub struct {
	getYearlyReportFn                func(userID uuid.UUID, year int) ([]YearlyReportRow, error)
	getMonthlyReportRangeFn          func(userID uuid.UUID, startDate, endDate time.Time) ([]YearlyReportRow, error)
	getDashboardMonthlyReportRangeFn func(userID uuid.UUID, startDate, endDate time.Time) ([]YearlyReportRow, error)
	getTopReportItemsFn              func(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]YearlyReportTopItemRow, error)
	getBalanceByAccountCodeFn        func(userID uuid.UUID, code string) (*int64, error)
	getAllBalancesFn                 func(userID uuid.UUID) ([]*AccountBalance, error)
	getRecentExpenseFn               func(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]*transaction.TransactionResponseItem, error)
	getTopExpenseFn                  func(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]*transaction.TransactionResponseItem, error)
}

func (s *repositoryStub) GetYearlyReport(userID uuid.UUID, year int) ([]YearlyReportRow, error) {
	return s.getYearlyReportFn(userID, year)
}

func (s *repositoryStub) GetMonthlyReportRange(userID uuid.UUID, startDate, endDate time.Time) ([]YearlyReportRow, error) {
	return s.getMonthlyReportRangeFn(userID, startDate, endDate)
}

func (s *repositoryStub) GetDashboardMonthlyReportRange(userID uuid.UUID, startDate, endDate time.Time) ([]YearlyReportRow, error) {
	return s.getDashboardMonthlyReportRangeFn(userID, startDate, endDate)
}

func (s *repositoryStub) GetTopReportItems(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]YearlyReportTopItemRow, error) {
	return s.getTopReportItemsFn(userID, startDate, endDate, limit)
}

func (s *repositoryStub) GetBalanceByAccountCode(userID uuid.UUID, code string) (*int64, error) {
	return s.getBalanceByAccountCodeFn(userID, code)
}

func (s *repositoryStub) GetAllBalances(userID uuid.UUID) ([]*AccountBalance, error) {
	return s.getAllBalancesFn(userID)
}

func (s *repositoryStub) GetRecentExpenseTransactions(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]*transaction.TransactionResponseItem, error) {
	return s.getRecentExpenseFn(userID, startDate, endDate, limit)
}

func (s *repositoryStub) GetTopExpenseTransactions(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]*transaction.TransactionResponseItem, error) {
	return s.getTopExpenseFn(userID, startDate, endDate, limit)
}

func TestGetReportBuildsNestedCategoriesAndPreloadsTopItems(t *testing.T) {
	t.Parallel()

	childCode := "salary"
	svc := &service{
		repo: &repositoryStub{
			getMonthlyReportRangeFn: func(userID uuid.UUID, startDate, endDate time.Time) ([]YearlyReportRow, error) {
				return []YearlyReportRow{
					{Year: 2026, Month: 1, Parent: "income", Total: 120000, IsParent: true},
					{Year: 2026, Month: 1, Parent: "income", Child: &childCode, Total: 120000, IsParent: false},
					{Year: 2025, Month: 12, Parent: "income", Total: 999, IsParent: true},
				}, nil
			},
			getTopReportItemsFn: func(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]YearlyReportTopItemRow, error) {
				if limit != 5 {
					t.Fatalf("expected top item limit 5, got %d", limit)
				}
				return []YearlyReportTopItemRow{
					{Year: 2026, Month: 1, Child: childCode, Description: "January salary", Amount: 90000, Rank: 1},
					{Year: 2026, Month: 1, Child: childCode, Description: "Bonus", Amount: 30000, Rank: 2},
					{Year: 2025, Month: 12, Child: childCode, Description: "Ignored", Amount: 1000, Rank: 1},
				}, nil
			},
		},
	}

	got, err := svc.GetReport(uuid.New(), 2026)
	if err != nil {
		t.Fatalf("GetReport returned error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 parent category, got %d", len(got))
	}

	parent := got[0]
	if parent.CategoryCode != "income" {
		t.Fatalf("expected parent code income, got %q", parent.CategoryCode)
	}
	if parent.Monthly[0] != 120000 {
		t.Fatalf("expected January total 120000, got %d", parent.Monthly[0])
	}
	if len(parent.Children) != 1 {
		t.Fatalf("expected one child category, got %d", len(parent.Children))
	}
	if parent.Children[0].CategoryCode != childCode {
		t.Fatalf("expected child code %q, got %q", childCode, parent.Children[0].CategoryCode)
	}
	if parent.Children[0].Monthly[0] != 120000 {
		t.Fatalf("expected child January total 120000, got %d", parent.Children[0].Monthly[0])
	}
	if len(parent.Children[0].TopItemsByMonth) != 12 {
		t.Fatalf("expected child top-items array to align with 12 months, got %d", len(parent.Children[0].TopItemsByMonth))
	}
	if len(parent.Children[0].TopItemsByMonth[0]) != 2 {
		t.Fatalf("expected 2 top items in January, got %d", len(parent.Children[0].TopItemsByMonth[0]))
	}
	if parent.Children[0].TopItemsByMonth[0][0].Description != "January salary" {
		t.Fatalf("unexpected first top item: %#v", parent.Children[0].TopItemsByMonth[0][0])
	}
	if parent.TopItemsByMonth != nil {
		t.Fatalf("expected parent row to have no top items, got %#v", parent.TopItemsByMonth)
	}
}

func TestGetReportRangeMapsMonthsRelativeToRangeAndKeepsParentOrder(t *testing.T) {
	t.Parallel()

	childCode := "rent"
	startDate := time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC)
	endDate := startDate.AddDate(1, 0, 0)

	svc := &service{
		repo: &repositoryStub{
			getMonthlyReportRangeFn: func(userID uuid.UUID, gotStart, gotEnd time.Time) ([]YearlyReportRow, error) {
				if !gotStart.Equal(startDate) || !gotEnd.Equal(endDate) {
					t.Fatalf("unexpected range: got %s - %s", gotStart, gotEnd)
				}

				return []YearlyReportRow{
					{Year: 2025, Month: 4, Parent: "income", Total: 2000, IsParent: true},
					{Year: 2025, Month: 6, Parent: "housing", Total: -900, IsParent: true},
					{Year: 2025, Month: 6, Parent: "housing", Child: &childCode, Total: -900, IsParent: false},
					{Year: 2026, Month: 4, Parent: "ignored", Total: 999, IsParent: true},
				}, nil
			},
			getTopReportItemsFn: func(userID uuid.UUID, gotStart, gotEnd time.Time, limit int) ([]YearlyReportTopItemRow, error) {
				return []YearlyReportTopItemRow{
					{Year: 2025, Month: 6, Child: childCode, Description: "Rent", Amount: -900, Rank: 1},
					{Year: 2026, Month: 4, Child: childCode, Description: "Ignored", Amount: -50, Rank: 1},
				}, nil
			},
		},
	}

	got, err := svc.getReportRange(uuid.New(), startDate, endDate)
	if err != nil {
		t.Fatalf("getReportRange returned error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 parent categories, got %d", len(got))
	}
	if got[0].CategoryCode != "income" || got[1].CategoryCode != "housing" {
		t.Fatalf("expected parent order [income housing], got [%s %s]", got[0].CategoryCode, got[1].CategoryCode)
	}
	if got[0].Monthly[0] != 2000 {
		t.Fatalf("expected income total in first month, got %d", got[0].Monthly[0])
	}
	if got[1].Monthly[2] != -900 {
		t.Fatalf("expected housing total in third month slot, got %d", got[1].Monthly[2])
	}
	if len(got[1].Children) != 1 || got[1].Children[0].Monthly[2] != -900 {
		t.Fatalf("expected nested child total in third month slot, got %#v", got[1].Children)
	}
	if len(got[1].Children[0].TopItemsByMonth[2]) != 1 || got[1].Children[0].TopItemsByMonth[2][0].Description != "Rent" {
		t.Fatalf("expected nested child top item in third month slot, got %#v", got[1].Children[0].TopItemsByMonth)
	}
}

func TestGetDashboardBuildsExpectedTimeWindowsAndCombinesData(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	expectedBalances := []*AccountBalance{{AccountCode: "checking", Balance: 1500}}
	expectedRecent := []*transaction.TransactionResponseItem{{Description: "rent"}}
	expectedTop := []*transaction.TransactionResponseItem{{Description: "flight"}}

	var gotTimelineStart time.Time
	var gotTimelineEnd time.Time
	var gotRecentStart time.Time
	var gotRecentEnd time.Time
	var gotRecentLimit int
	var gotTopStart time.Time
	var gotTopEnd time.Time
	var gotTopLimit int

	svc := &service{
		repo: &repositoryStub{
			getDashboardMonthlyReportRangeFn: func(gotUserID uuid.UUID, startDate, endDate time.Time) ([]YearlyReportRow, error) {
				if gotUserID != userID {
					t.Fatalf("unexpected user id: %s", gotUserID)
				}
				gotTimelineStart = startDate
				gotTimelineEnd = endDate
				return []YearlyReportRow{{Year: 2025, Month: 4, Parent: "income", Total: 10, IsParent: true}}, nil
			},
			getAllBalancesFn: func(gotUserID uuid.UUID) ([]*AccountBalance, error) {
				if gotUserID != userID {
					t.Fatalf("unexpected user id for balances: %s", gotUserID)
				}
				return expectedBalances, nil
			},
			getRecentExpenseFn: func(gotUserID uuid.UUID, startDate, endDate time.Time, limit int) ([]*transaction.TransactionResponseItem, error) {
				gotRecentStart = startDate
				gotRecentEnd = endDate
				gotRecentLimit = limit
				return expectedRecent, nil
			},
			getTopExpenseFn: func(gotUserID uuid.UUID, startDate, endDate time.Time, limit int) ([]*transaction.TransactionResponseItem, error) {
				gotTopStart = startDate
				gotTopEnd = endDate
				gotTopLimit = limit
				return expectedTop, nil
			},
			getTopReportItemsFn: func(userID uuid.UUID, startDate, endDate time.Time, limit int) ([]YearlyReportTopItemRow, error) {
				return nil, nil
			},
			getMonthlyReportRangeFn:   func(userID uuid.UUID, startDate, endDate time.Time) ([]YearlyReportRow, error) { return nil, nil },
			getYearlyReportFn:         func(userID uuid.UUID, year int) ([]YearlyReportRow, error) { return nil, nil },
			getBalanceByAccountCodeFn: func(userID uuid.UUID, code string) (*int64, error) { return nil, nil },
		},
	}

	got, err := svc.GetDashboard(userID, 2026, 3)
	if err != nil {
		t.Fatalf("GetDashboard returned error: %v", err)
	}

	expectedTimelineStart := time.Date(2025, time.April, 1, 0, 0, 0, 0, time.UTC)
	expectedTimelineEnd := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)
	expectedMonthStart := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	expectedMonthEnd := time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC)

	if !gotTimelineStart.Equal(expectedTimelineStart) || !gotTimelineEnd.Equal(expectedTimelineEnd) {
		t.Fatalf("unexpected timeline range: got %s - %s", gotTimelineStart, gotTimelineEnd)
	}
	if !gotRecentStart.Equal(expectedMonthStart) || !gotRecentEnd.Equal(expectedMonthEnd) || gotRecentLimit != 6 {
		t.Fatalf("unexpected recent transaction query: %s - %s limit %d", gotRecentStart, gotRecentEnd, gotRecentLimit)
	}
	if !gotTopStart.Equal(expectedMonthStart) || !gotTopEnd.Equal(expectedMonthEnd) || gotTopLimit != 5 {
		t.Fatalf("unexpected top transaction query: %s - %s limit %d", gotTopStart, gotTopEnd, gotTopLimit)
	}
	if got.Year != 2026 || got.Month != 3 {
		t.Fatalf("unexpected dashboard period: %#v", got)
	}
	if len(got.Balances) != 1 || got.Balances[0] != expectedBalances[0] {
		t.Fatalf("expected balances to be propagated, got %#v", got.Balances)
	}
	if len(got.RecentTransactions) != 1 || got.RecentTransactions[0] != expectedRecent[0] {
		t.Fatalf("expected recent transactions to be propagated, got %#v", got.RecentTransactions)
	}
	if len(got.TopExpenses) != 1 || got.TopExpenses[0] != expectedTop[0] {
		t.Fatalf("expected top expenses to be propagated, got %#v", got.TopExpenses)
	}
}

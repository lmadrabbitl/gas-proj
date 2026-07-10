package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"expense-tracker/internal/account"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type StatementRow struct {
	RowNumber    int
	Date         time.Time
	Description  string
	AmountCents  int64
	BalanceCents *int64
	RawCells     []string
}

type DBRow struct {
	ID              uuid.UUID
	Date            time.Time
	Description     string
	AmountCents     int64
	AccountCode     string
	CategoryCode    string
	TransferID      *int64
	TransferAccount *string
	IsVisible       bool
	ExcludeFromDash bool
}

type DayBucket struct {
	Date       time.Time
	Rows       []StatementRow
	TotalCents int64
	Balance    *int64
}

type DBDayBucket struct {
	Date       time.Time
	Rows       []DBRow
	TotalCents int64
}

type DayMatch struct {
	Statement DayBucket
	DB        *DBDayBucket
	Score     int64
	Delta     int64
}

type RowMatch struct {
	Statement StatementRow
	DB        *DBRow
	Score     int64
	Delta     int64
}

type Result struct {
	StatementRows        []StatementRow
	DBRows               []DBRow
	StatementDays        []DayBucket
	DBDays               []DBDayBucket
	DayMatches           []DayMatch
	UnmatchedStatement   []DayBucket
	UnmatchedDB          []DBDayBucket
	ResidualDiffCents    int64
	MatchedPairs         int
	MatchedStatementDays int
	MatchedDBDays        int
}

const maxInt64 = int64(^uint64(0) >> 1)

func (r Result) Summary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "statement rows: %d\n", len(r.StatementRows))
	fmt.Fprintf(&b, "db rows: %d\n", len(r.DBRows))
	fmt.Fprintf(&b, "matched days: %d\n", r.MatchedPairs)
	fmt.Fprintf(&b, "unmatched statement days: %d\n", len(r.UnmatchedStatement))
	fmt.Fprintf(&b, "unmatched db days: %d\n", len(r.UnmatchedDB))
	fmt.Fprintf(&b, "residual difference: %s\n", formatMoneyCents(r.ResidualDiffCents))

	if len(r.UnmatchedStatement) > 0 {
		b.WriteString("\nUnmatched statement days:\n")
		for _, day := range r.UnmatchedStatement {
			fmt.Fprintf(&b, "  %s total=%s rows=%d\n", day.Date.Format("2006-01-02"), formatMoneyCents(day.TotalCents), len(day.Rows))
		}
	}
	if len(r.UnmatchedDB) > 0 {
		b.WriteString("\nUnmatched db days:\n")
		for _, day := range r.UnmatchedDB {
			fmt.Fprintf(&b, "  %s total=%s rows=%d\n", day.Date.Format("2006-01-02"), formatMoneyCents(day.TotalCents), len(day.Rows))
			for _, row := range day.Rows {
				fmt.Fprintf(&b, "    db %s | %s | %s\n", row.Date.Format("2006-01-02"), row.Description, formatMoneyCents(row.AmountCents))
			}
		}
	}

	if len(r.DayMatches) > 0 {
		b.WriteString("\nDiscrepant matched days:\n")
		for _, match := range r.DayMatches {
			if match.Delta == 0 {
				continue
			}
			if match.DB == nil {
				continue
			}
			fmt.Fprintf(&b, "  stmt %s -> db %s | delta=%s | stmt=%s | db=%s\n",
				match.Statement.Date.Format("2006-01-02"),
				match.DB.Date.Format("2006-01-02"),
				formatMoneyCents(match.Delta),
				formatMoneyCents(match.Statement.TotalCents),
				formatMoneyCents(match.DB.TotalCents),
			)
			for _, row := range match.Statement.Rows {
				fmt.Fprintf(&b, "    stmt %s | %s | %s\n", row.Date.Format("2006-01-02"), row.Description, formatMoneyCents(row.AmountCents))
			}
			for _, row := range match.DB.Rows {
				fmt.Fprintf(&b, "    db   %s | %s | %s\n", row.Date.Format("2006-01-02"), row.Description, formatMoneyCents(row.AmountCents))
			}
		}
	}
	return b.String()
}

func Run(db *gorm.DB, cfg *Config) (*Result, error) {
	stmtRows, err := readStatementRows(cfg.StatementFile, cfg.StatementSheet)
	if err != nil {
		return nil, err
	}
	sort.Slice(stmtRows, func(i, j int) bool {
		if stmtRows[i].Date.Equal(stmtRows[j].Date) {
			return stmtRows[i].RowNumber < stmtRows[j].RowNumber
		}
		return stmtRows[i].Date.Before(stmtRows[j].Date)
	})

	userID, err := uuid.Parse(strings.TrimSpace(cfg.UserID))
	if err != nil {
		return nil, fmt.Errorf("parse user_id: %w", err)
	}

	dbRows, err := loadDBRows(db, userID, cfg.BrokerageAccountCode, cfg.IncludeHidden, stmtRows)
	if err != nil {
		return nil, err
	}

	stmtDays := bucketStatementRows(stmtRows)
	dbDays := bucketDBRows(dbRows)
	dayMatches, unmatchedStmt, unmatchedDB := matchDays(stmtDays, dbDays, cfg.DateToleranceDays, cfg.AmountToleranceCents)

	var residual int64
	for _, match := range dayMatches {
		if match.DB != nil {
			residual += match.Statement.TotalCents - match.DB.TotalCents
		}
	}
	for _, day := range unmatchedStmt {
		residual += day.TotalCents
	}
	for _, day := range unmatchedDB {
		residual -= day.TotalCents
	}

	return &Result{
		StatementRows:        stmtRows,
		DBRows:               dbRows,
		StatementDays:        stmtDays,
		DBDays:               dbDays,
		DayMatches:           dayMatches,
		UnmatchedStatement:   unmatchedStmt,
		UnmatchedDB:          unmatchedDB,
		ResidualDiffCents:    residual,
		MatchedPairs:         len(dayMatches),
		MatchedStatementDays: len(dayMatches),
		MatchedDBDays:        len(dayMatches),
	}, nil
}

func loadDBRows(db *gorm.DB, userID uuid.UUID, accountCode string, includeHidden bool, statementRows []StatementRow) ([]DBRow, error) {
	if len(statementRows) == 0 {
		return nil, fmt.Errorf("no statement rows found")
	}

	startDate := statementRows[0].Date.AddDate(0, 0, -5)
	endDate := statementRows[len(statementRows)-1].Date.AddDate(0, 0, 5)

	var acc account.Account
	if err := db.Where("user_id = ? AND code = ?", userID, accountCode).First(&acc).Error; err != nil {
		return nil, err
	}

	type row struct {
		ID              uuid.UUID `gorm:"column:id"`
		Date            time.Time `gorm:"column:date"`
		Description     string    `gorm:"column:description"`
		Amount          int64     `gorm:"column:amount"`
		AccountCode     string    `gorm:"column:account_code"`
		CategoryCode    string    `gorm:"column:category_code"`
		TransferID      *int64    `gorm:"column:transfer_id"`
		TransferAccount *string   `gorm:"column:transfer_account_code"`
		IsVisible       bool      `gorm:"column:is_visible"`
		ExcludeFromDash bool      `gorm:"column:exclude_from_dashboard"`
	}

	var rows []row
	query := db.Table("transactions t").
		Select(`
			t.id,
			t.date,
			t.description,
			t.amount,
			a.code as account_code,
			c.code as category_code,
			t.transfer_id,
			a2.code as transfer_account_code,
			COALESCE(t.is_visible, TRUE) as is_visible,
			COALESCE(t.exclude_from_dashboard, FALSE) as exclude_from_dashboard
		`).
		Joins("JOIN accounts a ON a.id = t.account_id").
		Joins("JOIN categories c ON c.id = t.category_id").
		Joins("LEFT JOIN accounts a2 ON a2.id = t.transfer_account_id").
		Where("t.user_id = ?", userID).
		Where("a.code = ?", accountCode).
		Where("t.date >= ? AND t.date <= ?", startDate, endDate).
		Order("t.date ASC, t.created_at ASC, t.id ASC")
	if !includeHidden {
		query = query.Where("COALESCE(t.is_visible, TRUE) = TRUE")
	}

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make([]DBRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, DBRow{
			ID:              r.ID,
			Date:            truncateDate(r.Date),
			Description:     r.Description,
			AmountCents:     r.Amount,
			AccountCode:     r.AccountCode,
			CategoryCode:    r.CategoryCode,
			TransferID:      r.TransferID,
			TransferAccount: r.TransferAccount,
			IsVisible:       r.IsVisible,
			ExcludeFromDash: r.ExcludeFromDash,
		})
	}
	return out, nil
}

func bucketStatementRows(rows []StatementRow) []DayBucket {
	type acc struct {
		day DayBucket
	}
	m := map[string]*DayBucket{}
	for _, r := range rows {
		key := r.Date.Format("2006-01-02")
		b, ok := m[key]
		if !ok {
			day := truncateDate(r.Date)
			b = &DayBucket{Date: day}
			m[key] = b
		}
		b.Rows = append(b.Rows, r)
		b.TotalCents += r.AmountCents
		if r.BalanceCents != nil {
			b.Balance = r.BalanceCents
		}
	}
	out := make([]DayBucket, 0, len(m))
	for _, b := range m {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

func bucketDBRows(rows []DBRow) []DBDayBucket {
	m := map[string]*DBDayBucket{}
	for _, r := range rows {
		key := r.Date.Format("2006-01-02")
		b, ok := m[key]
		if !ok {
			day := truncateDate(r.Date)
			b = &DBDayBucket{Date: day}
			m[key] = b
		}
		b.Rows = append(b.Rows, r)
		b.TotalCents += r.AmountCents
	}
	out := make([]DBDayBucket, 0, len(m))
	for _, b := range m {
		out = append(out, *b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

func matchDays(stmtDays []DayBucket, dbDays []DBDayBucket, toleranceDays int, amountTolerance int64) ([]DayMatch, []DayBucket, []DBDayBucket) {
	matches := make([]DayMatch, 0, len(stmtDays))
	unmatchedStmt := make([]DayBucket, 0)
	usedDB := make([]bool, len(dbDays))
	dbIndex := 0

	for _, stmt := range stmtDays {
		bestIdx := -1
		bestScore := maxInt64
		for i := dbIndex; i < len(dbDays); i++ {
			if usedDB[i] {
				continue
			}
			diffDays := absInt(daysBetween(stmt.Date, dbDays[i].Date))
			if diffDays > toleranceDays {
				if dbDays[i].Date.After(stmt.Date) && diffDays > toleranceDays {
					break
				}
				continue
			}
			amountDiff := absInt64(stmt.TotalCents - dbDays[i].TotalCents)
			score := int64(diffDays)*1000 + amountDiff
			if amountDiff <= amountTolerance {
				score -= 100
			}
			if score < bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		if bestIdx == -1 {
			unmatchedStmt = append(unmatchedStmt, stmt)
			continue
		}
		usedDB[bestIdx] = true
		dbIndex = bestIdx + 1
		diff := stmt.TotalCents - dbDays[bestIdx].TotalCents
		matches = append(matches, DayMatch{
			Statement: stmt,
			DB:        &dbDays[bestIdx],
			Score:     bestScore,
			Delta:     diff,
		})
	}

	unmatchedDB := make([]DBDayBucket, 0)
	for i, day := range dbDays {
		if !usedDB[i] {
			unmatchedDB = append(unmatchedDB, day)
		}
	}
	return matches, unmatchedStmt, unmatchedDB
}

func matchRows(stmtRows []StatementRow, dbRows []DBRow, toleranceDays int, amountTolerance int64) ([]RowMatch, []StatementRow, []DBRow) {
	matches := make([]RowMatch, 0, len(stmtRows))
	unmatchedStmt := make([]StatementRow, 0)
	usedDB := make([]bool, len(dbRows))

	for _, stmt := range stmtRows {
		bestIdx := -1
		bestScore := maxInt64
		for i := range dbRows {
			if usedDB[i] {
				continue
			}
			diffDays := absInt(daysBetween(stmt.Date, dbRows[i].Date))
			if diffDays > toleranceDays {
				continue
			}
			amountDiff := absInt64(stmt.AmountCents - dbRows[i].AmountCents)
			score := int64(diffDays)*1000 + amountDiff + int64(descriptionDistance(stmt.Description, dbRows[i].Description))
			if amountDiff <= amountTolerance {
				score -= 50
			}
			if score < bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		if bestIdx == -1 {
			unmatchedStmt = append(unmatchedStmt, stmt)
			continue
		}
		usedDB[bestIdx] = true
		matches = append(matches, RowMatch{
			Statement: stmt,
			DB:        &dbRows[bestIdx],
			Score:     bestScore,
			Delta:     stmt.AmountCents - dbRows[bestIdx].AmountCents,
		})
	}

	unmatchedDB := make([]DBRow, 0)
	for i, row := range dbRows {
		if !usedDB[i] {
			unmatchedDB = append(unmatchedDB, row)
		}
	}
	return matches, unmatchedStmt, unmatchedDB
}

func descriptionDistance(a, b string) int {
	na := normalizeText(a)
	nb := normalizeText(b)
	if na == nb {
		return 0
	}
	if strings.Contains(na, nb) || strings.Contains(nb, na) {
		return 1
	}
	at := textTokens(na)
	bt := textTokens(nb)
	if len(at) == 0 || len(bt) == 0 {
		return 8
	}
	shared := 0
	for t := range at {
		if _, ok := bt[t]; ok {
			shared++
		}
	}
	union := len(at) + len(bt) - shared
	if union == 0 {
		return 8
	}
	similarity := float64(shared) / float64(union)
	switch {
	case similarity >= 0.8:
		return 1
	case similarity >= 0.5:
		return 3
	case similarity >= 0.25:
		return 6
	default:
		return 10
	}
}

func normalizeText(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	replacer := strings.NewReplacer(
		"Á", "A", "À", "A", "Ã", "A", "Â", "A",
		"É", "E", "È", "E", "Ê", "E",
		"Í", "I",
		"Ó", "O", "Ò", "O", "Õ", "O", "Ô", "O",
		"Ú", "U",
		"Ç", "C",
		"á", "A", "à", "A", "ã", "A", "â", "A",
		"é", "E", "è", "E", "ê", "E",
		"í", "I",
		"ó", "O", "ò", "O", "õ", "O", "ô", "O",
		"ú", "U",
		"ç", "C",
		"(", " ", ")", " ", ".", " ", ",", " ", ":", " ", ";", " ",
		"/", " ", "-", " ", "_", " ",
	)
	s = replacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func textTokens(s string) map[string]struct{} {
	stop := map[string]struct{}{
		"DE": {}, "DA": {}, "DO": {}, "DAS": {}, "DOS": {}, "E": {}, "S": {}, "S/": {},
		"CLIENTES": {}, "CLIENTE": {}, "NOTA": {}, "NOTAS": {}, "OPERACOES": {}, "OPERACAO": {},
		"EM": {}, "BOLSA": {}, "COMPRA": {}, "VENDA": {}, "RESULTADO": {}, "RENDIMENTOS": {},
		"DIVIDENDOS": {}, "JUROS": {}, "CAPITAL": {}, "PAGAMENTO": {}, "AMORTIZACAO": {},
		"TRANSFERENCIA": {}, "DEBITO": {}, "CREDITO": {}, "BONIFICACAO": {},
	}
	out := map[string]struct{}{}
	for _, token := range strings.Fields(s) {
		if _, ok := stop[token]; ok {
			continue
		}
		if len(token) < 2 {
			continue
		}
		out[token] = struct{}{}
	}
	return out
}

func readStatementRows(path, sheetName string) ([]StatementRow, error) {
	if strings.HasSuffix(strings.ToLower(path), ".csv") {
		return readStatementCSV(path)
	}
	return readStatementXLSX(path, sheetName)
}

func readStatementCSV(path string) ([]StatementRow, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	return parseStatementTable(records)
}

func parseStatementTable(records [][]string) ([]StatementRow, error) {
	rows := make([]StatementRow, 0)
	for i, rec := range records {
		if len(rec) == 0 {
			continue
		}
		row, ok := parseStatementRecord(i+1, rec)
		if !ok {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseStatementRecord(rowNum int, cells []string) (StatementRow, bool) {
	if len(cells) == 0 {
		return StatementRow{}, false
	}

	var dateIdx = -1
	for i, cell := range cells {
		if _, err := parseDate(cell); err == nil {
			dateIdx = i
			break
		}
	}
	if dateIdx == -1 {
		return StatementRow{}, false
	}

	date, err := parseDate(cells[dateIdx])
	if err != nil {
		return StatementRow{}, false
	}

	descIdx := -1
	for i := dateIdx + 1; i < len(cells); i++ {
		if strings.TrimSpace(cells[i]) == "" {
			continue
		}
		if _, err := parseDate(cells[i]); err == nil {
			continue
		}
		if isMoneyLike(cells[i]) {
			break
		}
		descIdx = i
		break
	}
	if descIdx == -1 {
		descIdx = minInt(dateIdx+1, len(cells)-1)
	}

	description := strings.TrimSpace(cells[descIdx])
	moneyCells := make([]int, 0, 2)
	for i := descIdx + 1; i < len(cells); i++ {
		if isMoneyLike(cells[i]) {
			moneyCells = append(moneyCells, i)
		}
	}
	if len(moneyCells) == 0 {
		return StatementRow{}, false
	}

	amount, err := parseMoneyCents(cells[moneyCells[0]])
	if err != nil {
		return StatementRow{}, false
	}
	var balance *int64
	if len(moneyCells) > 1 {
		if parsed, err := parseMoneyCents(cells[moneyCells[len(moneyCells)-1]]); err == nil {
			balance = &parsed
		}
	}

	return StatementRow{
		RowNumber:    rowNum,
		Date:         truncateDate(date),
		Description:  description,
		AmountCents:  amount,
		BalanceCents: balance,
		RawCells:     append([]string(nil), cells...),
	}, true
}

func isMoneyLike(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, "R$") {
		return true
	}
	if strings.ContainsAny(s, ",.") {
		_, err := parseMoneyCents(s)
		return err == nil
	}
	return false
}

func parseMoneyCents(input string) (int64, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return 0, fmt.Errorf("empty money string")
	}
	s = strings.ReplaceAll(s, "R$", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\u00a0", "")

	sign := int64(1)
	if strings.HasPrefix(s, "-") {
		sign = -1
		s = strings.TrimPrefix(s, "-")
	}
	if strings.HasPrefix(s, "+") {
		s = strings.TrimPrefix(s, "+")
	}

	decimalSep := byte(0)
	lastDot := strings.LastIndexByte(s, '.')
	lastComma := strings.LastIndexByte(s, ',')
	switch {
	case lastDot >= 0 && lastComma >= 0:
		if lastDot > lastComma {
			decimalSep = '.'
		} else {
			decimalSep = ','
		}
	case lastComma >= 0:
		if len(s)-lastComma-1 <= 2 {
			decimalSep = ','
		}
	case lastDot >= 0:
		if len(s)-lastDot-1 <= 2 {
			decimalSep = '.'
		}
	}

	if decimalSep == ',' {
		s = strings.ReplaceAll(s, ".", "")
		s = strings.ReplaceAll(s, ",", ".")
	} else if decimalSep == '.' {
		s = strings.ReplaceAll(s, ",", "")
	}

	if !strings.Contains(s, ".") {
		parsed, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, err
		}
		return sign * parsed * 100, nil
	}

	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 {
		return 0, fmt.Errorf("invalid money string: %q", input)
	}
	whole := parts[0]
	frac := parts[1]
	if len(frac) > 2 {
		frac = frac[:2]
	}
	for len(frac) < 2 {
		frac += "0"
	}

	wholeVal := int64(0)
	if whole != "" {
		var err error
		wholeVal, err = strconv.ParseInt(whole, 10, 64)
		if err != nil {
			return 0, err
		}
	}
	fracVal, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, err
	}
	return sign * (wholeVal*100 + fracVal), nil
}

func formatMoneyCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%sR$ %d,%02d", sign, cents/100, cents%100)
}

func truncateDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func daysBetween(a, b time.Time) int {
	da := truncateDate(a)
	db := truncateDate(b)
	return int(db.Sub(da).Hours() / 24)
}

func absInt(v int) int {
	if v < 0 {
		return v * -1
	}
	return v
}

func absInt64(v int64) int64 {
	if v < 0 {
		return v * -1
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

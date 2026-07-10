package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"expense-tracker/internal/database"
)

type Config struct {
	DatabaseURL          string `json:"database_url"`
	UserID               string `json:"user_id"`
	BrokerageAccountCode string `json:"brokerage_account_code"`
	StatementFile        string `json:"statement_file"`
	StatementSheet       string `json:"statement_sheet"`
	DateToleranceDays    int    `json:"date_tolerance_days"`
	AmountToleranceCents int64  `json:"amount_tolerance_cents"`
	IncludeHidden        bool   `json:"include_hidden"`
}

func main() {
	var configPath string
	var statementFile string
	var sheetName string
	var userID string
	var accountCode string
	var dateTolerance int
	var amountTolerance int64

	flag.StringVar(&configPath, "config", "cmd/reconcile/config.json", "path to JSON config file")
	flag.StringVar(&statementFile, "statement", "", "path to brokerage statement (.xlsx or .csv)")
	flag.StringVar(&sheetName, "sheet", "", "sheet name for xlsx input")
	flag.StringVar(&userID, "user-id", "", "user UUID")
	flag.StringVar(&accountCode, "account-code", "", "brokerage account code")
	flag.IntVar(&dateTolerance, "date-tolerance-days", 5, "max day distance for fuzzy matching")
	flag.Int64Var(&amountTolerance, "amount-tolerance-cents", 1, "max amount difference in cents")
	flag.Parse()

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if statementFile != "" {
		cfg.StatementFile = statementFile
	}
	if sheetName != "" {
		cfg.StatementSheet = sheetName
	}
	if userID != "" {
		cfg.UserID = userID
	}
	if accountCode != "" {
		cfg.BrokerageAccountCode = accountCode
	}
	if dateTolerance > 0 {
		cfg.DateToleranceDays = dateTolerance
	}
	if amountTolerance >= 0 {
		cfg.AmountToleranceCents = amountTolerance
	}

	if strings.TrimSpace(cfg.UserID) == "" {
		log.Fatal("user_id is required")
	}
	if strings.TrimSpace(cfg.BrokerageAccountCode) == "" {
		log.Fatal("brokerage_account_code is required")
	}
	if strings.TrimSpace(cfg.StatementFile) == "" {
		log.Fatal("statement_file is required")
	}
	if cfg.DateToleranceDays <= 0 {
		cfg.DateToleranceDays = 5
	}

	if strings.TrimSpace(cfg.DatabaseURL) != "" {
		if err := os.Setenv("DATABASE_URL", cfg.DatabaseURL); err != nil {
			log.Fatalf("set DATABASE_URL: %v", err)
		}
	}

	db := database.New()
	result, err := Run(db, cfg)
	if err != nil {
		log.Fatalf("reconcile: %v", err)
	}

	fmt.Println(result.Summary())
}

func loadConfig(path string) (*Config, error) {
	cfg := &Config{
		DateToleranceDays:    5,
		AmountToleranceCents: 1,
		IncludeHidden:        true,
	}

	if strings.TrimSpace(path) == "" {
		return cfg, nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, err
	}

	if cfg.DateToleranceDays == 0 {
		cfg.DateToleranceDays = 5
	}
	if cfg.AmountToleranceCents == 0 {
		cfg.AmountToleranceCents = 1
	}
	return cfg, nil
}

func parseDate(input string) (time.Time, error) {
	layouts := []string{
		"2006-01-02",
		"02/01/2006",
		"2/1/2006",
		"02/01/06",
		"2/1/06",
		"02 Jan 2006",
		"02 Jan. 2006",
		"2 Jan 2006",
		"2 Jan. 2006",
		"02 / Jan / 2006",
		"02 / Jan. / 2006",
		"02 / jan / 2006",
		"02 / jan. / 2006",
		"2 / jan / 2006",
		"2 / jan. / 2006",
	}

	cleaned := strings.TrimSpace(strings.ReplaceAll(input, "\u00a0", " "))
	cleaned = strings.ReplaceAll(cleaned, "  ", " ")
	cleaned = strings.ReplaceAll(cleaned, " / ", "/")
	cleaned = strings.ReplaceAll(cleaned, " . ", ".")
	cleaned = strings.ReplaceAll(cleaned, "jan.", "jan")
	cleaned = strings.ReplaceAll(cleaned, "fev.", "fev")
	cleaned = strings.ReplaceAll(cleaned, "mar.", "mar")
	cleaned = strings.ReplaceAll(cleaned, "abr.", "abr")
	cleaned = strings.ReplaceAll(cleaned, "mai.", "mai")
	cleaned = strings.ReplaceAll(cleaned, "jun.", "jun")
	cleaned = strings.ReplaceAll(cleaned, "jul.", "jul")
	cleaned = strings.ReplaceAll(cleaned, "ago.", "ago")
	cleaned = strings.ReplaceAll(cleaned, "set.", "set")
	cleaned = strings.ReplaceAll(cleaned, "out.", "out")
	cleaned = strings.ReplaceAll(cleaned, "nov.", "nov")
	cleaned = strings.ReplaceAll(cleaned, "dez.", "dez")
	cleaned = strings.ReplaceAll(strings.ToLower(cleaned), "  ", " ")
	cleaned = strings.ReplaceAll(cleaned, " /", "/")
	cleaned = strings.ReplaceAll(cleaned, "/ ", "/")

	for _, layout := range layouts {
		if parsed, err := time.ParseInLocation(layout, cleaned, time.UTC); err == nil {
			return parsed, nil
		}
	}

	monthMap := map[string]string{
		"jan": "Jan", "fev": "Feb", "mar": "Mar", "abr": "Apr", "mai": "May", "jun": "Jun",
		"jul": "Jul", "ago": "Aug", "set": "Sep", "out": "Oct", "nov": "Nov", "dez": "Dec",
	}
	for k, v := range monthMap {
		cleaned = strings.ReplaceAll(cleaned, k, v)
	}
	for _, layout := range []string{"02/Jan/2006", "2/Jan/2006", "02/Jan/06", "2/Jan/06"} {
		if parsed, err := time.ParseInLocation(layout, cleaned, time.UTC); err == nil {
			return parsed, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date %q", input)
}

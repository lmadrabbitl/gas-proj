package suggestion

import (
	"expense-tracker/internal/account"
	"expense-tracker/internal/category"
	appErr "expense-tracker/internal/errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type inMemorySuggestionRepo struct {
	items map[uuid.UUID]*Suggestion
}

func newInMemorySuggestionRepo() *inMemorySuggestionRepo {
	return &inMemorySuggestionRepo{
		items: map[uuid.UUID]*Suggestion{},
	}
}

func (r *inMemorySuggestionRepo) Create(suggestion *Suggestion) (*Suggestion, error) {
	copyValue := *suggestion
	now := time.Now()
	copyValue.CreatedAt = now
	copyValue.UpdatedAt = now
	r.items[suggestion.ID] = &copyValue
	return &copyValue, nil
}

func (r *inMemorySuggestionRepo) GetByID(userID, suggestionID uuid.UUID) (*Suggestion, error) {
	item, ok := r.items[suggestionID]
	if !ok || item.UserID != userID {
		return nil, appErr.ErrSuggestionNotFound()
	}
	copyValue := *item
	return &copyValue, nil
}

func (r *inMemorySuggestionRepo) GetDTOByID(userID, suggestionID uuid.UUID) (*SuggestionResponseItem, error) {
	item, err := r.GetByID(userID, suggestionID)
	if err != nil {
		return nil, err
	}
	return &SuggestionResponseItem{
		ID:                  item.ID,
		DescriptionContains: item.DescriptionContains,
		Priority:            item.Priority,
		EntryType:           item.EntryType,
		CategoryCode:        codeFromCategoryID(item.CategoryID),
		AccountCode:         codeFromAccountID(item.AccountID),
		TransferAccountCode: codeFromAccountID(item.TransferAccountID),
		CreatedAt:           item.CreatedAt,
		UpdatedAt:           item.UpdatedAt,
	}, nil
}

func (r *inMemorySuggestionRepo) GetByUser(userID uuid.UUID) ([]SuggestionResponseItem, error) {
	items := make([]SuggestionResponseItem, 0, len(r.items))
	for _, item := range r.items {
		if item.UserID != userID {
			continue
		}
		dto, err := r.GetDTOByID(userID, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, *dto)
	}
	return items, nil
}

func (r *inMemorySuggestionRepo) Update(userID, suggestionID uuid.UUID, update *UpdateSuggestion) (*Suggestion, error) {
	item, err := r.GetByID(userID, suggestionID)
	if err != nil {
		return nil, err
	}
	if update.DescriptionContains != nil {
		item.DescriptionContains = *update.DescriptionContains
	}
	if update.Priority != nil {
		item.Priority = *update.Priority
	}
	if update.SetEntryType {
		item.EntryType = update.EntryType
	}
	if update.SetCategoryID {
		item.CategoryID = update.CategoryID
	}
	if update.SetAccountID {
		item.AccountID = update.AccountID
	}
	if update.SetTransferAccountID {
		item.TransferAccountID = update.TransferAccountID
	}
	item.UpdatedAt = time.Now()
	r.items[suggestionID] = item
	return item, nil
}

func (r *inMemorySuggestionRepo) Delete(userID, suggestionID uuid.UUID) error {
	item, err := r.GetByID(userID, suggestionID)
	if err != nil {
		return err
	}
	delete(r.items, item.ID)
	return nil
}

type accountReaderStub struct {
	byCode map[string]*account.Account
}

func (s *accountReaderStub) GetAccountByCode(userID uuid.UUID, code string) (*account.Account, error) {
	_ = userID
	if value, ok := s.byCode[code]; ok {
		return value, nil
	}
	return nil, appErr.ErrAccountNotFound()
}

type categoryReaderStub struct {
	byCode map[string]*category.Category
}

func (s *categoryReaderStub) GetCategoryByCode(userID uuid.UUID, code string) (*category.Category, error) {
	_ = userID
	if value, ok := s.byCode[code]; ok {
		return value, nil
	}
	return nil, appErr.ErrCategoryNotFound()
}

var accountIDs = map[uuid.UUID]string{}
var categoryIDs = map[uuid.UUID]string{}

func codeFromAccountID(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	value := accountIDs[*id]
	return &value
}

func codeFromCategoryID(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	value := categoryIDs[*id]
	return &value
}

func TestAddSuggestionTrimsDescriptionAndAllowsDuplicatePriority(t *testing.T) {
	userID := uuid.New()
	santanderID := uuid.New()
	quitandaID := uuid.New()
	parentID := uuid.New()

	accountIDs[santanderID] = "santander"
	categoryIDs[quitandaID] = "quitanda"

	repo := newInMemorySuggestionRepo()
	service := NewService(repo, &accountReaderStub{
		byCode: map[string]*account.Account{
			"santander": {ID: santanderID, Code: "santander"},
		},
	}, &categoryReaderStub{
		byCode: map[string]*category.Category{
			"quitanda": {ID: quitandaID, Code: "quitanda", ParentID: &parentID, Type: category.CategoryTypeExpense},
		},
	})

	first, err := service.AddSuggestion(userID, CreateSuggestionRequest{
		DescriptionContains: "  padaria ",
		Priority:            1,
		AccountCode:         stringPtr("santander"),
	})
	if err != nil {
		t.Fatalf("first AddSuggestion returned error: %v", err)
	}
	second, err := service.AddSuggestion(userID, CreateSuggestionRequest{
		DescriptionContains: "mercado",
		Priority:            1,
		CategoryCode:        stringPtr("quitanda"),
	})
	if err != nil {
		t.Fatalf("second AddSuggestion returned error: %v", err)
	}

	if first.DescriptionContains != "padaria" {
		t.Fatalf("expected trimmed description, got %q", first.DescriptionContains)
	}
	if second.Priority != 1 {
		t.Fatalf("expected duplicate priority to be accepted")
	}
}

func TestAddSuggestionRejectsParentCategory(t *testing.T) {
	userID := uuid.New()
	rootID := uuid.New()

	service := NewService(newInMemorySuggestionRepo(), &accountReaderStub{byCode: map[string]*account.Account{}}, &categoryReaderStub{
		byCode: map[string]*category.Category{
			"alimentacao": {ID: rootID, Code: "alimentacao", Type: category.CategoryTypeExpense},
		},
	})

	_, err := service.AddSuggestion(userID, CreateSuggestionRequest{
		DescriptionContains: "mercado",
		Priority:            1,
		CategoryCode:        stringPtr("alimentacao"),
	})
	if err == nil {
		t.Fatal("expected error for parent category")
	}
}

func TestAddSuggestionRejectsTransferAccountWithoutTransferType(t *testing.T) {
	userID := uuid.New()
	interID := uuid.New()
	accountIDs[interID] = "inter"

	service := NewService(newInMemorySuggestionRepo(), &accountReaderStub{
		byCode: map[string]*account.Account{
			"inter": {ID: interID, Code: "inter"},
		},
	}, &categoryReaderStub{byCode: map[string]*category.Category{}})

	_, err := service.AddSuggestion(userID, CreateSuggestionRequest{
		DescriptionContains: "pix",
		Priority:            1,
		TransferAccountCode: stringPtr("inter"),
	})
	if err == nil {
		t.Fatal("expected error for transfer account without transfer type")
	}
}

func TestAddSuggestionAllowsTransferTypeWithoutTransferAccount(t *testing.T) {
	userID := uuid.New()
	transferCategoryID := uuid.New()
	parentID := uuid.New()
	categoryIDs[transferCategoryID] = "transferencias"

	service := NewService(newInMemorySuggestionRepo(), &accountReaderStub{
		byCode: map[string]*account.Account{},
	}, &categoryReaderStub{
		byCode: map[string]*category.Category{
			"transferencias": {
				ID:       transferCategoryID,
				Code:     "transferencias",
				ParentID: &parentID,
				Type:     category.CategoryTypeMovement,
			},
		},
	})

	entryType := SuggestionEntryTypeTransfer
	created, err := service.AddSuggestion(userID, CreateSuggestionRequest{
		DescriptionContains: "pix",
		Priority:            1,
		EntryType:           &entryType,
		CategoryCode:        stringPtr("transferencias"),
	})
	if err != nil {
		t.Fatalf("expected transfer suggestion without transfer account to be allowed, got %v", err)
	}

	if created.EntryType == nil || *created.EntryType != SuggestionEntryTypeTransfer {
		t.Fatalf("expected transfer entry type to be preserved")
	}
	if created.TransferAccountCode != nil {
		t.Fatalf("expected transfer account to remain empty")
	}
}

func TestUpdateSuggestionCanClearOptionalFields(t *testing.T) {
	userID := uuid.New()
	santanderID := uuid.New()
	interID := uuid.New()
	transferCategoryID := uuid.New()
	parentID := uuid.New()

	accountIDs[santanderID] = "santander"
	accountIDs[interID] = "inter"
	categoryIDs[transferCategoryID] = "transferencias"

	service := NewService(newInMemorySuggestionRepo(), &accountReaderStub{
		byCode: map[string]*account.Account{
			"santander": {ID: santanderID, Code: "santander"},
			"inter":     {ID: interID, Code: "inter"},
		},
	}, &categoryReaderStub{
		byCode: map[string]*category.Category{
			"transferencias": {ID: transferCategoryID, Code: "transferencias", ParentID: &parentID, Type: category.CategoryTypeMovement},
		},
	})

	entryType := SuggestionEntryTypeTransfer
	created, err := service.AddSuggestion(userID, CreateSuggestionRequest{
		DescriptionContains: "pix",
		Priority:            1,
		EntryType:           &entryType,
		AccountCode:         stringPtr("santander"),
		TransferAccountCode: stringPtr("inter"),
		CategoryCode:        stringPtr("transferencias"),
	})
	if err != nil {
		t.Fatalf("AddSuggestion returned error: %v", err)
	}

	updated, err := service.UpdateSuggestion(userID, created.ID, UpdateSuggestionRequest{
		EntryType:           stringPtr(""),
		TransferAccountCode: stringPtr(""),
	})
	if err != nil {
		t.Fatalf("UpdateSuggestion returned error: %v", err)
	}

	if updated.EntryType != nil {
		t.Fatalf("expected entry_type to be cleared")
	}
	if updated.TransferAccountCode != nil {
		t.Fatalf("expected transfer_account_code to be cleared")
	}
}

func TestUpdateSuggestionNormalizesEntryTypeAndDescription(t *testing.T) {
	userID := uuid.New()
	santanderID := uuid.New()
	quitandaID := uuid.New()
	parentID := uuid.New()

	accountIDs[santanderID] = "santander"
	categoryIDs[quitandaID] = "quitanda"

	service := NewService(newInMemorySuggestionRepo(), &accountReaderStub{
		byCode: map[string]*account.Account{
			"santander": {ID: santanderID, Code: "santander"},
		},
	}, &categoryReaderStub{
		byCode: map[string]*category.Category{
			"quitanda": {ID: quitandaID, Code: "quitanda", ParentID: &parentID, Type: category.CategoryTypeExpense},
		},
	})

	created, err := service.AddSuggestion(userID, CreateSuggestionRequest{
		DescriptionContains: "mercado",
		Priority:            1,
		AccountCode:         stringPtr("santander"),
	})
	if err != nil {
		t.Fatalf("AddSuggestion returned error: %v", err)
	}

	updated, err := service.UpdateSuggestion(userID, created.ID, UpdateSuggestionRequest{
		DescriptionContains: stringPtr("  feira livre  "),
		EntryType:           stringPtr(" expense "),
		CategoryCode:        stringPtr("quitanda"),
	})
	if err != nil {
		t.Fatalf("UpdateSuggestion returned error: %v", err)
	}

	if updated.DescriptionContains != "feira livre" {
		t.Fatalf("expected trimmed description, got %q", updated.DescriptionContains)
	}
	if updated.EntryType == nil || *updated.EntryType != SuggestionEntryTypeExpense {
		t.Fatalf("expected normalized EXPENSE entry type, got %v", updated.EntryType)
	}
	if updated.AccountCode == nil || *updated.AccountCode != "santander" {
		t.Fatalf("expected existing account code to be preserved, got %v", updated.AccountCode)
	}
	if updated.CategoryCode == nil || *updated.CategoryCode != "quitanda" {
		t.Fatalf("expected category code to be set, got %v", updated.CategoryCode)
	}
}

func TestUpdateSuggestionRejectsMismatchedCategoryType(t *testing.T) {
	userID := uuid.New()
	quitandaID := uuid.New()
	parentID := uuid.New()
	categoryIDs[quitandaID] = "quitanda"

	service := NewService(newInMemorySuggestionRepo(), &accountReaderStub{
		byCode: map[string]*account.Account{},
	}, &categoryReaderStub{
		byCode: map[string]*category.Category{
			"quitanda": {ID: quitandaID, Code: "quitanda", ParentID: &parentID, Type: category.CategoryTypeExpense},
		},
	})

	created, err := service.AddSuggestion(userID, CreateSuggestionRequest{
		DescriptionContains: "mercado",
		Priority:            1,
		CategoryCode:        stringPtr("quitanda"),
	})
	if err != nil {
		t.Fatalf("AddSuggestion returned error: %v", err)
	}

	_, err = service.UpdateSuggestion(userID, created.ID, UpdateSuggestionRequest{
		EntryType: stringPtr("REVENUE"),
	})
	if err == nil {
		t.Fatal("expected mismatched category type to be rejected")
	}
}

func TestDeleteSuggestionRemovesStoredSuggestion(t *testing.T) {
	userID := uuid.New()
	santanderID := uuid.New()
	accountIDs[santanderID] = "santander"

	service := NewService(newInMemorySuggestionRepo(), &accountReaderStub{
		byCode: map[string]*account.Account{
			"santander": {ID: santanderID, Code: "santander"},
		},
	}, &categoryReaderStub{byCode: map[string]*category.Category{}})

	created, err := service.AddSuggestion(userID, CreateSuggestionRequest{
		DescriptionContains: "salario",
		Priority:            1,
		AccountCode:         stringPtr("santander"),
	})
	if err != nil {
		t.Fatalf("AddSuggestion returned error: %v", err)
	}

	if err := service.DeleteSuggestion(userID, created.ID); err != nil {
		t.Fatalf("DeleteSuggestion returned error: %v", err)
	}

	_, err = service.GetSuggestionByID(userID, created.ID)
	if err == nil {
		t.Fatal("expected deleted suggestion to be missing")
	}
}

func stringPtr(value string) *string {
	return &value
}

package category

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	appErr "expense-tracker/internal/errors"

	"github.com/google/uuid"
)

type categoryRepoStub struct {
	createFn      func(category *Category) (*Category, error)
	getNextSortFn func(userID uuid.UUID, parentID *uuid.UUID) (int, error)
	getByCodeFn   func(userID uuid.UUID, code string, getDeactivated bool) (*Category, error)
	getByCodesFn  func(userID uuid.UUID, codes []string, getDeactivated bool) ([]Category, error)
	getByParentFn func(userID, parentID uuid.UUID, getDeactivated bool) ([]Category, error)
	getByUserFn   func(userID uuid.UUID, getDeactivated bool) ([]Category, error)
	updateFn      func(userID uuid.UUID, code string, category *UpdateCategory) (*Category, error)
	reorderFn     func(userID uuid.UUID, parentID *uuid.UUID, codes []string) error
	deactivateFn  func(userID uuid.UUID, code string) error
}

func (s *categoryRepoStub) Create(category *Category) (*Category, error) {
	return s.createFn(category)
}

func (s *categoryRepoStub) GetNextSortOrder(userID uuid.UUID, parentID *uuid.UUID) (int, error) {
	if s.getNextSortFn == nil {
		return 0, nil
	}
	return s.getNextSortFn(userID, parentID)
}

func (s *categoryRepoStub) GetByCode(userID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
	return s.getByCodeFn(userID, code, getDeactivated)
}

func (s *categoryRepoStub) GetByCodes(userID uuid.UUID, codes []string, getDeactivated bool) ([]Category, error) {
	return s.getByCodesFn(userID, codes, getDeactivated)
}

func (s *categoryRepoStub) GetByParentID(userID, parentID uuid.UUID, getDeactivated bool) ([]Category, error) {
	return s.getByParentFn(userID, parentID, getDeactivated)
}

func (s *categoryRepoStub) GetByUser(userID uuid.UUID, getDeactivated bool) ([]Category, error) {
	return s.getByUserFn(userID, getDeactivated)
}

func (s *categoryRepoStub) Update(userID uuid.UUID, code string, category *UpdateCategory) (*Category, error) {
	return s.updateFn(userID, code, category)
}

func (s *categoryRepoStub) Reorder(userID uuid.UUID, parentID *uuid.UUID, codes []string) error {
	return s.reorderFn(userID, parentID, codes)
}

func (s *categoryRepoStub) Deactivate(userID uuid.UUID, code string) error {
	return s.deactivateFn(userID, code)
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

func TestServiceAddCategoryGeneratesCode(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	var created *Category

	service := NewService(&categoryRepoStub{
		getByUserFn: func(gotUserID uuid.UUID, getDeactivated bool) ([]Category, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			if !getDeactivated {
				t.Fatal("expected create to inspect active and deactivated categories")
			}
			return []Category{
				{ID: uuid.New(), Code: "salary", Name: "Salary", DeactivatedAt: ptrTime(time.Now())},
			}, nil
		},
		getNextSortFn: func(gotUserID uuid.UUID, parentID *uuid.UUID) (int, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			if parentID != nil {
				t.Fatalf("expected root category to have nil parent, got %v", *parentID)
			}
			return 3, nil
		},
		createFn: func(category *Category) (*Category, error) {
			created = category
			return category, nil
		},
	})

	category, err := service.AddCategory(userID, CreateCategoryRequest{
		Name:        "Salary",
		Type:        CategoryTypeIncome,
		Description: "monthly income",
	})
	if err != nil {
		t.Fatalf("expected category creation to succeed, got error: %v", err)
	}

	if created == nil {
		t.Fatal("expected repository create to be called")
	}
	if created.Code != "salary-2" {
		t.Fatalf("expected generated unique code, got %q", created.Code)
	}
	if created.SortOrder == nil || *created.SortOrder != 3 {
		t.Fatalf("expected service to assign sort order 3, got %+v", created.SortOrder)
	}
	if category != created {
		t.Fatal("expected returned category to match repository result")
	}
}

func TestServiceAddCategoryWithParentRejectsChildAndDeactivatedParents(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	parentCode := "income"
	childID := uuid.New()
	now := time.Now()

	testCases := []struct {
		name   string
		parent Category
		want   string
	}{
		{
			name:   "child parent",
			parent: Category{ID: childID, Code: "income-child", ParentID: &childID},
			want:   "cannot create a child category of another child category",
		},
		{
			name:   "deactivated parent",
			parent: Category{ID: childID, Code: "income", DeactivatedAt: &now},
			want:   "deactivated",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := NewService(&categoryRepoStub{
				getByUserFn: func(gotUserID uuid.UUID, getDeactivated bool) ([]Category, error) {
					return []Category{}, nil
				},
				getByCodeFn: func(gotUserID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
					if gotUserID != userID {
						t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
					}
					if code != "income" {
						t.Fatalf("expected lowercase parent code, got %q", code)
					}
					return &tc.parent, nil
				},
				createFn: func(category *Category) (*Category, error) {
					t.Fatal("expected repository create not to be called for invalid parent")
					return nil, nil
				},
			})

			_, err := service.AddCategory(userID, CreateCategoryRequest{
				Name:        "Bonus",
				Type:        CategoryTypeIncome,
				Description: "annual",
				ParentCode:  &parentCode,
			})
			if err == nil {
				t.Fatal("expected invalid parent to fail")
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestServiceAddCategoryWithParentUsesResolvedParentID(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	parentCode := "income"
	parentID := uuid.New()
	var created *Category

	service := NewService(&categoryRepoStub{
		getByUserFn: func(gotUserID uuid.UUID, getDeactivated bool) ([]Category, error) {
			if gotUserID != userID || !getDeactivated {
				t.Fatalf("unexpected category list args: %s %t", gotUserID, getDeactivated)
			}
			return []Category{{ID: parentID, Code: "income", Name: "Income"}}, nil
		},
		getByCodeFn: func(gotUserID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
			if gotUserID != userID || code != "income" {
				t.Fatalf("unexpected parent lookup args: %s %q", gotUserID, code)
			}
			return &Category{ID: parentID, Code: "income"}, nil
		},
		getNextSortFn: func(gotUserID uuid.UUID, gotParentID *uuid.UUID) (int, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			if gotParentID == nil || *gotParentID != parentID {
				t.Fatalf("expected parent ID %s, got %+v", parentID, gotParentID)
			}
			return 5, nil
		},
		createFn: func(category *Category) (*Category, error) {
			created = category
			return category, nil
		},
	})

	_, err := service.AddCategory(userID, CreateCategoryRequest{
		Name:        "Bonus",
		Type:        CategoryTypeIncome,
		Description: "annual",
		ParentCode:  &parentCode,
	})
	if err != nil {
		t.Fatalf("expected category creation with parent to succeed, got %v", err)
	}
	if created == nil || created.ParentID == nil || *created.ParentID != parentID {
		t.Fatalf("expected resolved parent ID %s, got %+v", parentID, created)
	}
	if created.SortOrder == nil || *created.SortOrder != 5 {
		t.Fatalf("expected service to assign child sort order 5, got %+v", created.SortOrder)
	}
}

func TestServiceAddCategoryRejectsDuplicateActiveName(t *testing.T) {
	t.Parallel()

	service := NewService(&categoryRepoStub{
		getByUserFn: func(userID uuid.UUID, getDeactivated bool) ([]Category, error) {
			return []Category{
				{ID: uuid.New(), Code: "alimentacao", Name: "Alimentacao"},
				{ID: uuid.New(), Code: "alimentacao-antiga", Name: "Alimentacao", DeactivatedAt: ptrTime(time.Now())},
			}, nil
		},
		createFn: func(category *Category) (*Category, error) {
			t.Fatal("expected create not to be called for duplicate active name")
			return nil, nil
		},
	})

	_, err := service.AddCategory(uuid.New(), CreateCategoryRequest{
		Name:        "alimentacao",
		Type:        CategoryTypeExpense,
		Description: "mensal",
	})
	if err == nil || !strings.Contains(err.Error(), "active category") {
		t.Fatalf("expected duplicate active name error, got %v", err)
	}
}

func TestServiceGetCategoriesBuildsNestedResult(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	parentID := uuid.New()

	service := NewService(&categoryRepoStub{
		getByUserFn: func(gotUserID uuid.UUID, getDeactivated bool) ([]Category, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			if getDeactivated {
				t.Fatal("expected active category lookup")
			}
			return []Category{
				{ID: parentID, Code: "income", Name: "Income"},
				{ID: uuid.New(), ParentID: &parentID, Code: "salary", Name: "Salary"},
			}, nil
		},
	})

	categories, err := service.GetCategories(userID, false)
	if err != nil {
		t.Fatalf("expected category list to succeed, got error: %v", err)
	}
	if len(categories) != 1 {
		t.Fatalf("expected one parent category, got %d", len(categories))
	}
	if len(categories[0].SubCategories) != 1 {
		t.Fatalf("expected one subcategory, got %d", len(categories[0].SubCategories))
	}
	if categories[0].SubCategories[0].Code != "salary" {
		t.Fatalf("expected nested child code %q, got %q", "salary", categories[0].SubCategories[0].Code)
	}
}

func TestServiceGetCategoriesPropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("boom")
	service := NewService(&categoryRepoStub{
		getByUserFn: func(userID uuid.UUID, getDeactivated bool) ([]Category, error) {
			return nil, expectedErr
		},
	})

	_, err := service.GetCategories(uuid.New(), false)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected repository error to be returned, got %v", err)
	}
}

func TestServiceGetCategoryByCodeLoadsChildrenWhenCategoryHasParent(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	parentID := uuid.New()
	categoryID := uuid.New()
	child := Category{ID: uuid.New(), ParentID: &categoryID, Code: "salary"}

	service := NewService(&categoryRepoStub{
		getByCodeFn: func(gotUserID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			if code != "income" {
				t.Fatalf("expected lowercase code, got %q", code)
			}
			return &Category{ID: categoryID, ParentID: &parentID, Code: "income"}, nil
		},
		getByParentFn: func(gotUserID, gotParentID uuid.UUID, getDeactivated bool) ([]Category, error) {
			if gotParentID != categoryID {
				t.Fatalf("expected category ID %s, got %s", categoryID, gotParentID)
			}
			return []Category{child}, nil
		},
	})

	category, err := service.GetCategoryByCode(userID, "InCoMe")
	if err != nil {
		t.Fatalf("expected category lookup to succeed, got error: %v", err)
	}
	if len(category.SubCategories) != 1 || category.SubCategories[0].Code != child.Code {
		t.Fatalf("expected child category to be loaded, got %+v", category.SubCategories)
	}
}

func TestServiceGetCategoriesByCodeNormalizesCodesAndLoadsChildren(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	parentID := uuid.New()
	childID := uuid.New()
	service := NewService(&categoryRepoStub{
		getByCodesFn: func(gotUserID uuid.UUID, codes []string, getDeactivated bool) ([]Category, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			expected := []string{"income", "salary"}
			if !reflect.DeepEqual(codes, expected) {
				t.Fatalf("expected normalized codes %v, got %v", expected, codes)
			}
			return []Category{
				{ID: parentID, Code: "income"},
				{ID: childID, ParentID: &parentID, Code: "salary"},
			}, nil
		},
		getByParentFn: func(gotUserID, gotParentID uuid.UUID, getDeactivated bool) ([]Category, error) {
			if gotParentID != childID {
				t.Fatalf("expected child category id %s, got %s", childID, gotParentID)
			}
			return []Category{{ID: uuid.New(), ParentID: &childID, Code: "bonus"}}, nil
		},
	})

	got, err := service.GetCategoriesByCode(userID, []string{"InCoMe", "SALARY"})
	if err != nil {
		t.Fatalf("expected category lookup to succeed, got %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 categories, got %d", len(got))
	}
	if len(got[1].SubCategories) != 1 || got[1].SubCategories[0].Code != "bonus" {
		t.Fatalf("expected nested child categories to be loaded, got %+v", got[1].SubCategories)
	}
}

func TestServiceUpdateCategoryRequiresAtLeastOneField(t *testing.T) {
	t.Parallel()

	service := NewService(&categoryRepoStub{})

	_, err := service.UpdateCategory(uuid.New(), "income", UpdateCategoryRequest{})
	if err == nil {
		t.Fatal("expected empty category update to fail")
	}
	if !strings.Contains(err.Error(), "at least one") {
		t.Fatalf("expected at-least-one validation error, got: %v", err)
	}
}

func TestServiceUpdateCategoryRejectsDeactivatedCategory(t *testing.T) {
	t.Parallel()

	now := time.Now()
	service := NewService(&categoryRepoStub{
		getByCodeFn: func(userID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
			return &Category{Code: code, DeactivatedAt: &now}, nil
		},
		updateFn: func(userID uuid.UUID, code string, category *UpdateCategory) (*Category, error) {
			t.Fatal("expected repository update not to be called for deactivated category")
			return nil, nil
		},
	})

	name := "Updated"
	_, err := service.UpdateCategory(uuid.New(), "income", UpdateCategoryRequest{Name: &name})
	var appError *appErr.AppError
	if !errors.As(err, &appError) || appError.Code != appErr.ErrCategoryDeactivated().Code {
		t.Fatalf("expected category deactivated error, got: %v", err)
	}
}

func TestServiceUpdateCategoryRejectsPermanentCategory(t *testing.T) {
	t.Parallel()

	name := "Updated"
	service := NewService(&categoryRepoStub{
		getByCodeFn: func(userID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
			return &Category{ID: uuid.New(), Code: PermanentInvestmentMovementCode}, nil
		},
		updateFn: func(userID uuid.UUID, code string, category *UpdateCategory) (*Category, error) {
			t.Fatal("expected repository update not to be called for permanent category")
			return nil, nil
		},
	})

	_, err := service.UpdateCategory(uuid.New(), PermanentInvestmentMovementCode, UpdateCategoryRequest{Name: &name})
	if err == nil {
		t.Fatal("expected permanent category update to fail")
	}
	var appError *appErr.AppError
	if !errors.As(err, &appError) || appError.Code != "category.permanent.read_only" {
		t.Fatalf("expected category.permanent.read_only, got %v", err)
	}
}

func TestServiceUpdateCategoryResolvesParentCode(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	parentCode := "ROOT"
	parentID := uuid.New()
	current := &Category{ID: uuid.New(), Code: "salary"}
	name := "Salary"
	description := "updated"
	categoryType := CategoryTypeIncome

	getByCodeCalls := 0
	var gotUpdate *UpdateCategory

	service := NewService(&categoryRepoStub{
		getByCodeFn: func(gotUserID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
			getByCodeCalls++
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			switch getByCodeCalls {
			case 1:
				if code != "salary" {
					t.Fatalf("expected first lookup to use category code, got %q", code)
				}
				return current, nil
			case 2:
				if code != "root" {
					t.Fatalf("expected second lookup to use lowercase parent code, got %q", code)
				}
				return &Category{ID: parentID, Code: "root"}, nil
			default:
				t.Fatalf("unexpected extra getByCode call with code %q", code)
				return nil, nil
			}
		},
		getByUserFn: func(gotUserID uuid.UUID, getDeactivated bool) ([]Category, error) {
			return []Category{{ID: current.ID, Code: current.Code, Name: current.Name}}, nil
		},
		updateFn: func(gotUserID uuid.UUID, code string, category *UpdateCategory) (*Category, error) {
			if code != "salary" {
				t.Fatalf("expected lowercase update code, got %q", code)
			}
			gotUpdate = category
			return &Category{ID: current.ID, Code: code, ParentID: category.ParentID, Name: *category.Name, Description: *category.Description, Type: *category.Type}, nil
		},
	})

	updated, err := service.UpdateCategory(userID, "SaLaRy", UpdateCategoryRequest{
		Name:        &name,
		Description: &description,
		Type:        &categoryType,
		ParentCode:  &parentCode,
	})
	if err != nil {
		t.Fatalf("expected category update to succeed, got error: %v", err)
	}
	if gotUpdate == nil || gotUpdate.ParentID == nil || *gotUpdate.ParentID != parentID {
		t.Fatalf("expected resolved parent ID %s, got %+v", parentID, gotUpdate)
	}
	if updated.ParentID == nil || *updated.ParentID != parentID {
		t.Fatalf("expected updated category to include parent ID %s, got %+v", parentID, updated.ParentID)
	}
}

func TestServiceUpdateCategoryRejectsDuplicateActiveName(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	currentID := uuid.New()
	name := "Moradia"

	service := NewService(&categoryRepoStub{
		getByCodeFn: func(gotUserID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
			return &Category{ID: currentID, UserID: gotUserID, Code: code, Name: "Conta antiga"}, nil
		},
		getByUserFn: func(gotUserID uuid.UUID, getDeactivated bool) ([]Category, error) {
			return []Category{
				{ID: currentID, Code: "conta-antiga", Name: "Conta antiga"},
				{ID: uuid.New(), Code: "moradia", Name: "Moradia"},
			}, nil
		},
		updateFn: func(userID uuid.UUID, code string, category *UpdateCategory) (*Category, error) {
			t.Fatal("expected update not to be called for duplicate active name")
			return nil, nil
		},
	})

	_, err := service.UpdateCategory(userID, "conta-antiga", UpdateCategoryRequest{Name: &name})
	if err == nil || !strings.Contains(err.Error(), "active category") {
		t.Fatalf("expected duplicate active name error, got %v", err)
	}
}

func TestServiceUpdateCategoryRejectsChildOrDeactivatedParent(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	current := &Category{ID: uuid.New(), Code: "salary"}
	childID := uuid.New()
	now := time.Now()
	parentCode := "income"
	name := "Salary"

	testCases := []struct {
		name   string
		parent Category
		want   string
	}{
		{
			name:   "child parent",
			parent: Category{ID: childID, Code: "income-child", ParentID: &childID},
			want:   "child category",
		},
		{
			name:   "deactivated parent",
			parent: Category{ID: childID, Code: "income", DeactivatedAt: &now},
			want:   "deactivated",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			calls := 0
			service := NewService(&categoryRepoStub{
				getByCodeFn: func(gotUserID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
					calls++
					if gotUserID != userID {
						t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
					}
					if calls == 1 {
						return current, nil
					}
					return &tc.parent, nil
				},
				getByUserFn: func(gotUserID uuid.UUID, getDeactivated bool) ([]Category, error) {
					return []Category{{ID: current.ID, Code: current.Code, Name: current.Name}}, nil
				},
				updateFn: func(userID uuid.UUID, code string, category *UpdateCategory) (*Category, error) {
					t.Fatal("expected update not to be called for invalid parent")
					return nil, nil
				},
			})

			_, err := service.UpdateCategory(userID, "salary", UpdateCategoryRequest{Name: &name, ParentCode: &parentCode})
			if err == nil {
				t.Fatal("expected invalid parent to fail update")
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("expected error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestServiceReorderCategoriesRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	service := NewService(&categoryRepoStub{
		getByUserFn: func(userID uuid.UUID, getDeactivated bool) ([]Category, error) {
			t.Fatal("expected category list not to be fetched for invalid reorder input")
			return nil, nil
		},
	})

	if err := service.ReorderCategories(uuid.New(), nil, nil); err == nil {
		t.Fatal("expected empty reorder list to fail")
	}

	if err := service.ReorderCategories(uuid.New(), nil, []string{"salary", "SALARY"}); err == nil {
		t.Fatal("expected duplicate reorder codes to fail")
	}
}

func TestServiceReorderCategoriesForRootAppendsHiddenMovementCategories(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	var reordered []string
	var gotParentID *uuid.UUID

	service := NewService(&categoryRepoStub{
		getByUserFn: func(gotUserID uuid.UUID, getDeactivated bool) ([]Category, error) {
			if gotUserID != userID {
				t.Fatalf("expected user ID %s, got %s", userID, gotUserID)
			}
			return []Category{
				{Code: "salary", Type: CategoryTypeIncome},
				{Code: "rent", Type: CategoryTypeExpense},
				{Code: "transfer", Type: CategoryTypeMovement},
			}, nil
		},
		reorderFn: func(gotUserID uuid.UUID, parentID *uuid.UUID, codes []string) error {
			gotParentID = parentID
			reordered = append([]string(nil), codes...)
			return nil
		},
	})

	err := service.ReorderCategories(userID, nil, []string{"RENT", "salary"})
	if err != nil {
		t.Fatalf("expected reorder to succeed, got error: %v", err)
	}
	if gotParentID != nil {
		t.Fatalf("expected root reorder to keep nil parent ID, got %v", gotParentID)
	}
	expected := []string{"rent", "salary", "transfer"}
	if len(reordered) != len(expected) {
		t.Fatalf("expected %d reordered codes, got %v", len(expected), reordered)
	}
	for i, code := range expected {
		if reordered[i] != code {
			t.Fatalf("expected reordered[%d] = %q, got %q", i, code, reordered[i])
		}
	}
}

func TestServiceReorderCategoriesForParentUsesSiblingGroupOnly(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	parentCode := "Income"
	parentID := uuid.New()
	otherParentID := uuid.New()
	var reordered []string
	var gotParentID *uuid.UUID

	service := NewService(&categoryRepoStub{
		getByCodeFn: func(gotUserID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
			if code != "income" {
				t.Fatalf("expected lowercase parent code, got %q", code)
			}
			return &Category{ID: parentID, Code: "income"}, nil
		},
		getByUserFn: func(gotUserID uuid.UUID, getDeactivated bool) ([]Category, error) {
			return []Category{
				{Code: "salary", ParentID: &parentID, Type: CategoryTypeIncome},
				{Code: "bonus", ParentID: &parentID, Type: CategoryTypeIncome},
				{Code: "rent", ParentID: &otherParentID, Type: CategoryTypeExpense},
			}, nil
		},
		reorderFn: func(gotUserID uuid.UUID, parentIDArg *uuid.UUID, codes []string) error {
			gotParentID = parentIDArg
			reordered = append([]string(nil), codes...)
			return nil
		},
	})

	err := service.ReorderCategories(userID, &parentCode, []string{"BONUS"})
	if err != nil {
		t.Fatalf("expected parent reorder to succeed, got error: %v", err)
	}
	if gotParentID == nil || *gotParentID != parentID {
		t.Fatalf("expected reorder to target parent ID %s, got %v", parentID, gotParentID)
	}
	expected := []string{"bonus"}
	if len(reordered) != len(expected) {
		t.Fatalf("expected %d reordered codes, got %v", len(expected), reordered)
	}
	for i, code := range expected {
		if reordered[i] != code {
			t.Fatalf("expected reordered[%d] = %q, got %q", i, code, reordered[i])
		}
	}
}

func TestServiceReorderCategoriesRejectsInvalidParentGroupScenarios(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	parentCode := "income"
	parentID := uuid.New()
	otherParentID := uuid.New()

	testCases := []struct {
		name        string
		parent      Category
		categories  []Category
		codes       []string
		wantMessage string
	}{
		{
			name:        "parent is child",
			parent:      Category{ID: parentID, Code: "income", ParentID: &otherParentID},
			codes:       []string{"salary"},
			wantMessage: "root category",
		},
		{
			name:        "no categories in group",
			parent:      Category{ID: parentID, Code: "income"},
			categories:  []Category{{Code: "rent", ParentID: &otherParentID}},
			codes:       []string{"salary"},
			wantMessage: "no categories found",
		},
		{
			name:        "code outside sibling group",
			parent:      Category{ID: parentID, Code: "income"},
			categories:  []Category{{Code: "salary", ParentID: &parentID}},
			codes:       []string{"rent"},
			wantMessage: "selected parent group",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			service := NewService(&categoryRepoStub{
				getByCodeFn: func(gotUserID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
					return &tc.parent, nil
				},
				getByUserFn: func(gotUserID uuid.UUID, getDeactivated bool) ([]Category, error) {
					return tc.categories, nil
				},
				reorderFn: func(userID uuid.UUID, parentID *uuid.UUID, codes []string) error {
					t.Fatal("expected reorder not to be called for invalid reorder scenario")
					return nil
				},
			})

			err := service.ReorderCategories(userID, &parentCode, tc.codes)
			if err == nil {
				t.Fatal("expected reorder to fail")
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wantMessage)) {
				t.Fatalf("expected error containing %q, got %v", tc.wantMessage, err)
			}
		})
	}
}

func TestServiceDeactivateCategoryRejectsActiveChildren(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categoryID := uuid.New()
	service := NewService(&categoryRepoStub{
		getByCodeFn: func(gotUserID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
			return &Category{ID: categoryID, Code: "income"}, nil
		},
		getByParentFn: func(gotUserID, gotParentID uuid.UUID, getDeactivated bool) ([]Category, error) {
			return []Category{{ID: uuid.New(), ParentID: &categoryID, Code: "salary"}}, nil
		},
		deactivateFn: func(userID uuid.UUID, code string) error {
			t.Fatal("expected deactivate not to be called when active children exist")
			return nil
		},
	})

	err := service.DeactivateCategory(userID, "Income")
	if err == nil {
		t.Fatal("expected deactivate with active children to fail")
	}
	if !strings.Contains(err.Error(), "active children") {
		t.Fatalf("expected active children error, got: %v", err)
	}
}

func TestServiceDeactivateCategoryNormalizesCode(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categoryID := uuid.New()
	var gotCode string

	service := NewService(&categoryRepoStub{
		getByCodeFn: func(gotUserID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
			if code != "income" {
				t.Fatalf("expected lowercase getByCode lookup, got %q", code)
			}
			return &Category{ID: categoryID, Code: code}, nil
		},
		getByParentFn: func(gotUserID, gotParentID uuid.UUID, getDeactivated bool) ([]Category, error) {
			return nil, nil
		},
		deactivateFn: func(gotUserID uuid.UUID, code string) error {
			gotCode = code
			return nil
		},
	})

	if err := service.DeactivateCategory(userID, "InCoMe"); err != nil {
		t.Fatalf("expected deactivate to succeed, got error: %v", err)
	}
	if gotCode != "income" {
		t.Fatalf("expected lowercase deactivate code, got %q", gotCode)
	}
}

func TestServiceDeactivateCategoryRejectsPermanentCategory(t *testing.T) {
	t.Parallel()

	service := NewService(&categoryRepoStub{
		getByCodeFn: func(userID uuid.UUID, code string, getDeactivated bool) (*Category, error) {
			return &Category{ID: uuid.New(), Code: PermanentMovementRootCode}, nil
		},
		getByParentFn: func(userID, parentID uuid.UUID, getDeactivated bool) ([]Category, error) {
			t.Fatal("expected child lookup not to run for permanent category")
			return nil, nil
		},
		deactivateFn: func(userID uuid.UUID, code string) error {
			t.Fatal("expected deactivate not to be called for permanent category")
			return nil
		},
	})

	err := service.DeactivateCategory(uuid.New(), PermanentMovementRootCode)
	if err == nil {
		t.Fatal("expected permanent category deactivate to fail")
	}
	var appError *appErr.AppError
	if !errors.As(err, &appError) || appError.Code != "category.permanent.read_only" {
		t.Fatalf("expected category.permanent.read_only, got %v", err)
	}
}

func TestGetNestedCategoriesSameParentGroupAndContainsStringHelpers(t *testing.T) {
	t.Parallel()

	rootID := uuid.New()
	childID := uuid.New()
	nested, err := getNestedCategories([]Category{
		{ID: rootID, Code: "income"},
		{ID: childID, ParentID: &rootID, Code: "salary"},
		{ID: uuid.New(), ParentID: ptrUUID(uuid.New()), Code: "orphan"},
	})
	if err != nil {
		t.Fatalf("expected nesting to succeed, got %v", err)
	}
	if len(nested) != 1 || len(nested[0].SubCategories) != 1 {
		t.Fatalf("expected one parent with one child, got %+v", nested)
	}

	if !sameParentGroup(nil, nil) {
		t.Fatal("expected nil parent groups to match")
	}
	if sameParentGroup(&rootID, nil) {
		t.Fatal("expected one nil parent group not to match")
	}
	if containsString([]string{"salary"}, "rent") {
		t.Fatal("expected containsString to return false for missing value")
	}
}

func ptrUUID(v uuid.UUID) *uuid.UUID {
	return &v
}

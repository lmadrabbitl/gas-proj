package category

import (
	"expense-tracker/internal/errors"
	"expense-tracker/internal/slugutil"
	"strings"

	"github.com/google/uuid"
)

type Service interface {
	AddCategory(userID uuid.UUID, req CreateCategoryRequest) (*Category, error)
	GetCategories(userID uuid.UUID, includeDeactivated bool) ([]Category, error)
	GetCategoryByCode(userID uuid.UUID, code string) (*Category, error)
	GetCategoriesByCode(userID uuid.UUID, codes []string) ([]Category, error)
	UpdateCategory(userID uuid.UUID, code string, req UpdateCategoryRequest) (*Category, error)
	ReorderCategories(userID uuid.UUID, parentCode *string, codes []string) error
	DeactivateCategory(userID uuid.UUID, code string) error
}

type service struct {
	repo Repository
}

type CreateCategoryRequest struct {
	Name        string
	Type        CategoryType
	Description string
	ParentCode  *string
}

type UpdateCategoryRequest struct {
	Name        *string
	Type        *CategoryType
	Description *string
	ParentCode  *string
}

func NewService(repo Repository) Service {
	return &service{
		repo: repo,
	}
}

func (serv *service) AddCategory(userID uuid.UUID, req CreateCategoryRequest) (*Category, error) {
	if err := CheckCategoryName(req.Name); err != nil {
		return nil, err
	}

	if err := CheckCategoryType(req.Type); err != nil {
		return nil, err
	}

	existingCategories, err := serv.repo.GetByUser(userID, true)
	if err != nil {
		return nil, err
	}
	if hasDuplicateActiveCategoryName(existingCategories, req.Name, uuid.Nil) {
		return nil, errors.ErrInvalidInputWithMessage("there's already one active category with that name for this user", nil)
	}

	existingCodes := make(map[string]struct{}, len(existingCategories))
	for _, category := range existingCategories {
		existingCodes[category.Code] = struct{}{}
	}

	var parentID *uuid.UUID
	if req.ParentCode != nil && *req.ParentCode != "" {
		lowerCaseParentCode := strings.ToLower(*req.ParentCode)
		parentCategory, err := serv.repo.GetByCode(userID, lowerCaseParentCode, false)
		if err != nil {
			return nil, err
		}
		if parentCategory.ParentID != nil {
			return nil, errors.ErrInvalidInputWithMessage(
				"cannot create a child category of another child category", nil)
		}
		if parentCategory.DeactivatedAt != nil {
			return nil, errors.ErrInvalidInputWithMessage(
				"Parent code can't be a deactivated category", nil)
		}
		parentID = &parentCategory.ID
	}

	category := &Category{
		ID:          uuid.New(),
		UserID:      userID,
		Code:        slugutil.GenerateUnique(req.Name, "category", existingCodes),
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		ParentID:    parentID,
	}

	if category.DeactivatedAt == nil && category.SortOrder == nil {
		nextSortOrder, err := serv.repo.GetNextSortOrder(userID, parentID)
		if err != nil {
			return nil, err
		}
		category.SortOrder = &nextSortOrder
	}

	return serv.repo.Create(category)
}

func (serv *service) GetCategories(userID uuid.UUID, includeDeactivated bool) ([]Category, error) {
	categories, err := serv.repo.GetByUser(userID, includeDeactivated)
	if err != nil {
		return nil, err
	}

	nestedCategories, err := getNestedCategories(categories)
	if err != nil {
		return nil, err
	}

	return nestedCategories, nil
}

func (serv *service) GetCategoriesByCode(userID uuid.UUID, codes []string) ([]Category, error) {

	if err := CheckCategoryCodes(codes); err != nil {
		return nil, err
	}

	lowerCaseCategoryCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		lowerCaseCategoryCodes = append(lowerCaseCategoryCodes, strings.ToLower(code))
	}
	categories, err := serv.repo.GetByCodes(userID, lowerCaseCategoryCodes, false)
	if err != nil {
		return nil, err
	}

	// TODO: not a good idea?
	for i := range categories {
		if categories[i].ParentID == nil {
			continue
		}
		children, err := serv.repo.GetByParentID(userID, categories[i].ID, false)
		if err != nil {
			return nil, err
		}
		categories[i].SubCategories = children
	}

	return categories, nil

}

func (serv *service) GetCategoryByCode(userID uuid.UUID, code string) (*Category, error) {

	if err := CheckCategoryCode(code); err != nil {
		return nil, err
	}

	category, err := serv.repo.GetByCode(userID, strings.ToLower(code), false)
	if err != nil {
		return nil, err
	}

	if category.ParentID != nil {
		children, err := serv.repo.GetByParentID(userID, category.ID, false)
		if err != nil {
			return nil, err
		}
		category.SubCategories = children
	}

	return category, nil

}

func (serv *service) UpdateCategory(userID uuid.UUID, code string, req UpdateCategoryRequest) (*Category, error) {

	if req.Description == nil && req.ParentCode == nil && req.Name == nil && req.Type == nil {
		return nil, errors.ErrInvalidInputWithMessage(
			"at least one of these can't be empty: name, parent code, description or type", nil)
	}

	if err := CheckCategoryCode(code); err != nil {
		return nil, err
	}

	category, err := serv.repo.GetByCode(userID, strings.ToLower(code), false)
	if err != nil {
		return nil, err
	}

	if category.DeactivatedAt != nil {
		return nil, errors.ErrCategoryDeactivated()
	}

	if req.Name != nil {
		existingCategories, err := serv.repo.GetByUser(userID, true)
		if err != nil {
			return nil, err
		}
		if hasDuplicateActiveCategoryName(existingCategories, *req.Name, category.ID) {
			return nil, errors.ErrInvalidInputWithMessage("there's already one active category with that name for this user", nil)
		}
	}

	updateCategory := &UpdateCategory{}

	if req.Name != nil {
		if err := CheckCategoryName(*req.Name); err != nil {
			return nil, err
		}
		updateCategory.Name = req.Name
	}
	if req.Description != nil {
		updateCategory.Description = req.Description
	}
	if req.Type != nil {
		if err := CheckCategoryType(*req.Type); err != nil {
			return nil, err
		}
		updateCategory.Type = req.Type
	}
	if req.ParentCode != nil {

		parentCategory, err := serv.repo.GetByCode(userID, strings.ToLower(*req.ParentCode), false)
		if err != nil {
			return nil, err
		}

		if parentCategory.DeactivatedAt != nil {
			return nil, errors.ErrInvalidInputWithMessage(
				"Parent code can't be a deactivated category", nil)
		}
		if parentCategory.ParentID != nil {
			return nil, errors.ErrInvalidInputWithMessage(
				"Parent code can't be a child category", nil)
		}

		parentID := &parentCategory.ID
		updateCategory.ParentID = parentID
	}

	return serv.repo.Update(userID, strings.ToLower(code), updateCategory)

}

func hasDuplicateActiveCategoryName(categories []Category, name string, currentID uuid.UUID) bool {
	normalizedName := strings.TrimSpace(name)
	for _, category := range categories {
		if category.DeactivatedAt != nil {
			continue
		}
		if currentID != uuid.Nil && category.ID == currentID {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(category.Name), normalizedName) {
			return true
		}
	}
	return false
}

func (serv *service) ReorderCategories(userID uuid.UUID, parentCode *string, codes []string) error {
	if len(codes) == 0 {
		return errors.ErrInvalidInputWithMessage("category order cannot be empty", nil)
	}

	var parentID *uuid.UUID
	var normalizedParentCode *string
	if parentCode != nil && *parentCode != "" {
		lowerParentCode := strings.ToLower(*parentCode)
		parentCategory, err := serv.repo.GetByCode(userID, lowerParentCode, false)
		if err != nil {
			return err
		}
		if parentCategory.ParentID != nil {
			return errors.ErrInvalidInputWithMessage("parent_code must be a root category", nil)
		}
		parentID = &parentCategory.ID
		normalizedParentCode = &lowerParentCode
	}

	seenCodes := make(map[string]struct{}, len(codes))
	normalizedCodes := make([]string, 0, len(codes))
	for _, code := range codes {
		if err := CheckCategoryCode(code); err != nil {
			return err
		}

		lowerCode := strings.ToLower(code)
		if _, exists := seenCodes[lowerCode]; exists {
			return errors.ErrInvalidInputWithMessage("category order cannot contain duplicate codes", nil)
		}

		seenCodes[lowerCode] = struct{}{}
		normalizedCodes = append(normalizedCodes, lowerCode)
	}

	categories, err := serv.repo.GetByUser(userID, false)
	if err != nil {
		return err
	}

	siblingCodes := make([]string, 0)
	hiddenSiblingCodes := make([]string, 0)
	for _, category := range categories {
		if sameParentGroup(category.ParentID, parentID) {
			siblingCodes = append(siblingCodes, category.Code)
			if category.Type == CategoryTypeMovement {
				hiddenSiblingCodes = append(hiddenSiblingCodes, category.Code)
			}
		}
	}

	if len(siblingCodes) == 0 {
		return errors.ErrInvalidInputWithMessage("no categories found for reorder group", nil)
	}

	fullOrder := make([]string, 0, len(siblingCodes))
	includedCodes := make(map[string]struct{}, len(siblingCodes))
	for _, code := range normalizedCodes {
		found := false
		for _, siblingCode := range siblingCodes {
			if siblingCode == code {
				found = true
				break
			}
		}
		if !found {
			return errors.ErrInvalidInputWithMessage("all category codes must belong to the selected parent group", nil)
		}

		includedCodes[code] = struct{}{}
		fullOrder = append(fullOrder, code)
	}

	for _, siblingCode := range siblingCodes {
		if _, exists := includedCodes[siblingCode]; exists {
			continue
		}

		if normalizedParentCode == nil || containsString(hiddenSiblingCodes, siblingCode) {
			fullOrder = append(fullOrder, siblingCode)
		}
	}

	return serv.repo.Reorder(userID, parentID, fullOrder)
}

func (serv *service) DeactivateCategory(userID uuid.UUID, code string) error {

	if err := CheckCategoryCode(code); err != nil {
		return err
	}

	category, err := serv.repo.GetByCode(userID, strings.ToLower(code), false)
	if err != nil {
		return err
	}

	childCategories, err := serv.repo.GetByParentID(userID, category.ID, false)
	if err != nil {
		return err
	}
	if len(childCategories) > 0 {
		return errors.ErrInvalidInputWithMessage(
			"cannot deactivate a category with active children", nil)
	}

	return serv.repo.Deactivate(userID, strings.ToLower(code))
}

func getNestedCategories(categories []Category) ([]Category, error) {

	parentMap := map[uuid.UUID]*Category{}
	parentOrder := make([]uuid.UUID, 0)
	childBuckets := make(map[uuid.UUID][]Category)

	for i := range categories {
		cat := categories[i]
		if cat.ParentID == nil {
			cat.SubCategories = nil
			parentMap[cat.ID] = &cat
			parentOrder = append(parentOrder, cat.ID)
			continue
		}

		childBuckets[*cat.ParentID] = append(childBuckets[*cat.ParentID], cat)
	}

	parentCategoryArray := make([]Category, 0, len(parentOrder))
	for _, parentID := range parentOrder {
		parentCategory, ok := parentMap[parentID]
		if !ok {
			continue
		}

		parentCategory.SubCategories = childBuckets[parentID]
		parentCategoryArray = append(parentCategoryArray, *parentCategory)
	}

	return parentCategoryArray, nil
}

func sameParentGroup(categoryParentID *uuid.UUID, targetParentID *uuid.UUID) bool {
	if categoryParentID == nil && targetParentID == nil {
		return true
	}
	if categoryParentID == nil || targetParentID == nil {
		return false
	}

	return *categoryParentID == *targetParentID
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}

	return false
}

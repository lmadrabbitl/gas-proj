package category

import (
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	PermanentMovementRootCode       = "movimentacoes"
	PermanentInvestmentMovementCode = "aporteretirada"
)

type permanentCategoryDef struct {
	Code        string
	Name        string
	Type        CategoryType
	Description string
	SortOrder   int
}

var permanentMovementRoot = permanentCategoryDef{
	Code:        PermanentMovementRootCode,
	Name:        "Movimentações",
	Type:        CategoryTypeMovement,
	Description: "",
	SortOrder:   0,
}

var permanentMovementChildren = []permanentCategoryDef{
	{Code: "saque", Name: "Saque", Type: CategoryTypeMovement, Description: "", SortOrder: 1},
	{Code: "transferencias", Name: "Transf. entre contas", Type: CategoryTypeMovement, Description: "", SortOrder: 2},
	{Code: "pagamentos", Name: "Pagamento de conta", Type: CategoryTypeMovement, Description: "", SortOrder: 3},
	{Code: PermanentInvestmentMovementCode, Name: "Investimento - aporte/retirada", Type: CategoryTypeMovement, Description: "", SortOrder: 4},
}

func IsPermanentCategoryCode(code string) bool {
	normalized := strings.ToLower(strings.TrimSpace(code))
	if normalized == permanentMovementRoot.Code {
		return true
	}
	for _, def := range permanentMovementChildren {
		if normalized == def.Code {
			return true
		}
	}
	return false
}

func EnsurePermanentCategories(db *gorm.DB, userID uuid.UUID) error {
	root, err := ensurePermanentCategory(db, userID, nil, permanentMovementRoot)
	if err != nil {
		return err
	}
	for _, def := range permanentMovementChildren {
		parentID := root.ID
		if _, err := ensurePermanentCategory(db, userID, &parentID, def); err != nil {
			return err
		}
	}
	return nil
}

func ensurePermanentCategory(db *gorm.DB, userID uuid.UUID, parentID *uuid.UUID, def permanentCategoryDef) (*Category, error) {
	var category Category
	err := db.Where("user_id = ? AND code = ?", userID, def.Code).First(&category).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	if err == gorm.ErrRecordNotFound {
		category = Category{
			ID:          uuid.New(),
			UserID:      userID,
			ParentID:    parentID,
			Code:        def.Code,
			Name:        def.Name,
			Type:        def.Type,
			Description: def.Description,
			SortOrder:   intPtr(def.SortOrder),
		}
		if err := db.Create(&category).Error; err != nil {
			return nil, err
		}
		return &category, nil
	}

	category.ParentID = parentID
	category.Name = def.Name
	category.Type = def.Type
	category.Description = def.Description
	category.SortOrder = intPtr(def.SortOrder)
	category.DeactivatedAt = nil

	if err := db.Model(&Category{}).
		Where("id = ?", category.ID).
		Updates(map[string]any{
			"parent_id":      category.ParentID,
			"name":           category.Name,
			"type":           category.Type,
			"description":    category.Description,
			"sort_order":     def.SortOrder,
			"deactivated_at": nil,
		}).Error; err != nil {
		return nil, err
	}

	return &category, nil
}

func intPtr(v int) *int {
	return &v
}

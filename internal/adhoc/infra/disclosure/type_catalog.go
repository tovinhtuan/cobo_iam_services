package disclosure

import (
	"context"
	"strings"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

type TypeCatalogAdapter struct {
	repo disclosureapp.Repository
}

func NewTypeCatalogAdapter(repo disclosureapp.Repository) *TypeCatalogAdapter {
	return &TypeCatalogAdapter{repo: repo}
}

func (a *TypeCatalogAdapter) GetTemplateCategory(ctx context.Context, companyID, typeID string) (string, error) {
	item, err := a.repo.GetTypeDetail(ctx, companyID, typeID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(item.TemplateCategory), nil
}

func (a *TypeCatalogAdapter) GetTypeDisplayName(ctx context.Context, companyID, typeID string) (string, error) {
	item, err := a.repo.GetTypeDetail(ctx, companyID, typeID)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(item.Name), nil
}

package workflowdoctemplate

import "context"

// BinderAdapter adapts Service.AssertCanBind to a simple error-returning binder.
type BinderAdapter struct {
	Svc *Service
}

// AssertCanBind validates file ownership/scope for workflow document binding.
func (a BinderAdapter) AssertCanBind(ctx context.Context, fileID, bindScope, bindCompanyID string) error {
	if a.Svc == nil {
		return nil
	}
	_, err := a.Svc.AssertCanBind(ctx, fileID, bindScope, bindCompanyID)
	return err
}

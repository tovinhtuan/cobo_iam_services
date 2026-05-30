package app

import (
	"context"
	"errors"
	"testing"
)

// fakeMembershipQuerier is an in-memory MembershipEmailQuerier for tests.
type fakeMembershipQuerier struct {
	// departmentEmails maps companyID:deptID → emails
	departmentEmails map[string][]string
	// roleEmails maps companyID:roleID → emails
	roleEmails map[string][]string
	err        error
}

func (f *fakeMembershipQuerier) EmailsByDepartments(_ context.Context, companyID string, deptIDs []string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []string
	for _, d := range deptIDs {
		key := companyID + ":" + d
		out = append(out, f.departmentEmails[key]...)
	}
	return out, nil
}

func (f *fakeMembershipQuerier) EmailsByRoles(_ context.Context, companyID string, roleIDs []string, _ string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []string
	for _, r := range roleIDs {
		key := companyID + ":" + r
		out = append(out, f.roleEmails[key]...)
	}
	return out, nil
}

var _ MembershipEmailQuerier = (*fakeMembershipQuerier)(nil)

// fakeStepReader returns configured WorkflowStepConfig by stepID.
type fakeStepReader struct {
	steps map[string]*WorkflowStepConfig
}

func (f *fakeStepReader) GetStepByID(_ context.Context, stepID string) (*WorkflowStepConfig, error) {
	if s, ok := f.steps[stepID]; ok {
		return s, nil
	}
	return nil, nil
}

var _ WorkflowStepReader = (*fakeStepReader)(nil)

// fakeConfigRepoForResolver wraps fakeAlertConfigRepo in tests — we need a ConfigRepository fake.
type fakeReminderConfigReader struct {
	configs map[string]*ReminderConfigDTO // key: scopeID
}

func (f *fakeReminderConfigReader) UpsertByScope(_ context.Context, in ReminderConfigDTO) (*ReminderConfigDTO, error) {
	if f.configs == nil {
		f.configs = make(map[string]*ReminderConfigDTO)
	}
	f.configs[in.ScopeID] = &in
	return &in, nil
}

func (f *fakeReminderConfigReader) GetByScope(_ context.Context, _ ScopeType, scopeID string) (*ReminderConfigDTO, error) {
	if c, ok := f.configs[scopeID]; ok {
		return c, nil
	}
	return nil, errors.New("not found")
}

var _ ConfigRepository = (*fakeReminderConfigReader)(nil)

// ── ResolveForDeadline tests ───────────────────────────────────────────────

func TestResolveForDeadline_Individuals(t *testing.T) {
	cfgRepo := &fakeReminderConfigReader{configs: map[string]*ReminderConfigDTO{
		"scope-1": {
			ScopeType: ScopeTypeDisclosure,
			ScopeID:   "scope-1",
			Config: ReminderConfigInput{
				RecipientType: ReminderRecipientTypeIndividuals,
				Recipients:    []string{"alice@example.com", "bob@example.com"},
			},
		},
	}}
	r := NewRecipientResolver(cfgRepo, nil, nil, nil)
	emails, err := r.ResolveForDeadline(context.Background(), "company-1", "scope-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 2 {
		t.Errorf("expected 2 emails, got %v", emails)
	}
}

func TestResolveForDeadline_Departments(t *testing.T) {
	cfgRepo := &fakeReminderConfigReader{configs: map[string]*ReminderConfigDTO{
		"scope-2": {
			ScopeType: ScopeTypeDisclosure,
			ScopeID:   "scope-2",
			Config: ReminderConfigInput{
				RecipientType: ReminderRecipientTypeDepartments,
				Departments:   []string{"dept-a"},
			},
		},
	}}
	querier := &fakeMembershipQuerier{
		departmentEmails: map[string][]string{
			"c1:dept-a": {"cfo@co.com", "ceo@co.com"},
		},
	}
	r := NewRecipientResolver(cfgRepo, nil, querier, nil)
	emails, err := r.ResolveForDeadline(context.Background(), "c1", "scope-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 2 {
		t.Errorf("expected 2 emails, got %v", emails)
	}
}

func TestResolveForDeadline_Both_Deduplicates(t *testing.T) {
	cfgRepo := &fakeReminderConfigReader{configs: map[string]*ReminderConfigDTO{
		"scope-3": {
			ScopeType: ScopeTypeDisclosure,
			ScopeID:   "scope-3",
			Config: ReminderConfigInput{
				RecipientType: ReminderRecipientTypeBoth,
				Departments:   []string{"dept-b"},
				Recipients:    []string{"shared@co.com", "solo@co.com"},
			},
		},
	}}
	querier := &fakeMembershipQuerier{
		departmentEmails: map[string][]string{
			"c1:dept-b": {"shared@co.com", "dept-only@co.com"},
		},
	}
	r := NewRecipientResolver(cfgRepo, nil, querier, nil)
	emails, err := r.ResolveForDeadline(context.Background(), "c1", "scope-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// shared@co.com appears in both; after dedup: 3 unique
	if len(emails) != 3 {
		t.Errorf("expected 3 unique emails, got %v", emails)
	}
}

func TestResolveForDeadline_CrossTenant_Empty(t *testing.T) {
	// Querier maps "company-correct:dept-x" → emails, but we call with wrong company.
	cfgRepo := &fakeReminderConfigReader{configs: map[string]*ReminderConfigDTO{
		"scope-4": {
			ScopeType: ScopeTypeDisclosure,
			ScopeID:   "scope-4",
			Config: ReminderConfigInput{
				RecipientType: ReminderRecipientTypeDepartments,
				Departments:   []string{"dept-x"},
			},
		},
	}}
	querier := &fakeMembershipQuerier{
		departmentEmails: map[string][]string{
			"company-correct:dept-x": {"secret@other.com"},
		},
	}
	r := NewRecipientResolver(cfgRepo, nil, querier, nil)
	emails, err := r.ResolveForDeadline(context.Background(), "company-wrong", "scope-4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// company-wrong does not exist in querier → empty result
	if len(emails) != 0 {
		t.Errorf("expected no emails for wrong company, got %v", emails)
	}
}

func TestResolveForDeadline_NoConfig_ReturnsEmpty(t *testing.T) {
	cfgRepo := &fakeReminderConfigReader{configs: map[string]*ReminderConfigDTO{}} // empty
	r := NewRecipientResolver(cfgRepo, nil, nil, nil)
	emails, err := r.ResolveForDeadline(context.Background(), "c1", "missing-scope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 0 {
		t.Errorf("expected empty, got %v", emails)
	}
}

// ── ResolveForWorkflowStep tests ────────────────────────────────────────────

func TestResolveForWorkflowStep_ByRole(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", AssigneeRoleIDs: []string{"role-reviewer"}, DepartmentID: ""},
	}}
	querier := &fakeMembershipQuerier{
		roleEmails: map[string][]string{
			"c1:role-reviewer": {"rev1@co.com", "rev2@co.com"},
		},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 2 {
		t.Errorf("expected 2 emails, got %v", emails)
	}
}

func TestResolveForWorkflowStep_CrossTenant_Empty(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-2": {StepID: "step-2", AssigneeRoleIDs: []string{"role-approver"}, DepartmentID: ""},
	}}
	querier := &fakeMembershipQuerier{
		roleEmails: map[string][]string{
			"company-correct:role-approver": {"secret@other.com"},
		},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "company-wrong", "step-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 0 {
		t.Errorf("expected empty for wrong company, got %v", emails)
	}
}

func TestResolveForWorkflowStep_StepNotFound_ReturnsEmpty(t *testing.T) {
	r := NewRecipientResolver(nil, &fakeStepReader{steps: map[string]*WorkflowStepConfig{}}, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "nonexistent-step")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 0 {
		t.Errorf("expected empty, got %v", emails)
	}
}

func TestResolveForWorkflowStep_EmptyCompanyID_ReturnsEmpty(t *testing.T) {
	r := NewRecipientResolver(nil, nil, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "", "step-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 0 {
		t.Errorf("expected empty for empty companyID, got %v", emails)
	}
}

// ── DispatchCandidate new fields regression test ───────────────────────────

func TestDispatchCandidate_NewFieldsDefaultToZero(t *testing.T) {
	// Existing tests construct DispatchCandidate without new fields — must still compile and work.
	c := DispatchCandidate{
		OccurrenceID:    "occ-1",
		IdempotencyKey:  "idem-1",
		TemplateCode:    "REMINDER_DISCLOSURE_DUE",
		TemplatePayload: map[string]any{},
		RecipientEmails: []string{"a@example.com"},
	}
	// New fields default to zero values — no panic.
	if c.ScopeType != "" {
		t.Errorf("expected empty ScopeType, got %q", c.ScopeType)
	}
	if c.DisclosureTypeID != "" {
		t.Errorf("expected empty DisclosureTypeID, got %q", c.DisclosureTypeID)
	}
	if c.CompanyID != "" {
		t.Errorf("expected empty CompanyID, got %q", c.CompanyID)
	}
	if c.CompanyName != "" {
		t.Errorf("expected empty CompanyName, got %q", c.CompanyName)
	}
	if c.ScopeID != "" {
		t.Errorf("expected empty ScopeID, got %q", c.ScopeID)
	}
}

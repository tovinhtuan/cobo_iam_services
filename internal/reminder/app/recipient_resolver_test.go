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
	// taskAssigneeEmails maps companyID:workflowInstanceID:stepCode → emails
	taskAssigneeEmails map[string][]string
	// adminEmails maps companyID → emails (fallback admin list)
	adminEmails map[string][]string
	// membershipEmails maps companyID:membershipID → emails
	membershipEmails map[string][]string
	// companyDepartments maps companyID:snapshotKey → company department_id
	companyDepartments map[string]string
	// departmentHeadEmails maps companyID:departmentID → emails
	departmentHeadEmails map[string][]string
	err                  error
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

func (f *fakeMembershipQuerier) AdminEmailsByCompany(_ context.Context, companyID string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.adminEmails[companyID], nil
}

func (f *fakeMembershipQuerier) EmailsByMemberships(_ context.Context, companyID string, membershipIDs []string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []string
	for _, id := range membershipIDs {
		out = append(out, f.membershipEmails[companyID+":"+id]...)
	}
	return out, nil
}

func (f *fakeMembershipQuerier) ResolveCompanyDepartment(_ context.Context, companyID, snapshotDepartmentKey string) (string, bool, error) {
	if f.err != nil {
		return "", false, f.err
	}
	id, ok := f.companyDepartments[companyID+":"+snapshotDepartmentKey]
	if !ok || id == "" {
		return "", false, nil
	}
	return id, true, nil
}

func (f *fakeMembershipQuerier) EmailsByDepartmentHead(_ context.Context, companyID, departmentID string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.departmentHeadEmails[companyID+":"+departmentID], nil
}

func (f *fakeMembershipQuerier) AssigneeEmailsByStep(_ context.Context, companyID, workflowInstanceID, stepCode string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.taskAssigneeEmails == nil {
		return nil, nil
	}
	return f.taskAssigneeEmails[companyID+":"+workflowInstanceID+":"+stepCode], nil
}

var _ MembershipEmailQuerier = (*fakeMembershipQuerier)(nil)
var _ WorkflowTaskAssigneeReader = (*fakeMembershipQuerier)(nil)

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
	r := NewRecipientResolver(cfgRepo, nil, nil, nil, nil)
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
	r := NewRecipientResolver(cfgRepo, nil, querier, nil, nil)
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
	r := NewRecipientResolver(cfgRepo, nil, querier, nil, nil)
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
	r := NewRecipientResolver(cfgRepo, nil, querier, nil, nil)
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
	r := NewRecipientResolver(cfgRepo, nil, nil, nil, nil)
	emails, err := r.ResolveForDeadline(context.Background(), "c1", "missing-scope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 0 {
		t.Errorf("expected empty, got %v", emails)
	}
}

// ── ResolveForWorkflowStep tests ────────────────────────────────────────────

func TestResolveForWorkflowStep_InstanceSnapshotPreferredOverGlobal(t *testing.T) {
	global := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-001": {StepID: "step-001", AssigneeRoleIDs: []string{"wrong-role"}, DepartmentID: "cms-dept-x"},
	}}
	instance := &fakeInstanceStepReader{steps: map[string]*WorkflowStepConfig{
		"wf-a:step-001": {
			StepID:       "step-001",
			DepartmentID: "dept-a",
			StageName:    "QA Alert Step 1 Dept A",
		},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments: map[string]string{
			"c1:dept-a":     "dept-a",
			"c1:cms-dept-x": "cms-dept-x",
		},
		departmentHeadEmails: map[string][]string{
			"c1:dept-a":     {"head-a@co.com"},
			"c1:cms-dept-x": {"cms-head@co.com"},
		},
		roleEmails: map[string][]string{
			"c1:wrong-role": {"wrong@co.com"},
		},
		taskAssigneeEmails: map[string][]string{
			"c1:wf-a:step-001": {"creator@co.com"},
		},
		adminEmails: map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, global, querier, querier, nil)
	SetInstanceStepReader(r, instance)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "wf-a", "step-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "head-a@co.com" {
		t.Fatalf("expected snapshot department head, got %v", emails)
	}
}

type fakeInstanceStepReader struct {
	steps map[string]*WorkflowStepConfig
}

func (f *fakeInstanceStepReader) GetStepByInstance(_ context.Context, companyID, workflowInstanceID, stepID string) (*WorkflowStepConfig, error) {
	if f == nil || f.steps == nil {
		return nil, nil
	}
	return f.steps[workflowInstanceID+":"+stepID], nil
}

func TestResolveForWorkflowStep_CMSDefaultMatchingDepartmentHead(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {
			StepID:          "step-1",
			DepartmentID:    "dept-x",
			AssigneeRoleIDs: []string{"admin_doanh_nghiep"},
		},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{"c1:dept-x": {"head-x@co.com"}},
		roleEmails:           map[string][]string{"c1:admin_doanh_nghiep": {"admin@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "head-x@co.com" {
		t.Fatalf("expected department head, not admin role, got %v", emails)
	}
}

func TestResolveForWorkflowStep_CMSDefaultMissingDepartment_EnterpriseAdmin(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "cms-dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments: map[string]string{},
		taskAssigneeEmails: map[string][]string{"c1:wf-1:step-1": {"stale-assignee@co.com"}},
		departmentHeadEmails: map[string][]string{
			"c1:unrelated": {"other-head@co.com"},
		},
		adminEmails: map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, querier, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "wf-1", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "admin@co.com" {
		t.Fatalf("expected enterprise admin fallback, got %v", emails)
	}
}

func TestResolveForWorkflowStep_CompanyOverrideDepartmentHead(t *testing.T) {
	instance := &fakeInstanceStepReader{steps: map[string]*WorkflowStepConfig{
		"wf-c:step-1": {StepID: "step-1", DepartmentID: "dept-y"},
	}}
	global := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x", AssigneeRoleIDs: []string{"cms-role"}},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments: map[string]string{
			"c1:dept-x": "dept-x",
			"c1:dept-y": "dept-y",
		},
		departmentHeadEmails: map[string][]string{
			"c1:dept-x": {"head-x@co.com"},
			"c1:dept-y": {"head-y@co.com"},
		},
		roleEmails:  map[string][]string{"c1:cms-role": {"cms-role@co.com"}},
		adminEmails: map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, global, querier, nil, nil)
	SetInstanceStepReader(r, instance)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "wf-c", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "head-y@co.com" {
		t.Fatalf("expected override department head, got %v", emails)
	}
}

func TestResolveForWorkflowStep_CompanyOverrideDirectAssignee(t *testing.T) {
	instance := &fakeInstanceStepReader{steps: map[string]*WorkflowStepConfig{
		"wf-d:step-1": {
			StepID:                "step-1",
			DepartmentID:          "dept-x",
			AssigneeMembershipIDs: []string{"mem-c"},
		},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x", "c1:dept-y": "dept-y"},
		departmentHeadEmails: map[string][]string{"c1:dept-x": {"head-x@co.com"}, "c1:dept-y": {"head-y@co.com"}},
		membershipEmails:     map[string][]string{"c1:mem-c": {"user-c@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, nil, querier, nil, nil)
	SetInstanceStepReader(r, instance)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "wf-d", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "user-c@co.com" {
		t.Fatalf("expected direct assignee, got %v", emails)
	}
}

func TestResolveForWorkflowStep_DirectAssigneeOutranksDepartment(t *testing.T) {
	instance := &fakeInstanceStepReader{steps: map[string]*WorkflowStepConfig{
		"wf-d:step-1": {
			StepID:                "step-1",
			DepartmentID:          "dept-y",
			AssigneeMembershipIDs: []string{"mem-c"},
		},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-y": "dept-y"},
		departmentHeadEmails: map[string][]string{"c1:dept-y": {"head-y@co.com"}},
		membershipEmails:     map[string][]string{"c1:mem-c": {"user-c@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, nil, querier, nil, nil)
	SetInstanceStepReader(r, instance)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "wf-d", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "user-c@co.com" {
		t.Fatalf("direct assignee must outrank department head, got %v", emails)
	}
}

func TestResolveForWorkflowStep_NoCrossCompanyDepartmentLeak(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"company-correct:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{"company-correct:dept-x": {"secret@other.com"}},
		adminEmails:          map[string][]string{"company-wrong": {"admin-wrong@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "company-wrong", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "admin-wrong@co.com" {
		t.Fatalf("expected same-company admin fallback, not cross-company head, got %v", emails)
	}
}

func TestResolveForWorkflowStep_NoCrossDepartmentHeadLeak(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments: map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{
			"c1:dept-x": {"head-x@co.com"},
			"c1:dept-y": {"head-y@co.com"},
		},
		adminEmails: map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "head-x@co.com" {
		t.Fatalf("expected dept X head only, got %v", emails)
	}
}

func TestResolveForWorkflowStep_InactiveMembershipIgnoredForDirectAssignee(t *testing.T) {
	instance := &fakeInstanceStepReader{steps: map[string]*WorkflowStepConfig{
		"wf-d:step-1": {StepID: "step-1", DepartmentID: "dept-x", AssigneeMembershipIDs: []string{"mem-inactive"}},
	}}
	querier := &fakeMembershipQuerier{
		membershipEmails:     map[string][]string{}, // inactive → no email
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{"c1:dept-x": {"head-x@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, nil, querier, nil, nil)
	SetInstanceStepReader(r, instance)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "wf-d", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 0 {
		t.Fatalf("inactive direct assignee must not fall through to dept/admin, got %v", emails)
	}
}

func TestResolveForWorkflowStep_E1A_NoHeadOneActiveEmployee(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{},
		departmentEmails:     map[string][]string{"c1:dept-x": {"emp-a@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "emp-a@co.com" {
		t.Fatalf("E1-A expected employee A only, got %v", emails)
	}
}

func TestResolveForWorkflowStep_E1A_NoHeadMultipleActiveEmployees(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{},
		departmentEmails:     map[string][]string{"c1:dept-x": {"emp-a@co.com", "emp-b@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 2 {
		t.Fatalf("E1-A expected A and B, got %v", emails)
	}
	seen := map[string]bool{}
	for _, e := range emails {
		seen[e] = true
	}
	if !seen["emp-a@co.com"] || !seen["emp-b@co.com"] {
		t.Fatalf("E1-A expected A and B, got %v", emails)
	}
	if seen["admin@co.com"] {
		t.Fatalf("E1-A must not add EA fallback, got %v", emails)
	}
}

func TestResolveForWorkflowStep_E1A_InactiveEmployeeExcluded(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{},
		departmentEmails:     map[string][]string{"c1:dept-x": {"emp-a@co.com"}}, // inactive B omitted by query
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "emp-a@co.com" {
		t.Fatalf("E1-A3 expected A only, got %v", emails)
	}
}

func TestResolveForWorkflowStep_E1A_WrongDepartmentExcluded(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{},
		departmentEmails: map[string][]string{
			"c1:dept-x": {"emp-a@co.com"},
			"c1:dept-y": {"emp-b@co.com"},
		},
		adminEmails: map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "emp-a@co.com" {
		t.Fatalf("E1-A4 expected Dept X employee only, got %v", emails)
	}
}

func TestResolveForWorkflowStep_E1A_CrossCompanyEmployeeExcluded(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x", "c2:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{},
		departmentEmails: map[string][]string{
			"c1:dept-x": {"emp-a@co.com"},
			"c2:dept-x": {"emp-other@co.com"},
		},
		adminEmails: map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "emp-a@co.com" {
		t.Fatalf("E1-A5 expected same-company employee only, got %v", emails)
	}
}

func TestResolveForWorkflowStep_E1B_NoHeadNoEmployees_EnterpriseAdmin(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{},
		departmentEmails:     map[string][]string{},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "admin@co.com" {
		t.Fatalf("E1-B expected EA fallback, got %v", emails)
	}
}

func TestResolveForWorkflowStep_E1A_ValidHeadDoesNotBroadcastEmployees(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{"c1:dept-x": {"head-x@co.com"}},
		departmentEmails:     map[string][]string{"c1:dept-x": {"emp-a@co.com", "emp-b@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "head-x@co.com" {
		t.Fatalf("valid head must not broadcast employees, got %v", emails)
	}
}

func TestResolveForWorkflowStep_E1A_OverrideDepartmentUsesEffectiveDept(t *testing.T) {
	instance := &fakeInstanceStepReader{steps: map[string]*WorkflowStepConfig{
		"wf-c:step-1": {StepID: "step-1", DepartmentID: "dept-y"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments: map[string]string{"c1:dept-x": "dept-x", "c1:dept-y": "dept-y"},
		departmentHeadEmails: map[string][]string{
			"c1:dept-x": {"head-x@co.com"},
		},
		departmentEmails: map[string][]string{
			"c1:dept-x": {"emp-x@co.com"},
			"c1:dept-y": {"emp-y1@co.com", "emp-y2@co.com"},
		},
		adminEmails: map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, nil, querier, nil, nil)
	SetInstanceStepReader(r, instance)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "wf-c", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 2 {
		t.Fatalf("override E1 expected Dept Y employees, got %v", emails)
	}
	seen := map[string]bool{}
	for _, e := range emails {
		seen[e] = true
	}
	if !seen["emp-y1@co.com"] || !seen["emp-y2@co.com"] || seen["emp-x@co.com"] || seen["admin@co.com"] || seen["head-x@co.com"] {
		t.Fatalf("override E1 leak, got %v", emails)
	}
}

func TestResolveForWorkflowStep_E1A_DirectAssigneeOutranksNoHeadBroadcast(t *testing.T) {
	instance := &fakeInstanceStepReader{steps: map[string]*WorkflowStepConfig{
		"wf-d:step-1": {StepID: "step-1", DepartmentID: "dept-x", AssigneeMembershipIDs: []string{"mem-c"}},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{},
		departmentEmails:     map[string][]string{"c1:dept-x": {"emp-a@co.com"}},
		membershipEmails:     map[string][]string{"c1:mem-c": {"user-c@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, nil, querier, nil, nil)
	SetInstanceStepReader(r, instance)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "wf-d", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "user-c@co.com" {
		t.Fatalf("direct assignee must outrank E1 broadcast, got %v", emails)
	}
}

func TestResolveForWorkflowStep_E1A_DuplicateEmployeeDeduped(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{},
		departmentEmails:     map[string][]string{"c1:dept-x": {"emp-a@co.com", "EMP-A@co.com", "emp-a@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "emp-a@co.com" {
		t.Fatalf("duplicate employee must dedupe to one, got %v", emails)
	}
}

func TestResolveForWorkflowStep_E1A_AdminInDepartmentAppearsOnceNotAsFallback(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{},
		departmentEmails:     map[string][]string{"c1:dept-x": {"admin@co.com", "emp-a@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 2 {
		t.Fatalf("EA as department employee may appear once with peers, got %v", emails)
	}
	seen := map[string]int{}
	for _, e := range emails {
		seen[e]++
	}
	if seen["admin@co.com"] != 1 || seen["emp-a@co.com"] != 1 {
		t.Fatalf("expected one admin (as employee) + A, got %v", emails)
	}
}

func TestResolveForWorkflowStep_MatchingDepartmentNoHead_DoesNotInventAdmin(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{},
		departmentEmails:     map[string][]string{"c1:dept-x": {"emp-a@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "emp-a@co.com" {
		t.Fatalf("E1 unmatched head with employees must broadcast employees, not EA, got %v", emails)
	}
}

func TestResolveForWorkflowStep_StepNotFound_AdminFallbackNotTaskAssignee(t *testing.T) {
	querier := &fakeMembershipQuerier{
		taskAssigneeEmails: map[string][]string{
			"c1:wf-1:nonexistent-step": {"assignee@co.com"},
		},
		adminEmails: map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, &fakeStepReader{steps: map[string]*WorkflowStepConfig{}}, querier, querier, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "wf-1", "nonexistent-step")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "admin@co.com" {
		t.Fatalf("expected admin fallback, not stale task assignee, got %v", emails)
	}
}

func TestResolveForWorkflowStep_EmptyCompanyID_ReturnsEmpty(t *testing.T) {
	r := NewRecipientResolver(nil, nil, nil, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "", "", "step-3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 0 {
		t.Errorf("expected empty for empty companyID, got %v", emails)
	}
}

func TestResolveForWorkflowStep_IgnoresAssigneeRoleWhenDepartmentHeadResolves(t *testing.T) {
	stepReader := &fakeStepReader{steps: map[string]*WorkflowStepConfig{
		"step-1": {StepID: "step-1", AssigneeRoleIDs: []string{"role-reviewer"}, DepartmentID: "dept-x"},
	}}
	querier := &fakeMembershipQuerier{
		companyDepartments:   map[string]string{"c1:dept-x": "dept-x"},
		departmentHeadEmails: map[string][]string{"c1:dept-x": {"head-x@co.com"}},
		roleEmails:           map[string][]string{"c1:role-reviewer": {"reviewer@co.com"}},
		adminEmails:          map[string][]string{"c1": {"admin@co.com"}},
	}
	r := NewRecipientResolver(nil, stepReader, querier, nil, nil)
	emails, err := r.ResolveForWorkflowStep(context.Background(), "c1", "", "step-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(emails) != 1 || emails[0] != "head-x@co.com" {
		t.Errorf("expected department head (not role), got %v", emails)
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

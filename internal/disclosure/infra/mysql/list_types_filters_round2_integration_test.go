package mysql

import (
	"context"
	"database/sql"
	"testing"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// Round 2 of docs/ai-cache/bug-disclosure-type-list-flows-2026-10-14 (F5, F8, F9, F10, F13).
// Fixtures reuse dfCompany / dfTenantDept from list_types_department_filter_integration_test.go.
const (
	r2OtherCompany   = "it-dtf-co-b"
	r2OtherDept      = "it-dtf-dep-b"
	r2TypeOverridden = "it-dtf-r2-overridden"     // template dept-001, active company override → tenant dept
	r2TypeCompany    = "it-dtf-r2-company"        // company type, no template workflow, active override → tenant dept
	r2TypeOtherCo    = "it-dtf-r2-other-override" // template dept-002, override only for another company
	r2TypeDraftOvr   = "it-dtf-r2-draft-override" // template dept-003, override not approved (active_version_no = 0)
	// System template (company_id NULL) in display group 003, template dept-004, overridden by dfCompany with a
	// department that is now inactive. Visible to every tenant while the test runs (fixture is removed after).
	r2TypeGlobalOvr = "it-dtf-r2-global-override"
	r2InactiveDept  = "it-dtf-dep-old"
)

func seedRound2Fixture(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	cleanup := func() {
		for _, q := range []string{
			`DELETE FROM company_template_workflow_overrides WHERE type_id LIKE 'it-dtf-%'`,
			`DELETE FROM disclosure_type_versions WHERE type_id LIKE 'it-dtf-%'`,
			`DELETE FROM disclosure_types WHERE type_id LIKE 'it-dtf-%'`,
			`DELETE FROM departments WHERE department_id IN ('` + dfTenantDept + `', '` + r2OtherDept + `', '` + r2InactiveDept + `')`,
			`DELETE FROM companies WHERE company_id IN ('` + dfCompany + `', '` + r2OtherCompany + `')`,
		} {
			if _, err := db.ExecContext(ctx, q); err != nil {
				t.Errorf("cleanup %q: %v", q, err)
			}
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	for _, co := range []string{dfCompany, r2OtherCompany} {
		exec(`INSERT INTO companies (company_id, company_code, company_name, status) VALUES (?, ?, ?, 'active')`, co, co, co)
	}
	exec(`INSERT INTO departments (department_id, company_id, department_code, department_name, status) VALUES (?, ?, 'IT-DTF-HR', 'Phòng Nhân sự IT', 'active')`, dfTenantDept, dfCompany)
	exec(`INSERT INTO departments (department_id, company_id, department_code, department_name, status) VALUES (?, ?, 'IT-DTF-B', 'Phòng của công ty B', 'active')`, r2OtherDept, r2OtherCompany)
	exec(`INSERT INTO departments (department_id, company_id, department_code, department_name, status) VALUES (?, ?, 'IT-DTF-OLD', 'Phòng cũ đã ngừng', 'inactive')`, r2InactiveDept, dfCompany)

	addTypeOwned := func(owner any, typeID, name, mode string, manifest any, tags string) {
		exec(`INSERT INTO disclosure_types (type_id, group_id, company_id, active_version_no) VALUES (?, 'group-006', ?, 1)`, typeID, owner)
		exec(`INSERT INTO disclosure_type_versions
			(type_id, version_no, name, category, template_category, description, deadline_rule, updated_by, workflow_manifest_json, workflow_authority_mode, tags_json)
			VALUES (?, 1, ?, 'Tùy chỉnh', 'irregular', '', 'T+1', 'it', ?, ?, ?)`, typeID, name, manifest, mode, tags)
	}
	addType := func(typeID, name, mode string, manifest any, tags string) {
		addTypeOwned(dfCompany, typeID, name, mode, manifest, tags)
	}
	addOverride := func(companyID, typeID string, activeVersion int, state, workflow string) {
		overrideID := "it-dtf-ovr-" + companyID + "-" + typeID
		exec(`INSERT INTO company_template_workflow_overrides (override_id, company_id, type_id, active_version_no, status, created_by, updated_by)
			VALUES (?, ?, ?, ?, ?, 'it', 'it')`, overrideID, companyID, typeID, activeVersion, state)
		exec(`INSERT INTO company_template_workflow_override_versions (override_id, version_no, workflow_json, state, created_by)
			VALUES (?, 1, ?, ?, 'it')`, overrideID, workflow, state)
	}

	addType(r2TypeOverridden, "it-dtf r2 tiến độ 50% kỳ", "TEMPLATE_PINNED", `{"steps":[{"department_id":"dept-001"}]}`, `["BCTC"]`)
	addOverride(dfCompany, r2TypeOverridden, 1, "approved", `[{"department_id":"`+dfTenantDept+`"}]`)

	addType(r2TypeCompany, "it-dtf r2 loại riêng", "LEGACY_COMPAT", nil, `["Báo cáo, quý"]`)
	addOverride(dfCompany, r2TypeCompany, 1, "approved", `[{"department_id":"`+dfTenantDept+`"},{"department_id":""}]`)

	addType(r2TypeOtherCo, "it-dtf r2 override công ty khác", "TEMPLATE_PINNED", `{"steps":[{"department_id":"dept-002"}]}`, `["bctc"]`)
	addOverride(r2OtherCompany, r2TypeOtherCo, 1, "approved", `[{"department_id":"`+r2OtherDept+`"}]`)

	addType(r2TypeDraftOvr, "it-dtf r2 override nháp", "TEMPLATE_PINNED", `{"steps":[{"department_id":"dept-003"}]}`, `[]`)
	addOverride(dfCompany, r2TypeDraftOvr, 0, "draft", `[{"department_id":"`+dfTenantDept+`"}]`)

	addTypeOwned(nil, r2TypeGlobalOvr, "it-dtf r2 mẫu hệ thống có override", "TEMPLATE_PINNED", `{"steps":[{"department_id":"dept-004"}]}`, `[]`)
	exec(`INSERT INTO template_display_groups (template_id, display_group_code) VALUES (?, 'display_groups_003')`, r2TypeGlobalOvr)
	addOverride(dfCompany, r2TypeGlobalOvr, 1, "approved", `[{"department_id":"`+r2InactiveDept+`"}]`)
}

func listIDs(t *testing.T, repo *Repository, params disclosureapp.ListTypesParams) map[string]bool {
	t.Helper()
	return listIDsFor(t, repo, dfCompany, params)
}

func listIDsFor(t *testing.T, repo *Repository, companyID string, params disclosureapp.ListTypesParams) map[string]bool {
	t.Helper()
	params.CompanyID = companyID
	params.Page, params.PageSize = 1, 100
	items, _, err := repo.ListTypes(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, it := range items {
		got[it.TypeID] = true
	}
	return got
}

// F5: the department filter follows the effective workflow (active company override > template).
func TestMySQL_ListTypes_DepartmentFilterFollowsEffectiveWorkflow(t *testing.T) {
	db := departmentFilterTestDB(t)
	seedRound2Fixture(t, db)
	repo := NewRepository(db)

	byTenant := listIDs(t, repo, disclosureapp.ListTypesParams{DepartmentID: dfTenantDept})
	if !byTenant[r2TypeOverridden] || !byTenant[r2TypeCompany] {
		t.Fatalf("tenant department from active overrides must match the overridden and the company type; got %v", byTenant)
	}
	if byTenant[r2TypeDraftOvr] {
		t.Fatalf("a draft (not approved) override must not count; got %v", byTenant)
	}
	if got := listIDs(t, repo, disclosureapp.ListTypesParams{DepartmentID: "dept-001"}); got[r2TypeOverridden] {
		t.Fatalf("the template department is replaced by the active override; got %v", got)
	}
	if got := listIDs(t, repo, disclosureapp.ListTypesParams{DepartmentID: "dept-003"}); !got[r2TypeDraftOvr] {
		t.Fatalf("without an approved override the template workflow applies; got %v", got)
	}
	if got := listIDs(t, repo, disclosureapp.ListTypesParams{DepartmentID: "dept-002"}); !got[r2TypeOtherCo] {
		t.Fatalf("another company's override must not replace this company's template workflow; got %v", got)
	}
	if got := listIDs(t, repo, disclosureapp.ListTypesParams{DepartmentID: r2OtherDept}); len(got) != 0 {
		t.Fatalf("another company's override department must match nothing; got %v", got)
	}
}

// F5 + F8: options come from the effective workflows of the given (visible) types only.
func TestMySQL_ListTypeFilterOptions_EffectiveWorkflowAndVisibleTypes(t *testing.T) {
	db := departmentFilterTestDB(t)
	seedRound2Fixture(t, db)
	repo := NewRepository(db)

	resp, err := repo.ListTypeFilterOptions(context.Background(), dfCompany, []string{r2TypeOverridden, r2TypeCompany, r2TypeDraftOvr})
	if err != nil {
		t.Fatal(err)
	}
	depts := map[string]string{}
	for _, d := range resp.Departments {
		depts[d.ID] = d.Name
	}
	if depts[dfTenantDept] != "Phòng Nhân sự IT" {
		t.Fatalf("tenant department from active overrides must be offered with its name; got %v", depts)
	}
	if _, ok := depts["dept-003"]; !ok {
		t.Fatalf("template department of a type without an approved override must be offered; got %v", depts)
	}
	for _, unexpected := range []string{"dept-001", "dept-002", r2OtherDept, ""} {
		if _, ok := depts[unexpected]; ok {
			t.Fatalf("%q must not be offered (overridden, not visible, other company or empty); got %v", unexpected, depts)
		}
	}
	tags := map[string]bool{}
	for _, tag := range resp.Tags {
		tags[tag.ID] = true
	}
	if !tags["BCTC"] || !tags["Báo cáo, quý"] || tags["bctc"] {
		t.Fatalf("tags must come from the given types only; got %v", tags)
	}
	// Every offered department selects at least one of the given types.
	for id, name := range depts {
		got := listIDs(t, repo, disclosureapp.ListTypesParams{DepartmentID: id})
		if !got[r2TypeOverridden] && !got[r2TypeCompany] && !got[r2TypeDraftOvr] {
			t.Fatalf("option %q (%s) selects none of the visible types; got %v", id, name, got)
		}
	}

	empty, err := repo.ListTypeFilterOptions(context.Background(), dfCompany, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(empty.Departments) != 0 || len(empty.Tags) != 0 {
		t.Fatalf("no visible type → no options; got %+v", empty)
	}
}

// F9: LIKE wildcards in the search are literal characters.
func TestMySQL_ListTypes_SearchEscapesLikeWildcards(t *testing.T) {
	db := departmentFilterTestDB(t)
	seedRound2Fixture(t, db)
	repo := NewRepository(db)

	got := listIDs(t, repo, disclosureapp.ListTypesParams{Query: "%"})
	if !got[r2TypeOverridden] || got[r2TypeCompany] || got[r2TypeOtherCo] {
		t.Fatalf(`q="%%" must match only names containing "%%"; got %v`, got)
	}
	if got := listIDs(t, repo, disclosureapp.ListTypesParams{Query: "_"}); got[r2TypeOverridden] || got[r2TypeCompany] {
		t.Fatalf(`q="_" must not match names without "_"; got %v`, got)
	}
}

// F10: tag matching ignores case; a tag containing a comma matches as one value.
func TestMySQL_ListTypes_TagFilterIgnoresCaseAndKeepsCommas(t *testing.T) {
	db := departmentFilterTestDB(t)
	seedRound2Fixture(t, db)
	repo := NewRepository(db)

	got := listIDs(t, repo, disclosureapp.ListTypesParams{Tags: []string{"bctc"}})
	if !got[r2TypeOverridden] || !got[r2TypeOtherCo] {
		t.Fatalf(`tag "bctc" must match "BCTC" and "bctc"; got %v`, got)
	}
	if got := listIDs(t, repo, disclosureapp.ListTypesParams{Tags: []string{"báo cáo, quý"}}); !got[r2TypeCompany] || len(got) != 1 {
		t.Fatalf("a tag with a comma must match exactly that tag; got %v", got)
	}
}

// F13: archiving a company type (review_status) frees its quota slot.
func TestMySQL_CountCompanyTemplates_ExcludesArchivedReviewStatus(t *testing.T) {
	db := departmentFilterTestDB(t)
	seedRound2Fixture(t, db)
	repo := NewRepository(db)
	ctx := context.Background()

	before, err := repo.CountCompanyTemplatesByCompanyID(ctx, dfCompany)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE disclosure_types SET review_status = 'archived' WHERE type_id = ?`, r2TypeCompany); err != nil {
		t.Fatal(err)
	}
	after, err := repo.CountCompanyTemplatesByCompanyID(ctx, dfCompany)
	if err != nil {
		t.Fatal(err)
	}
	if after != before-1 {
		t.Fatalf("archived company type must not count toward the quota: before=%d after=%d", before, after)
	}
}

// F5 on a system template (the common case): the company override replaces the template workflow for
// that company only; combined with the display-group JOIN (its argument is prepended) the order holds.
func TestMySQL_ListTypes_GlobalTemplateOverrideAndDisplayGroup(t *testing.T) {
	db := departmentFilterTestDB(t)
	seedRound2Fixture(t, db)
	repo := NewRepository(db)

	if got := listIDs(t, repo, disclosureapp.ListTypesParams{DepartmentID: r2InactiveDept}); !got[r2TypeGlobalOvr] {
		t.Fatalf("company override department must match the system template for that company; got %v", got)
	}
	if got := listIDsFor(t, repo, r2OtherCompany, disclosureapp.ListTypesParams{DepartmentID: "dept-004"}); !got[r2TypeGlobalOvr] {
		t.Fatalf("another company keeps the template workflow; got %v", got)
	}
	if got := listIDsFor(t, repo, r2OtherCompany, disclosureapp.ListTypesParams{DepartmentID: r2InactiveDept}); got[r2TypeGlobalOvr] {
		t.Fatalf("another company must not match dfCompany's override department; got %v", got)
	}
	withGroup := listIDs(t, repo, disclosureapp.ListTypesParams{DisplayGroupCode: "display_groups_003", DepartmentID: r2InactiveDept})
	if !withGroup[r2TypeGlobalOvr] || len(withGroup) != 1 {
		t.Fatalf("display group + department filter; got %v", withGroup)
	}
	if got := listIDs(t, repo, disclosureapp.ListTypesParams{DisplayGroupCode: "display_groups_003", DepartmentID: "dept-004"}); got[r2TypeGlobalOvr] {
		t.Fatalf("template department replaced by the override, also with a display group; got %v", got)
	}

	// A department that was deactivated after the override was saved still shows its name.
	resp, err := repo.ListTypeFilterOptions(context.Background(), dfCompany, []string{r2TypeGlobalOvr})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Departments) != 1 || resp.Departments[0].ID != r2InactiveDept || resp.Departments[0].Name != "Phòng cũ đã ngừng" {
		t.Fatalf("inactive department must keep its name, not the raw id: %+v", resp.Departments)
	}
}

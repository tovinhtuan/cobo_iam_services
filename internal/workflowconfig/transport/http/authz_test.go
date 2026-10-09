package http_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	authapp "github.com/cobo/cobo_iam_services/internal/authorization/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	wfcapp "github.com/cobo/cobo_iam_services/internal/workflowconfig/app"
	wfchttp "github.com/cobo/cobo_iam_services/internal/workflowconfig/transport/http"
)

// Personas: token == membership id; permissions resolved by fakeAuthorizer.
const (
	tokTenantAdmin  = "m-tenant-admin"
	tokTenantMember = "m-tenant-member"
	tokCMSReader    = "m-cms-reader"
	tokCMSMaker     = "m-cms-maker"
	tokCMSChecker   = "m-cms-checker"
	tokLegacyMaker  = "m-legacy-maker"   // disclosure_type.manage
	tokLegacyCheck  = "m-legacy-checker" // disclosure_type.publish
	tokLegacyConfig = "m-legacy-config"  // rbac.manage (read-only legacy alias)
	tokAuthzError   = "m-authz-error"    // fakeAuthorizer returns an error
)

var personaPermissions = map[string][]string{
	tokTenantAdmin:  {"rbac.manage", "admin.membership.invite", "system.settings"},
	tokTenantMember: {},
	tokCMSReader:    {"platform.cms.view", "cms.template.read"},
	tokCMSMaker:     {"platform.cms.view", "cms.template.write"},
	tokCMSChecker:   {"platform.cms.view", "cms.template.activate"},
	tokLegacyMaker:  {"platform.cms.view", "disclosure_type.manage"},
	tokLegacyCheck:  {"platform.cms.view", "disclosure_type.publish"},
	tokLegacyConfig: {"platform.cms.view", "rbac.manage"},
	tokAuthzError:   {},
}

type fakeInspector struct{}

func (fakeInspector) InspectAccessToken(_ context.Context, token string) (*iamapp.AccessTokenClaims, error) {
	if _, ok := personaPermissions[token]; !ok {
		return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid token", nil)
	}
	return &iamapp.AccessTokenClaims{Sub: "u-" + token, MembershipID: token, CompanyID: "c-" + token}, nil
}

func (fakeInspector) InspectPreCompanyToken(context.Context, string) (*iamapp.PreCompanyTokenClaims, error) {
	return nil, perr.NewHTTPError(http.StatusUnauthorized, perr.CodeSessionExpired, "invalid token", nil)
}

type fakeAuthorizer struct{}

func (fakeAuthorizer) GetEffectiveAccess(_ context.Context, membershipID, companyID string) (*authapp.EffectiveAccessSummary, error) {
	if membershipID == tokAuthzError {
		return nil, fmt.Errorf("resolver unavailable")
	}
	return &authapp.EffectiveAccessSummary{
		CompanyID:    companyID,
		MembershipID: membershipID,
		Permissions:  personaPermissions[membershipID],
	}, nil
}

// fakeVersionRepo: minimal in-memory VersionRepository (one published version "1").
type fakeVersionRepo struct {
	versions map[int]*wfcapp.VersionRecord
}

func newFakeVersionRepo() *fakeVersionRepo {
	at := time.Unix(0, 0)
	info := wfcapp.VersionInfo{TypeID: "t1", VersionNo: 1, State: wfcapp.VersionStatePublished, PublishedAt: &at}
	return &fakeVersionRepo{versions: map[int]*wfcapp.VersionRecord{1: {VersionInfo: info}}}
}

func (f *fakeVersionRepo) BuildManifest(_ context.Context, typeID string) (wfcapp.Manifest, error) {
	return wfcapp.Manifest{TypeID: typeID}, nil
}

func (f *fakeVersionRepo) ListVersions(context.Context, string) ([]wfcapp.VersionInfo, error) {
	out := make([]wfcapp.VersionInfo, 0, len(f.versions))
	for _, v := range f.versions {
		out = append(out, v.VersionInfo)
	}
	return out, nil
}

func (f *fakeVersionRepo) GetVersion(_ context.Context, _ string, n int) (*wfcapp.VersionRecord, error) {
	return f.versions[n], nil
}

func (f *fakeVersionRepo) GetActiveVersion(context.Context, string) (*wfcapp.VersionRecord, error) {
	return nil, nil
}

func (f *fakeVersionRepo) Publish(_ context.Context, m wfcapp.Manifest, note, actor string, at time.Time) (wfcapp.VersionInfo, error) {
	return wfcapp.VersionInfo{TypeID: m.TypeID, VersionNo: len(f.versions) + 1, State: wfcapp.VersionStatePublished, ChangeNote: note, PublishedBy: actor}, nil
}

func (f *fakeVersionRepo) Activate(_ context.Context, _ string, n int, actor string, _ time.Time) (wfcapp.VersionInfo, error) {
	v, ok := f.versions[n]
	if !ok {
		return wfcapp.VersionInfo{}, fmt.Errorf("version %d not found", n)
	}
	v.State = wfcapp.VersionStateActive
	v.ActivatedBy = actor
	return v.VersionInfo, nil
}

// newServer wires the versioning + catalog routes the same way internal/httpserver does.
func newServer(authorizer wfchttp.AccessResolver) *http.ServeMux {
	versionSvc := wfcapp.NewVersionService(newFakeVersionRepo(), nil)
	catalogSvc := wfcapp.NewAssigneeRoleCatalogService(wfcapp.NewInMemoryAssigneeRoleCatalog())
	readinessSvc := wfcapp.NewReadinessService(versionSvc, wfcapp.DefaultRoleRegistry())
	configSvc := wfcapp.NewConfigService(versionSvc, readinessSvc)
	mux := http.NewServeMux()
	wfchttp.RegisterAssigneeRoleCatalog(mux, catalogSvc, fakeInspector{}, authorizer)
	wfchttp.NewHandler(versionSvc, configSvc, catalogSvc, fakeInspector{}, authorizer).Register(mux)
	return mux
}

func do(t *testing.T, mux *http.ServeMux, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

const (
	pathPublish  = "/api/v1/platform/cms/templates/t1/workflow/publish"
	pathActivate = "/api/v1/platform/cms/templates/t1/workflow/versions/1/activate"
)

func isAuthzStatus(code int) bool {
	return code == http.StatusUnauthorized || code == http.StatusForbidden || code == http.StatusServiceUnavailable
}

func TestAuthz_PublishActivate(t *testing.T) {
	mux := newServer(fakeAuthorizer{})
	cases := []struct {
		name  string
		path  string
		token string
		want  int // exact status; 0 = "allowed" (any status that is not 401/403/503)
	}{
		{"tenant admin cannot publish", pathPublish, tokTenantAdmin, http.StatusForbidden},
		{"tenant member cannot publish", pathPublish, tokTenantMember, http.StatusForbidden},
		{"cms reader cannot publish", pathPublish, tokCMSReader, http.StatusForbidden},
		{"cms checker cannot publish", pathPublish, tokCMSChecker, http.StatusForbidden},
		{"cms maker can publish", pathPublish, tokCMSMaker, 0},
		{"tenant admin cannot activate", pathActivate, tokTenantAdmin, http.StatusForbidden},
		{"tenant member cannot activate", pathActivate, tokTenantMember, http.StatusForbidden},
		{"cms maker cannot activate (checker action)", pathActivate, tokCMSMaker, http.StatusForbidden},
		{"cms checker can activate", pathActivate, tokCMSChecker, 0},
		{"legacy manage can publish", pathPublish, tokLegacyMaker, 0},
		{"legacy manage cannot activate (maker)", pathActivate, tokLegacyMaker, http.StatusForbidden},
		{"legacy publish can activate", pathActivate, tokLegacyCheck, 0},
		{"legacy publish cannot publish", pathPublish, tokLegacyCheck, http.StatusForbidden},
		{"rbac.manage alias cannot publish", pathPublish, tokLegacyConfig, http.StatusForbidden},
		{"rbac.manage alias cannot activate", pathActivate, tokLegacyConfig, http.StatusForbidden},
		{"no token publish", pathPublish, "", http.StatusUnauthorized},
		{"no token activate", pathActivate, "", http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, mux, http.MethodPost, tc.path, tc.token, `{"change_note":"x"}`)
			if tc.want == 0 {
				if isAuthzStatus(rec.Code) {
					t.Fatalf("expected allowed, got %d: %s", rec.Code, rec.Body.String())
				}
				return
			}
			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d: %s", tc.want, rec.Code, rec.Body.String())
			}
			if tc.want == http.StatusForbidden && !strings.Contains(rec.Body.String(), string(perr.CodePermissionDenied)) {
				t.Fatalf("expected %s body, got %s", perr.CodePermissionDenied, rec.Body.String())
			}
		})
	}
}

func TestAuthz_NilAuthorizerFailsClosed(t *testing.T) {
	mux := newServer(nil)
	for _, rt := range allRoutes {
		if rt.tenantOK {
			continue
		}
		rec := do(t, mux, rt.method, rt.path, tokCMSMaker, `{}`)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s %s: expected 503 with nil authorizer, got %d", rt.method, rt.path, rec.Code)
		}
	}
}

func TestAuthz_ResolverErrorFailsClosed(t *testing.T) {
	mux := newServer(fakeAuthorizer{})
	for _, rt := range allRoutes {
		if rt.tenantOK {
			continue
		}
		if rec := do(t, mux, rt.method, rt.path, tokAuthzError, `{}`); rec.Code < 400 {
			t.Errorf("%s %s: expected error status when resolver fails, got %d", rt.method, rt.path, rec.Code)
		}
	}
}

func TestAuthz_ActorIsTokenUser(t *testing.T) {
	mux := newServer(fakeAuthorizer{})
	rec := do(t, mux, http.MethodPost, pathActivate, tokCMSChecker, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"activated_by":"u-`+tokCMSChecker+`"`) {
		t.Fatalf("expected activated_by to be the token user, got %d: %s", rec.Code, rec.Body.String())
	}
}

var readRoutes = []struct{ method, path string }{
	{http.MethodGet, "/api/v1/platform/cms/templates/t1/workflow/configuration"},
	{http.MethodGet, "/api/v1/platform/cms/templates/t1/workflow/readiness"},
	{http.MethodPost, "/api/v1/platform/cms/templates/t1/workflow/validate"},
	{http.MethodGet, "/api/v1/platform/cms/templates/t1/workflow/lifecycle"},
	{http.MethodGet, "/api/v1/platform/cms/templates/t1/workflow/versions"},
	{http.MethodGet, "/api/v1/platform/cms/templates/t1/workflow/versions/1"},
}

func TestAuthz_ReadRoutes(t *testing.T) {
	mux := newServer(fakeAuthorizer{})
	for _, rt := range readRoutes {
		for _, tok := range []string{tokTenantAdmin, tokTenantMember} {
			rec := do(t, mux, rt.method, rt.path, tok, "")
			if rec.Code != http.StatusForbidden {
				t.Errorf("%s %s as %s: expected 403, got %d", rt.method, rt.path, tok, rec.Code)
			}
		}
		for _, tok := range []string{tokCMSReader, tokCMSMaker, tokCMSChecker, tokLegacyMaker, tokLegacyConfig} {
			if rec := do(t, mux, rt.method, rt.path, tok, ""); isAuthzStatus(rec.Code) {
				t.Errorf("%s %s as %s: expected allowed, got %d: %s", rt.method, rt.path, tok, rec.Code, rec.Body.String())
			}
		}
		if rec := do(t, mux, rt.method, rt.path, "", ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: expected 401, got %d", rt.method, rt.path, rec.Code)
		}
	}
}

const pathAssigneeRoles = "/api/v1/platform/cms/workflow/assignee-roles"

func TestAuthz_AssigneeRoles(t *testing.T) {
	mux := newServer(fakeAuthorizer{})
	// GET stays token-only: the tenant portal resolves role labels from it.
	for _, tok := range []string{tokTenantMember, tokTenantAdmin, tokCMSReader} {
		if rec := do(t, mux, http.MethodGet, pathAssigneeRoles, tok, ""); rec.Code != http.StatusOK {
			t.Errorf("GET as %s: expected 200, got %d", tok, rec.Code)
		}
	}
	if rec := do(t, mux, http.MethodGet, pathAssigneeRoles, "", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("GET without token: expected 401, got %d", rec.Code)
	}
	for _, tok := range []string{tokTenantAdmin, tokTenantMember, tokCMSReader, tokCMSChecker} {
		if rec := do(t, mux, http.MethodPost, pathAssigneeRoles, tok, `{"role_name":"Vai trò thử"}`); rec.Code != http.StatusForbidden {
			t.Errorf("POST as %s: expected 403, got %d: %s", tok, rec.Code, rec.Body.String())
		}
	}
	if rec := do(t, mux, http.MethodPost, pathAssigneeRoles, tokCMSMaker, `{"role_name":"Vai trò thử"}`); rec.Code != http.StatusCreated {
		t.Errorf("POST as cms maker: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

// allRoutes must list every route registered by Register + RegisterAssigneeRoleCatalog. Only GET
// assignee-roles may be reachable without platform CMS access.
var allRoutes = []struct {
	method, path string
	tenantOK     bool
}{
	{http.MethodGet, pathAssigneeRoles, true},
	{http.MethodPost, pathAssigneeRoles, false},
	{http.MethodGet, "/api/v1/platform/cms/templates/t1/workflow/configuration", false},
	{http.MethodGet, "/api/v1/platform/cms/templates/t1/workflow/readiness", false},
	{http.MethodPost, "/api/v1/platform/cms/templates/t1/workflow/validate", false},
	{http.MethodGet, "/api/v1/platform/cms/templates/t1/workflow/lifecycle", false},
	{http.MethodGet, "/api/v1/platform/cms/templates/t1/workflow/versions", false},
	{http.MethodGet, "/api/v1/platform/cms/templates/t1/workflow/versions/1", false},
	{http.MethodPost, pathPublish, false},
	{http.MethodPost, pathActivate, false},
}

func TestAuthz_EveryRouteGatedForTenant(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	registered := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		registered += strings.Count(string(src), "HandleFunc(") + strings.Count(string(src), ".Handle(")
	}
	if registered != len(allRoutes) {
		t.Fatalf("package registers %d routes but allRoutes lists %d: add the new route here with its expected gate", registered, len(allRoutes))
	}
	mux := newServer(fakeAuthorizer{})
	for _, rt := range allRoutes {
		req := httptest.NewRequest(rt.method, rt.path, nil)
		if _, pattern := mux.Handler(req); pattern == "" {
			t.Fatalf("%s %s is not registered", rt.method, rt.path)
		}
		rec := do(t, mux, rt.method, rt.path, tokTenantAdmin, `{"role_name":"x"}`)
		switch {
		case rt.tenantOK && isAuthzStatus(rec.Code):
			t.Errorf("%s %s: tenant should be allowed, got %d", rt.method, rt.path, rec.Code)
		case !rt.tenantOK && rec.Code != http.StatusForbidden:
			t.Errorf("%s %s: tenant admin must get 403, got %d", rt.method, rt.path, rec.Code)
		}
	}
}

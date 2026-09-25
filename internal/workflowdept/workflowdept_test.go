package workflowdept

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSnapshotCatalogTokensDedupes(t *testing.T) {
	got := SnapshotCatalogTokens([]string{" dept-001 ", "", "dept-001", "dept-002"})
	if len(got) != 2 || got[0] != "dept-001" || got[1] != "dept-002" {
		t.Fatalf("%v", got)
	}
}

func TestMigration0145BlocksCodeChangeNotName(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0145_catalog_department_code_guard.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(raw)
	if !strings.Contains(sqlText, "catalog department code cannot change") {
		t.Fatal("missing update guard")
	}
	if !strings.Contains(sqlText, "OLD.department_code") {
		t.Fatal("update guard must compare codes")
	}
	if !strings.Contains(sqlText, "catalog department code cannot be reused") {
		t.Fatal("missing retired reuse guard")
	}
	if !strings.Contains(sqlText, "JSON_TABLE") {
		t.Fatal("snapshot codes must be registered")
	}
}

func TestClassifyCatalogWinsOverCompanyCode(t *testing.T) {
	catalog := map[string]struct{}{"dept-001": {}}
	company := []DeptRef{{ID: "uuid-1", Code: "dept-001"}}
	if got := Classify("dept-001", catalog, company, func(string) bool { return true }); got != TokenTemplateDepartmentCode {
		t.Fatalf("got %s", got)
	}
}

func TestClassifyCompanyIDThenCodeThenLabel(t *testing.T) {
	company := []DeptRef{{ID: "uuid-1", Code: "legal"}}
	if got := Classify("uuid-1", nil, company, nil); got != TokenCompanyDepartmentID {
		t.Fatalf("id %s", got)
	}
	if got := Classify("legal", nil, company, nil); got != TokenCompanyDepartmentCode {
		t.Fatalf("code %s", got)
	}
	if got := Classify("Pháp chế", nil, company, func(string) bool { return false }); got != TokenLegacyLabel {
		t.Fatalf("label %s", got)
	}
	if got := Classify("dept-999", nil, company, func(string) bool { return true }); got != TokenUnknown {
		t.Fatalf("unknown %s", got)
	}
}

func TestResolveCatalogWithoutBindingIsMissing(t *testing.T) {
	res := Resolve(ResolveInput{
		Token: "dept-001", Kind: TokenTemplateDepartmentCode, BindingMode: true,
		ExactDepartmentID: "uuid-legal", ExactByCode: true,
	})
	if res.Status != StatusMissing {
		t.Fatalf("status %s path %s", res.Status, res.MatchPath)
	}
}

func TestResolveStepBeatsTypeBeatsDefault(t *testing.T) {
	now := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	maps := []OpenMapping{
		{MappingID: "d", Version: 1, TemplateDepartmentCode: "dept-001", CompanyDepartmentID: "def", TargetActive: true, TargetSameCompany: true, DepartmentName: "Default"},
		{MappingID: "t", Version: 1, TemplateDepartmentCode: "dept-001", DisclosureTypeID: "type-a", CompanyDepartmentID: "typ", TargetActive: true, TargetSameCompany: true, DepartmentName: "Type"},
		{MappingID: "s", Version: 2, TemplateDepartmentCode: "dept-001", DisclosureTypeID: "type-a", StepCode: "step-1", CompanyDepartmentID: "stp", TargetActive: true, TargetSameCompany: true, DepartmentName: "Step"},
	}
	res := Resolve(ResolveInput{
		Token: "dept-001", Kind: TokenTemplateDepartmentCode, BindingMode: true,
		DisclosureTypeID: "type-a", StepCode: "step-1", AsOf: now, Mappings: maps,
	})
	if res.Status != StatusMatched || res.CompanyDepartmentID != "stp" || res.MatchPath != PathStepMapping || res.MappingVersion != 2 {
		t.Fatalf("%+v", res)
	}
	typeOnly := Resolve(ResolveInput{
		Token: "dept-001", Kind: TokenTemplateDepartmentCode, BindingMode: true,
		DisclosureTypeID: "type-a", AsOf: now, Mappings: maps,
	})
	if typeOnly.CompanyDepartmentID != "typ" || typeOnly.MatchPath != PathTypeMapping {
		t.Fatalf("%+v", typeOnly)
	}
}

func TestResolveInactiveDoesNotExactFallback(t *testing.T) {
	res := Resolve(ResolveInput{
		Token: "dept-001", Kind: TokenTemplateDepartmentCode, BindingMode: true,
		Mappings: []OpenMapping{{
			MappingID: "m", Version: 1, TemplateDepartmentCode: "dept-001",
			CompanyDepartmentID: "gone", TargetActive: false, TargetSameCompany: true,
		}},
		ExactDepartmentID: "other",
	})
	if res.Status != StatusMappedInactive {
		t.Fatalf("%+v", res)
	}
}

func TestResolveLegacyModeUsesExact(t *testing.T) {
	res := Resolve(ResolveInput{
		Token: "dept-001", Kind: TokenTemplateDepartmentCode, BindingMode: false,
		ExactDepartmentID: "uuid-1", ExactName: "Pháp chế", ExactByCode: true,
	})
	if res.Status != StatusMatched || res.MatchPath != PathExactCode {
		t.Fatalf("%+v", res)
	}
}

func TestResolveNameDoesNotMatch(t *testing.T) {
	res := Resolve(ResolveInput{
		Token: "Pháp chế", Kind: TokenLegacyLabel, BindingMode: true,
		ExactDepartmentID: "uuid-1", ExactName: "Pháp chế",
	})
	if res.Status != StatusMissing {
		t.Fatalf("%+v", res)
	}
}

func TestScopeHashStableAndDistinct(t *testing.T) {
	a := ScopeKeyHash("c1", "dept-001", "", "")
	b := ScopeKeyHash("c1", "dept-001", "", "")
	if !bytes.Equal(a, b) || len(a) != 32 {
		t.Fatalf("hash not stable")
	}
	c := ScopeKeyHash("c1", "dept-001", "type", "step")
	if bytes.Equal(a, c) {
		t.Fatal("scope collision")
	}
}

func TestEncryptRoundTripAndAADMismatch(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	blob, err := EncryptRecipientEmails(key, 1, "c1", "occ", "res", []byte("a@co.test"))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := DecryptRecipientEmails(key, 1, "c1", "occ", "res", blob)
	if err != nil || string(plain) != "a@co.test" {
		t.Fatalf("plain %q err %v", plain, err)
	}
	if _, err := DecryptRecipientEmails(key, 1, "c1", "occ", "other", blob); err == nil {
		t.Fatal("aad mismatch should fail")
	}
	key2 := bytes.Repeat([]byte{9}, 32)
	blob2, err := EncryptRecipientEmails(key2, 2, "c1", "occ", "res", []byte("b@co.test"))
	if err != nil {
		t.Fatal(err)
	}
	old, err := DecryptRecipientEmails(key, 1, "c1", "occ", "res", blob)
	if err != nil || string(old) != "a@co.test" {
		t.Fatal("old key version must still decrypt")
	}
	neu, err := DecryptRecipientEmails(key2, 2, "c1", "occ", "res", blob2)
	if err != nil || string(neu) != "b@co.test" {
		t.Fatal(err)
	}
}

func TestDeliveryWatchdogDoesNotResend(t *testing.T) {
	if got := WatchdogStaleSending(SendSending); got != SendUnknown {
		t.Fatalf("watchdog %s", got)
	}
	if MayCallSMTP(SendUnknown, "") {
		t.Fatal("unknown must not send")
	}
	if MayCallSMTP(SendSent, "smtp-1") {
		t.Fatal("existing id must not send")
	}
	next, owned := ClaimToSending(SendPending)
	if !owned || next != SendSending {
		t.Fatalf("claim %s %v", next, owned)
	}
	if _, owned := ClaimToSending(SendSending); owned {
		t.Fatal("second worker must not own")
	}
	if AfterProvider(false, false, true) != SendUnknown {
		t.Fatal("uncertain timeout")
	}
	if AfterProvider(false, false, false) != SendRetryableFailed {
		t.Fatal("proven pre-accept failure")
	}
}

func TestFutureAndExpiredWindowsAreCallerFiltered(t *testing.T) {
	// Resolver trusts the caller to pass only rows inside the effective window.
	// An empty list is the expired/future case.
	res := Resolve(ResolveInput{
		Token: "dept-001", Kind: TokenTemplateDepartmentCode, BindingMode: true,
		AsOf: time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if res.Status != StatusMissing {
		t.Fatalf("%+v", res)
	}
}

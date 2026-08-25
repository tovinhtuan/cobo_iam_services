package mysql

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestBusinessDateHCM_fixedClock(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		t.Fatalf("load Asia/Ho_Chi_Minh: %v", err)
	}
	// 2026-08-24 22:00 UTC = 2026-08-25 05:00 HCM → business date 2026-08-25
	now := time.Date(2026, 8, 24, 22, 0, 0, 0, time.UTC)
	got := businessDateHCM(now)
	if got != "2026-08-25" {
		t.Fatalf("businessDateHCM=%q want 2026-08-25 (HCM calendar)", got)
	}
	// Same calendar day in HCM wall clock
	nowHCM := time.Date(2026, 8, 25, 23, 59, 0, 0, loc)
	if businessDateHCM(nowHCM) != "2026-08-25" {
		t.Fatalf("businessDateHCM from HCM wall=%q", businessDateHCM(nowHCM))
	}
}

func TestListRows_usesV1ObligationMembershipSQL(t *testing.T) {
	src := readDeadlineAlertsRepositorySrc(t)
	membSrc := readFileSrc(t, "list_rows_membership.go")

	if !strings.Contains(src, "listRowsV1ObligationMembershipSQL") {
		t.Fatal("ListRows must use listRowsV1ObligationMembershipSQL")
	}
	if !strings.Contains(src, "businessDateHCM") {
		t.Fatal("ListRows must bind TodayHCM via businessDateHCM")
	}
	if !strings.Contains(src, "todayHCM") {
		t.Fatal("ListRows must pass todayHCM as query arg")
	}
	listRowsSrc := listRowsFuncSource(t, src)
	listLower := strings.ToLower(listRowsSrc)
	if strings.Contains(listLower, "<> 'draft'") || strings.Contains(listLower, `<> "draft"`) {
		t.Fatal("legacy status<>Draft membership must be removed from ListRows (helpers may still use it)")
	}
	if strings.Contains(listLower, "curdate()") || strings.Contains(listLower, "current_date") {
		t.Fatal("must not use DB CURRENT_DATE/CURDATE for OpenAt membership")
	}
	if strings.Contains(membSrc, "created_at") && strings.Contains(membSrc, "today") {
		t.Fatal("must not use created_at as alert business boundary")
	}
	if !strings.Contains(membSrc, "LOWER(TRIM(dr.status)) = 'draft'") {
		t.Fatal("NeedsCompanyAction requires status=Draft")
	}
	if !strings.Contains(membSrc, "dr.submitted_at IS NULL") {
		t.Fatal("NeedsCompanyAction requires submitted_at IS NULL")
	}
	if !strings.Contains(membSrc, "NOT EXISTS") || !strings.Contains(membSrc, "periodic_cycles pc_ir") {
		t.Fatal("irregular branch requires NOT EXISTS periodic_cycles")
	}
	if !strings.Contains(membSrc, "COALESCE(pc.open_at, pc.cycle_start)") {
		t.Fatal("periodic AlertFrom must use COALESCE(open_at, cycle_start)")
	}
	if !strings.Contains(membSrc, "EXISTS") {
		t.Fatal("periodic branch must use EXISTS (not JOIN) to avoid row duplication")
	}
	if strings.Contains(strings.ToLower(membSrc), "distinct") {
		t.Fatal("must not bandage multiplicity with DISTINCT")
	}
	if strings.Contains(membSrc, "now +") || strings.Contains(membSrc, "INTERVAL 7") {
		t.Fatal("must not use materialization +7d lookahead as alert policy")
	}
}

func TestListRowsV1Membership_acceptanceMatrix(t *testing.T) {
	today := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	d := func(y int, m time.Month, day int) *time.Time {
		t := time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
		return &t
	}
	cycle := func(openAt, cycleStart *time.Time) []periodicCycleDates {
		return []periodicCycleDates{{openAt: openAt, cycleStart: cycleStart}}
	}

	cases := []struct {
		name            string
		status          string
		submittedAtNull bool
		cycles          []periodicCycleDates
		want            bool
		flag            string
	}{
		{
			name:            "periodic draft open_at past",
			status:          "Draft",
			submittedAtNull: true,
			cycles:          cycle(d(2026, 8, 24), d(2026, 8, 1)),
			want:            true,
			flag:            "PERIODIC_DRAFT_OPENAT_PAST_INCLUDED",
		},
		{
			name:            "periodic draft open_at today",
			status:          "Draft",
			submittedAtNull: true,
			cycles:          cycle(d(2026, 8, 25), d(2026, 8, 1)),
			want:            true,
			flag:            "PERIODIC_DRAFT_OPENAT_TODAY_INCLUDED",
		},
		{
			name:            "periodic draft open_at future",
			status:          "Draft",
			submittedAtNull: true,
			cycles:          cycle(d(2026, 8, 26), d(2026, 8, 1)),
			want:            false,
			flag:            "PERIODIC_DRAFT_OPENAT_FUTURE_EXCLUDED",
		},
		{
			name:            "legacy null open_at cycle_start past",
			status:          "Draft",
			submittedAtNull: true,
			cycles:          cycle(nil, d(2026, 8, 20)),
			want:            true,
			flag:            "LEGACY_PERIODIC_NULL_OPENAT_CYCLE_START_PAST_INCLUDED",
		},
		{
			name:            "legacy null open_at cycle_start future",
			status:          "Draft",
			submittedAtNull: true,
			cycles:          cycle(nil, d(2026, 8, 30)),
			want:            false,
			flag:            "LEGACY_PERIODIC_NULL_OPENAT_CYCLE_START_FUTURE_EXCLUDED",
		},
		{
			name:            "irregular draft unsubmitted",
			status:          "Draft",
			submittedAtNull: true,
			cycles:          nil,
			want:            true,
			flag:            "IRREGULAR_DRAFT_UNSUBMITTED_INCLUDED",
		},
		{
			name:            "draft submitted excluded",
			status:          "Draft",
			submittedAtNull: false,
			cycles:          cycle(d(2026, 8, 24), d(2026, 8, 1)),
			want:            false,
			flag:            "DRAFT_SUBMITTED_EXCLUDED",
		},
		{
			name:            "non-draft pending review excluded",
			status:          "PendingReview",
			submittedAtNull: false,
			cycles:          cycle(d(2026, 8, 24), d(2026, 8, 1)),
			want:            false,
			flag:            "NON_DRAFT_EXCLUDED",
		},
		{
			name:            "non-draft submitted_at null excluded",
			status:          "PendingReview",
			submittedAtNull: true,
			cycles:          nil,
			want:            false,
			flag:            "NON_DRAFT_SUBMITTED_NULL_EXCLUDED",
		},
		{
			name:            "irregular post-submit excluded",
			status:          "PendingReview",
			submittedAtNull: false,
			cycles:          nil,
			want:            false,
			flag:            "IRREGULAR_POST_SUBMIT_EXCLUDED",
		},
		{
			name:            "malformed periodic both dates null",
			status:          "Draft",
			submittedAtNull: true,
			cycles:          cycle(nil, nil),
			want:            false,
			flag:            "MALFORMED_PERIODIC_FAIL_SAFE",
		},
	}

	for _, tc := range cases {
		t.Run(tc.flag, func(t *testing.T) {
			got := listRowsV1MembershipEligible(tc.status, tc.submittedAtNull, tc.cycles, today)
			if got != tc.want {
				t.Fatalf("%s (%s): got=%v want=%v", tc.flag, tc.name, got, tc.want)
			}
		})
	}
}

func TestListRows_preservesCompanyOrderScopeWiring(t *testing.T) {
	src := readDeadlineAlertsRepositorySrc(t)
	if !strings.Contains(src, "WHERE dr.company_id = ?") {
		t.Fatal("COMPANY_SCOPE must remain on ListRows")
	}
	if !strings.Contains(src, "BuildListRowsScopeSQL") {
		t.Fatal("ACCESS_SCOPE SQL builder must remain wired")
	}
	if !strings.Contains(src, "ORDER BY dr.created_at DESC") {
		t.Fatal("ORDER BY must remain created_at DESC")
	}
	if strings.Contains(strings.ToLower(src), "limit ?") || strings.Contains(src, "OFFSET") {
		t.Fatal("pagination must not move into SQL in Phase 1")
	}
	if !strings.Contains(src, "ListRowsActiveTemplateSQLJoin") {
		t.Fatal("ACTIVE_TEMPLATE filter must remain")
	}
}

func TestWithNow_overridesBusinessDateArgPath(t *testing.T) {
	fixed := time.Date(2026, 8, 25, 3, 0, 0, 0, time.UTC)
	r := NewRepository(nil, WithNow(func() time.Time { return fixed }))
	if r.now == nil {
		t.Fatal("WithNow must set clock")
	}
	if businessDateHCM(r.now()) != "2026-08-25" {
		// 03:00 UTC = 10:00 HCM on 2026-08-25
		t.Fatalf("got %s", businessDateHCM(r.now()))
	}
}

func listRowsFuncSource(t *testing.T, repoSrc string) string {
	t.Helper()
	const start = "func (r *Repository) ListRows"
	idx := strings.Index(repoSrc, start)
	if idx < 0 {
		t.Fatal("ListRows not found in repository.go")
	}
	rest := repoSrc[idx+len(start):]
	next := strings.Index(rest, "\nfunc (r *Repository)")
	if next < 0 {
		return repoSrc[idx:]
	}
	return repoSrc[idx : idx+len(start)+next]
}

func readFileSrc(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

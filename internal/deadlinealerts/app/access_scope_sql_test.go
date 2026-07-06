package app

import "testing"

func TestBuildListRowsScopeSQL_viewAllNoClause(t *testing.T) {
	clause, args := BuildListRowsScopeSQL(DeadlineAlertAccessScope{CanViewAll: true})
	if clause != "" || len(args) != 0 {
		t.Fatalf("got clause=%q args=%v", clause, args)
	}
}

func TestBuildListRowsScopeSQL_scopedIncludesDepartmentAndMembership(t *testing.T) {
	clause, args := BuildListRowsScopeSQL(DeadlineAlertAccessScope{
		MembershipID:  "m_105",
		DepartmentIDs: []string{"d_legal"},
	})
	if clause == "" {
		t.Fatal("expected scope clause")
	}
	if len(args) != 3 {
		t.Fatalf("expected 3 args, got %v", args)
	}
	if args[0] != "d_legal" || args[1] != "m_105" || args[2] != "m_105" {
		t.Fatalf("args %v", args)
	}
}

func TestBuildListRowsScopeSQL_noScopeDeniesAll(t *testing.T) {
	clause, args := BuildListRowsScopeSQL(DeadlineAlertAccessScope{})
	if clause != " AND 1=0" || len(args) != 0 {
		t.Fatalf("got clause=%q args=%v", clause, args)
	}
}

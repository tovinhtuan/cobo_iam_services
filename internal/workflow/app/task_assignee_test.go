package app

import "testing"

func TestBuildTaskAssignee_SystemWithoutUser(t *testing.T) {
	a := BuildTaskAssignee("m_system_oneshot", "", "", "")
	if a == nil {
		t.Fatal("expected assignee")
	}
	if a.ActorType != ActorTypeSystem {
		t.Fatalf("actor_type=%q", a.ActorType)
	}
	if a.DisplayName != SystemAssigneeDisplayName {
		t.Fatalf("display_name=%q", a.DisplayName)
	}
	if a.MembershipID != "m_system_oneshot" {
		t.Fatalf("membership=%q", a.MembershipID)
	}
}

func TestBuildTaskAssignee_NormalUserKeepsName(t *testing.T) {
	a := BuildTaskAssignee("m_001", "Alice", "a@example.com", "Legal")
	if a == nil || a.ActorType != "" || a.DisplayName != "Alice" || a.Email != "a@example.com" || a.DepartmentName != "Legal" {
		t.Fatalf("%+v", a)
	}
}

func TestBuildTaskAssignee_EmptyMembership(t *testing.T) {
	if BuildTaskAssignee("  ", "x", "", "") != nil {
		t.Fatal("expected nil")
	}
}

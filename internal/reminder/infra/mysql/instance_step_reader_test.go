package mysql

import "testing"

func TestMergeSnapshotMembershipIDs_ArrayOutranksSingular(t *testing.T) {
	got := mergeSnapshotMembershipIDs("legacy", []string{"m1", "m2"})
	if len(got) != 2 || got[0] != "m1" || got[1] != "m2" {
		t.Fatalf("got %#v", got)
	}
}

func TestMergeSnapshotMembershipIDs_SingularWhenArrayEmpty(t *testing.T) {
	got := mergeSnapshotMembershipIDs("mem-c", nil)
	if len(got) != 1 || got[0] != "mem-c" {
		t.Fatalf("got %#v", got)
	}
}

func TestMergeSnapshotMembershipIDs_IgnoresBlankAndDupes(t *testing.T) {
	got := mergeSnapshotMembershipIDs("", []string{"", "m1", "m1", " m2 "})
	if len(got) != 2 || got[0] != "m1" || got[1] != "m2" {
		t.Fatalf("got %#v", got)
	}
}

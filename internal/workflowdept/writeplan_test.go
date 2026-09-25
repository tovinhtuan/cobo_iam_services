package workflowdept

import "testing"

func TestPlanWriteCreateReplaceConflictIdempotent(t *testing.T) {
	act, err := PlanWrite(0, false, "", "d1", 0)
	if err != nil || act != ActionCreate {
		t.Fatalf("%s %v", act, err)
	}
	if _, err := PlanWrite(0, false, "", "d1", 2); err != ErrVersionConflict {
		t.Fatal(err)
	}
	act, err = PlanWrite(2, true, "d1", "d1", 2)
	if err != nil || act != ActionIdempotent {
		t.Fatalf("%s %v", act, err)
	}
	act, err = PlanWrite(2, true, "d1", "d2", 2)
	if err != nil || act != ActionReplace {
		t.Fatalf("%s %v", act, err)
	}
	if _, err := PlanWrite(2, true, "d1", "d2", 1); err != ErrVersionConflict {
		t.Fatal(err)
	}
}

func TestPlanClose(t *testing.T) {
	if _, err := PlanClose(0, false, 1); err != ErrNotFound {
		t.Fatal(err)
	}
	act, err := PlanClose(3, true, 3)
	if err != nil || act != ActionClose {
		t.Fatalf("%s %v", act, err)
	}
}

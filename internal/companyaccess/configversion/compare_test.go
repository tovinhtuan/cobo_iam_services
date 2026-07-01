package configversion

import (
	"testing"
)

func TestCompareJSON_equal(t *testing.T) {
	raw := []byte(`{"schema_version":"notification_rule_snapshot.v1","status":"active"}`)
	sum, err := CompareJSON(raw, raw, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !sum.Equal {
		t.Fatalf("expected equal, got %+v", sum)
	}
}

func TestCompareJSON_detectsChange(t *testing.T) {
	from := []byte(`{"status":"active","payload":{"a":1}}`)
	to := []byte(`{"status":"active","payload":{"a":2}}`)
	sum, err := CompareJSON(from, to, 1, 2)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Equal {
		t.Fatal("expected diff")
	}
	if len(sum.ChangedKeys) == 0 {
		t.Fatal("expected changed keys")
	}
}

package inmemory

import "testing"

func TestDefaultPolicies_DisclosureSubmitRequiresCreateNotPublish(t *testing.T) {
	p, ok := defaultPolicies()["disclosure.submit"]
	if !ok {
		t.Fatal("disclosure.submit policy missing")
	}
	if p.RequiredPermission != "disclosure.create" {
		t.Fatalf("RequiredPermission=%q want disclosure.create", p.RequiredPermission)
	}
}

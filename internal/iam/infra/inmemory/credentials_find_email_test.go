package inmemory_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/iam/infra/inmemory"
)

func userIDs(t *testing.T, v *inmemory.StaticCredentialVerifier, address string, limit int) []string {
	t.Helper()
	users, err := v.FindRecoveryUsers(context.Background(), address, limit)
	if err != nil {
		t.Fatalf("FindRecoveryUsers(%q) error = %v", address, err)
	}
	ids := []string{}
	for _, u := range users {
		ids = append(ids, u.UserID)
	}
	return ids
}

// bug-login-old-accounts-2026-10-16 (BUG-2): an address shared by several accounts used to resolve to
// whichever one map iteration met first. Every account whose mailbox is the address must get a link.
func TestFindRecoveryUsers_EveryAccountOfTheMailboxThenLoginIDOwner(t *testing.T) {
	v := &inmemory.StaticCredentialVerifier{Users: map[string]inmemory.StaticUser{
		"owner@x.test":  {UserID: "u_owner", LoginID: "owner@x.test"},
		"holder@x.test": {UserID: "u_holder", LoginID: "holder@x.test", Email: "Owner@x.test"},
		"admin_dn":      {UserID: "u_dn", LoginID: "admin_dn", Email: "boss@x.test"},
		"a@x.test":      {UserID: "u_a", LoginID: "a@x.test", Email: "shared@x.test"},
		"b@x.test":      {UserID: "u_b", LoginID: "b@x.test", Email: "shared@x.test"},
		"c@x.test":      {UserID: "u_c", LoginID: "c@x.test", Email: "shared@x.test"},
	}}

	// Map iteration order is random: repeat so an order-dependent answer shows up.
	for i := 0; i < 30; i++ {
		if got := userIDs(t, v, " OWNER@x.test ", 5); !reflect.DeepEqual(got, []string{"u_holder", "u_owner"}) {
			t.Fatalf("mailbox shared by login_id owner and email holder: got %v", got)
		}
		if got := userIDs(t, v, "shared@x.test", 2); !reflect.DeepEqual(got, []string{"u_a", "u_b"}) {
			t.Fatalf("shared email, limit 2: got %v", got)
		}
	}
	// No account receives mail at a login_id whose mailbox is elsewhere: fall back to its owner.
	if got := userIDs(t, v, "admin_dn", 5); !reflect.DeepEqual(got, []string{"u_dn"}) {
		t.Fatalf("login_id fallback: got %v", got)
	}
	// holder@x.test is a login_id; its mailbox is Owner@x.test, so the mail goes there.
	users, _ := v.FindRecoveryUsers(context.Background(), "holder@x.test", 5)
	if len(users) != 1 || users[0].UserID != "u_holder" || users[0].Email != "Owner@x.test" {
		t.Fatalf("login_id with another mailbox: got %+v", users)
	}
	for _, addr := range []string{"nobody@x.test", "  "} {
		if got := userIDs(t, v, addr, 5); len(got) != 0 {
			t.Fatalf("%q: got %v, want none", addr, got)
		}
	}
}

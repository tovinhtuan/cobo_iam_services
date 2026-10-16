package mysql

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// bug-login-old-accounts-2026-10-16 (BUG-2): `email = ? OR login_id = ? LIMIT 1` without ORDER BY
// returned any matching row. On DEV a seed account's email was another account's login_id, so a
// reset for that address went to an arbitrary one of the two accounts.
func TestAuthRecovery_FindRecoveryUsers_EveryAccountOfTheMailboxThenLoginIDOwner(t *testing.T) {
	db := openRecoveryLimitsDB(t)
	ctx := context.Background()
	p := fmt.Sprintf("fru%d", time.Now().UnixNano())
	// Inserted in this order, so created_at (then user_id) order is: z-owner, a-holder, b-one, c-two, d-dn.
	users := []struct{ id, login, email string }{
		{p + "-z-owner", p + "-owner@example.test", ""},
		{p + "-a-holder", p + "-holder@example.test", p + "-OWNER@example.test"},
		{p + "-b-one", p + "-one@example.test", p + "-shared@example.test"},
		{p + "-c-two", p + "-two@example.test", p + "-shared@example.test"},
		{p + "-d-dn", p + "-admin_dn", p + "-boss@example.test"},
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE user_id LIKE ?`, p+"-%")
	})
	for i, u := range users {
		var email any
		if u.email != "" {
			email = u.email
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO users (user_id, login_id, full_name, email, account_status, created_at)
			VALUES (?, ?, 'Find Recovery Test', ?, 'active', UTC_TIMESTAMP() - INTERVAL ? SECOND)
		`, u.id, u.login, email, 100-i); err != nil {
			t.Fatal(err)
		}
	}
	repo := NewAuthRecoveryRepository(db)
	ids := func(address string, limit int) []string {
		t.Helper()
		got, err := repo.FindRecoveryUsers(ctx, address, limit)
		if err != nil {
			t.Fatalf("FindRecoveryUsers(%q) error = %v", address, err)
		}
		out := []string{}
		for _, u := range got {
			out = append(out, strings.TrimPrefix(u.UserID, p))
		}
		return out
	}

	if got := ids(" "+strings.ToUpper(p+"-owner@example.test")+" ", 5); !reflect.DeepEqual(got, []string{"-z-owner", "-a-holder"}) {
		t.Fatalf("mailbox shared by login_id owner and email holder: got %v (oldest first)", got)
	}
	if got := ids(p+"-shared@example.test", 1); !reflect.DeepEqual(got, []string{"-b-one"}) {
		t.Fatalf("shared email, limit 1: got %v", got)
	}
	if got := ids(p+"-admin_dn", 5); !reflect.DeepEqual(got, []string{"-d-dn"}) {
		t.Fatalf("login_id fallback: got %v", got)
	}
	holder, err := repo.FindRecoveryUsers(ctx, p+"-holder@example.test", 5)
	if err != nil || len(holder) != 1 || holder[0].UserID != p+"-a-holder" || holder[0].Email != p+"-OWNER@example.test" {
		t.Fatalf("login_id with another mailbox: got %+v, %v", holder, err)
	}
	if got := ids(p+"-nobody@example.test", 5); len(got) != 0 {
		t.Fatalf("unknown: got %v", got)
	}
}

package companystatus

import "testing"

func TestNormalizeOperationalStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"active", StatusActive, true},
		{"inactive", StatusInactive, true},
		{" ACTIVE ", StatusActive, true},
		{"InActive", StatusInactive, true},
		{"", "", false},
		{"   ", "", false},
		{"suspended", StatusSuspended, true},
		{" Suspended ", StatusSuspended, true},
		{"pending", "", false},
		{"verified", "", false},
		{"activee", "", false},
		{"unknown", "", false},
	}
	for _, tc := range cases {
		got, err := NormalizeOperationalStatus(tc.in)
		if tc.ok {
			if err != nil || got != tc.want {
				t.Fatalf("in=%q got=%q err=%v want=%q", tc.in, got, err, tc.want)
			}
			continue
		}
		if err == nil {
			t.Fatalf("in=%q expected error, got %q", tc.in, got)
		}
	}
}

func TestNormalizeVerificationStatus(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"verified", VerificationVerified, true},
		{"unverified", VerificationUnverified, true},
		{" VERIFIED ", VerificationVerified, true},
		{"", "", false},
		{"   ", "", false},
		{"pending", "", false},
		{"verifiedcvx", "", false},
		{"rejected", "", false},
		{"verify", "", false},
		{"unknown", "", false},
	}
	for _, tc := range cases {
		got, err := NormalizeVerificationStatus(tc.in)
		if tc.ok {
			if err != nil || got != tc.want {
				t.Fatalf("in=%q got=%q err=%v want=%q", tc.in, got, err, tc.want)
			}
			continue
		}
		if err == nil {
			t.Fatalf("in=%q expected error, got %q", tc.in, got)
		}
	}
}

func TestAccessByStatus(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		status           string
		blocks, readOnly bool
	}{
		{"active", false, false},
		{"inactive", true, false},
		{" INACTIVE ", true, false},
		{"suspended", false, true},
		{"Suspended", false, true},
		{"legacy-value", false, false},
	} {
		if got := BlocksAccess(tc.status); got != tc.blocks {
			t.Errorf("BlocksAccess(%q) = %v, want %v", tc.status, got, tc.blocks)
		}
		if got := IsReadOnly(tc.status); got != tc.readOnly {
			t.Errorf("IsReadOnly(%q) = %v, want %v", tc.status, got, tc.readOnly)
		}
	}
}

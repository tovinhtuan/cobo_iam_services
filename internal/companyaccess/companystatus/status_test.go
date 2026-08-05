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
		{"suspended", "", false},
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

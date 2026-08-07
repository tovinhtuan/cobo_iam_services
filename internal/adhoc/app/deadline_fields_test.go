package app

import "testing"

func TestNormalizeDateOnly(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"2026-08-07", "2026-08-07"},
		{"2026-08-07T00:00:00Z", "2026-08-07"},
		{"2026-08-07T00:00:00+00:00", "2026-08-07"},
		{"2026-08-07 00:00:00", "2026-08-07"},
		{" 2026-08-07T15:04:05Z ", "2026-08-07"},
	}
	for _, tc := range cases {
		if got := normalizeDateOnly(tc.in); got != tc.want {
			t.Fatalf("normalizeDateOnly(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}

package http

import "testing"

func TestParseOptionalBoolQuery(t *testing.T) {
	cases := []struct {
		raw    string
		want   *bool
		wantOK bool
	}{
		{"", nil, true},
		{"  ", nil, true},
		{"true", ptrBool(true), true},
		{"1", ptrBool(true), true},
		{"FALSE", ptrBool(false), true},
		{"0", ptrBool(false), true},
		{"yes", nil, false},
	}
	for _, tc := range cases {
		got, ok := parseOptionalBoolQuery(tc.raw)
		if ok != tc.wantOK {
			t.Fatalf("%q ok=%v want %v", tc.raw, ok, tc.wantOK)
		}
		if (got == nil) != (tc.want == nil) || (got != nil && *got != *tc.want) {
			t.Fatalf("%q got=%v want=%v", tc.raw, got, tc.want)
		}
	}
}

func ptrBool(v bool) *bool { return &v }

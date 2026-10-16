package mysql

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// bug-login-old-accounts-2026-10-16 (review BES-04): a raw byte or a rune cut in half made the
// utf8mb4 insert fail, so an attacker could keep failed attempts out of login_attempts.
func TestTruncStr_KeepsValidUTF8WithinTheRuneLimit(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"Mozilla\xff\xfe/5.0", 512, "Mozilla�/5.0"},
		{"đăng nhập", 4, "đăng"},
		{strings.Repeat("ệ", 300), 191, strings.Repeat("ệ", 191)},
		{"127.0.0.1:443", 64, "127.0.0.1:443"},
	}
	for _, c := range cases {
		got := truncStr(c.in, c.max)
		if got != c.want || !utf8.ValidString(got) {
			t.Errorf("truncStr(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}

package config

import "testing"

func TestNormalizeMySQLDSN(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "append charset and collation when missing",
			in:   "cobo:cobo@tcp(mysql:3306)/cobo_iam?parseTime=true&loc=UTC&tls=false",
			want: "cobo:cobo@tcp(mysql:3306)/cobo_iam?parseTime=true&loc=UTC&tls=false&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		},
		{
			name: "keep existing charset and collation",
			in:   "u:p@tcp(localhost:3306)/db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
			want: "u:p@tcp(localhost:3306)/db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		},
		{
			name: "append query params when no query exists",
			in:   "u:p@tcp(localhost:3306)/db",
			want: "u:p@tcp(localhost:3306)/db?charset=utf8mb4&collation=utf8mb4_unicode_ci",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeMySQLDSN(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeMySQLDSN() = %q, want %q", got, tc.want)
			}
		})
	}
}

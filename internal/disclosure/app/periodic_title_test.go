package app

import "testing"

func TestAutoRecordTitle(t *testing.T) {
	tests := []struct {
		name string
		row  PeriodicCycleRow
		want string
	}{
		{
			name: "template name and cycle",
			row: PeriodicCycleRow{
				TypeName:   "Báo cáo tài chính quý 1",
				CycleLabel: "2026-Q1",
			},
			want: "Báo cáo tài chính quý 1 — 2026-Q1",
		},
		{
			name: "fallback type id",
			row: PeriodicCycleRow{
				TypeID:     "dt-sys-q1",
				CycleLabel: "2026-Q1",
			},
			want: "dt-sys-q1 — 2026-Q1",
		},
		{
			name: "name only",
			row: PeriodicCycleRow{TypeName: "Báo cáo năm"},
			want: "Báo cáo năm",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := autoRecordTitle(tc.row); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

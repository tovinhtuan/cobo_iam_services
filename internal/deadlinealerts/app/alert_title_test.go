package app

import "testing"

func TestDisplayAlertTitle(t *testing.T) {
	tests := []struct {
		name string
		row  AlertRow
		want string
	}{
		{
			name: "ad hoc proposal title line",
			row: AlertRow{
				Title:          "Ad-hoc: dt-1\nMô tả",
				AdHocTitleLine: "Cảnh báo bất thường Q2",
			},
			want: "Cảnh báo bất thường Q2",
		},
		{
			name: "legacy ad hoc record title",
			row: AlertRow{
				Title: "Ad-hoc: Tiêu đề đề xuất\nMô tả dài",
			},
			want: "Tiêu đề đề xuất",
		},
		{
			name: "legacy periodic worker title",
			row: AlertRow{
				Title:            "[Tự động] dt-sys-q1 — 2026-Q1",
				TypeName:         "Báo cáo tài chính quý 1",
				TemplateCategory: "periodic",
			},
			want: "Báo cáo tài chính quý 1 — 2026-Q1",
		},
		{
			name: "periodic record title already correct",
			row: AlertRow{
				Title:            "Báo cáo tài chính quý 1 — 2026-Q1",
				TypeName:         "Báo cáo tài chính quý 1",
				TemplateCategory: "periodic",
			},
			want: "Báo cáo tài chính quý 1 — 2026-Q1",
		},
		{
			name: "periodic prefers type name over submission/test record title",
			row: AlertRow{
				Title:            "Sprint3 Verify Submission 3",
				TypeName:         "Báo cáo tài chính quý 1",
				TemplateCategory: "periodic",
			},
			want: "Báo cáo tài chính quý 1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DisplayAlertTitle(tc.row); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

package app

import (
	"strings"
	"testing"
)

func TestSplitChangeNote(t *testing.T) {
	tests := []struct {
		name        string
		note        string
		wantTitle   string
		wantContent string
	}{
		{
			name:        "two lines",
			note:        "Báo cáo sự cố hệ thống test\nBáo cáo sự cố hệ thống test content",
			wantTitle:   "Báo cáo sự cố hệ thống test",
			wantContent: "Báo cáo sự cố hệ thống test content",
		},
		{
			name:        "single line has no content",
			note:        "P0 Email Test Rejection - Strategy Change",
			wantTitle:   "P0 Email Test Rejection - Strategy Change",
			wantContent: "",
		},
		{
			name:        "empty note",
			note:        "",
			wantTitle:   "",
			wantContent: "",
		},
		{
			name:        "multiple content lines joined",
			note:        "Tiêu đề\nDòng 1\nDòng 2",
			wantTitle:   "Tiêu đề",
			wantContent: "Dòng 1\nDòng 2",
		},
		{
			name:        "long content is truncated at 300 chars",
			note:        "Title\n" + strings.Repeat("x", 400),
			wantTitle:   "Title",
			wantContent: strings.Repeat("x", 297) + "...",
		},
		{
			name:        "content exactly at limit is not truncated",
			note:        "Title\n" + strings.Repeat("y", 300),
			wantTitle:   "Title",
			wantContent: strings.Repeat("y", 300),
		},
		{
			name:        "whitespace-only second line yields empty content",
			note:        "Title\n   ",
			wantTitle:   "Title",
			wantContent: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			title, content := splitChangeNote(tc.note)
			if title != tc.wantTitle {
				t.Errorf("title = %q, want %q", title, tc.wantTitle)
			}
			if content != tc.wantContent {
				t.Errorf("content = %q, want %q", content, tc.wantContent)
			}
		})
	}
}

func TestResolveAdHocRecordTitle(t *testing.T) {
	tests := []struct {
		name     string
		note     string
		typeName string
		typeID   string
		want     string
	}{
		{
			name: "first line of change note",
			note: "Bổ nhiệm Phó TGĐ\nMô tả chi tiết",
			want: "Bổ nhiệm Phó TGĐ",
		},
		{
			name:     "fallback template name",
			typeName: "Thay đổi nhân sự cấp cao",
			typeID:   "dt-hr",
			want:     "Thay đổi nhân sự cấp cao",
		},
		{
			name:   "fallback type id",
			typeID: "dt-hr",
			want:   "dt-hr",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveAdHocRecordTitle(tc.note, tc.typeName, tc.typeID)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

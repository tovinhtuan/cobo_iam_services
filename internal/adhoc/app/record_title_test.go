package app

import "testing"

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

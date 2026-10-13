package pagination

import (
	"math"
	"testing"
)

func TestWindow(t *testing.T) {
	cases := []struct {
		name                string
		page, pageSize, tot int
		wantStart, wantEnd  int
	}{
		{"first page", 1, 20, 45, 0, 20},
		{"middle page", 2, 20, 45, 20, 40},
		{"last partial page", 3, 20, 45, 40, 45},
		{"last full page", 3, 15, 45, 30, 45},
		{"first page past end", 4, 15, 45, 45, 45},
		{"page zero is page one", 0, 20, 45, 0, 20},
		{"negative page is page one", -3, 20, 45, 0, 20},
		{"empty total", 1, 20, 0, 0, 0},
		{"page past empty total", 7, 20, 0, 0, 0},
		{"zero page size", 1, 0, 45, 0, 0},
		{"page size larger than total", 1, 100, 3, 0, 3},
		// (page-1)*pageSize used to wrap negative or back to zero for these.
		{"overflow page", 500000000000000000, 20, 45, 45, 45},
		{"max int page", math.MaxInt, 100, 45, 45, 45},
		{"wraps to zero page", 1<<62 + 1, 4, 45, 45, 45},
		{"max int page size", 1, math.MaxInt, 45, 0, 45},
		{"max int page and page size", math.MaxInt, math.MaxInt, 45, 45, 45},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start, end := Window(tc.page, tc.pageSize, tc.tot)
			if start != tc.wantStart || end != tc.wantEnd {
				t.Fatalf("Window(%d, %d, %d) = [%d:%d] want [%d:%d]", tc.page, tc.pageSize, tc.tot, start, end, tc.wantStart, tc.wantEnd)
			}
			items := make([]int, tc.tot)
			_ = items[start:end]
		})
	}
}

func TestOffset(t *testing.T) {
	cases := []struct {
		name           string
		page, pageSize int
		want           int
	}{
		{"first page", 1, 20, 0},
		{"second page", 2, 20, 20},
		{"page zero", 0, 20, 0},
		{"negative page", -1, 20, 0},
		{"zero page size", 5, 0, 0},
		{"largest exact offset", math.MaxInt/20 + 1, 20, math.MaxInt / 20 * 20},
		{"overflow page", 500000000000000000, 20, math.MaxInt},
		{"max int page", math.MaxInt, 100, math.MaxInt},
		{"wraps to zero page", 1<<62 + 1, 4, math.MaxInt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Offset(tc.page, tc.pageSize); got != tc.want {
				t.Fatalf("Offset(%d, %d) = %d want %d", tc.page, tc.pageSize, got, tc.want)
			}
		})
	}
}

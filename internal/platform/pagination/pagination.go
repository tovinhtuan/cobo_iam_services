// Package pagination computes 1-based page bounds without int overflow.
//
// Handlers parse page with strconv.Atoi, which clamps an out-of-range value to
// math.MaxInt, and services only clamp page from below. (page-1)*pageSize then
// wraps negative, passes a "start > total" guard and panics on slice[start:end],
// or reaches MySQL as a negative OFFSET.
package pagination

import "math"

// Window returns the [start, end) bounds of a 1-based page over total items.
// page < 1 is treated as 1; pageSize < 1 yields an empty window. A page past the
// end returns start == end == total, so slice[start:end] is always in range.
func Window(page, pageSize, total int) (start, end int) {
	if total <= 0 || pageSize <= 0 {
		return 0, 0
	}
	if page < 1 {
		page = 1
	}
	// Compare before multiplying: the last page index is (total-1)/pageSize.
	if page-1 > (total-1)/pageSize {
		return total, total
	}
	start = (page - 1) * pageSize
	if pageSize >= total-start {
		return start, total
	}
	return start, start + pageSize
}

// Offset returns the SQL OFFSET of a 1-based page. It saturates at math.MaxInt
// instead of wrapping negative: MySQL rejects a negative OFFSET, while an
// offset past the last row returns no rows.
func Offset(page, pageSize int) int {
	if page <= 1 || pageSize <= 0 {
		return 0
	}
	if page-1 > math.MaxInt/pageSize {
		return math.MaxInt
	}
	return (page - 1) * pageSize
}

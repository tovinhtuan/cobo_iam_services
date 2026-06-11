package deadlineengine

import (
	"fmt"
	"time"
)

// FormatHintLabelVI builds the Portal hint string per UX Contract v2 §D.1.
//
//	Calendar: "Hạn gợi ý: 01/04/2026 + 20 ngày dương lịch → 20/04/2026"
//	Working:  "Hạn gợi ý: 01/04/2026 + 20 ngày làm việc → 28/04/2026"
//
// Suffixes (appended in order, both may apply):
//
//	usedStructure       -> " (theo cơ cấu doanh nghiệp)"
//	usedCompanyOverride -> " (theo T0 doanh nghiệp đã cấu hình)"
func FormatHintLabelVI(resolvedT0 time.Time, n int, dayType DayType, plannedDate time.Time, usedStructure, usedCompanyOverride bool) string {
	dayTypeLabel := "ngày dương lịch"
	if dayType == DayTypeWorking {
		dayTypeLabel = "ngày làm việc"
	}

	label := fmt.Sprintf("Hạn gợi ý: %s + %d %s → %s",
		resolvedT0.Format("02/01/2006"), n, dayTypeLabel, plannedDate.Format("02/01/2006"))

	if usedStructure {
		label += " (theo cơ cấu doanh nghiệp)"
	}
	if usedCompanyOverride {
		label += " (theo T0 doanh nghiệp đã cấu hình)"
	}

	return label
}

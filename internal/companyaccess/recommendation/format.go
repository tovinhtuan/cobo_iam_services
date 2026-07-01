package recommendation

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	PriorityHigh   = "high"
	PriorityMedium = "medium"
	PriorityLow    = "low"

	fallbackTitle       = "Xem chi tiết cấu hình liên quan"
	fallbackDescription = "Có một kiểm tra cấu hình cần được xem xét."
	fallbackActionLabel = "Xem chi tiết cấu hình liên quan"
	defaultActionLink   = "/app/admin"
)

type mapRow struct {
	title       string
	actionLabel string
	actionLink  string
	category    string
}

var codeMap = map[string]mapRow{
	"conflict.notification.prefs_invalid": {
		title:       "Kiểm tra lại cấu hình kênh cảnh báo",
		actionLabel: "Mở kênh nhận cảnh báo",
		actionLink:  "/app/admin?tab=notifications",
		category:    "notification",
	},
	"notification.storage_not_configured": {
		title:       "Thiết lập kênh nhận cảnh báo",
		actionLabel: "Mở kênh nhận cảnh báo",
		actionLink:  "/app/admin?tab=notifications",
		category:    "notification",
	},
	"conflict.subscription.tier_prefs_mismatch": {
		title:       "Rà soát cấu hình theo gói subscription",
		actionLabel: "Mở kênh nhận cảnh báo",
		actionLink:  "/app/admin?tab=notifications",
		category:    "subscription",
	},
	"conflict.permission.critical_role_empty": {
		title:       "Bổ sung thành viên cho vai trò quan trọng",
		actionLabel: "Mở vai trò & phân quyền",
		actionLink:  "/app/admin?tab=rbac",
		category:    "rbac",
	},
	"conflict.permission.grantable_violation": {
		title:       "Sửa quyền gán trực tiếp không hợp lệ",
		actionLabel: "Mở vai trò & phân quyền",
		actionLink:  "/app/admin?tab=rbac",
		category:    "rbac",
	},
	"conflict.workflow.assignee_role_missing": {
		title:       "Gán vai trò còn thiếu trong workflow",
		actionLabel: "Mở vai trò & phân quyền",
		actionLink:  "/app/admin?tab=rbac",
		category:    "workflow",
	},
	"conflict.rbac.role_unassigned_in_workflow": {
		title:       "Rà soát vai trò trong luồng workflow",
		actionLabel: "Mở vai trò & phân quyền",
		actionLink:  "/app/admin?tab=rbac",
		category:    "workflow",
	},
	"conflict.org.department_inactive_referenced": {
		title:       "Rà soát phòng ban không còn hoạt động",
		actionLabel: "Mở cơ cấu tổ chức",
		actionLink:  "/app/admin?tab=org",
		category:    "org",
	},
	"conflict.workflow.override_stale": {
		title:       "Rebase workflow override đã lỗi thời",
		actionLabel: "Mở loại CBTT",
		actionLink:  "/app/disclosure-types",
		category:    "workflow",
	},
}

// Format maps health checks to recommendations (pure — no I/O).
func Format(checks []CheckInput, evaluatedAt time.Time) []Item {
	if len(checks) == 0 {
		return []Item{}
	}
	if evaluatedAt.IsZero() {
		evaluatedAt = time.Now().UTC()
	}
	idCount := map[string]int{}
	out := make([]Item, 0, len(checks))
	for _, check := range checks {
		out = append(out, formatOne(check, evaluatedAt, idCount))
	}
	sort.Slice(out, func(i, j int) bool {
		pi, pj := priorityRank(out[i].Priority), priorityRank(out[j].Priority)
		if pi != pj {
			return pi < pj
		}
		return out[i].SourceCode < out[j].SourceCode
	})
	return out
}

func formatOne(check CheckInput, evaluatedAt time.Time, idCount map[string]int) Item {
	row, known := codeMap[check.Code]
	title := strings.TrimSpace(check.Title)
	description := strings.TrimSpace(check.Description)
	actionLink := strings.TrimSpace(check.ActionLink)
	actionLabel := fallbackActionLabel
	category := strings.TrimSpace(check.Domain)
	reason := description

	if known {
		if row.title != "" {
			title = row.title
		}
		actionLabel = row.actionLabel
		if row.actionLink != "" {
			actionLink = row.actionLink
		}
		if row.category != "" {
			category = row.category
		}
	} else {
		if title == "" {
			title = fallbackTitle
		}
		if description == "" {
			description = fallbackDescription
		}
		reason = description
	}

	if actionLink == "" {
		actionLink = defaultActionLink
	}
	if reason == "" {
		reason = description
	}
	if title == "" {
		title = fallbackTitle
	}
	if description == "" {
		description = fallbackDescription
	}

	baseID := "rec.configuration_health." + check.Code
	idCount[baseID]++
	id := baseID
	if idCount[baseID] > 1 {
		id = fmt.Sprintf("%s.%d", baseID, idCount[baseID])
	}

	return Item{
		ID:             id,
		Title:          title,
		Description:    description,
		Source:         SourceConfigurationHealth,
		SourceCode:     check.Code,
		Severity:       check.Severity,
		Priority:       SeverityToPriority(check.Severity),
		Reason:         reason,
		ActionLabel:    actionLabel,
		ActionLink:     actionLink,
		Evidence:       sanitizeEvidence(check.Evidence),
		GeneratedAt:    evaluatedAt,
		Category:       category,
		RelatedCheckID: check.Code,
	}
}

// SeverityToPriority maps health severity to recommendation priority (Batch 7A).
func SeverityToPriority(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "blocking":
		return PriorityHigh
	case "warning":
		return PriorityMedium
	case "info":
		return PriorityLow
	default:
		return PriorityLow
	}
}

func sanitizeEvidence(evidence map[string]any) map[string]any {
	if evidence == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(evidence))
	for k, v := range evidence {
		kl := strings.ToLower(k)
		if kl == "password" || kl == "token" || kl == "secret" || kl == "access_token" {
			continue
		}
		out[k] = v
	}
	return out
}

func priorityRank(priority string) int {
	switch priority {
	case PriorityHigh:
		return 0
	case PriorityMedium:
		return 1
	default:
		return 2
	}
}

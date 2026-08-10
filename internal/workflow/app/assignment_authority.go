package app

import "strings"

// ResolveTaskAssigneeMembershipIDs returns the authoritative assignee set for a task.
//
// Authority:
//   - relation rows present → v3 relation is sole authority (ignore singular)
//   - relation empty + singular non-empty → v2 singular
//   - both empty → none
//
// Never unions relation + singular.
func ResolveTaskAssigneeMembershipIDs(singular string, relationIDs []string) []string {
	out := make([]string, 0, len(relationIDs))
	seen := map[string]struct{}{}
	for _, id := range relationIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) > 0 {
		return out
	}
	if s := strings.TrimSpace(singular); s != "" {
		return []string{s}
	}
	return nil
}

// IsMembershipTaskAssignee reports whether membership is in the resolved assignee set.
func IsMembershipTaskAssignee(membershipID, singular string, relationIDs []string) bool {
	want := strings.TrimSpace(membershipID)
	if want == "" {
		return false
	}
	for _, id := range ResolveTaskAssigneeMembershipIDs(singular, relationIDs) {
		if id == want {
			return true
		}
	}
	return false
}

// SnapshotStepAssigneeIDs returns frozen step assignees (v3 array preferred, else singular).
func SnapshotStepAssigneeIDs(step StepSnapshot) []string {
	return ResolveTaskAssigneeMembershipIDs(step.AssigneeMembershipID, step.AssigneeMembershipIDs)
}

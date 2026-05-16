package app

import (
	"strconv"
	"strings"
)

const workflowDraftEtagPrefix = "v:"

// WorkflowDraftEtagFromVersion builds the FE-compatible synthetic etag for a draft version.
func WorkflowDraftEtagFromVersion(versionNo int) string {
	if versionNo <= 0 {
		return ""
	}
	return workflowDraftEtagPrefix + strconv.Itoa(versionNo)
}

// ResolveWorkflowBaseVersionNo accepts legacy base_version_no or FE base_etag ("v:<n>").
func ResolveWorkflowBaseVersionNo(baseVersionNo int, baseEtag string) int {
	if baseVersionNo > 0 {
		return baseVersionNo
	}
	etag := strings.TrimSpace(baseEtag)
	if strings.HasPrefix(etag, workflowDraftEtagPrefix) {
		if n, err := strconv.Atoi(strings.TrimPrefix(etag, workflowDraftEtagPrefix)); err == nil && n > 0 {
			return n
		}
	}
	if n, err := strconv.Atoi(etag); err == nil && n > 0 {
		return n
	}
	return 0
}

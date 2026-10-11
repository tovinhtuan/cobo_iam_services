package app

import (
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// templateRevisionPrefix versions the opaque revision token format. The token is derived
// from publication_candidate_hash, a canonical hash of the whole template content, so any
// content change produces a new revision (no extra column needed).
const templateRevisionPrefix = "pc1:"

// RevisionFromCandidateHash builds the opaque revision token; "" for legacy rows without a hash.
func RevisionFromCandidateHash(hash string) string {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return ""
	}
	return templateRevisionPrefix + hash
}

// CandidateHashFromRevision returns the hash inside a revision token. Unknown formats are
// returned unchanged so they simply fail the comparison (never match by accident).
func CandidateHashFromRevision(revision string) string {
	revision = strings.TrimSpace(revision)
	if !strings.HasPrefix(revision, templateRevisionPrefix) {
		return "\x00invalid-revision:" + revision
	}
	return strings.TrimPrefix(revision, templateRevisionPrefix)
}

// ErrStaleTemplateRevision is the 409 returned when the write target no longer matches
// the revision the client loaded (optimistic concurrency).
func ErrStaleTemplateRevision(currentHash string, versionNo int) error {
	return &perr.HTTPError{
		HTTPStatus: http.StatusConflict,
		Code:       perr.CodeStaleEtag,
		Message:    "template changed since it was loaded; reload and retry",
		Details: map[string]any{
			"current_revision":  RevisionFromCandidateHash(currentHash),
			"target_version_no": versionNo,
		},
	}
}

// headRevision picks the revision of the upsert write target from a version list:
// the open draft (not active, not released) when present, otherwise the active version.
func headRevision(items []DisclosureTypeVersionDTO) string {
	for _, item := range items {
		if !item.IsActive && !item.IsReleased {
			return item.Revision
		}
	}
	for _, item := range items {
		if item.IsActive {
			return item.Revision
		}
	}
	return ""
}

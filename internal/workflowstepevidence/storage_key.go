package workflowstepevidence

import (
	"fmt"
	"strings"
)

// StorageNamespace is the object-key prefix under DiskStorage root.
// Distinct from workflow-step-document-fulfillments — no key collision.
const StorageNamespace = "workflow-step-evidence"

// EvidenceObjectKey builds a server-side storage key.
// safeFileName must already be basename-sanitized; client never supplies the key.
func EvidenceObjectKey(companyID, fileID, safeFileName string) string {
	companyID = strings.TrimSpace(companyID)
	fileID = strings.TrimSpace(fileID)
	safeFileName = strings.TrimSpace(safeFileName)
	if safeFileName == "" {
		safeFileName = "file.bin"
	}
	return fmt.Sprintf("%s/%s/%s/%s", StorageNamespace, companyID, fileID, safeFileName)
}

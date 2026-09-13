package workflowfulfillment

import (
	"fmt"
	"strings"
)

// StorageNamespace is the object-key prefix under DiskStorage root.
const StorageNamespace = "workflow-step-document-fulfillments"

// FulfillmentObjectKey builds a server-side storage key.
// safeFileName must already be SanitizeFileName output (basename only).
func FulfillmentObjectKey(companyID, fileID, safeFileName string) string {
	companyID = strings.TrimSpace(companyID)
	fileID = strings.TrimSpace(fileID)
	safeFileName = strings.TrimSpace(safeFileName)
	if safeFileName == "" {
		safeFileName = "file.bin"
	}
	return fmt.Sprintf("%s/%s/%s/%s", StorageNamespace, companyID, fileID, safeFileName)
}

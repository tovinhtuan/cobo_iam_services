package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	// maxTemplateWriteBodyBytes bounds full template writes (upsert, global workflow PUT,
	// import confirm). Import validate already caps the source file at 2 MiB.
	maxTemplateWriteBodyBytes int64 = 4 << 20
	// maxSmallJSONBodyBytes bounds small command bodies (activate, clone, config, archive, restore).
	maxSmallJSONBodyBytes int64 = 64 << 10
)

// readJSONBody reads at most limit bytes. Oversized → 413, other read errors → 400,
// so malformed or abusive input never reaches the service layer or surfaces as 500.
func readJSONBody(w http.ResponseWriter, r *http.Request, limit int64) ([]byte, error) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "request body too large", nil)
		}
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid request body", nil)
	}
	return data, nil
}

// decodeJSONBody reads (bounded) and decodes into dst. allowEmpty treats an empty
// body as "no fields" (dst left as zero value).
func decodeJSONBody(w http.ResponseWriter, r *http.Request, limit int64, dst any, allowEmpty bool) error {
	data, err := readJSONBody(w, r, limit)
	if err != nil {
		return err
	}
	if allowEmpty && len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return invalidJSONError()
	}
	return nil
}

const maxAuditTextRunes = 500

// truncateAuditText bounds free text copied into audit metadata.
func truncateAuditText(s string) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= maxAuditTextRunes {
		return s
	}
	return string(runes[:maxAuditTextRunes]) + "…"
}

func invalidJSONError() error {
	return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON body", nil)
}

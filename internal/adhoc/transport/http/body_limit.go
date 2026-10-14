package http

import (
	"encoding/json"
	"errors"
	"net/http"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// maxProposalWriteBodyBytes bounds create/PATCH proposal bodies. Step additions are capped per
// proposal (20000 note characters, 100 documents of ≤ 255 characters), well under 1 MiB even as
// 3-byte UTF-8.
const maxProposalWriteBodyBytes int64 = 1 << 20

// decodeProposalWriteBody decodes a bounded JSON body. Oversized → 413, malformed → 400, so
// abusive or broken input never reaches the service layer or surfaces as 500.
func decodeProposalWriteBody(w http.ResponseWriter, r *http.Request, dst any) error {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxProposalWriteBodyBytes)).Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "request body too large", nil)
		}
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid request body", nil)
	}
	return nil
}

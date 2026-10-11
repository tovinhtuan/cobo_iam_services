package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	cmsCompanyBodyLimit       = 64 << 10 // create / PATCH
	cmsCompanyStatusBodyLimit = 16 << 10 // activate / suspend / deactivate
)

// decodeCMSCompanyBody decodes a size-limited JSON body: 413 when too large, 400 when malformed.
// allowEmpty accepts an empty body (status endpoints; the service then reports missing fields).
func decodeCMSCompanyBody(w http.ResponseWriter, r *http.Request, dst any, limit int64, allowEmpty bool) error {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return perr.NewHTTPError(http.StatusRequestEntityTooLarge, perr.CodeInvalidRequest, "request body too large", nil)
		}
		if allowEmpty && errors.Is(err, io.EOF) {
			return nil
		}
		return perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid JSON payload", err)
	}
	return nil
}

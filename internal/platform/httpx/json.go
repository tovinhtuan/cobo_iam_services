package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

// ErrorBody matches docs/api-contracts-json.md.
type ErrorBody struct {
	Error struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details,omitempty"`
	} `json:"error"`
}

// WriteJSONRaw writes already-encoded JSON bytes (Content-Type application/json).
func WriteJSONRaw(w http.ResponseWriter, status int, rawJSON []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if len(rawJSON) > 0 {
		_, _ = w.Write(rawJSON)
	}
}

// WriteJSON writes a JSON response with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
}

// WriteError maps errors to API JSON. Unknown errors become 500 INTERNAL_ERROR.
func WriteError(w http.ResponseWriter, log *slog.Logger, err error) {
	if err == nil {
		return
	}
	var he *perr.HTTPError
	if errors.As(err, &he) && he != nil {
		details := he.Details
		if details == nil {
			details = map[string]any{}
		}
		body := ErrorBody{}
		body.Error.Code = string(he.Code)
		body.Error.Message = he.Message
		body.Error.Details = details
		WriteJSON(w, he.HTTPStatus, body)
		return
	}
	if log != nil {
		log.Error("unhandled error", slog.String("err", err.Error()))
	}
	body := ErrorBody{}
	body.Error.Code = string(perr.CodeInternal)
	body.Error.Message = "Internal server error"
	body.Error.Details = map[string]any{}
	WriteJSON(w, http.StatusInternalServerError, body)
}

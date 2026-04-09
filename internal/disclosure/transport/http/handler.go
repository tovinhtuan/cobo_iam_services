package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
)

type Handler struct {
	svc       disclosureapp.Service
	inspector iamapp.TokenInspector
	idem      idempotency.Store
}

func NewHandler(svc disclosureapp.Service, inspector iamapp.TokenInspector, idem idempotency.Store) *Handler {
	return &Handler{svc: svc, inspector: inspector, idem: idem}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/disclosures", h.createRecord)
	mux.HandleFunc("GET /api/v1/disclosures", h.listRecords)
	mux.HandleFunc("GET /api/v1/disclosures/{record_id}", h.getRecord)
	mux.HandleFunc("PATCH /api/v1/disclosures/{record_id}", h.updateRecord)
	mux.HandleFunc("POST /api/v1/disclosures/{record_id}/submit", h.submitRecord)
	mux.HandleFunc("POST /api/v1/disclosures/{record_id}/confirm", h.confirmRecord)
}

func (h *Handler) createRecord(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload disclosureapp.RecordPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.CreateRecord(r.Context(), disclosureapp.CreateRecordRequest{Subject: sub, Payload: payload})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *Handler) updateRecord(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	recordID := r.PathValue("record_id")
	var payload disclosureapp.RecordPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.UpdateRecord(r.Context(), disclosureapp.UpdateRecordRequest{Subject: sub, RecordID: recordID, Payload: payload})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) submitRecord(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	recordID := r.PathValue("record_id")
	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var res idempotency.Result
	if idemKey != "" && h.idem != nil {
		hash := disclosureRequestHash(sub.CompanyID, recordID, sub.UserID, "submit")
		res, err = h.idem.TryReserve(r.Context(), idempotency.Params{
			CompanyID: sub.CompanyID, Scope: "disclosure.submit", Key: idemKey, RequestHash: hash,
		})
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		if res.Replay {
			httpx.WriteJSONRaw(w, res.ReplayHTTPStatus, res.ReplayBody)
			return
		}
		if res.Conflict {
			httpx.WriteJSON(w, http.StatusConflict, idempotencyConflictBody("idempotency conflict or request in progress"))
			return
		}
	}
	resp, err := h.svc.SubmitRecord(r.Context(), disclosureapp.SubmitRecordRequest{Subject: sub, RecordID: recordID})
	if res.ReservationID != "" && h.idem != nil {
		if err != nil {
			_ = h.idem.Abandon(r.Context(), res.ReservationID)
		} else {
			body, _ := json.Marshal(resp)
			env := idempotency.Envelope{HTTPStatus: http.StatusOK, Body: body}
			envBytes, _ := json.Marshal(&env)
			_ = h.idem.Complete(r.Context(), res.ReservationID, envBytes)
		}
	}
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) confirmRecord(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	recordID := r.PathValue("record_id")
	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var res idempotency.Result
	if idemKey != "" && h.idem != nil {
		hash := disclosureRequestHash(sub.CompanyID, recordID, sub.UserID, "confirm")
		res, err = h.idem.TryReserve(r.Context(), idempotency.Params{
			CompanyID: sub.CompanyID, Scope: "disclosure.confirm", Key: idemKey, RequestHash: hash,
		})
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		if res.Replay {
			httpx.WriteJSONRaw(w, res.ReplayHTTPStatus, res.ReplayBody)
			return
		}
		if res.Conflict {
			httpx.WriteJSON(w, http.StatusConflict, idempotencyConflictBody("idempotency conflict or request in progress"))
			return
		}
	}
	resp, err := h.svc.ConfirmRecord(r.Context(), disclosureapp.ConfirmRecordRequest{Subject: sub, RecordID: recordID})
	if res.ReservationID != "" && h.idem != nil {
		if err != nil {
			_ = h.idem.Abandon(r.Context(), res.ReservationID)
		} else {
			body, _ := json.Marshal(resp)
			env := idempotency.Envelope{HTTPStatus: http.StatusOK, Body: body}
			envBytes, _ := json.Marshal(&env)
			_ = h.idem.Complete(r.Context(), res.ReservationID, envBytes)
		}
	}
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) listRecords(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ListRecords(r.Context(), disclosureapp.ListRecordsRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getRecord(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	recordID := r.PathValue("record_id")
	resp, err := h.svc.GetRecord(r.Context(), disclosureapp.GetRecordRequest{Subject: sub, RecordID: recordID})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) subjectFromToken(r *http.Request) (disclosureapp.Subject, error) {
	tok := bearerToken(r.Header.Get("Authorization"))
	claims, err := h.inspector.InspectAccessToken(r.Context(), tok)
	if err != nil {
		return disclosureapp.Subject{}, err
	}
	return disclosureapp.Subject{UserID: claims.Sub, MembershipID: claims.MembershipID, CompanyID: claims.CompanyID}, nil
}

func disclosureRequestHash(companyID, recordID, userID, op string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s", companyID, recordID, userID, op)))
	return hex.EncodeToString(h[:])
}

func idempotencyConflictBody(msg string) map[string]any {
	return map[string]any{
		"error": map[string]any{
			"code":    "IDEMPOTENCY_CONFLICT",
			"message": msg,
		},
	}
}

func bearerToken(h string) string {
	h = strings.TrimSpace(h)
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
		return strings.TrimSpace(parts[1])
	}
	return h
}

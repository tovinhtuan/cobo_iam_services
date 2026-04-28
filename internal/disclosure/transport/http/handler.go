package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	auditapp "github.com/cobo/cobo_iam_services/internal/audit/app"
	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
)

type Handler struct {
	svc       disclosureapp.Service
	inspector iamapp.TokenInspector
	idem      idempotency.Store
	audit     auditapp.Service
}

func NewHandler(svc disclosureapp.Service, inspector iamapp.TokenInspector, idem idempotency.Store, audit auditapp.Service) *Handler {
	return &Handler{svc: svc, inspector: inspector, idem: idem, audit: audit}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/disclosures", h.createRecord)
	mux.HandleFunc("GET /api/v1/disclosures", h.listRecords)
	mux.HandleFunc("GET /api/v1/disclosures/{record_id}", h.getRecord)
	mux.HandleFunc("PATCH /api/v1/disclosures/{record_id}", h.updateRecord)
	mux.HandleFunc("POST /api/v1/disclosures/{record_id}/submit", h.submitRecord)
	mux.HandleFunc("POST /api/v1/disclosures/{record_id}/confirm", h.confirmRecord)
	mux.HandleFunc("GET /api/v1/disclosure-groups", h.listTypeGroups)
	mux.HandleFunc("GET /api/v1/disclosure-types", h.listTypes)
	mux.HandleFunc("GET /api/v1/disclosure-types/{type_id}", h.getTypeDetail)
	mux.HandleFunc("PUT /api/v1/admin/disclosure-types/{type_id}", h.upsertTypeVersion)
	mux.HandleFunc("GET /api/v1/admin/disclosure-types/{type_id}/versions", h.listTypeVersions)
	mux.HandleFunc("POST /api/v1/admin/disclosure-types/{type_id}/activate", h.activateTypeVersion)
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

func (h *Handler) listTypeGroups(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ListTypeGroups(r.Context(), disclosureapp.ListTypeGroupsRequest{Subject: sub})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) listTypes(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ListTypes(r.Context(), disclosureapp.ListTypesRequest{
		Subject: sub,
		GroupID: strings.TrimSpace(r.URL.Query().Get("group_id")),
		Query:   strings.TrimSpace(r.URL.Query().Get("q")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) getTypeDetail(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.GetTypeDetail(r.Context(), disclosureapp.GetTypeDetailRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) upsertTypeVersion(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload disclosureapp.UpsertTypeVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	payload.Subject = sub
	payload.TypeID = strings.TrimSpace(r.PathValue("type_id"))
	prev, _ := h.svc.GetTypeDetail(r.Context(), disclosureapp.GetTypeDetailRequest{Subject: sub, TypeID: payload.TypeID})
	resp, err := h.svc.UpsertTypeVersion(r.Context(), payload)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	prevVersion := 0
	if prev != nil {
		prevVersion = prev.VersionNo
	}
	h.auditLog(r, sub, "disclosure.type.version.upsert", "disclosure_type", payload.TypeID, map[string]any{
		"old_version_no": prevVersion,
		"new_version_no": resp.VersionNo,
		"change_note":    strings.TrimSpace(payload.ChangeNote),
	})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) listTypeVersions(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	resp, err := h.svc.ListTypeVersions(r.Context(), disclosureapp.ListTypeVersionsRequest{
		Subject: sub,
		TypeID:  strings.TrimSpace(r.PathValue("type_id")),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) activateTypeVersion(w http.ResponseWriter, r *http.Request) {
	sub, err := h.subjectFromToken(r)
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var payload struct {
		VersionNo int    `json:"version_no"`
		Reason    string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	typeID := strings.TrimSpace(r.PathValue("type_id"))
	prev, _ := h.svc.GetTypeDetail(r.Context(), disclosureapp.GetTypeDetailRequest{
		Subject: sub,
		TypeID:  typeID,
	})
	resp, err := h.svc.ActivateTypeVersion(r.Context(), disclosureapp.ActivateTypeVersionRequest{
		Subject:   sub,
		TypeID:    typeID,
		VersionNo: payload.VersionNo,
		Reason:    strings.TrimSpace(payload.Reason),
	})
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	prevVersion := 0
	if prev != nil {
		prevVersion = prev.VersionNo
	}
	h.auditLog(r, sub, "disclosure.type.version.activate", "disclosure_type", typeID, map[string]any{
		"old_version_no": prevVersion,
		"new_version_no": resp.VersionNo,
		"reason":         strings.TrimSpace(payload.Reason),
	})
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) auditLog(r *http.Request, sub disclosureapp.Subject, action, resourceType, resourceID string, metadata map[string]any) {
	if h.audit == nil {
		return
	}
	_ = h.audit.AppendAuditLog(r.Context(), auditapp.AppendAuditLogRequest{
		ActorUserID:       sub.UserID,
		ActorMembershipID: sub.MembershipID,
		CompanyID:         sub.CompanyID,
		Action:            action,
		ResourceType:      resourceType,
		ResourceID:        resourceID,
		Decision:          "allow",
		RequestID:         httpx.RequestIDFromContext(r.Context()),
		IP:                r.RemoteAddr,
		UserAgent:         r.UserAgent(),
		Metadata:          metadata,
	})
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

package http

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	caapp "github.com/cobo/cobo_iam_services/internal/companyaccess/app"
	iamapp "github.com/cobo/cobo_iam_services/internal/iam/app"
	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/httpx"
	"github.com/cobo/cobo_iam_services/internal/platform/idempotency"
)

type provisionRouteConfig struct {
	requireSelfCreateEnabled bool
	idemScope                string
	idemHashOp               string
	auditAction              string
	invoke                   func(context.Context, companyProvisionBody, string) (*caapp.InitializeCompanyResult, error)
}

func (h *AdminHandler) initializeCompany(w http.ResponseWriter, r *http.Request) {
	h.handleSelfServiceProvision(w, r, provisionRouteConfig{
		idemScope:   "company.initialize",
		idemHashOp:  "initialize",
		auditAction: "company.initialize",
		invoke: func(ctx context.Context, p companyProvisionBody, userID string) (*caapp.InitializeCompanyResult, error) {
			return h.svc.InitializeCompany(ctx, caapp.InitializeCompanyRequest{
				UserID: userID, CompanyName: p.CompanyName, TaxCode: p.TaxCode,
				RegistrationNumber: p.RegistrationNumber, Address: p.Address,
				Phone: p.Phone, ContactEmail: p.ContactEmail,
			})
		},
	})
}

func (h *AdminHandler) createSelfServiceCompany(w http.ResponseWriter, r *http.Request) {
	h.handleSelfServiceProvision(w, r, provisionRouteConfig{
		requireSelfCreateEnabled: true,
		idemScope:                "company.create",
		idemHashOp:               "create",
		auditAction:              "company.create_self_service",
		invoke: func(ctx context.Context, p companyProvisionBody, userID string) (*caapp.InitializeCompanyResult, error) {
			return h.svc.CreateSelfServiceCompany(ctx, caapp.CreateSelfServiceCompanyRequest{
				UserID: userID, CompanyName: p.CompanyName, TaxCode: p.TaxCode,
				RegistrationNumber: p.RegistrationNumber, Address: p.Address,
				Phone: p.Phone, ContactEmail: p.ContactEmail,
			})
		},
	})
}

func (h *AdminHandler) handleSelfServiceProvision(w http.ResponseWriter, r *http.Request, cfg provisionRouteConfig) {
	if cfg.requireSelfCreateEnabled && !h.selfCreateEnabled {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeFeatureDisabled, "FEATURE_DISABLED", nil))
		return
	}

	claims, err := h.inspector.InspectAccessToken(r.Context(), bearerToken(r.Header.Get("Authorization")))
	if err != nil {
		httpx.WriteError(w, nil, err)
		return
	}
	var p companyProvisionBody
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "invalid json body", err))
		return
	}

	idemKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if h.requireProvisionIdempotency && idemKey == "" {
		httpx.WriteError(w, nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "Idempotency-Key header required", nil))
		return
	}

	var idemRes idempotency.Result
	if idemKey != "" && h.idem != nil {
		hash := companyProvisionRequestHash(claims.Sub, p, cfg.idemHashOp)
		idemRes, err = h.idem.TryReserve(r.Context(), idempotency.Params{
			Scope: cfg.idemScope, Key: idemKey, RequestHash: hash,
		})
		if err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
		if idemRes.Replay {
			httpx.WriteJSONRaw(w, idemRes.ReplayHTTPStatus, idemRes.ReplayBody)
			return
		}
		if idemRes.Conflict {
			httpx.WriteJSON(w, http.StatusConflict, idempotencyConflictBody("idempotency conflict or request in progress"))
			return
		}
	}

	result, err := cfg.invoke(r.Context(), p, claims.Sub)
	if err != nil {
		if idemRes.ReservationID != "" && h.idem != nil {
			_ = h.idem.Abandon(r.Context(), idemRes.ReservationID)
		}
		httpx.WriteError(w, nil, err)
		return
	}

	resp, sessionErr := h.issueProvisionSession(r, claims, result.MembershipID, result.CompanyID)
	if sessionErr != nil {
		_ = h.svc.RollbackSelfServiceBootstrap(r.Context(), result.CompanyID, claims.Sub)
		if idemRes.ReservationID != "" && h.idem != nil {
			_ = h.idem.Abandon(r.Context(), idemRes.ReservationID)
		}
		httpx.WriteError(w, nil, sessionErr)
		return
	}

	resp["company_id"] = result.CompanyID
	resp["company_code"] = result.CompanyCode
	resp["company_name"] = result.CompanyName
	resp["membership_id"] = result.MembershipID

	if idemRes.ReservationID != "" && h.idem != nil {
		body, _ := json.Marshal(resp)
		env := idempotency.Envelope{HTTPStatus: http.StatusCreated, Body: body}
		envBytes, _ := json.Marshal(&env)
		if err := h.idem.Complete(r.Context(), idemRes.ReservationID, envBytes); err != nil {
			httpx.WriteError(w, nil, err)
			return
		}
	}

	h.logProvisionSuccess(r, claims, result, cfg)
	h.auditProvision(r, claims.Sub, result.MembershipID, result.CompanyID, cfg.auditAction)
	httpx.WriteJSON(w, http.StatusCreated, resp)
}

func (h *AdminHandler) logProvisionSuccess(r *http.Request, claims *iamapp.AccessTokenClaims, result *caapp.InitializeCompanyResult, cfg provisionRouteConfig) {
	attrs := []any{
		slog.String("user_id", claims.Sub),
		slog.String("company_id", result.CompanyID),
		slog.String("membership_id", result.MembershipID),
		slog.String("idempotency_scope", cfg.idemScope),
	}
	if cfg.idemHashOp == "create" {
		attrs = append(attrs, slog.String("provisioning_source", caapp.ProvisioningSourceSelfServiceCreate))
	} else {
		attrs = append(attrs, slog.String("provisioning_source", caapp.ProvisioningSourceSelfServiceInitialize))
	}
	slog.InfoContext(r.Context(), "company self-service provision", attrs...)
}

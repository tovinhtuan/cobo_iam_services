# Endpoint contract audit

## 1) `GET /api/v1/admin/company` (GetOwnCompany)
| Layer | Exact |
|-------|-------|
| Route | `admin_handler.go` `mux.HandleFunc("GET /api/v1/admin/company", h.getOwnCompany)` |
| Handler | `AdminHandler.getOwnCompany` → `httpx.WriteJSON(200, out)` |
| Service | `adminService.GetOwnCompany` |
| Authz | `authorize(..., "company.view", sub.CompanyID)`; requires non-empty `CompanyID` |
| Repo | `GetCompanyPlatform(ctx, companyID)` |
| DTO | `PlatformCompanyDetail` (`platform_company.go`) — legal/contact/applicability/counts; **no plan** |
| Cache | none on this path |
| FE | `companyProfileApi.getOwnCompany` → Company Information |

## 2) `GET /api/v1/me/companies`
| Layer | Exact |
|-------|-------|
| Route | `me_handler.go` `GET /api/v1/me/companies` |
| Handler | `MeHandler.companies` |
| Authn | access token inspector |
| Data | `members.GetMembershipsByUser` + roles/titles/address |
| Fields | `company_id`, `company_code`, `membership_id`, `company_name`, `membership_status`, `roles`, `titles`, `address` — **no plan** |
| FE | company switcher / `AuthorizedCompany` normalizer |

## 3) Selected company context
- JWT claims carry `company_id` + `membership_id`; `POST /v1/auth/select-company` switches.
- GetOwnCompany always uses **subject company**, not arbitrary path ID (IDOR-resistant for this route).

## 4) Platform CMS company detail
- `PlatformCompanyDetail` same struct family; also no plan — out of Portal company-info primary path but consider consistency later.

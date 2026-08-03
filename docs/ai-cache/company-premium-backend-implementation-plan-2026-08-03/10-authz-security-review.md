# Authz / security review

| Question | Evidence-based answer |
|----------|----------------------|
| Member of company? | GetOwnCompany: JWT company context + `company.view` |
| Platform admin? | Platform CMS routes separate; do not auto-expose Portal plan without review |
| User not in company? | Cannot call GetOwnCompany for other IDs (no company_id path param) |
| `/me/companies` leak? | Only memberships of caller — OK if plan attached per item |
| Sensitive? | Plan code is low sensitivity vs billing; still minimize |
| Field-level permission? | Not required for badge; keep under `company.view` / membership |
| IDOR | Prefer keep GetOwnCompany subject-scoped; if Option C added, validate membership |
| Logging | Log `company_id` + plan code; never payment QR/invoice |

## Remaining
`REQUIRES_SECURITY_REVIEW`: whether platform CMS company detail should include same `plan`.

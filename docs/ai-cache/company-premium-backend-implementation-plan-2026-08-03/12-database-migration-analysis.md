# Database / migration analysis

## Path depending on Product SoT

### If B1 interim (entitlement display approved)
- **No migration**
- Read-only exposure of resolver
- Document badge = effective member entitlement, not purchase

### If B2/C commercial SoT (recommended long-term)
**NEW TABLE PROPOSED** (conceptual — do not create now):

```text
company_subscriptions
  id              VARCHAR(36) PK
  company_id      VARCHAR(36) NOT NULL FK companies
  plan_code       VARCHAR(32) NOT NULL  -- Free|Premium|Enterprise or commercial codes
  status          VARCHAR(32) NOT NULL  -- ACTIVE|TRIAL|EXPIRED|SUSPENDED|CANCELLED
  effective_from  TIMESTAMP NULL
  expires_at      TIMESTAMP NULL
  source          VARCHAR(64) NOT NULL
  created_at / updated_at
  UNIQUE active constraint strategy: partial unique or app-enforced single ACTIVE
  INDEX (company_id, status)
```

Backfill: Product-defined (default Free/NONE; optional map from current max-member for DEV only with audit flag).

Rollback: drop table only if never written in prod; else stop reads + feature flag.

# 06 — CMS API trace

| Endpoint | Status | Source/version | Step count | Purpose |
|----------|--------|----------------|------------|---------|
| GET .../workflow/configuration | 200 | empty / versions=[] | 0 | CMS Workflow tab |
| GET .../workflow | 200 | null | 0 | Raw write shape |
| GET .../workflow/versions | 200 | null | 0 | Version list |
| GET .../effective-workflow | 200 | global_template v1 | 4 | Runtime resolve |

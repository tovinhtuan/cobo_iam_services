# 41 — USER COMMIT EXCLUSIONS
USER_COMMIT_EXCLUSION_MANIFEST_COMPLETE=true

| PATH | CLASSIFICATION | REASON |
| cobo_iam_services/deploy-artifacts/** | I/J | Build/deploy mirrors; preexisting churn; not product source |
| cobo_iam_services/.playwright-mcp/g2c-smoke-last.json | I | Local smoke result |
| cobo_web_design/.playwright-mcp/g3-browser-smoke-last.json | I | Local smoke result |
| cobo_web_design/dist/** (if dirty) | I | Local build output; docker-owned |
| Any g4-materialize-draft binary | I | Local compiled helper — source main.go is INCLUDE |

PREEXISTING_USER_CHANGES_PRESERVED=true

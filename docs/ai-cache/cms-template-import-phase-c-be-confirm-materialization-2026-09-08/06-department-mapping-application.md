# Department Mapping Application

## Mapping Precedence
For each workflow step in the imported template, the target department is resolved according to strict precedence:
1. **Explicit Confirm Mapping**: `req.DepartmentMappings[stepDeptID]`
2. **Exact Department Code Match**: Matching against lowercase `department_code` in target catalog
3. **Exact Normalized Department Name Match**: Matching against lowercase `department_name` in target catalog
4. **Unresolved**: Rejects with HTTP 400 Bad Request

## Mapping Input Validation Rules
- **M1**: Exact code match resolves automatically.
- **M2**: Exact normalized name match resolves automatically.
- **M3**: Unknown source department with valid explicit target code resolves successfully.
- **M4**: Unknown source department without explicit mapping is BLOCKED (HTTP 400).
- **M5**: Mapping target nonexistent in catalog is BLOCKED (HTTP 400).
- **M6**: Target deleted between Validate and Confirm is revalidated and BLOCKED (HTTP 400).
- **M7**: Extra mapping key not referenced in the source template workflow is BLOCKED (HTTP 400).
- **M8**: Blank mapping target code is BLOCKED (HTTP 400).
- **M9**: Two source departments mapped to the same target department is ALLOWED.
- **M10**: Malicious mapping keys attempting field injection are BLOCKED (HTTP 400).

## Immutability & Defensive Copying
- Mappings are applied during materialization mapping to a new array of `WorkflowStepDTO`.
- The original `req.NormalizedTemplate` is NEVER mutated.
- Canonical payload hash is verified against the unchanged `req.NormalizedTemplate` before mapping resolution.

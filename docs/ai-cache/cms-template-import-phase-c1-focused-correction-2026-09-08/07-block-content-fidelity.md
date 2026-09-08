# Phase C.1 Evidence — 07 Block Content Fidelity

## Canonical Mandatory Blocks Fidelity
Six canonical blocks are generated at confirm materialization:
1. `legal_basis` (Rich Text):
   - Description synced from `norm.LegalBasis` / `norm.LegalBases[0].Title`.
   - Fresh server-generated `BlockID`.
2. `disclosure_content` (Rich Text):
   - Description populated from `norm.ReportContent`.
   - Fresh server-generated `BlockID`.
3. `deadline` (Text):
   - Description populated from `norm.DeadlineRule`.
   - Fresh server-generated `BlockID`.
4. `channels_and_format` (Rich Text):
   - Description populated from `norm.ChannelsText` and `norm.Format`.
   - Channel config populated with `norm.ChannelsText` and `norm.Format`.
   - Fresh server-generated `BlockID`.
5. `legal_risks` (Rich Text):
   - Description populated from `norm.LegalRisksText`.
   - Fresh server-generated `BlockID`.
6. `enterprise_workflow` (Rich Text):
   - `Config.steps` populated from resolved workflow steps.
   - Pinned into `WorkflowManifest` on the version.
   - Fresh server-generated `BlockID`.

## Result
Server-generated IDs are fresh; imported business content is 100% preserved.
Status: **PASS**.

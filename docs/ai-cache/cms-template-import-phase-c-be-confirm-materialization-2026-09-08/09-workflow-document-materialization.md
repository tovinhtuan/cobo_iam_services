# Workflow Document Materialization

## Document Requirement Semantics
Documents attached to workflow steps represent compliance file requirements (e.g. "Bảng cân đối kế toán", "Thuyết minh báo cáo tài chính").

## Invariants Enforced
- `DOCUMENT_ID_MATERIALIZATION_RULE = SERVER_OWNED_UUID`
  - Every document requirement is assigned a fresh server-owned UUID during materialization.
- `TEMPLATE_FILE_ID_PERSISTED = false`
  - `template_file_id` is strictly set to `""`.
  - Non-portable asset pointers from source systems are never stored.
- `DOCUMENT_BINARY_WRITE_COUNT = 0`
  - No binary file is created or written to disk.
- `MEDIA_UPLOAD_PERFORMED = false`
  - No media service upload or asset record is generated.
- `template_file_name`:
  - Retained as metadata indicating the recommended filename for template download (e.g. `bang_can_doi.xlsx`).

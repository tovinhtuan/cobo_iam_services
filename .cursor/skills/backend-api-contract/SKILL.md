---
name: backend-api-contract
description: Dùng khi xây endpoint hoặc thay đổi contract trong cobo_iam_services, bảo đảm validation, error model, backward compatibility và testability
---

# Backend API Contract

## When to use
Use this skill when the task is to:
- add an endpoint
- change request/response shape
- integrate frontend with IAM service
- harden validation and error mapping

## Workflow
1. Viết request/response contract trước khi code handler.
2. Xác định authn/authz requirements cho endpoint.
3. Xác định validation rules và error codes.
4. Tách handler/service/repository responsibilities.
5. Xác định transaction boundary và cache invalidation nếu có.
6. Xác định observability: logs, metrics, tracing hooks nếu stack có.
7. Viết tests cho success + validation fail + permission fail + not found/conflict.

## Guardrails
- Không bind DB model trực tiếp ra API response nếu chưa chủ đích.
- Không để handler chứa branching business logic lớn.
- Không trả lỗi mơ hồ khi có thể phân loại.
- Không tạo breaking response change mà không nêu migration path.

## Output format
- Endpoint contract
- Validation rules
- Authorization rules
- Layers changed
- Test matrix

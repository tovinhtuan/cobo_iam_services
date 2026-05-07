import json
import urllib.request


BASE_URL = "http://localhost:8080"
LOGIN_ID = "cms.operator@example.com"
PASSWORD = "secret"
COMPANY_ID = "c_001"
TARGET_TYPE_ID = "dt-qa-bao-cao-tai-chinh-quy-20260507-5"


def post(path: str, payload: dict, token: str | None = None) -> dict:
    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    req = urllib.request.Request(
        f"{BASE_URL}{path}",
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers=headers,
        method="POST",
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode("utf-8"))


def put(path: str, payload: dict, token: str) -> dict:
    req = urllib.request.Request(
        f"{BASE_URL}{path}",
        data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {token}",
        },
        method="PUT",
    )
    with urllib.request.urlopen(req) as resp:
        return json.loads(resp.read().decode("utf-8"))


def main() -> None:
    login = post("/api/v1/auth/login", {"login_id": LOGIN_ID, "password": PASSWORD})
    token = login.get("session", {}).get("access_token", "")
    if not token:
        pre_company_token = login["session"]["pre_company_token"]
        selected = post(
            "/api/v1/auth/select-company",
            {"company_id": COMPANY_ID},
            token=pre_company_token,
        )
        token = selected["access_token"]

    payload = {
        "type_id": TARGET_TYPE_ID,
        "scope": "global",
        "group_id": "group-001",
        "name": "Báo cáo tài chính quý",
        "category": "Định kỳ",
        "template_category": "periodic",
        "periodicity": "quarterly",
        "deadline_strategy": "fixed_cycle_days",
        "deadline_rule": "Trong vòng 20 ngày kể từ ngày kết thúc quý.",
        "tags": ["Tài chính", "Định kỳ"],
        "description": "Báo cáo kết quả kinh doanh, bảng cân đối kế toán và lưu chuyển tiền tệ hàng quý.",
        "legal_basis": "Thông tư 96/2020/TT-BTC; Nghị định 155/2020/NĐ-CP",
        "implementation_content": "Workflow nội bộ chuẩn cho báo cáo tài chính quý",
        "report_content": (
            "- Bảng cân đối kế toán\n"
            "- Báo cáo kết quả hoạt động kinh doanh\n"
            "- Báo cáo lưu chuyển tiền tệ\n"
            "- Bản thuyết minh báo cáo tài chính"
        ),
        "required_docs": (
            "- Bảng cân đối kế toán\n"
            "- Báo cáo kết quả hoạt động kinh doanh\n"
            "- Báo cáo lưu chuyển tiền tệ\n"
            "- Bản thuyết minh BCTC"
        ),
        "channels_text": "Trang thông tin điện tử của Công ty, Hệ thống IDS của UBCKNN, Sở Giao dịch Chứng khoán (HOSE/HNX)",
        "legal_risks_text": "Chậm công bố thông tin có thể bị xử phạt hành chính.",
        "general_info": (
            "Theo quy định tại Thông tư 96/2020/TT-BTC, công ty đại chúng phải công bố "
            "báo cáo tài chính quý trong thời hạn 20 ngày kể từ ngày kết thúc quý."
        ),
        "reminder_milestones": ["Trước 5 ngày", "Trước 3 ngày", "Trước 1 ngày"],
        "blocks": [
            {
                "block_id": "m-legal-basis",
                "block_key": "legal_basis",
                "block_type": "rich_text",
                "title": "Cơ sở pháp lý",
                "description": "Thông tư 96/2020/TT-BTC; Nghị định 155/2020/NĐ-CP",
                "config": {"allow_html": False, "max_length": 8000},
                "validation": {},
                "display_order": 1,
                "enabled": True,
            },
            {
                "block_id": "m-disclosure-content",
                "block_key": "disclosure_content",
                "block_type": "rich_text",
                "title": "Nội dung công bố/báo cáo",
                "description": (
                    "- Bảng cân đối kế toán\n"
                    "- Báo cáo kết quả hoạt động kinh doanh\n"
                    "- Báo cáo lưu chuyển tiền tệ\n"
                    "- Bản thuyết minh báo cáo tài chính"
                ),
                "config": {
                    "allow_html": True,
                    "max_length": 50000,
                    "sections": [
                        {
                            "title": "Nội dung phải công bố",
                            "items": [
                                "Bảng cân đối kế toán",
                                "Báo cáo kết quả hoạt động kinh doanh",
                                "Báo cáo lưu chuyển tiền tệ",
                                "Bản thuyết minh báo cáo tài chính",
                            ],
                        },
                        {
                            "title": "Thành phần báo cáo bắt buộc",
                            "items": [
                                "Báo cáo tài chính quý (đầy đủ)",
                                "Văn bản giải trình nếu lợi nhuận sau thuế thay đổi từ 10% trở lên so với cùng kỳ",
                            ],
                        },
                        {
                            "title": "Phạm vi áp dụng",
                            "items": [
                                "Công ty đại chúng quy mô lớn",
                                "Công ty niêm yết",
                                "Công ty đăng ký giao dịch (UPCoM)",
                            ],
                        },
                    ],
                },
                "validation": {},
                "display_order": 2,
                "enabled": True,
            },
            {
                "block_id": "m-deadline",
                "block_key": "deadline",
                "block_type": "text",
                "title": "Kỳ hạn công bố/báo cáo",
                "description": "Trong vòng 20 ngày kể từ ngày kết thúc quý.",
                "config": {"max_length": 4000},
                "validation": {},
                "display_order": 3,
                "enabled": True,
            },
            {
                "block_id": "m-channels",
                "block_key": "channels_and_format",
                "block_type": "rich_text",
                "title": "Kênh và hình thức",
                "description": "Trang thông tin điện tử của Công ty, Hệ thống IDS của UBCKNN, Sở Giao dịch Chứng khoán (HOSE/HNX)",
                "config": {
                    "allow_html": False,
                    "max_length": 12000,
                    "channels": [
                        {
                            "id": "ch-001",
                            "name": "Trang thông tin điện tử của Công ty",
                            "format": "Điện tử",
                            "file_type": "PDF",
                            "attachment_required": True,
                        },
                        {
                            "id": "ch-002",
                            "name": "Hệ thống IDS của UBCKNN",
                            "format": "Trực tuyến",
                            "file_type": "XML/PDF",
                            "attachment_required": True,
                        },
                        {
                            "id": "ch-003",
                            "name": "Sở Giao dịch Chứng khoán (HOSE/HNX)",
                            "format": "Trực tuyến",
                            "file_type": "PDF",
                            "attachment_required": True,
                        },
                    ],
                },
                "validation": {},
                "display_order": 4,
                "enabled": True,
            },
            {
                "block_id": "m-legal-risks",
                "block_key": "legal_risks",
                "block_type": "rich_text",
                "title": "Rủi ro pháp lý",
                "description": "Rủi ro pháp lý nếu không thực hiện đúng.",
                "config": {
                    "risks": [
                        {
                            "id": "risk-001",
                            "title": "Chậm công bố thông tin",
                            "impact": "Cao",
                            "penalty": "Phạt tiền từ 50 - 70 triệu đồng",
                            "legal_basis": "Nghị định 156/2020/NĐ-CP",
                        },
                        {
                            "id": "risk-002",
                            "title": "Nội dung công bố không đầy đủ",
                            "impact": "Trung bình",
                            "penalty": "Phạt tiền từ 10 - 30 triệu đồng",
                            "legal_basis": "Nghị định 156/2020/NĐ-CP",
                        },
                    ]
                },
                "validation": {},
                "display_order": 5,
                "enabled": True,
            },
            {
                "block_id": "m-enterprise-workflow",
                "block_key": "enterprise_workflow",
                "block_type": "rich_text",
                "title": "Workflow doanh nghiệp",
                "description": "Workflow nội bộ chuẩn cho báo cáo tài chính quý",
                "config": {
                    "steps": [
                        {
                            "id": "step-001",
                            "stage": "Xác định nghĩa vụ",
                            "department": "Phòng Pháp chế",
                            "assignee": "Trần Thị B",
                            "due_date": "T+1",
                            "status": "completed",
                            "documents": [
                                {
                                    "id": "doc-001",
                                    "name": "Văn bản thông báo sự kiện",
                                    "type": "PDF",
                                    "is_required": True,
                                },
                                {
                                    "id": "doc-002",
                                    "name": "Cơ sở pháp lý",
                                    "type": "PDF",
                                    "is_required": True,
                                },
                            ],
                        },
                        {
                            "id": "step-002",
                            "stage": "Chuẩn bị dữ liệu",
                            "department": "Phòng Kế toán",
                            "assignee": "Hoàng Văn E",
                            "due_date": "T+10",
                            "status": "completed",
                            "documents": [
                                {
                                    "id": "doc-003",
                                    "name": "Bảng cân đối kế toán",
                                    "type": "XLSX",
                                    "is_required": True,
                                },
                                {
                                    "id": "doc-004",
                                    "name": "Báo cáo KQKD",
                                    "type": "XLSX",
                                    "is_required": True,
                                },
                            ],
                        },
                        {
                            "id": "step-003",
                            "stage": "Rà soát pháp lý",
                            "department": "Phòng Pháp chế",
                            "assignee": "Trần Thị B",
                            "due_date": "T+15",
                            "status": "in_progress",
                            "documents": [
                                {
                                    "id": "doc-005",
                                    "name": "Dự thảo bản công bố",
                                    "type": "DOCX",
                                    "is_required": True,
                                },
                                {
                                    "id": "doc-006",
                                    "name": "Tờ trình phê duyệt",
                                    "type": "PDF",
                                    "is_required": True,
                                },
                            ],
                        },
                        {
                            "id": "step-004",
                            "stage": "Phê duyệt nội bộ",
                            "department": "Ban Tổng Giám đốc",
                            "assignee": "Nguyễn Văn A",
                            "due_date": "T+18",
                            "status": "not_started",
                            "documents": [
                                {
                                    "id": "doc-007",
                                    "name": "Bản công bố đã ký",
                                    "type": "PDF",
                                    "is_required": True,
                                },
                                {
                                    "id": "doc-008",
                                    "name": "Nghị quyết HĐQT",
                                    "type": "PDF",
                                    "is_required": True,
                                },
                            ],
                        },
                        {
                            "id": "step-005",
                            "stage": "Công bố/Báo cáo",
                            "department": "Phòng Quan hệ nhà đầu tư",
                            "assignee": "Lê Văn C",
                            "due_date": "T+20",
                            "status": "not_started",
                            "documents": [
                                {
                                    "id": "doc-009",
                                    "name": "Xác nhận gửi tin",
                                    "type": "PDF",
                                    "is_required": True,
                                },
                                {
                                    "id": "doc-010",
                                    "name": "Ảnh chụp website",
                                    "type": "JPG",
                                    "is_required": True,
                                },
                            ],
                        },
                        {
                            "id": "step-006",
                            "stage": "Lưu hồ sơ",
                            "department": "Phòng Pháp chế",
                            "assignee": "Trần Thị B",
                            "due_date": "T+21",
                            "status": "not_started",
                            "documents": [
                                {
                                    "id": "doc-011",
                                    "name": "Hồ sơ lưu trữ",
                                    "type": "PDF",
                                    "is_required": True,
                                }
                            ],
                        },
                    ]
                },
                "validation": {},
                "display_order": 6,
                "enabled": True,
            },
        ],
        "change_note": "Tech lead fix: match dt-001 mock on main",
    }

    result = put(f"/api/v1/admin/disclosure-types/{TARGET_TYPE_ID}", payload, token)
    print(json.dumps(result, ensure_ascii=False))


if __name__ == "__main__":
    main()

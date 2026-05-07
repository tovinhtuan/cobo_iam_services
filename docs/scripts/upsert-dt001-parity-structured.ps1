param(
  [string]$BaseUrl = "http://localhost:8080",
  [string]$LoginId = "cms.operator@example.com",
  [string]$Password = "secret",
  [string]$CompanyId = "c_001",
  [string]$TypeId = "dt-qa-bao-cao-tai-chinh-quy-20260507-1"
)

$ErrorActionPreference = "Stop"

$login = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/auth/login" -ContentType "application/json" -Body (@{
  login_id = $LoginId
  password = $Password
} | ConvertTo-Json)

$token = [string]$login.session.access_token
if ([string]::IsNullOrWhiteSpace($token)) {
  $preToken = [string]$login.session.pre_company_token
  $selected = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/auth/select-company" -Headers @{
    Authorization = "Bearer $preToken"
  } -ContentType "application/json" -Body (@{
    company_id = $CompanyId
  } | ConvertTo-Json)
  $token = [string]$selected.access_token
}

$payload = @{
  type_id = $TypeId
  scope = "global"
  group_id = "group-001"
  name = "Bao cao tai chinh quy"
  category = "Dinh ky"
  template_category = "periodic"
  periodicity = "quarterly"
  deadline_strategy = "fixed_cycle_days"
  deadline_rule = "Trong vong 20 ngay ke tu ngay ket thuc quy."
  tags = @("Dinh ky", "Quy", "BCTC", "CBTT")
  description = "Bao cao ket qua kinh doanh, bang can doi ke toan va luu chuyen tien te hang quy."
  legal_basis = "Thong tu 96/2020/TT-BTC; Nghi dinh 155/2020/ND-CP"
  report_content = "Bang can doi ke toan; Bao cao ket qua hoat dong kinh doanh; Bao cao luu chuyen tien te; Ban thuyet minh bao cao tai chinh"
  required_docs = "Bang can doi ke toan; Bao cao ket qua hoat dong kinh doanh; Bao cao luu chuyen tien te; Ban thuyet minh BCTC"
  channels_text = "Trang thong tin dien tu cua Cong ty; He thong IDS cua UBCKNN; So Giao dich Chung khoan (HOSE/HNX)"
  legal_risks_text = "Khuyen nghi: Thiet lap quy trinh ra soat 2 lop va hoan thanh ho so it nhat 2 ngay truoc han chot de tranh rui ro he thong."
  general_info = "Theo quy dinh tai Thong tu 96/2020/TT-BTC, cong ty dai chung phai cong bo bao cao tai chinh quy trong thoi han 20 ngay ke tu ngay ket thuc quy."
  reminder_milestones = @("Truoc 5 ngay", "Truoc 3 ngay", "Truoc 1 ngay")
  blocks = @(
    @{
      block_id = "blk-legal-basis"
      block_key = "legal_basis"
      block_type = "rich_text"
      title = "Co so phap ly"
      description = "Thong tu 96/2020/TT-BTC; Nghi dinh 155/2020/ND-CP"
      config = @{ max_length = 8000; allow_html = $false }
      validation = @{}
      display_order = 1
      enabled = $true
    },
    @{
      block_id = "blk-content"
      block_key = "disclosure_content"
      block_type = "rich_text"
      title = "Noi dung cong bo/bao cao"
      description = "Noi dung bao cao chinh va ho so bat buoc."
      config = @{
        max_length = 50000
        allow_html = $true
        sections = @(
          @{ title = "Noi dung phai cong bo"; items = @("Bang can doi ke toan", "Bao cao ket qua hoat dong kinh doanh", "Bao cao luu chuyen tien te", "Ban thuyet minh bao cao tai chinh") },
          @{ title = "Thanh phan bao cao bat buoc"; items = @("Bao cao tai chinh quy (day du)", "Van ban giai trinh neu loi nhuan sau thue thay doi tu 10% tro len so voi cung ky") },
          @{ title = "Pham vi ap dung"; items = @("Cong ty dai chung quy mo lon", "Cong ty niem yet", "Cong ty dang ky giao dich (UPCoM)") }
        )
      }
      validation = @{}
      display_order = 2
      enabled = $true
    },
    @{
      block_id = "blk-deadline"
      block_key = "deadline"
      block_type = "text"
      title = "Ky han cong bo/bao cao"
      description = "Trong vong 20 ngay ke tu ngay ket thuc quy."
      config = @{ max_length = 4000 }
      validation = @{}
      display_order = 3
      enabled = $true
    },
    @{
      block_id = "blk-channels"
      block_key = "channels_and_format"
      block_type = "rich_text"
      title = "Kenh va hinh thuc cong bo/bao cao"
      description = "Trang thong tin dien tu cua Cong ty; He thong IDS cua UBCKNN; So Giao dich Chung khoan (HOSE/HNX)"
      config = @{
        max_length = 12000
        allow_html = $false
        channels = @(
          @{ id = "ch-1"; name = "Trang thong tin dien tu cua Cong ty"; format = "Dien tu"; file_type = "PDF"; attachment_required = $true },
          @{ id = "ch-2"; name = "He thong IDS cua UBCKNN"; format = "Truc tuyen"; file_type = "XML/PDF"; attachment_required = $true },
          @{ id = "ch-3"; name = "So Giao dich Chung khoan (HOSE/HNX)"; format = "Truc tuyen"; file_type = "PDF"; attachment_required = $true }
        )
      }
      validation = @{}
      display_order = 4
      enabled = $true
    },
    @{
      block_id = "blk-risks"
      block_key = "legal_risks"
      block_type = "rich_text"
      title = "Rui ro phap ly neu khong thuc hien dung"
      description = "Rui ro phap ly neu khong thuc hien dung."
      config = @{
        max_length = 8000
        allow_html = $false
        risks = @(
          @{ id = "risk-1"; title = "Cham cong bo thong tin"; impact = "Cao"; penalty = "Phat tien tu 50 - 70 trieu dong"; legal_basis = "Nghi dinh 156/2020/ND-CP" },
          @{ id = "risk-2"; title = "Noi dung cong bo khong day du"; impact = "Trung binh"; penalty = "Phat tien tu 10 - 30 trieu dong"; legal_basis = "Nghi dinh 156/2020/ND-CP" }
        )
      }
      validation = @{}
      display_order = 5
      enabled = $true
    },
    @{
      block_id = "blk-workflow"
      block_key = "enterprise_workflow"
      block_type = "rich_text"
      title = "Workflow cua doanh nghiep"
      description = "Quy trinh chuan cho mau bao cao tai chinh quy."
      config = @{
        max_length = 12000
        allow_html = $true
        steps = @(
          @{ step_id = "wf-1"; stage = "Xac dinh nghia vu"; department = "Phong Phap che"; assignee = "Tran Thi B"; due_rule = "T+1"; documents = @("Van ban thong bao su kien", "Co so phap ly") },
          @{ step_id = "wf-2"; stage = "Chuan bi du lieu"; department = "Phong Ke toan"; assignee = "Hoang Van E"; due_rule = "T+10"; documents = @("Bang can doi ke toan", "Bao cao KQKD") },
          @{ step_id = "wf-3"; stage = "Ra soat phap ly"; department = "Phong Phap che"; assignee = "Tran Thi B"; due_rule = "T+15"; documents = @("Du thao ban cong bo", "To trinh phe duyet") },
          @{ step_id = "wf-4"; stage = "Phe duyet noi bo"; department = "Ban Tong Giam doc"; assignee = "Nguyen Van A"; due_rule = "T+18"; documents = @("Ban cong bo da ky", "Nghi quyet HDQT") },
          @{ step_id = "wf-5"; stage = "Cong bo/Bao cao"; department = "Phong Quan he nha dau tu"; assignee = "Le Van C"; due_rule = "T+20"; documents = @("Xac nhan gui tin", "Anh chup website") },
          @{ step_id = "wf-6"; stage = "Luu ho so"; department = "Phong Phap che"; assignee = "Tran Thi B"; due_rule = "T+21"; documents = @("Ho so luu tru") }
        )
      }
      validation = @{}
      display_order = 6
      enabled = $true
    }
  )
  change_note = "Structured parity update via smoke script"
}

$res = Invoke-RestMethod -Method Put -Uri "$BaseUrl/api/v1/admin/disclosure-types/$TypeId" -Headers @{
  Authorization = "Bearer $token"
} -ContentType "application/json" -Body ($payload | ConvertTo-Json -Depth 30)

$res | ConvertTo-Json -Depth 10

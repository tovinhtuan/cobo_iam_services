package timeline

import "strings"

type actionMeta struct {
	Summary    string
	Domain     string
	Category   string
	ActionLink string
}

var knownActions = map[string]actionMeta{
	"login_success":                        {Summary: "Đăng nhập thành công", Domain: "identity", Category: "auth", ActionLink: ""},
	"select_company":                       {Summary: "Chuyển công ty đang làm việc", Domain: "identity", Category: "auth", ActionLink: ""},
	"select_company_failure":               {Summary: "Chuyển công ty thất bại", Domain: "identity", Category: "auth", ActionLink: ""},
	"cms_workflow.upsert":                  {Summary: "Cập nhật workflow mẫu", Domain: "configuration", Category: "cms_workflow", ActionLink: "/app/admin/audit"},
	"cms_workflow.delete":                  {Summary: "Xóa workflow mẫu", Domain: "configuration", Category: "cms_workflow", ActionLink: "/app/admin/audit"},
	"admin.user.create":                    {Summary: "Tạo người dùng", Domain: "identity", Category: "user_management", ActionLink: "/app/admin?tab=users"},
	"admin.user.invite":                    {Summary: "Mời người dùng", Domain: "identity", Category: "user_management", ActionLink: "/app/admin?tab=users"},
	"admin.user.resend_invitation":         {Summary: "Gửi lại lời mời", Domain: "identity", Category: "user_management", ActionLink: "/app/admin?tab=users"},
	"admin.membership.create":              {Summary: "Thêm thành viên công ty", Domain: "identity", Category: "membership", ActionLink: "/app/admin?tab=users"},
	"admin.membership.update":              {Summary: "Cập nhật thành viên", Domain: "identity", Category: "membership", ActionLink: "/app/admin?tab=users"},
	"admin.membership.delete":              {Summary: "Xóa thành viên", Domain: "identity", Category: "membership", ActionLink: "/app/admin?tab=users"},
	"admin.membership.role.assign":         {Summary: "Gán vai trò cho thành viên", Domain: "rbac", Category: "rbac_change", ActionLink: "/app/admin?tab=roles"},
	"admin.membership.role.remove":         {Summary: "Gỡ vai trò khỏi thành viên", Domain: "rbac", Category: "rbac_change", ActionLink: "/app/admin?tab=roles"},
	"admin.membership.department.assign":   {Summary: "Gán phòng ban cho thành viên", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.membership.department.remove":   {Summary: "Gỡ phòng ban khỏi thành viên", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.membership.title.assign":        {Summary: "Gán chức danh cho thành viên", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.membership.title.remove":        {Summary: "Gỡ chức danh khỏi thành viên", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.membership.permission.add":      {Summary: "Thêm quyền trực tiếp cho thành viên", Domain: "rbac", Category: "rbac_change", ActionLink: "/app/admin?tab=users"},
	"admin.membership.permission.remove":   {Summary: "Gỡ quyền trực tiếp khỏi thành viên", Domain: "rbac", Category: "rbac_change", ActionLink: "/app/admin?tab=users"},
	"admin.role.permission.assign":         {Summary: "Gán quyền cho vai trò", Domain: "rbac", Category: "rbac_change", ActionLink: "/app/admin?tab=roles"},
	"admin.role.permission.remove":         {Summary: "Gỡ quyền khỏi vai trò", Domain: "rbac", Category: "rbac_change", ActionLink: "/app/admin?tab=roles"},
	"admin.department.create":              {Summary: "Tạo phòng ban", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.department.update":              {Summary: "Cập nhật phòng ban", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.department.delete":              {Summary: "Xóa phòng ban", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.department.member.add":          {Summary: "Thêm thành viên vào phòng ban", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.department.member.remove":       {Summary: "Gỡ thành viên khỏi phòng ban", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.title.create":                   {Summary: "Tạo chức danh", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.title.update":                   {Summary: "Cập nhật chức danh", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.title.delete":                   {Summary: "Xóa chức danh", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.title.member.add":               {Summary: "Gán chức danh (nhóm)", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.title.member.remove":            {Summary: "Gỡ chức danh (nhóm)", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.team.create":                    {Summary: "Tạo nhóm", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.team.update":                    {Summary: "Cập nhật nhóm", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.team.delete":                    {Summary: "Xóa nhóm", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.team.member.add":                {Summary: "Thêm thành viên vào nhóm", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.team.member.remove":             {Summary: "Gỡ thành viên khỏi nhóm", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"},
	"admin.company.patch":                  {Summary: "Cập nhật hồ sơ công ty", Domain: "company", Category: "company_profile", ActionLink: "/app/admin?tab=company"},
	"admin.company.admin.assign":           {Summary: "Chỉ định quản trị viên", Domain: "company", Category: "company_admin", ActionLink: "/app/admin?tab=users"},
	"admin.company.admin.revoke":           {Summary: "Thu hồi quyền quản trị", Domain: "company", Category: "company_admin", ActionLink: "/app/admin?tab=users"},
	"admin.company.ownership.transfer":     {Summary: "Chuyển quyền sở hữu công ty", Domain: "company", Category: "company_admin", ActionLink: "/app/admin?tab=users"},
	"admin.notification_rule.update":       {Summary: "Cập nhật cấu hình cảnh báo", Domain: "notification", Category: "notification_config", ActionLink: "/app/admin?tab=notifications"},
	"admin.notification_rule.delete":       {Summary: "Xóa rule cảnh báo", Domain: "notification", Category: "notification_config", ActionLink: "/app/admin?tab=notifications"},
	"admin.notification_rule.simulate":     {Summary: "Mô phỏng cảnh báo", Domain: "notification", Category: "notification_simulate", ActionLink: "/app/admin?tab=notifications"},
	"admin.account.settings.patch":         {Summary: "Cập nhật cài đặt tài khoản", Domain: "account", Category: "account_settings", ActionLink: "/app/admin"},
	"admin.account.password_change":        {Summary: "Đổi mật khẩu", Domain: "account", Category: "account_security", ActionLink: "/app/admin"},
	"admin.config.approval.requested":      {Summary: "Yêu cầu phê duyệt cấu hình", Domain: "configuration", Category: "approval", ActionLink: "/app/admin?tab=approvals"},
	"admin.config.approval.approved":       {Summary: "Phê duyệt và áp dụng cấu hình", Domain: "configuration", Category: "approval", ActionLink: "/app/admin/audit"},
	"admin.config.approval.rejected":       {Summary: "Từ chối yêu cầu cấu hình", Domain: "configuration", Category: "approval", ActionLink: "/app/admin/audit"},
	"admin.config.approval.cancelled":      {Summary: "Hủy yêu cầu phê duyệt", Domain: "configuration", Category: "approval", ActionLink: "/app/admin/audit"},
	"config.export.requested":              {Summary: "Xuất cấu hình doanh nghiệp", Domain: "configuration", Category: "export", ActionLink: "/app/admin"},
	"delegated.admin.granted":              {Summary: "Ủy quyền quản trị theo phạm vi", Domain: "delegation", Category: "delegation", ActionLink: "/app/admin?tab=staff"},
	"delegated.admin.revoked":              {Summary: "Thu hồi ủy quyền quản trị", Domain: "delegation", Category: "delegation", ActionLink: "/app/admin?tab=staff"},
	"delegated.admin.updated":              {Summary: "Cập nhật ủy quyền quản trị", Domain: "delegation", Category: "delegation", ActionLink: "/app/admin?tab=staff"},
	"breakglass.session.created":           {Summary: "Yêu cầu truy cập khẩn cấp", Domain: "break_glass", Category: "emergency_access", ActionLink: "/app/admin?tab=emergency"},
	"breakglass.session.approved":          {Summary: "Phê duyệt truy cập khẩn cấp", Domain: "break_glass", Category: "emergency_access", ActionLink: "/app/admin?tab=emergency"},
	"breakglass.session.activated":         {Summary: "Kích hoạt truy cập khẩn cấp", Domain: "break_glass", Category: "emergency_access", ActionLink: "/app/admin?tab=emergency"},
	"breakglass.session.used":              {Summary: "Sử dụng quyền truy cập khẩn cấp", Domain: "break_glass", Category: "emergency_access", ActionLink: "/app/admin?tab=emergency"},
	"breakglass.session.expired":           {Summary: "Hết hạn truy cập khẩn cấp", Domain: "break_glass", Category: "emergency_access", ActionLink: "/app/admin?tab=emergency"},
	"breakglass.session.revoked":           {Summary: "Thu hồi truy cập khẩn cấp", Domain: "break_glass", Category: "emergency_access", ActionLink: "/app/admin?tab=emergency"},
	"breakglass.session.denied":            {Summary: "Từ chối truy cập khẩn cấp", Domain: "break_glass", Category: "emergency_access", ActionLink: "/app/admin?tab=emergency"},
}

func lookupAction(action string) actionMeta {
	action = strings.TrimSpace(action)
	if m, ok := knownActions[action]; ok {
		return m
	}
	// Prefix heuristics for uncatalogued but common families.
	switch {
	case strings.HasPrefix(action, "login"):
		return actionMeta{Summary: "Đăng nhập", Domain: "identity", Category: "auth", ActionLink: ""}
	case strings.Contains(action, "select_company"):
		return actionMeta{Summary: "Chuyển công ty đang làm việc", Domain: "identity", Category: "auth", ActionLink: ""}
	case strings.Contains(action, "cms_workflow") || strings.Contains(action, "workflow"):
		return actionMeta{Summary: "Cập nhật workflow mẫu", Domain: "configuration", Category: "cms_workflow", ActionLink: "/app/admin/audit"}
	case strings.Contains(action, "department"):
		return actionMeta{Summary: "Thao tác phòng ban", Domain: "org", Category: "org_structure", ActionLink: "/app/admin?tab=org"}
	}
	return actionMeta{
		Summary:    "Hoạt động hệ thống",
		Domain:     "configuration",
		Category:   "configuration_change",
		ActionLink: "/app/admin/audit",
	}
}

var resourceTypeLabels = map[string]string{
	"disclosure_type": "loại thông tin báo cáo",
	"department":      "phòng ban",
	"company":         "doanh nghiệp",
	"user":            "người dùng",
	"membership":      "thành viên",
	"role":            "vai trò",
	"title":           "chức danh",
	"team":            "nhóm",
	"workflow":        "workflow mẫu",
}

// ResourceTypeLabel returns a short Vietnamese label for a resource type code.
func ResourceTypeLabel(resourceType string) string {
	key := strings.TrimSpace(resourceType)
	if key == "" {
		return ""
	}
	if label, ok := resourceTypeLabels[key]; ok {
		return label
	}
	if label, ok := resourceTypeLabels[strings.ToLower(key)]; ok {
		return label
	}
	return ""
}

// FriendlyDescription builds a user-facing description without raw UUIDs/codes.
func FriendlyDescription(action, resourceType string) string {
	summary := SummaryForAction(action)
	rt := ResourceTypeLabel(resourceType)
	if rt != "" {
		return "Bạn đã thực hiện: " + strings.ToLower(summary) + " liên quan đến " + rt + "."
	}
	return "Bạn đã thực hiện: " + strings.ToLower(summary) + "."
}

// DomainLabel returns a Vietnamese domain label when known.
func DomainLabel(domain string) string {
	switch strings.TrimSpace(domain) {
	case "identity":
		return "Người dùng & tài khoản"
	case "configuration":
		return "Cấu hình"
	case "org":
		return "Cơ cấu tổ chức"
	case "disclosure":
		return "Công bố thông tin"
	case "workflow":
		return "Workflow"
	case "rbac":
		return "Phân quyền"
	case "notification":
		return "Cảnh báo"
	case "company":
		return "Doanh nghiệp"
	case "account":
		return "Tài khoản"
	default:
		return strings.TrimSpace(domain)
	}
}

// SummaryForAction returns a human-readable summary for an audit action code.
func SummaryForAction(action string) string {
	return lookupAction(strings.TrimSpace(action)).Summary
}

// DomainForAction returns the domain code for an audit action.
func DomainForAction(action string) string {
	return lookupAction(strings.TrimSpace(action)).Domain
}

// ActionLinkFor returns a deep link for an audit action when known.
func ActionLinkFor(action string) string {
	return lookupAction(strings.TrimSpace(action)).ActionLink
}

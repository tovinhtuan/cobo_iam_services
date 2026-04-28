package app

var disclosureTypeGroups = []DisclosureGroupDTO{
	{
		GroupID:      "group-001",
		Name:         "Định kỳ",
		Description:  "Nghĩa vụ công bố theo chu kỳ.",
		Icon:         "Calendar",
		DisplayOrder: 1,
	},
	{
		GroupID:      "group-002",
		Name:         "Bất thường",
		Description:  "Nghĩa vụ công bố theo sự kiện phát sinh.",
		Icon:         "AlertCircle",
		DisplayOrder: 2,
	},
	{
		GroupID:      "group-006",
		Name:         "Tùy chỉnh doanh nghiệp",
		Description:  "Template nội bộ do doanh nghiệp cấu hình.",
		Icon:         "Settings",
		DisplayOrder: 3,
	},
}

var disclosureTypeCatalog = []DisclosureTypeDTO{
	{
		TypeID:                "dt-periodic-financial",
		GroupID:               "group-001",
		Name:                  "Báo cáo tài chính định kỳ",
		Category:              "Định kỳ",
		TemplateCategory:      "Định kỳ",
		Description:           "Công bố báo cáo tài chính theo quý.",
		LegalBasis:            "Thông tư 96/2020/TT-BTC",
		Applicability:         "Doanh nghiệp niêm yết",
		ImplementationContent: "Tổng hợp dữ liệu và lập báo cáo tài chính quý.",
		ImplementationNotes:   "Kiểm tra số liệu trước khi nộp.",
		SpecialCases:          "Điều chỉnh khi có kiểm toán ngoại lệ.",
		ReportContent:         "Báo cáo tài chính quý + thuyết minh.",
		RequiredDocs:          "BCTC quý, biên bản phê duyệt.",
		DeadlineRule:          "Trong vòng 20 ngày kể từ khi kết thúc quý.",
		Periodicity:           "Hàng quý",
		ChannelsText:          "UBCKNN, Sở GDCK",
		Beneficiaries:         "Nhà đầu tư, cơ quan quản lý",
		ReceivingAuthorities:  "UBCKNN, HOSE/HNX",
		Format:                "PDF",
		LegalRisksText:        "Phạt chậm công bố theo quy định hiện hành.",
		GeneralInfo:           "Áp dụng mặc định cho tất cả công ty niêm yết.",
		Tags:                  []string{"Định kỳ", "Tài chính"},
	},
	{
		TypeID:                "dt-event-major-change",
		GroupID:               "group-002",
		Name:                  "Công bố sự kiện bất thường",
		Category:              "Bất thường",
		TemplateCategory:      "Bất thường",
		Description:           "Công bố khi phát sinh sự kiện quan trọng.",
		LegalBasis:            "Luật chứng khoán hiện hành",
		Applicability:         "Doanh nghiệp niêm yết",
		ImplementationContent: "Tạo cảnh báo và công bố theo sự kiện.",
		ImplementationNotes:   "Ghi nhận thời điểm phát sinh chính xác.",
		SpecialCases:          "Sự kiện có yếu tố bảo mật nội bộ.",
		ReportContent:         "Thông báo sự kiện và tài liệu đính kèm.",
		RequiredDocs:          "Thông báo sự kiện, tài liệu pháp lý liên quan.",
		DeadlineRule:          "Trong vòng 24 giờ kể từ khi phát sinh sự kiện.",
		Periodicity:           "Bất thường",
		ChannelsText:          "UBCKNN, Website công ty",
		Beneficiaries:         "Nhà đầu tư, cổ đông",
		ReceivingAuthorities:  "UBCKNN",
		Format:                "PDF",
		LegalRisksText:        "Rủi ro xử phạt nếu chậm hoặc sai nội dung công bố.",
		GeneralInfo:           "Deadline xác định theo thời điểm sự kiện.",
		Tags:                  []string{"Bất thường", "Sự kiện"},
	},
	{
		TypeID:                "dt-custom-obligation",
		GroupID:               "group-006",
		Name:                  "Template nghĩa vụ tùy chỉnh",
		Category:              "Tùy chỉnh",
		TemplateCategory:      "Định kỳ",
		Description:           "Template nội bộ cho nghĩa vụ đặc thù doanh nghiệp.",
		LegalBasis:            "Quy chế nội bộ doanh nghiệp",
		Applicability:         "Theo phân quyền nội bộ",
		ImplementationContent: "Thiết lập workflow và checklist tài liệu.",
		ImplementationNotes:   "Điều chỉnh theo chính sách nội bộ.",
		SpecialCases:          "Thay đổi quy trình theo phiên bản template.",
		ReportContent:         "Nội dung theo template nội bộ.",
		RequiredDocs:          "Checklist hồ sơ theo cấu hình template.",
		DeadlineRule:          "Theo cấu hình của admin.",
		Periodicity:           "Hàng tháng",
		ChannelsText:          "Nội bộ",
		Beneficiaries:         "Ban điều hành",
		ReceivingAuthorities:  "Nội bộ",
		Format:                "PDF",
		LegalRisksText:        "Rủi ro tuân thủ nội bộ nếu không thực hiện đúng quy trình.",
		GeneralInfo:           "Dùng cho nghĩa vụ nội bộ chưa có mẫu chuẩn ngoài thị trường.",
		Tags:                  []string{"Tùy chỉnh"},
	},
}

func SeedDisclosureTypeGroups() []DisclosureGroupDTO {
	out := make([]DisclosureGroupDTO, len(disclosureTypeGroups))
	copy(out, disclosureTypeGroups)
	return out
}

func SeedDisclosureTypeCatalog() []DisclosureTypeDTO {
	out := make([]DisclosureTypeDTO, len(disclosureTypeCatalog))
	copy(out, disclosureTypeCatalog)
	return out
}

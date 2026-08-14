package app

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

const (
	SubscriptionUpgradeQRMaxBytes  int64 = 2 * 1024 * 1024
	subscriptionUpgradeQRObjectKey       = "platform/subscription-upgrade/qr"
)

var subscriptionUpgradeQRAllowedTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/webp": ".webp",
}

// SubscriptionUpgradePaymentDTO is the CMS/config payload.
type SubscriptionUpgradePaymentDTO struct {
	DescriptionVI        string     `json:"description_vi"`
	DescriptionEN        string     `json:"description_en"`
	Hotline              string     `json:"hotline"`
	BankName             string     `json:"bank_name"`
	AccountName          string     `json:"account_name"`
	AccountNumber        string     `json:"account_number"`
	TransferNoteTemplate string     `json:"transfer_note_template"`
	IsActive             bool       `json:"is_active"`
	QRConfigured         bool       `json:"qr_configured"`
	QRContentType        string     `json:"qr_content_type,omitempty"`
	QRFileName           string     `json:"qr_file_name,omitempty"`
	UpdatedAt            *time.Time `json:"updated_at,omitempty"`
	UpdatedBy            string     `json:"updated_by,omitempty"`
}

// UpdateSubscriptionUpgradePaymentRequest updates text fields (not the QR binary).
type UpdateSubscriptionUpgradePaymentRequest struct {
	DescriptionVI        string
	DescriptionEN        string
	Hotline              string
	BankName             string
	AccountName          string
	AccountNumber        string
	TransferNoteTemplate string
	IsActive             bool
	ActorID              string
}

// PortalSubscriptionUpgradeInstruction is returned to Portal Admin.
type PortalSubscriptionUpgradeInstruction struct {
	CurrentTier       string `json:"current_tier"`
	CurrentTierLabel  string `json:"current_tier_label"`
	IsConfigured      bool   `json:"is_configured"`
	Message           string `json:"message,omitempty"`
	QRURL             string `json:"qr_url,omitempty"`
	Description       string `json:"description,omitempty"`
	Hotline           string `json:"hotline,omitempty"`
	BankName          string `json:"bank_name,omitempty"`
	AccountName       string `json:"account_name,omitempty"`
	AccountNumber     string `json:"account_number,omitempty"`
	TransferNote      string `json:"transfer_note,omitempty"`
	LandingPricingURL string `json:"landing_pricing_url,omitempty"`
	ManualNote        string `json:"manual_note,omitempty"`
}

type SubscriptionUpgradePaymentRecord struct {
	DescriptionVI        string
	DescriptionEN        string
	Hotline              string
	BankName             string
	AccountName          string
	AccountNumber        string
	TransferNoteTemplate string
	IsActive             bool
	QRObjectKey          sql.NullString
	QRContentType        sql.NullString
	QRFileName           sql.NullString
	UpdatedBy            sql.NullString
	UpdatedAt            time.Time
}

type SubscriptionUpgradePaymentRepository interface {
	Get(ctx context.Context) (*SubscriptionUpgradePaymentRecord, error)
	UpsertFields(ctx context.Context, req UpdateSubscriptionUpgradePaymentRequest) error
	SetQR(ctx context.Context, objectKey, contentType, fileName, actorID string) error
	ClearQR(ctx context.Context, actorID string) error
}

type SubscriptionUpgradeObjectStorage interface {
	Write(objectKey string, body io.Reader) (int64, error)
	Read(objectKey string) ([]byte, error)
	Delete(objectKey string) error
	Exists(objectKey string) bool
}

type SubscriptionUpgradePaymentService interface {
	GetCMS(ctx context.Context) (*SubscriptionUpgradePaymentDTO, error)
	UpdateCMS(ctx context.Context, req UpdateSubscriptionUpgradePaymentRequest) (*SubscriptionUpgradePaymentDTO, error)
	UploadQR(ctx context.Context, actorID, contentType, fileName string, body io.Reader, size int64) (*SubscriptionUpgradePaymentDTO, error)
	DeleteQR(ctx context.Context, actorID string) (*SubscriptionUpgradePaymentDTO, error)
	ReadQR(ctx context.Context) (contentType string, fileName string, data []byte, err error)
	PortalInstruction(ctx context.Context, lang, currentTier, companyCode, qrPublicPath string) (*PortalSubscriptionUpgradeInstruction, error)
}

type subscriptionUpgradePaymentService struct {
	repo    SubscriptionUpgradePaymentRepository
	storage SubscriptionUpgradeObjectStorage
}

func NewSubscriptionUpgradePaymentService(repo SubscriptionUpgradePaymentRepository, storage SubscriptionUpgradeObjectStorage) SubscriptionUpgradePaymentService {
	return &subscriptionUpgradePaymentService{repo: repo, storage: storage}
}

func (s *subscriptionUpgradePaymentService) GetCMS(ctx context.Context) (*SubscriptionUpgradePaymentDTO, error) {
	rec, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	return mapSubscriptionUpgradeDTO(rec), nil
}

func (s *subscriptionUpgradePaymentService) UpdateCMS(ctx context.Context, req UpdateSubscriptionUpgradePaymentRequest) (*SubscriptionUpgradePaymentDTO, error) {
	req.DescriptionVI = strings.TrimSpace(req.DescriptionVI)
	req.DescriptionEN = strings.TrimSpace(req.DescriptionEN)
	req.Hotline = strings.TrimSpace(req.Hotline)
	req.BankName = strings.TrimSpace(req.BankName)
	req.AccountName = strings.TrimSpace(req.AccountName)
	req.AccountNumber = strings.TrimSpace(req.AccountNumber)
	req.TransferNoteTemplate = strings.TrimSpace(req.TransferNoteTemplate)
	if req.TransferNoteTemplate == "" {
		req.TransferNoteTemplate = "COBO {{company_code}} NANGCAPGOI"
	}
	if utf8.RuneCountInString(req.DescriptionVI) > 4000 || utf8.RuneCountInString(req.DescriptionEN) > 4000 {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "description too long", nil)
	}
	if req.IsActive {
		rec, err := s.repo.Get(ctx)
		if err != nil {
			return nil, err
		}
		if !rec.QRObjectKey.Valid || strings.TrimSpace(rec.QRObjectKey.String) == "" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "qr image required when active", nil)
		}
		if req.Hotline == "" {
			return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "hotline required when active", nil)
		}
	}
	if err := s.repo.UpsertFields(ctx, req); err != nil {
		return nil, err
	}
	return s.GetCMS(ctx)
}

func (s *subscriptionUpgradePaymentService) UploadQR(ctx context.Context, actorID, contentType, fileName string, body io.Reader, size int64) (*SubscriptionUpgradePaymentDTO, error) {
	if size <= 0 || size > SubscriptionUpgradeQRMaxBytes {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "qr file size must be > 0 and <= 2MB", nil)
	}
	sniffed, rest, err := sniffQRImageType(body)
	if err != nil {
		return nil, err
	}
	ext, ok := subscriptionUpgradeQRAllowedTypes[sniffed]
	if !ok {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported content_type (png/jpeg/webp)", nil)
	}
	fileName = sanitizeSubscriptionUpgradeFileName(fileName, ext)
	objectKey := subscriptionUpgradeQRObjectKey + ext
	if s.storage == nil {
		return nil, perr.NewHTTPError(http.StatusServiceUnavailable, perr.CodeServiceUnavailable, "qr storage unavailable", nil)
	}
	written, err := s.storage.Write(objectKey, io.LimitReader(rest, size+1))
	if err != nil {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to store qr", err)
	}
	if written > SubscriptionUpgradeQRMaxBytes {
		_ = s.storage.Delete(objectKey)
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "qr file size must be <= 2MB", nil)
	}
	if err := s.repo.SetQR(ctx, objectKey, sniffed, fileName, actorID); err != nil {
		_ = s.storage.Delete(objectKey)
		return nil, err
	}
	return s.GetCMS(ctx)
}

func sniffQRImageType(body io.Reader) (string, io.Reader, error) {
	buf := make([]byte, 512)
	n, err := io.ReadFull(body, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unable to read qr file", err)
	}
	buf = buf[:n]
	if n == 0 {
		return "", nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported content_type (png/jpeg/webp)", nil)
	}
	detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(buf), ";")[0]))
	if detected == "image/jpg" {
		detected = "image/jpeg"
	}
	if _, ok := subscriptionUpgradeQRAllowedTypes[detected]; !ok {
		return "", nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "unsupported content_type (png/jpeg/webp)", nil)
	}
	return detected, io.MultiReader(bytes.NewReader(buf), body), nil
}

func (s *subscriptionUpgradePaymentService) DeleteQR(ctx context.Context, actorID string) (*SubscriptionUpgradePaymentDTO, error) {
	rec, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	if rec.IsActive {
		return nil, perr.NewHTTPError(http.StatusBadRequest, perr.CodeInvalidRequest, "deactivate config before deleting qr", nil)
	}
	if rec.QRObjectKey.Valid && s.storage != nil {
		_ = s.storage.Delete(rec.QRObjectKey.String)
	}
	if err := s.repo.ClearQR(ctx, actorID); err != nil {
		return nil, err
	}
	return s.GetCMS(ctx)
}

func (s *subscriptionUpgradePaymentService) ReadQR(ctx context.Context) (string, string, []byte, error) {
	rec, err := s.repo.Get(ctx)
	if err != nil {
		return "", "", nil, err
	}
	if !rec.QRObjectKey.Valid || strings.TrimSpace(rec.QRObjectKey.String) == "" {
		return "", "", nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "qr not configured", nil)
	}
	if s.storage == nil || !s.storage.Exists(rec.QRObjectKey.String) {
		return "", "", nil, perr.NewHTTPError(http.StatusNotFound, perr.CodeNotFound, "qr file missing", nil)
	}
	data, err := s.storage.Read(rec.QRObjectKey.String)
	if err != nil {
		return "", "", nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "failed to read qr", err)
	}
	ct := "image/png"
	if rec.QRContentType.Valid && rec.QRContentType.String != "" {
		ct = rec.QRContentType.String
	}
	fn := "qr.png"
	if rec.QRFileName.Valid && rec.QRFileName.String != "" {
		fn = rec.QRFileName.String
	}
	return ct, fn, data, nil
}

func (s *subscriptionUpgradePaymentService) PortalInstruction(ctx context.Context, lang, currentTier, companyCode, qrPublicPath string) (*PortalSubscriptionUpgradeInstruction, error) {
	rec, err := s.repo.Get(ctx)
	if err != nil {
		return nil, err
	}
	tier := normalizeTierLabel(currentTier)
	out := &PortalSubscriptionUpgradeInstruction{
		CurrentTier:       strings.ToLower(tier),
		CurrentTierLabel:  tier,
		LandingPricingURL: "/pricing",
		ManualNote:        "Sau khi chuyển khoản, gói sẽ được kích hoạt sau khi quản trị nền tảng xác nhận thanh toán.",
	}
	if lang == "en" {
		out.ManualNote = "After the transfer, the package is activated only after platform admin confirms payment."
	}
	configured := rec.IsActive && rec.QRObjectKey.Valid && strings.TrimSpace(rec.QRObjectKey.String) != ""
	if !configured {
		out.IsConfigured = false
		if lang == "en" {
			out.Message = "Bank transfer payment is currently unavailable. Please contact platform administration."
		} else {
			out.Message = "Thanh toán chuyển khoản hiện chưa khả dụng. Vui lòng liên hệ quản trị nền tảng."
		}
		return out, nil
	}
	out.IsConfigured = true
	out.QRURL = qrPublicPath
	out.Hotline = rec.Hotline
	out.BankName = rec.BankName
	out.AccountName = rec.AccountName
	out.AccountNumber = rec.AccountNumber
	desc := rec.DescriptionVI
	if lang == "en" && strings.TrimSpace(rec.DescriptionEN) != "" {
		desc = rec.DescriptionEN
	}
	out.Description = desc
	tmpl := rec.TransferNoteTemplate
	if tmpl == "" {
		tmpl = "COBO {{company_code}} NANGCAPGOI"
	}
	code := strings.TrimSpace(companyCode)
	if code == "" {
		code = "COMPANY"
	}
	out.TransferNote = renderTransferNote(tmpl, code)
	return out, nil
}

func mapSubscriptionUpgradeDTO(rec *SubscriptionUpgradePaymentRecord) *SubscriptionUpgradePaymentDTO {
	if rec == nil {
		return &SubscriptionUpgradePaymentDTO{TransferNoteTemplate: "COBO {{company_code}} NANGCAPGOI"}
	}
	dto := &SubscriptionUpgradePaymentDTO{
		DescriptionVI:        rec.DescriptionVI,
		DescriptionEN:        rec.DescriptionEN,
		Hotline:              rec.Hotline,
		BankName:             rec.BankName,
		AccountName:          rec.AccountName,
		AccountNumber:        rec.AccountNumber,
		TransferNoteTemplate: rec.TransferNoteTemplate,
		IsActive:             rec.IsActive,
		QRConfigured:         rec.QRObjectKey.Valid && strings.TrimSpace(rec.QRObjectKey.String) != "",
	}
	if rec.QRContentType.Valid {
		dto.QRContentType = rec.QRContentType.String
	}
	if rec.QRFileName.Valid {
		dto.QRFileName = rec.QRFileName.String
	}
	if rec.UpdatedBy.Valid {
		dto.UpdatedBy = rec.UpdatedBy.String
	}
	if !rec.UpdatedAt.IsZero() {
		t := rec.UpdatedAt.UTC()
		dto.UpdatedAt = &t
	}
	return dto
}

func renderTransferNote(tmpl, companyCode string) string {
	out := tmpl
	out = strings.ReplaceAll(out, "{{company_code}}", companyCode)
	out = strings.ReplaceAll(out, "{company_code}", companyCode)
	return out
}

func normalizeTierLabel(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "premium":
		return "Premium"
	case "enterprise":
		return "Enterprise"
	default:
		return "Free"
	}
}

func sanitizeSubscriptionUpgradeFileName(name, fallbackExt string) string {
	name = path.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "..", "")
	if name == "" || name == "." || name == "/" {
		return "qr" + fallbackExt
	}
	if !strings.Contains(name, ".") {
		return name + fallbackExt
	}
	return name
}

// EnsureEmptyRecord helps tests when table has no row.
func EnsureEmptyRecord() *SubscriptionUpgradePaymentRecord {
	return &SubscriptionUpgradePaymentRecord{
		TransferNoteTemplate: "COBO {{company_code}} NANGCAPGOI",
		IsActive:             false,
	}
}

var ErrSubscriptionUpgradeNotFound = errors.New("subscription upgrade payment not found")

func FormatSubscriptionUpgradeObjectKey(ext string) string {
	return fmt.Sprintf("%s%s", subscriptionUpgradeQRObjectKey, ext)
}

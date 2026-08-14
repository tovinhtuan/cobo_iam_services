package app_test

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	platformcmsapp "github.com/cobo/cobo_iam_services/internal/platformcms/app"
)

type memUpgradeRepo struct {
	mu  sync.Mutex
	rec *platformcmsapp.SubscriptionUpgradePaymentRecord
}

func (m *memUpgradeRepo) Get(context.Context) (*platformcmsapp.SubscriptionUpgradePaymentRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rec == nil {
		return platformcmsapp.EnsureEmptyRecord(), nil
	}
	cp := *m.rec
	return &cp, nil
}

func (m *memUpgradeRepo) UpsertFields(_ context.Context, req platformcmsapp.UpdateSubscriptionUpgradePaymentRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rec == nil {
		m.rec = platformcmsapp.EnsureEmptyRecord()
	}
	m.rec.DescriptionVI = req.DescriptionVI
	m.rec.DescriptionEN = req.DescriptionEN
	m.rec.Hotline = req.Hotline
	m.rec.BankName = req.BankName
	m.rec.AccountName = req.AccountName
	m.rec.AccountNumber = req.AccountNumber
	m.rec.TransferNoteTemplate = req.TransferNoteTemplate
	m.rec.IsActive = req.IsActive
	m.rec.UpdatedBy = sql.NullString{String: req.ActorID, Valid: req.ActorID != ""}
	m.rec.UpdatedAt = time.Now().UTC()
	return nil
}

func (m *memUpgradeRepo) SetQR(_ context.Context, objectKey, contentType, fileName, actorID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rec == nil {
		m.rec = platformcmsapp.EnsureEmptyRecord()
	}
	m.rec.QRObjectKey = sql.NullString{String: objectKey, Valid: true}
	m.rec.QRContentType = sql.NullString{String: contentType, Valid: true}
	m.rec.QRFileName = sql.NullString{String: fileName, Valid: true}
	m.rec.UpdatedBy = sql.NullString{String: actorID, Valid: actorID != ""}
	return nil
}

func (m *memUpgradeRepo) ClearQR(_ context.Context, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rec == nil {
		return nil
	}
	m.rec.QRObjectKey = sql.NullString{}
	m.rec.QRContentType = sql.NullString{}
	m.rec.QRFileName = sql.NullString{}
	return nil
}

type bytesStorage struct {
	mu   sync.Mutex
	data map[string][]byte
}

func (s *bytesStorage) Write(objectKey string, body io.Reader) (int64, error) {
	b, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		s.data = map[string][]byte{}
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	s.data[objectKey] = cp
	return int64(len(cp)), nil
}

func (s *bytesStorage) Read(objectKey string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.data[objectKey]
	if !ok {
		return nil, io.EOF
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp, nil
}

func (s *bytesStorage) Delete(objectKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, objectKey)
	return nil
}

func (s *bytesStorage) Exists(objectKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[objectKey]
	return ok
}

func TestSubscriptionUpgradePayment_PortalInstructionAndActiveGuard(t *testing.T) {
	repo := &memUpgradeRepo{}
	store := &bytesStorage{}
	svc := platformcmsapp.NewSubscriptionUpgradePaymentService(repo, store)

	_, err := svc.UpdateCMS(context.Background(), platformcmsapp.UpdateSubscriptionUpgradePaymentRequest{
		Hotline:  "19001234",
		IsActive: true,
		ActorID:  "u1",
	})
	if err == nil {
		t.Fatal("expected error when activating without QR")
	}

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if _, err := svc.UploadQR(context.Background(), "u1", "image/png", "qr.png", bytes.NewReader(png), int64(len(png))); err != nil {
		t.Fatalf("upload qr: %v", err)
	}
	if _, err := svc.UpdateCMS(context.Background(), platformcmsapp.UpdateSubscriptionUpgradePaymentRequest{
		DescriptionVI: "Quét QR rồi gọi hotline",
		Hotline:       "19001234",
		BankName:      "VCB",
		AccountName:   "COBO",
		AccountNumber: "123",
		IsActive:      true,
		ActorID:       "u1",
	}); err != nil {
		t.Fatalf("activate: %v", err)
	}

	out, err := svc.PortalInstruction(context.Background(), "vi", "Premium", "CTP_TEST", "/api/v1/admin/subscription-upgrade/qr")
	if err != nil {
		t.Fatalf("portal: %v", err)
	}
	if !out.IsConfigured || out.QRURL == "" || out.Hotline != "19001234" {
		t.Fatalf("unexpected portal payload: %+v", out)
	}
	if out.CurrentTierLabel != "Premium" {
		t.Fatalf("tier: %+v", out)
	}
	if out.TransferNote != "COBO CTP_TEST NANGCAPGOI" {
		t.Fatalf("note=%q", out.TransferNote)
	}

	emptyCode, err := svc.PortalInstruction(context.Background(), "vi", "Premium", "", "/qr")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(emptyCode.TransferNote, "COMPANY") {
		t.Fatalf("empty company_code uses COMPANY placeholder, got %q", emptyCode.TransferNote)
	}

	if _, err := svc.UploadQR(context.Background(), "u1", "image/png", "evil.png", bytes.NewReader([]byte("<svg xmlns='x'></svg>")), 24); err == nil {
		t.Fatal("non-image bytes must be rejected")
	}

	if _, err := svc.UpdateCMS(context.Background(), platformcmsapp.UpdateSubscriptionUpgradePaymentRequest{
		Hotline:  "19001234",
		IsActive: false,
		ActorID:  "u1",
	}); err != nil {
		t.Fatal(err)
	}
	inactive, err := svc.PortalInstruction(context.Background(), "vi", "Free", "X", "/qr")
	if err != nil {
		t.Fatal(err)
	}
	if inactive.IsConfigured {
		t.Fatalf("expected unconfigured when inactive: %+v", inactive)
	}
	if !strings.Contains(inactive.Message, "chưa khả dụng") && !strings.Contains(inactive.Message, "liên hệ quản trị") {
		t.Fatalf("safe unconfigured message, got %q", inactive.Message)
	}
}

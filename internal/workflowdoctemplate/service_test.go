package workflowdoctemplate_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/cobo/cobo_iam_services/internal/platform/mediaupload"
	wdt "github.com/cobo/cobo_iam_services/internal/workflowdoctemplate"
	wdtinmem "github.com/cobo/cobo_iam_services/internal/workflowdoctemplate/infra/inmemory"
)

func newTestService(t *testing.T) *wdt.Service {
	t.Helper()
	disk, err := mediaupload.NewDiskStorage(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return wdt.NewService(wdtinmem.NewRepository(), disk)
}

func TestUploadXLSXAndRead(t *testing.T) {
	svc := newTestService(t)
	body := []byte("fake-xlsx-bytes")
	res, err := svc.UploadMultipart(context.Background(), wdt.OwnerScopeCompany, "co_a", "user_1",
		"form-q3.xlsx", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.FileID, "wdt_") {
		t.Fatalf("file_id=%q", res.FileID)
	}
	asset, data, err := svc.ReadContent(context.Background(), res.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if asset.FileName != "form-q3.xlsx" || string(data) != string(body) {
		t.Fatalf("mismatch name=%q data=%q", asset.FileName, data)
	}
}

func TestUploadRejectsUnsupportedMIME(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UploadMultipart(context.Background(), wdt.OwnerScopeCMS, "co_cms", "user_1",
		"evil.exe", "application/octet-stream", bytes.NewReader([]byte("x")), 1)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUploadRejectsMIMEExtensionMismatch(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UploadMultipart(context.Background(), wdt.OwnerScopeCMS, "co_cms", "user_1",
		"form.xlsx", "application/pdf", bytes.NewReader([]byte("%PDF")), 4)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestUploadRejectsOversize(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.UploadMultipart(context.Background(), wdt.OwnerScopeCompany, "co_a", "user_1",
		"big.pdf", "application/pdf", bytes.NewReader([]byte("x")), wdt.MaxSizeBytes+1)
	if err == nil {
		t.Fatal("expected oversize error")
	}
}

func TestReplaceCreatesNewFileID(t *testing.T) {
	svc := newTestService(t)
	a, err := svc.UploadMultipart(context.Background(), wdt.OwnerScopeCompany, "co_a", "user_1",
		"v1.pdf", "application/pdf", bytes.NewReader([]byte("v1")), 2)
	if err != nil {
		t.Fatal(err)
	}
	b, err := svc.UploadMultipart(context.Background(), wdt.OwnerScopeCompany, "co_a", "user_1",
		"v2.pdf", "application/pdf", bytes.NewReader([]byte("v2")), 2)
	if err != nil {
		t.Fatal(err)
	}
	if a.FileID == b.FileID {
		t.Fatal("replace must create new file_id")
	}
	_, data1, err := svc.ReadContent(context.Background(), a.FileID)
	if err != nil || string(data1) != "v1" {
		t.Fatalf("old file must remain readable: %v %q", err, data1)
	}
}

func TestCrossCompanyBindDenied(t *testing.T) {
	svc := newTestService(t)
	res, err := svc.UploadMultipart(context.Background(), wdt.OwnerScopeCompany, "co_a", "user_1",
		"a.pdf", "application/pdf", bytes.NewReader([]byte("a")), 1)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.AssertCanBind(context.Background(), res.FileID, wdt.OwnerScopeCompany, "co_b")
	if err == nil {
		t.Fatal("expected cross-company bind deny")
	}
}

func TestCompanyCanBindCMSFile(t *testing.T) {
	svc := newTestService(t)
	res, err := svc.UploadMultipart(context.Background(), wdt.OwnerScopeCMS, "co_cms", "user_1",
		"cms.pdf", "application/pdf", bytes.NewReader([]byte("cms")), 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AssertCanBind(context.Background(), res.FileID, wdt.OwnerScopeCompany, "co_a"); err != nil {
		t.Fatal(err)
	}
}

func TestCanDownloadACL(t *testing.T) {
	cms := &wdt.Asset{OwnerScope: wdt.OwnerScopeCMS, CompanyID: "co_cms", State: wdt.StateReady}
	coA := &wdt.Asset{OwnerScope: wdt.OwnerScopeCompany, CompanyID: "co_a", State: wdt.StateReady}
	if !wdt.CanDownload(cms, "co_b", false) {
		t.Fatal("company should download CMS template file")
	}
	if wdt.CanDownload(coA, "co_b", false) {
		t.Fatal("company B must not download company A file")
	}
	if !wdt.CanDownload(coA, "co_a", false) {
		t.Fatal("owner company should download")
	}
	if !wdt.CanDownload(coA, "co_b", true) {
		t.Fatal("CMS admin can download")
	}
}

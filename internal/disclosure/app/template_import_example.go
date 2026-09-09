package app

import (
	"context"
	_ "embed"
	"net/http"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

//go:embed artifacts/cobo-template-import-example-v1.0.json
var canonicalTemplateImportExampleV1JSON []byte

const (
	// TemplateImportExampleFilename is the server-owned download filename (never user-influenced).
	TemplateImportExampleFilename = "cobo-template-import-example-v1.0.json"
	// TemplateImportExampleContentType is the fixed response Content-Type.
	TemplateImportExampleContentType = "application/json"
)

// CanonicalTemplateImportExampleV1 returns the embedded schema_version=1.0 example artifact bytes.
// Zero DB access; immutable content for CMS admin reference / Import basis.
func CanonicalTemplateImportExampleV1() []byte {
	out := make([]byte, len(canonicalTemplateImportExampleV1JSON))
	copy(out, canonicalTemplateImportExampleV1JSON)
	return out
}

// GetTemplateImportExampleRequest is the auth-gated download request (no body).
type GetTemplateImportExampleRequest struct {
	Subject Subject
}

// GetTemplateImportExampleResponse carries the downloadable bytes and static headers metadata.
type GetTemplateImportExampleResponse struct {
	Filename    string
	ContentType string
	Payload     []byte
}

// GetTemplateImportExample returns the canonical Import example JSON for CMS authors.
// Authorization matches Import Validate/Confirm: platform.cms.view + cms.template.write.
// EXAMPLE_DOWNLOAD_DB_WRITE_COUNT=0.
func (s *service) GetTemplateImportExample(ctx context.Context, req GetTemplateImportExampleRequest) (*GetTemplateImportExampleResponse, error) {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}
	payload := CanonicalTemplateImportExampleV1()
	if len(payload) == 0 {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "canonical import example artifact is empty", nil)
	}
	return &GetTemplateImportExampleResponse{
		Filename:    TemplateImportExampleFilename,
		ContentType: TemplateImportExampleContentType,
		Payload:     payload,
	}, nil
}

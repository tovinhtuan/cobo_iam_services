package app

import (
	"context"
	_ "embed"
	"net/http"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
)

//go:embed artifacts/cobo-template-import-guide-v1.0.md
var canonicalTemplateImportGuideV1 []byte

const (
	TemplateImportGuideFilename    = "cobo-template-import-guide-v1.0.md"
	TemplateImportGuideContentType = "text/markdown; charset=utf-8"
)

// CanonicalTemplateImportGuideV1 returns a copy of the embedded authoring guide.
func CanonicalTemplateImportGuideV1() []byte {
	out := make([]byte, len(canonicalTemplateImportGuideV1))
	copy(out, canonicalTemplateImportGuideV1)
	return out
}

// GetTemplateImportGuideRequest is an auth-gated download with no body.
type GetTemplateImportGuideRequest struct {
	Subject Subject
}

// GetTemplateImportGuideResponse is the static guide download.
type GetTemplateImportGuideResponse struct {
	Filename    string
	ContentType string
	Payload     []byte
}

// GetTemplateImportGuide returns the embedded Markdown guide.
// Same permission as the canonical example. Zero database writes.
func (s *service) GetTemplateImportGuide(ctx context.Context, req GetTemplateImportGuideRequest) (*GetTemplateImportGuideResponse, error) {
	if err := s.requireCMSTemplateWrite(ctx, req.Subject); err != nil {
		return nil, err
	}
	payload := CanonicalTemplateImportGuideV1()
	if len(payload) == 0 {
		return nil, perr.NewHTTPError(http.StatusInternalServerError, perr.CodeInternal, "canonical import guide artifact is empty", nil)
	}
	return &GetTemplateImportGuideResponse{
		Filename:    TemplateImportGuideFilename,
		ContentType: TemplateImportGuideContentType,
		Payload:     payload,
	}, nil
}

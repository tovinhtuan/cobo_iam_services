package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	perr "github.com/cobo/cobo_iam_services/internal/platform/errors"
	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// Legal Basis Phase 12.1A / 12.2 limits (Unicode runes).
const (
	LegalBasisMaxItems           = 20
	LegalBasisTitleMaxRunes      = 500
	LegalBasisCodeMaxRunes       = 100
	LegalBasisAuthorityMaxRunes  = 200
	LegalBasisSummaryMaxRunes    = 8000
	LegalBasisLinkMaxRunes       = 2048
	LegalBasisProjectionMaxRunes = 8000
	LegalBasisIDMaxRunes         = 64
	legalBasisLegacyTitle        = "Cơ sở pháp lý"
)

// NormalizeLegalBasisString trims outer whitespace. Empty → "".
func NormalizeLegalBasisString(s string) string {
	return strings.TrimSpace(s)
}

// NormalizeLegalBasisItemForRead trims fields; does not drop the item.
func NormalizeLegalBasisItemForRead(item LegalBasisDTO) LegalBasisDTO {
	return LegalBasisDTO{
		ID:        NormalizeLegalBasisString(item.ID),
		Title:     NormalizeLegalBasisString(item.Title),
		Code:      NormalizeLegalBasisString(item.Code),
		Authority: NormalizeLegalBasisString(item.Authority),
		IssueDate: NormalizeLegalBasisString(item.IssueDate),
		Summary:   strings.TrimSpace(item.Summary), // ends only; preserve internal NL after outer trim
		Link:      NormalizeLegalBasisString(item.Link),
	}
}

func legalBasisItemContentValid(item LegalBasisDTO) bool {
	return item.Title != "" || item.Summary != ""
}

// NormalizeLegalBasesForRead drops items without title∨summary; keeps valid items.
// Returns normalized list and count of dropped items.
func NormalizeLegalBasesForRead(items []LegalBasisDTO) (out []LegalBasisDTO, dropped int) {
	if len(items) == 0 {
		return []LegalBasisDTO{}, 0
	}
	out = make([]LegalBasisDTO, 0, len(items))
	for _, raw := range items {
		item := NormalizeLegalBasisItemForRead(raw)
		if !legalBasisItemContentValid(item) {
			dropped++
			continue
		}
		out = append(out, item)
	}
	return out, dropped
}

// ProjectLegalBasesToLegacy builds deterministic flat projection (OD-7).
// Does not mutate input. Rejects if projection exceeds max runes.
func ProjectLegalBasesToLegacy(items []LegalBasisDTO) (string, error) {
	normalized, _ := NormalizeLegalBasesForRead(items)
	parts := make([]string, 0, len(normalized))
	for _, item := range normalized {
		text := item.Title
		if text == "" {
			text = item.Summary
		}
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}
	projected := strings.TrimSpace(strings.Join(parts, "\n\n"))
	if utf8.RuneCountInString(projected) > LegalBasisProjectionMaxRunes {
		return "", newValidationError(map[string]string{
			"legal_basis": fmt.Sprintf("projected legal_basis must be at most %d Unicode runes", LegalBasisProjectionMaxRunes),
		})
	}
	return projected, nil
}

// SynthesizeLegacyLegalBasis builds one display item from flat text (OD-2). Not for persistence IDs.
func SynthesizeLegacyLegalBasis(typeID, flat string) LegalBasisDTO {
	flat = strings.TrimSpace(flat)
	id := strings.TrimSpace(typeID) + "-lb-legacy-1"
	if strings.TrimSpace(typeID) == "" {
		id = "unknown-lb-legacy-1"
	}
	return LegalBasisDTO{
		ID:        id,
		Title:     legalBasisLegacyTitle,
		Code:      "",
		Authority: "",
		IssueDate: "",
		Summary:   flat,
		Link:      "",
	}
}

func legalBasisExactDupKey(item LegalBasisDTO) string {
	// Exact normalized object without id (OD-5).
	return strings.Join([]string{
		item.Title,
		item.Code,
		item.Summary,
		item.Link,
	}, "\x1e")
}

func validateLegalBasisLink(link, fieldPath string, fieldErrors map[string]string) {
	if link == "" {
		return
	}
	if utf8.RuneCountInString(link) > LegalBasisLinkMaxRunes {
		fieldErrors[fieldPath] = fmt.Sprintf("link must be at most %d Unicode runes", LegalBasisLinkMaxRunes)
		return
	}
	lower := strings.ToLower(link)
	if strings.HasPrefix(lower, "javascript:") || link == "#" || strings.HasPrefix(link, "#") {
		fieldErrors[fieldPath] = "invalid URL"
		return
	}
	if strings.HasPrefix(link, "/") {
		return
	}
	u, err := url.Parse(link)
	if err != nil || u.Scheme == "" || u.Host == "" {
		fieldErrors[fieldPath] = "invalid URL"
		return
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		fieldErrors[fieldPath] = "invalid URL"
	}
}

func validateLegalBasisIssueDate(date, fieldPath string, fieldErrors map[string]string) {
	if date == "" {
		return
	}
	if _, err := time.Parse("2006-01-02", date); err != nil {
		fieldErrors[fieldPath] = "issue_date must be YYYY-MM-DD"
	}
}

// ValidateLegalBasesForWrite strict-validates structured payload. Fills blank IDs via idg when provided.
func ValidateLegalBasesForWrite(items []LegalBasisDTO, idg idgen.Generator) ([]LegalBasisDTO, error) {
	if items == nil {
		items = []LegalBasisDTO{}
	}
	if len(items) > LegalBasisMaxItems {
		return nil, newValidationError(map[string]string{
			"legal_bases": fmt.Sprintf("legal_bases must have at most %d items", LegalBasisMaxItems),
		})
	}
	fieldErrors := map[string]string{}
	out := make([]LegalBasisDTO, 0, len(items))
	seenExact := map[string]struct{}{}

	for i, raw := range items {
		prefix := fmt.Sprintf("legal_bases[%d]", i)
		item := NormalizeLegalBasisItemForRead(raw)
		if !legalBasisItemContentValid(item) {
			fieldErrors[prefix] = "title or summary is required"
			continue
		}
		if utf8.RuneCountInString(item.Title) > LegalBasisTitleMaxRunes {
			fieldErrors[prefix+".title"] = fmt.Sprintf("title must be at most %d Unicode runes", LegalBasisTitleMaxRunes)
		}
		if utf8.RuneCountInString(item.Code) > LegalBasisCodeMaxRunes {
			fieldErrors[prefix+".code"] = fmt.Sprintf("code must be at most %d Unicode runes", LegalBasisCodeMaxRunes)
		}
		if utf8.RuneCountInString(item.Authority) > LegalBasisAuthorityMaxRunes {
			fieldErrors[prefix+".authority"] = fmt.Sprintf("authority must be at most %d Unicode runes", LegalBasisAuthorityMaxRunes)
		}
		if utf8.RuneCountInString(item.Summary) > LegalBasisSummaryMaxRunes {
			fieldErrors[prefix+".summary"] = fmt.Sprintf("summary must be at most %d Unicode runes", LegalBasisSummaryMaxRunes)
		}
		if utf8.RuneCountInString(item.ID) > LegalBasisIDMaxRunes {
			fieldErrors[prefix+".id"] = fmt.Sprintf("id must be at most %d Unicode runes", LegalBasisIDMaxRunes)
		}
		validateLegalBasisIssueDate(item.IssueDate, prefix+".issue_date", fieldErrors)
		validateLegalBasisLink(item.Link, prefix+".link", fieldErrors)

		key := legalBasisExactDupKey(item)
		if _, ok := seenExact[key]; ok {
			fieldErrors["legal_bases"] = "exact duplicate legal_bases item"
		}
		seenExact[key] = struct{}{}

		if item.ID == "" && idg != nil {
			item.ID = idg.NewUUID()
		}
		out = append(out, item)
	}
	if len(fieldErrors) > 0 {
		return nil, newValidationError(fieldErrors)
	}
	if _, err := ProjectLegalBasesToLegacy(out); err != nil {
		return nil, err
	}
	return out, nil
}

func hashLegalBasisText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

func emitLegalBasisEvent(ctx context.Context, event string, attrs ...any) {
	args := make([]any, 0, 2+len(attrs))
	args = append(args, slog.String("event", event))
	args = append(args, attrs...)
	slog.WarnContext(ctx, event, args...)
}

// ApplyLegalBasisReadCompat mutates dto in place for API response. Never writes DB.
func ApplyLegalBasisReadCompat(ctx context.Context, dto *DisclosureTypeDTO, legacyFallbackEnabled, divergenceWarningEnabled bool) {
	if dto == nil {
		return
	}
	persistedFlat := strings.TrimSpace(dto.LegalBasis)
	normalized, dropped := NormalizeLegalBasesForRead(dto.LegalBases)
	if dropped > 0 {
		emitLegalBasisEvent(ctx, "legal_basis_invalid_persisted_item_dropped",
			slog.String("type_id", dto.TypeID),
			slog.Int("version_no", dto.VersionNo),
			slog.Int("dropped", dropped),
			slog.Int("retained", len(normalized)),
		)
	}
	if len(normalized) >= 1 {
		projected, err := ProjectLegalBasesToLegacy(normalized)
		if err != nil {
			// Persisted overflow should not fail the whole page: keep structured, leave flat as stored.
			dto.LegalBases = normalized
			return
		}
		if divergenceWarningEnabled && persistedFlat != "" && persistedFlat != projected {
			emitLegalBasisEvent(ctx, "legal_basis_divergence_detected",
				slog.String("type_id", dto.TypeID),
				slog.Int("version_no", dto.VersionNo),
				slog.Int("structured_count", len(normalized)),
				slog.String("flat_hash", hashLegalBasisText(persistedFlat)),
				slog.String("projection_hash", hashLegalBasisText(projected)),
				slog.String("operation", "read"),
			)
		}
		dto.LegalBases = normalized
		dto.LegalBasis = projected
		return
	}
	if legacyFallbackEnabled && persistedFlat != "" {
		emitLegalBasisEvent(ctx, "legacy_legal_basis_fallback_used",
			slog.String("type_id", dto.TypeID),
			slog.Int("version_no", dto.VersionNo),
			slog.String("flat_hash", hashLegalBasisText(persistedFlat)),
			slog.String("operation", "read"),
		)
		dto.LegalBases = []LegalBasisDTO{SynthesizeLegacyLegalBasis(dto.TypeID, persistedFlat)}
		dto.LegalBasis = persistedFlat
		return
	}
	dto.LegalBases = []LegalBasisDTO{}
	// Keep empty-string flat for JSON compatibility with current clients.
	if persistedFlat == "" {
		dto.LegalBasis = ""
	}
}

// ResolveLegalBasisWrite applies Phase 12.2 write precedence on upsert request.
// Mutates req.LegalBases / req.LegalBasis / req.PreserveLegalBases.
func ResolveLegalBasisWrite(ctx context.Context, req *UpsertTypeVersionRequest, structuredWriteEnabled bool, idg idgen.Generator) error {
	if req == nil {
		return nil
	}
	flat := strings.TrimSpace(req.LegalBasis)

	if req.LegalBasesProvided {
		req.PreserveLegalBases = false
		if structuredWriteEnabled {
			validated, err := ValidateLegalBasesForWrite(req.LegalBases, idg)
			if err != nil {
				return err
			}
			projected, err := ProjectLegalBasesToLegacy(validated)
			if err != nil {
				return err
			}
			if flat != "" && flat != projected {
				emitLegalBasisEvent(ctx, "legal_basis_client_flat_ignored",
					slog.String("type_id", req.TypeID),
					slog.Int("structured_count", len(validated)),
					slog.String("flat_hash", hashLegalBasisText(flat)),
					slog.String("projection_hash", hashLegalBasisText(projected)),
					slog.String("operation", "write"),
				)
			}
			req.LegalBases = validated
			req.LegalBasis = projected
			return nil
		}
		// Flag OFF: light sanitize, keep client flat (compat with existing clients / link="#").
		req.LegalBases = sanitizeLegalBases(req.LegalBases)
		return nil
	}

	// legal_bases omitted/null → preserve existing structured JSON unless wrap path fills it.
	req.PreserveLegalBases = true
	if structuredWriteEnabled && flat != "" {
		// Wrap OD-2 item for structured storage but KEEP original flat text.
		// Projection(title="Cơ sở pháp lý") would destroy legacy content — do not overwrite flat.
		item := SynthesizeLegacyLegalBasis(req.TypeID, flat)
		if idg != nil {
			item.ID = idg.NewUUID()
		}
		validated, err := ValidateLegalBasesForWrite([]LegalBasisDTO{item}, idg)
		if err != nil {
			return err
		}
		req.LegalBases = validated
		req.LegalBasis = flat
		req.PreserveLegalBases = false
		emitLegalBasisEvent(ctx, "legacy_legal_basis_write_used",
			slog.String("type_id", req.TypeID),
			slog.String("flat_hash", hashLegalBasisText(flat)),
			slog.String("operation", "write"),
		)
		return nil
	}
	if flat != "" {
		emitLegalBasisEvent(ctx, "legacy_legal_basis_write_used",
			slog.String("type_id", req.TypeID),
			slog.String("flat_hash", hashLegalBasisText(flat)),
			slog.String("operation", "write"),
		)
	}
	// Ensure non-nil for marshal when not preserving (should not happen).
	if req.LegalBases == nil {
		req.LegalBases = []LegalBasisDTO{}
	}
	return nil
}

// ensureLegalBasisValidationCode documents that validation uses INVALID_REQUEST + field_errors
// (existing envelope). Contract field paths are keys in field_errors.
var _ = perr.CodeInvalidRequest

func syncLegalBasisBlockDescriptionFromFlat(req *UpsertTypeVersionRequest) {
	if req == nil || len(req.Blocks) == 0 {
		return
	}
	for i := range req.Blocks {
		if strings.EqualFold(strings.TrimSpace(req.Blocks[i].BlockKey), "legal_basis") {
			req.Blocks[i].Description = req.LegalBasis
			return
		}
	}
}

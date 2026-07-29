package app

import (
	"context"
	"log/slog"
	"strings"

	"github.com/cobo/cobo_iam_services/internal/platform/idgen"
)

// DeepCopyLegalBases allocates a new slice of value-copied items (no shared backing array).
func DeepCopyLegalBases(items []LegalBasisDTO) []LegalBasisDTO {
	if len(items) == 0 {
		return []LegalBasisDTO{}
	}
	out := make([]LegalBasisDTO, len(items))
	copy(out, items)
	return out
}

// RegenerateLegalBasisIDs returns a deep copy with every item ID replaced via idg.
// Blank or non-blank source IDs are all replaced. Source slice is not mutated.
func RegenerateLegalBasisIDs(items []LegalBasisDTO, idg idgen.Generator) []LegalBasisDTO {
	out := DeepCopyLegalBases(items)
	for i := range out {
		if idg != nil {
			out[i].ID = idg.NewUUID()
		} else {
			out[i].ID = ""
		}
	}
	return out
}

// PrepareLegalBasesForNewVersion builds independent structured bases for a new version/clone-equivalent row.
//
// - preserve=true: copy from sourceBases (normalized), regenerate IDs, derive projection; legacy-only → flat only.
// - preserve=false with providedBases: deep-copy provided, regenerate IDs; keep incomingFlat when projection not forced.
// - explicit empty provided (clear): returns empty bases + trimmed incomingFlat.
//
// Does not mutate sourceBases / providedBases. Does not auto-repair the source aggregate.
func PrepareLegalBasesForNewVersion(
	ctx context.Context,
	typeID string,
	sourceBases []LegalBasisDTO,
	sourceFlat string,
	providedBases []LegalBasisDTO,
	incomingFlat string,
	preserve bool,
	idg idgen.Generator,
) (bases []LegalBasisDTO, flat string, dropped int) {
	sourceFlat = strings.TrimSpace(sourceFlat)
	incomingFlat = strings.TrimSpace(incomingFlat)

	if preserve {
		normalized, dropCount := NormalizeLegalBasesForRead(sourceBases)
		dropped = dropCount
		if dropCount > 0 {
			emitLegalBasisEvent(ctx, "legal_basis_lifecycle_malformed_items_dropped",
				slog.String("type_id", typeID),
				slog.Int("dropped", dropCount),
				slog.String("operation", "new_version"),
			)
		}
		if len(normalized) == 0 {
			// Legacy-only: copy flat only; do not invent structured metadata from text.
			emitLegalBasisEvent(ctx, "legal_basis_lifecycle_preserved",
				slog.String("type_id", typeID),
				slog.String("mode", "legacy_flat_only"),
				slog.String("operation", "new_version"),
			)
			return []LegalBasisDTO{}, sourceFlat, dropped
		}
		copied := RegenerateLegalBasisIDs(normalized, idg)
		projected, err := ProjectLegalBasesToLegacy(copied)
		if err != nil {
			flat = sourceFlat
		} else {
			flat = projected
		}
		emitLegalBasisEvent(ctx, "legal_basis_lifecycle_ids_regenerated",
			slog.String("type_id", typeID),
			slog.Int("count", len(copied)),
			slog.String("operation", "new_version"),
		)
		return copied, flat, dropped
	}

	// Client provided structured payload for the new version.
	normalized, dropCount := NormalizeLegalBasesForRead(providedBases)
	dropped = dropCount
	if len(normalized) == 0 {
		return []LegalBasisDTO{}, incomingFlat, dropped
	}
	copied := RegenerateLegalBasisIDs(normalized, idg)
	projected, err := ProjectLegalBasesToLegacy(copied)
	if err != nil {
		flat = incomingFlat
	} else {
		// Prefer deterministic projection for independent version rows.
		flat = projected
		if incomingFlat != "" && incomingFlat != projected {
			emitLegalBasisEvent(ctx, "legal_basis_lifecycle_divergence_normalized_in_target",
				slog.String("type_id", typeID),
				slog.String("flat_hash", hashLegalBasisText(incomingFlat)),
				slog.String("projection_hash", hashLegalBasisText(projected)),
				slog.String("operation", "new_version"),
			)
		}
	}
	emitLegalBasisEvent(ctx, "legal_basis_lifecycle_ids_regenerated",
		slog.String("type_id", typeID),
		slog.Int("count", len(copied)),
		slog.String("operation", "new_version"),
	)
	return copied, flat, dropped
}

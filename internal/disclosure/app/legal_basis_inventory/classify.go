package inventory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	disclosureapp "github.com/cobo/cobo_iam_services/internal/disclosure/app"
)

// Group is A–E classification for Phase 12.6A.
type Group string

const (
	GroupA Group = "A"
	GroupB Group = "B"
	GroupC Group = "C"
	GroupD Group = "D"
	GroupE Group = "E"
)

// Record is a redacted inventory input (no full legal text required after classify).
type Record struct {
	TypeID          string
	VersionNo       int
	CompanyID       string // empty => global
	TypeStatus      string
	ActiveVersionNo int
	IsReleased      bool
	LegalBasis      string
	LegalBasesJSON  []byte // raw JSON bytes from DB (may be nil/null)
}

// Anomaly codes attached besides A–E group.
type Anomaly string

const (
	AnomalyMalformedJSON      Anomaly = "MALFORMED_STRUCTURED_JSON"
	AnomalyProjectionOverflow Anomaly = "PROJECTION_OVERFLOW"
	AnomalyContractViolation  Anomaly = "CONTRACT_VIOLATION"
)

// ProposedAction for dry-run.
type ProposedAction string

const (
	ActionWrapLegacyFlat    ProposedAction = "WRAP_LEGACY_FLAT"
	ActionProjectStructured ProposedAction = "PROJECT_STRUCTURED"
	ActionNormalizeMatched  ProposedAction = "NORMALIZE_MATCHED"
	ActionManualReview      ProposedAction = "MANUAL_REVIEW"
	ActionNoOp              ProposedAction = "NO_OP"
	ActionBlockedMalformed  ProposedAction = "BLOCKED_MALFORMED"
	ActionBlockedOverflow   ProposedAction = "BLOCKED_OVERFLOW"
)

// Result is one classified + dry-run preview row (redacted).
type Result struct {
	TypeID              string         `json:"typeId"`
	VersionNo           int            `json:"versionNo"`
	CompanyMarker       string         `json:"companyMarker"` // "global" or "company"
	TypeStatus          string         `json:"typeStatus"`
	ActiveVersionNo     int            `json:"activeVersionNo"`
	IsReleased          bool           `json:"isReleased"`
	Group               Group          `json:"group"`
	Anomalies           []Anomaly      `json:"anomalies,omitempty"`
	StructuredCount     int            `json:"structuredCount"`
	FlatRuneCount       int            `json:"flatRuneCount"`
	ProjectionRuneCount int            `json:"projectionRuneCount"`
	FlatHash            string         `json:"flatHash"`
	ProjectionHash      string         `json:"projectionHash"`
	JSONByteLength      int            `json:"jsonByteLength"`
	JSONHash            string         `json:"jsonHash,omitempty"`
	ParseError          string         `json:"parseError,omitempty"`
	ViolationCodes      []string       `json:"violationCodes,omitempty"`
	DivergenceClass     string         `json:"divergenceClass,omitempty"`
	ProposedAction      ProposedAction `json:"proposedAction"`
	TargetStructCount   int            `json:"targetStructuredCount"`
	TargetFlatRuneCount int            `json:"targetFlatRuneCount"`
	TargetFlatHash      string         `json:"targetFlatHash"`
	TargetStructHash    string         `json:"targetStructuredHash"`
	Warnings            []string       `json:"warnings,omitempty"`
}

func hashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8]) // short hash for evidence
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:8])
}

func companyMarker(companyID string) string {
	if strings.TrimSpace(companyID) == "" {
		return "global"
	}
	return "company"
}

// parseLegalBasesJSON returns items, parseOK, parseError.
// null / empty / missing → empty slice, parseOK=true.
func parseLegalBasesJSON(raw []byte) (items []disclosureapp.LegalBasisDTO, parseOK bool, parseErr string) {
	if len(raw) == 0 || string(raw) == "null" {
		return []disclosureapp.LegalBasisDTO{}, true, ""
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return []disclosureapp.LegalBasisDTO{}, true, ""
	}
	if trimmed[0] != '[' {
		return nil, false, "root_not_array"
	}
	var anyItems []json.RawMessage
	if err := json.Unmarshal(raw, &anyItems); err != nil {
		return nil, false, "invalid_json"
	}
	out := make([]disclosureapp.LegalBasisDTO, 0, len(anyItems))
	for _, elm := range anyItems {
		t := strings.TrimSpace(string(elm))
		if t == "null" {
			return nil, false, "item_null"
		}
		if !strings.HasPrefix(t, "{") {
			return nil, false, "item_not_object"
		}
		var item disclosureapp.LegalBasisDTO
		if err := json.Unmarshal(elm, &item); err != nil {
			return nil, false, "item_decode_error"
		}
		out = append(out, item)
	}
	return out, true, ""
}

// ClassifyRecord assigns Group A–E and dry-run proposal without mutating DB state.
func ClassifyRecord(rec Record) Result {
	flat := strings.TrimSpace(rec.LegalBasis)
	flatNonEmpty := flat != ""

	res := Result{
		TypeID:          rec.TypeID,
		VersionNo:       rec.VersionNo,
		CompanyMarker:   companyMarker(rec.CompanyID),
		TypeStatus:      rec.TypeStatus,
		ActiveVersionNo: rec.ActiveVersionNo,
		IsReleased:      rec.IsReleased,
		FlatRuneCount:   utf8.RuneCountInString(flat),
		FlatHash:        hashText(flat),
		JSONByteLength:  len(rec.LegalBasesJSON),
	}
	if len(rec.LegalBasesJSON) > 0 {
		res.JSONHash = hashBytes(rec.LegalBasesJSON)
	}

	rawItems, parseOK, parseErr := parseLegalBasesJSON(rec.LegalBasesJSON)
	if !parseOK {
		res.Anomalies = append(res.Anomalies, AnomalyMalformedJSON)
		res.ParseError = parseErr
		if flatNonEmpty {
			res.Group = GroupA
		} else {
			res.Group = GroupE
		}
		res.ProposedAction = ActionBlockedMalformed
		res.Warnings = append(res.Warnings, "malformed structured JSON — no auto mutate")
		return res
	}

	valid, dropped := disclosureapp.NormalizeLegalBasesForRead(rawItems)
	res.StructuredCount = len(valid)
	structNonEmpty := len(valid) > 0

	violations := collectViolations(rawItems, valid)
	if len(violations) > 0 {
		res.Anomalies = append(res.Anomalies, AnomalyContractViolation)
		res.ViolationCodes = violations
	}
	if dropped > 0 {
		res.Warnings = append(res.Warnings, fmt.Sprintf("dropped_invalid_items:%d", dropped))
	}

	var projection string
	var projErr error
	if structNonEmpty {
		projection, projErr = disclosureapp.ProjectLegalBasesToLegacy(valid)
		if projErr != nil {
			res.Anomalies = append(res.Anomalies, AnomalyProjectionOverflow)
			res.ProjectionRuneCount = -1
			res.ProposedAction = ActionBlockedOverflow
			// Still classify A–E by presence rules.
			res.Group = classifyPresence(flatNonEmpty, structNonEmpty, flat, "")
			if res.Group == GroupD {
				res.DivergenceClass = "UNKNOWN"
			}
			res.Warnings = append(res.Warnings, "projection overflow or project error — blocked auto")
			return finalizeDryRunBlocked(res, valid, flat)
		}
		res.ProjectionRuneCount = utf8.RuneCountInString(projection)
		res.ProjectionHash = hashText(projection)
		if res.ProjectionRuneCount > disclosureapp.LegalBasisProjectionMaxRunes {
			res.Anomalies = append(res.Anomalies, AnomalyProjectionOverflow)
			res.ProposedAction = ActionBlockedOverflow
			res.Group = classifyPresence(flatNonEmpty, structNonEmpty, flat, projection)
			return finalizeDryRunBlocked(res, valid, flat)
		}
	}

	res.Group = classifyPresence(flatNonEmpty, structNonEmpty, flat, projection)
	if res.Group == GroupD {
		res.DivergenceClass = classifyDivergence(flat, projection)
	}

	switch res.Group {
	case GroupA:
		res.ProposedAction = ActionWrapLegacyFlat
		// Simulate OD-2 wrap + project (title only → projection = title)
		sim := []disclosureapp.LegalBasisDTO{{
			ID:      "<NEW_UUID>",
			Title:   "Cơ sở pháp lý",
			Summary: flat,
		}}
		proj, _ := disclosureapp.ProjectLegalBasesToLegacy(sim)
		res.TargetStructCount = 1
		res.TargetFlatRuneCount = utf8.RuneCountInString(proj)
		res.TargetFlatHash = hashText(proj)
		res.TargetStructHash = hashText(structFingerprint(sim))
	case GroupB, GroupC:
		if res.Group == GroupB {
			res.ProposedAction = ActionProjectStructured
		} else {
			res.ProposedAction = ActionNormalizeMatched
		}
		proj := projection
		res.TargetStructCount = len(valid)
		res.TargetFlatRuneCount = utf8.RuneCountInString(proj)
		res.TargetFlatHash = hashText(proj)
		res.TargetStructHash = hashText(structFingerprint(valid))
	case GroupD:
		res.ProposedAction = ActionManualReview
		res.TargetStructCount = len(valid)
		res.TargetFlatRuneCount = res.FlatRuneCount
		res.TargetFlatHash = res.FlatHash
		res.TargetStructHash = hashText(structFingerprint(valid))
	default: // E
		res.ProposedAction = ActionNoOp
		res.TargetStructCount = 0
		res.TargetFlatRuneCount = 0
		res.TargetFlatHash = hashText("")
		res.TargetStructHash = hashText("")
	}
	return res
}

func finalizeDryRunBlocked(res Result, valid []disclosureapp.LegalBasisDTO, flat string) Result {
	res.TargetStructCount = len(valid)
	res.TargetFlatRuneCount = utf8.RuneCountInString(strings.TrimSpace(flat))
	res.TargetFlatHash = hashText(strings.TrimSpace(flat))
	res.TargetStructHash = hashText(structFingerprint(valid))
	return res
}

func classifyPresence(flatNonEmpty, structNonEmpty bool, flat, projection string) Group {
	switch {
	case flatNonEmpty && !structNonEmpty:
		return GroupA
	case !flatNonEmpty && structNonEmpty:
		return GroupB
	case flatNonEmpty && structNonEmpty:
		if flat == projection {
			return GroupC
		}
		return GroupD
	default:
		return GroupE
	}
}

func classifyDivergence(flat, projection string) string {
	if strings.TrimSpace(flat) == strings.TrimSpace(projection) {
		return "WHITESPACE_ONLY"
	}
	// Collapse internal whitespace for soft compare
	norm := func(s string) string {
		return strings.Join(strings.Fields(s), " ")
	}
	if norm(flat) == norm(projection) {
		return "WHITESPACE_ONLY"
	}
	if strings.Contains(flat, projection) || strings.Contains(projection, flat) {
		return "LEGACY_EXTRA_TEXT"
	}
	return "CONTENT_DIFFERENCE"
}

func structFingerprint(items []disclosureapp.LegalBasisDTO) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, strings.Join([]string{
			it.ID, it.Title, it.Code, it.Authority, it.IssueDate, it.Summary, it.Link,
		}, "\x1e"))
	}
	return strings.Join(parts, "\x1f")
}

func collectViolations(raw, valid []disclosureapp.LegalBasisDTO) []string {
	var codes []string
	if len(raw) > disclosureapp.LegalBasisMaxItems {
		codes = append(codes, "TOO_MANY_ITEMS")
	}
	seenID := map[string]struct{}{}
	seenExact := map[string]struct{}{}
	for i, item := range disclosureapp.DeepCopyLegalBases(raw) {
		n := disclosureapp.NormalizeLegalBasisItemForRead(item)
		if n.Title == "" && n.Summary == "" {
			codes = append(codes, fmt.Sprintf("EMPTY_TITLE_AND_SUMMARY[%d]", i))
		}
		if utf8.RuneCountInString(n.Title) > disclosureapp.LegalBasisTitleMaxRunes {
			codes = append(codes, fmt.Sprintf("TITLE_OVERFLOW[%d]", i))
		}
		if utf8.RuneCountInString(n.Code) > disclosureapp.LegalBasisCodeMaxRunes {
			codes = append(codes, fmt.Sprintf("CODE_OVERFLOW[%d]", i))
		}
		if utf8.RuneCountInString(n.Authority) > disclosureapp.LegalBasisAuthorityMaxRunes {
			codes = append(codes, fmt.Sprintf("AUTHORITY_OVERFLOW[%d]", i))
		}
		if utf8.RuneCountInString(n.Summary) > disclosureapp.LegalBasisSummaryMaxRunes {
			codes = append(codes, fmt.Sprintf("SUMMARY_OVERFLOW[%d]", i))
		}
		if utf8.RuneCountInString(n.Link) > disclosureapp.LegalBasisLinkMaxRunes {
			codes = append(codes, fmt.Sprintf("LINK_OVERFLOW[%d]", i))
		}
		if n.ID != "" {
			if _, ok := seenID[n.ID]; ok {
				codes = append(codes, "DUPLICATE_ID")
			}
			seenID[n.ID] = struct{}{}
		}
		key := strings.Join([]string{n.Title, n.Code, n.Summary, n.Link}, "\x1e")
		if _, ok := seenExact[key]; ok && (n.Title != "" || n.Summary != "") {
			codes = append(codes, "EXACT_DUPLICATE")
		}
		seenExact[key] = struct{}{}
	}
	_ = valid
	return uniqueStrings(codes)
}

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// SimulateApply applies dry-run transform in memory for idempotency check (no UUID real gen needed).
func SimulateApply(rec Record, first Result) Record {
	switch first.ProposedAction {
	case ActionWrapLegacyFlat:
		flat := strings.TrimSpace(rec.LegalBasis)
		sim := []disclosureapp.LegalBasisDTO{{
			ID: "<NEW_UUID>", Title: "Cơ sở pháp lý", Summary: flat,
		}}
		b, _ := json.Marshal(sim)
		proj, _ := disclosureapp.ProjectLegalBasesToLegacy(sim)
		return Record{
			TypeID: rec.TypeID, VersionNo: rec.VersionNo, CompanyID: rec.CompanyID,
			TypeStatus: rec.TypeStatus, ActiveVersionNo: rec.ActiveVersionNo, IsReleased: rec.IsReleased,
			LegalBasis: proj, LegalBasesJSON: b,
		}
	case ActionProjectStructured, ActionNormalizeMatched:
		raw, ok, _ := parseLegalBasesJSON(rec.LegalBasesJSON)
		if !ok {
			return rec
		}
		valid, _ := disclosureapp.NormalizeLegalBasesForRead(raw)
		proj, err := disclosureapp.ProjectLegalBasesToLegacy(valid)
		if err != nil {
			return rec
		}
		b, _ := json.Marshal(valid)
		return Record{
			TypeID: rec.TypeID, VersionNo: rec.VersionNo, CompanyID: rec.CompanyID,
			TypeStatus: rec.TypeStatus, ActiveVersionNo: rec.ActiveVersionNo, IsReleased: rec.IsReleased,
			LegalBasis: proj, LegalBasesJSON: b,
		}
	default:
		// MANUAL / NO_OP / BLOCKED — unchanged
		return rec
	}
}

// Reconciliation tallies groups.
type Reconciliation struct {
	Total         int           `json:"total"`
	Groups        map[Group]int `json:"groups"`
	SumGroups     int           `json:"sumGroups"`
	Unclassified  int           `json:"unclassified"`
	Overlap       int           `json:"overlap"`
	AnalyzerMatch bool          `json:"sqlVsAnalyzerMatch"`
}

func Reconcile(results []Result) Reconciliation {
	g := map[Group]int{GroupA: 0, GroupB: 0, GroupC: 0, GroupD: 0, GroupE: 0}
	for _, r := range results {
		if _, ok := g[r.Group]; !ok {
			continue
		}
		g[r.Group]++
	}
	sum := g[GroupA] + g[GroupB] + g[GroupC] + g[GroupD] + g[GroupE]
	return Reconciliation{
		Total:         len(results),
		Groups:        g,
		SumGroups:     sum,
		Unclassified:  len(results) - sum,
		Overlap:       0,
		AnalyzerMatch: true, // filled by runner after SQL compare
	}
}

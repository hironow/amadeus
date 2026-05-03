// Package policy rival_contract.go: Rival Contract v1 parser and projection.
//
// Pure parsing utilities for the Rival Contract v1 format. The parser is
// deterministic, performs no I/O, and never invokes an LLM. Consumers in
// sightjack, paintress, amadeus, and dominator are expected to keep the
// canonical type and function names in sync via copy-sync until duplicate
// maintenance becomes a real cost.
//
// amadeus is the contract owner of selection: ProjectCurrentContracts here
// is the deterministic projection that picks the winning revision per
// contract_id and emits ContractConflict for ambiguous cases.
//
// Refs: refs/plans/2026-05-03-rival-contract-v1.md
package policy

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
)

// RivalContract is the parsed body of a Rival Contract v1 specification.
type RivalContract struct { // nosemgrep: structure.multiple-exported-structs-go -- Rival Contract v1 type family (RivalContract/RivalContractMetadata/EvidenceItem/CurrentContract/ContractConflict) is a cohesive parsed-document set; the metadata wrapper co-locates with the body it describes [permanent]
	Title      string
	Intent     string
	Domain     string
	Decisions  string
	Steps      string
	Boundaries string
	Evidence   string
}

// RivalContractMetadata is the parsed view of contract metadata fields
// embedded in the existing D-Mail metadata map.
type RivalContractMetadata struct { // nosemgrep: structure.multiple-exported-structs-go -- Rival Contract v1 type family co-located with RivalContract [permanent]
	Schema     string
	ID         string
	Revision   int
	Supersedes string
}

// EvidenceItem is a single deterministic bullet parsed from the Evidence
// section. Prose bullets and unknown keys are dropped before reaching this
// representation.
type EvidenceItem struct { // nosemgrep: structure.multiple-exported-structs-go -- Rival Contract v1 type family co-located with RivalContract [permanent]
	Key      string
	Operator string
	Value    string
}

// CurrentContract pairs a parsed contract body with its metadata and the
// originating D-Mail name. amadeus uses this as the projection result.
type CurrentContract struct { // nosemgrep: structure.multiple-exported-structs-go -- Rival Contract v1 type family co-located with RivalContract [permanent]
	DMailName string
	Metadata  RivalContractMetadata
	Contract  RivalContract
}

// ContractConflict is emitted when two D-Mails claim the same contract id
// at the same revision but disagree on body or supersedes link.
type ContractConflict struct { // nosemgrep: structure.multiple-exported-structs-go -- Rival Contract v1 type family co-located with RivalContract [permanent]
	ContractID string
	Reason     string
	Names      []string
}

// SchemaRivalContractV1 is the only accepted contract_schema value for v1.
const SchemaRivalContractV1 = "rival-contract-v1"

// ErrContractIDUnavailable is returned by DeriveContractID when no stable
// non-D-Mail-name input is available.
var ErrContractIDUnavailable = errors.New("rival contract: no stable contract id input available")

// ErrPartialContractBody is returned by ParseRivalContractBody when a body
// uses the Rival Contract title but is missing one or more required
// sections.
var ErrPartialContractBody = errors.New("rival contract: body has Contract title but missing required sections")

// ErrDMailNameAsContractID is returned by metadata parsing when the
// supplied contract_id matches a D-Mail name pattern (e.g. starts with
// "spec-" and ends with an "_<8 hex>" suffix).
var ErrDMailNameAsContractID = errors.New("rival contract: contract_id must not be a D-Mail name")

// dmailNamePattern matches the conventional D-Mail name shape:
//
//	<prefix>-<words>_<8 lowercase hex>
//
// Used to guard against accidentally using a per-message identity as a
// contract identity.
var dmailNamePattern = regexp.MustCompile(`^[a-z]+-[a-z0-9-]+_[0-9a-f]{8,}$`)

// rivalSectionHeadings is the canonical ordered set of required `##`
// headings inside a Rival Contract v1 body.
var rivalSectionHeadings = []string{
	"Intent",
	"Domain",
	"Decisions",
	"Steps",
	"Boundaries",
	"Evidence",
}

// supportedEvidenceKeys lists the keys that ParseEvidenceItems will keep.
// Unknown keys (including unknown nfr.* keys) are ignored on purpose.
var supportedEvidenceKeys = map[string]struct{}{
	"check":                    {},
	"test":                     {},
	"lint":                     {},
	"semgrep":                  {},
	"nfr.p95_latency_ms":       {},
	"nfr.error_rate_percent":   {},
	"nfr.success_rate_percent": {},
	"nfr.target_rps":           {},
}

// ParseRivalContractBody parses a Markdown body into a RivalContract.
//
// Returns:
//   - (contract, true,  nil)   — body has a `# Contract:` title and all
//     six required `## section` headings.
//   - (zero,     false, nil)   — body has no `# Contract:` title (legacy
//     specification body); consumers should fall back to existing logic.
//   - (zero,     false, err)   — body has the `# Contract:` title but is
//     missing one or more required sections (partial Rival Contract).
func ParseRivalContractBody(body string) (RivalContract, bool, error) {
	title, ok := extractContractTitle(body)
	if !ok {
		return RivalContract{}, false, nil
	}

	sections := splitRivalSections(body)
	for _, name := range rivalSectionHeadings {
		if _, present := sections[name]; !present {
			return RivalContract{}, false, fmt.Errorf("%w: missing %q section", ErrPartialContractBody, name)
		}
	}

	return RivalContract{
		Title:      title,
		Intent:     sections["Intent"],
		Domain:     sections["Domain"],
		Decisions:  sections["Decisions"],
		Steps:      sections["Steps"],
		Boundaries: sections["Boundaries"],
		Evidence:   sections["Evidence"],
	}, true, nil
}

// ParseRivalContractMetadata extracts Rival Contract v1 fields from the
// existing D-Mail metadata map. The map is `map[string]string` because it
// comes from D-Mail YAML frontmatter.
//
// Returns:
//   - (parsed, true,  nil) — schema is exactly rival-contract-v1 and all
//     required fields parse cleanly.
//   - (zero,   false, nil) — contract_schema is missing entirely (legacy
//     specification metadata); consumers should ignore.
//   - (zero,   false, err) — schema is present but invalid, revision is
//     not a positive integer, or contract_id resembles a D-Mail name.
func ParseRivalContractMetadata(meta map[string]string) (RivalContractMetadata, bool, error) {
	schema, hasSchema := meta["contract_schema"]
	if !hasSchema || schema == "" {
		return RivalContractMetadata{}, false, nil
	}
	if schema != SchemaRivalContractV1 {
		return RivalContractMetadata{}, false, fmt.Errorf("rival contract: unsupported schema %q", schema)
	}

	id := strings.TrimSpace(meta["contract_id"])
	if id == "" {
		return RivalContractMetadata{}, false, errors.New("rival contract: contract_id is required")
	}
	if isDMailName(id) {
		return RivalContractMetadata{}, false, fmt.Errorf("%w: %q", ErrDMailNameAsContractID, id)
	}

	revStr := strings.TrimSpace(meta["contract_revision"])
	if revStr == "" {
		return RivalContractMetadata{}, false, errors.New("rival contract: contract_revision is required")
	}
	rev, err := strconv.Atoi(revStr)
	if err != nil {
		return RivalContractMetadata{}, false, fmt.Errorf("rival contract: contract_revision %q is not an integer: %w", revStr, err)
	}
	if rev <= 0 {
		return RivalContractMetadata{}, false, fmt.Errorf("rival contract: contract_revision must be positive, got %d", rev)
	}

	return RivalContractMetadata{
		Schema:     schema,
		ID:         id,
		Revision:   rev,
		Supersedes: strings.TrimSpace(meta["supersedes"]),
	}, true, nil
}

// ParseEvidenceItems parses the Evidence section into deterministic
// machine-readable bullets. Prose bullets, unknown keys, and unknown
// `nfr.*` keys are silently dropped. The parser does not execute any
// command and treats values as opaque strings.
func ParseEvidenceItems(evidence string) []EvidenceItem {
	if evidence == "" {
		return nil
	}
	var items []EvidenceItem
	for _, raw := range strings.Split(evidence, "\n") {
		line := strings.TrimSpace(raw)
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		body := strings.TrimSpace(strings.TrimPrefix(line, "- "))
		colon := strings.IndexByte(body, ':')
		if colon < 0 {
			continue
		}
		key := strings.TrimSpace(body[:colon])
		if _, supported := supportedEvidenceKeys[key]; !supported {
			continue
		}
		valuePart := strings.TrimSpace(body[colon+1:])
		op, val := splitOperator(valuePart)
		items = append(items, EvidenceItem{Key: key, Operator: op, Value: val})
	}
	return items
}

// DeriveContractID returns a stable contract identifier suitable for
// `metadata.contract_id`. Preference order:
//  1. waveID, if non-empty.
//  2. issueIDs, joined by '+' in deterministic sorted order.
//  3. clusterName, if non-empty.
//  4. otherwise, ErrContractIDUnavailable. The plan forbids using a
//     D-Mail name as the contract id because D-Mail names are message
//     identities, not contract identities.
func DeriveContractID(waveID string, issueIDs []string, clusterName string) (string, error) {
	if id := strings.TrimSpace(waveID); id != "" {
		return id, nil
	}
	cleaned := make([]string, 0, len(issueIDs))
	for _, raw := range issueIDs {
		if id := strings.TrimSpace(raw); id != "" {
			cleaned = append(cleaned, id)
		}
	}
	if len(cleaned) > 0 {
		slices.Sort(cleaned)
		cleaned = slices.Compact(cleaned)
		return strings.Join(cleaned, "+"), nil
	}
	if name := strings.TrimSpace(clusterName); name != "" {
		return name, nil
	}
	return "", ErrContractIDUnavailable
}

// ProjectCurrentContracts is the amadeus-specific deterministic projection
// that selects the winning Rival Contract per contract_id from a stream of
// D-Mails.
//
// Selection rules (per plan §"Current Contract Selection"):
//  1. Filter to dmails of kind == specification.
//  2. Filter to those with metadata.contract_schema == rival-contract-v1.
//  3. Group by metadata.contract_id.
//  4. Within each group, parse metadata.contract_revision as a positive
//     integer.
//  5. Pick the highest revision.
//  6. If two D-Mails share the same contract_id and the same highest
//     revision:
//     - If exact duplicate (same idempotency_key, or same body+supersedes
//     when idempotency keys are absent) → tolerate, pick deterministically
//     (lexicographically smallest D-Mail name).
//     - Else → emit ContractConflict with reason "same-revision conflict".
//  7. If supersedes is non-empty, verify it equals a previous D-Mail name
//     for the same contract_id at a strictly lower revision. If not → emit
//     ContractConflict with reason "invalid supersedes lineage".
//
// The function is pure and deterministic. It does not invoke an LLM and
// does not perform I/O. Output slices are sorted by contract_id for stable
// ordering.
func ProjectCurrentContracts(dmails []domain.DMail) ([]CurrentContract, []ContractConflict) {
	groups := groupContractsByID(dmails)
	contractIDs := sortedContractIDs(groups)

	var current []CurrentContract
	var conflicts []ContractConflict
	for _, id := range contractIDs {
		group := groups[id]
		winners, sameRevConflict := pickHighestRevision(id, group)
		if sameRevConflict != nil {
			conflicts = append(conflicts, *sameRevConflict)
			continue
		}
		if len(winners) == 0 {
			continue
		}
		winner := winners[0]
		if conflict := checkSupersedesLineage(id, winner, group); conflict != nil {
			conflicts = append(conflicts, *conflict)
			continue
		}
		current = append(current, winner)
	}
	return current, conflicts
}

// contractCandidate is a parsed projection of a single D-Mail.
type contractCandidate struct {
	dmail    domain.DMail
	metadata RivalContractMetadata
	contract RivalContract
}

// groupContractsByID parses each specification D-Mail and groups parsed
// candidates by contract_id. D-Mails that fail metadata or body parsing
// are silently skipped — invalid messages do not produce conflicts on
// their own; selection conflicts arise from disagreement between valid
// candidates.
func groupContractsByID(dmails []domain.DMail) map[string][]contractCandidate {
	groups := make(map[string][]contractCandidate)
	for _, d := range dmails {
		if d.Kind != domain.KindSpecification {
			continue
		}
		meta, ok, err := ParseRivalContractMetadata(d.Metadata)
		if err != nil || !ok {
			continue
		}
		body, ok, err := ParseRivalContractBody(d.Body)
		if err != nil || !ok {
			continue
		}
		groups[meta.ID] = append(groups[meta.ID], contractCandidate{
			dmail:    d,
			metadata: meta,
			contract: body,
		})
	}
	return groups
}

// sortedContractIDs returns the keys of groups in lexicographic order so
// that ProjectCurrentContracts produces a stable output regardless of the
// input order.
func sortedContractIDs(groups map[string][]contractCandidate) []string {
	ids := make([]string, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// pickHighestRevision returns the winning candidate(s) at the highest
// revision in a group. When multiple candidates share the same highest
// revision, exact duplicates collapse to a single deterministic winner;
// non-duplicate ties produce a same-revision ContractConflict.
func pickHighestRevision(contractID string, group []contractCandidate) ([]CurrentContract, *ContractConflict) {
	if len(group) == 0 {
		return nil, nil
	}
	maxRev := 0
	for _, c := range group {
		if c.metadata.Revision > maxRev {
			maxRev = c.metadata.Revision
		}
	}
	var top []contractCandidate
	for _, c := range group {
		if c.metadata.Revision == maxRev {
			top = append(top, c)
		}
	}
	if len(top) == 1 {
		return []CurrentContract{toCurrentContract(top[0])}, nil
	}
	if duplicates(top) {
		winner := pickDeterministic(top)
		return []CurrentContract{toCurrentContract(winner)}, nil
	}
	names := dmailNamesOf(top)
	conflict := ContractConflict{
		ContractID: contractID,
		Reason:     "same-revision conflict",
		Names:      names,
	}
	return nil, &conflict
}

// checkSupersedesLineage validates that a winner's supersedes pointer
// names a real predecessor D-Mail in the same group. An empty supersedes
// is always accepted (initial revision). When supersedes is set, the
// referenced D-Mail must exist in the group at a strictly lower revision.
func checkSupersedesLineage(contractID string, winner CurrentContract, group []contractCandidate) *ContractConflict {
	prev := strings.TrimSpace(winner.Metadata.Supersedes)
	if prev == "" {
		return nil
	}
	for _, c := range group {
		if c.dmail.Name == prev && c.metadata.Revision < winner.Metadata.Revision {
			return nil
		}
	}
	return &ContractConflict{
		ContractID: contractID,
		Reason:     "invalid supersedes lineage",
		Names:      []string{winner.DMailName, prev},
	}
}

// duplicates reports whether all candidates in top represent the same
// content. Equivalence is decided first by idempotency_key in metadata
// (the stored hash) and falls back to a full body+supersedes comparison
// when no idempotency keys are present.
func duplicates(top []contractCandidate) bool {
	if len(top) < 2 {
		return true
	}
	keys := make(map[string]struct{}, len(top))
	for _, c := range top {
		key, ok := c.dmail.Metadata["idempotency_key"]
		if !ok || key == "" {
			return bodiesEqual(top)
		}
		keys[key] = struct{}{}
	}
	return len(keys) == 1
}

// bodiesEqual reports whether all candidates have identical body text
// and supersedes pointers. Used as a fallback when idempotency keys are
// not available.
func bodiesEqual(top []contractCandidate) bool {
	if len(top) < 2 {
		return true
	}
	first := top[0]
	for _, c := range top[1:] {
		if c.dmail.Body != first.dmail.Body {
			return false
		}
		if c.metadata.Supersedes != first.metadata.Supersedes {
			return false
		}
	}
	return true
}

// pickDeterministic returns the candidate with the lexicographically
// smallest D-Mail name. Used to break ties between exact duplicates.
func pickDeterministic(top []contractCandidate) contractCandidate {
	winner := top[0]
	for _, c := range top[1:] {
		if c.dmail.Name < winner.dmail.Name {
			winner = c
		}
	}
	return winner
}

// dmailNamesOf returns sorted D-Mail names for the candidates. Used for
// ContractConflict.Names so output is deterministic.
func dmailNamesOf(top []contractCandidate) []string {
	names := make([]string, 0, len(top))
	for _, c := range top {
		names = append(names, c.dmail.Name)
	}
	sort.Strings(names)
	return names
}

// toCurrentContract projects a contractCandidate onto the public
// CurrentContract type.
func toCurrentContract(c contractCandidate) CurrentContract {
	return CurrentContract{
		DMailName: c.dmail.Name,
		Metadata:  c.metadata,
		Contract:  c.contract,
	}
}

// extractContractTitle returns the title text of the first `# Contract:`
// heading. The heading must be the first level-1 heading; otherwise the
// body is treated as legacy.
func extractContractTitle(body string) (string, bool) {
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "# ") {
			// First non-blank line is not a top-level heading -> legacy.
			return "", false
		}
		titleLine := strings.TrimSpace(strings.TrimPrefix(line, "# "))
		if !strings.HasPrefix(titleLine, "Contract:") {
			return "", false
		}
		return strings.TrimSpace(strings.TrimPrefix(titleLine, "Contract:")), true
	}
	return "", false
}

// splitRivalSections returns a map from section heading (e.g. "Intent")
// to the trimmed body text under that heading. Only the canonical six
// headings are honored; any other `## X` headings are ignored.
func splitRivalSections(body string) map[string]string {
	known := make(map[string]struct{}, len(rivalSectionHeadings))
	for _, name := range rivalSectionHeadings {
		known[name] = struct{}{}
	}
	out := make(map[string]string)
	var current string
	var buf strings.Builder
	flush := func() {
		if current == "" {
			return
		}
		out[current] = strings.TrimSpace(buf.String())
		buf.Reset()
	}
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			heading := strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			if _, ok := known[heading]; ok {
				flush()
				current = heading
				continue
			}
			// Unknown ## heading: end current section without recording new.
			flush()
			current = ""
			continue
		}
		if current != "" {
			buf.WriteString(line)
			buf.WriteByte('\n')
		}
	}
	flush()
	return out
}

// splitOperator separates a comparison operator (`<=`, `>=`, `<`, `>`,
// `=`, `==`) from a numeric or string value. For non-comparison values
// (e.g. command strings) it returns ("", value).
func splitOperator(s string) (string, string) {
	for _, op := range []string{"<=", ">=", "==", "<", ">", "="} {
		if strings.HasPrefix(s, op) {
			return op, strings.TrimSpace(strings.TrimPrefix(s, op))
		}
	}
	return "", s
}

// isDMailName reports whether id has the shape of a D-Mail name. The
// guard is intentionally narrow: it only matches the documented D-Mail
// naming convention so it does not reject legitimate stable identifiers.
func isDMailName(id string) bool {
	return dmailNamePattern.MatchString(id)
}

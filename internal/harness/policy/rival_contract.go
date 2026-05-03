// Package policy rival_contract.go: Rival Contract v1 parser.
//
// Pure parsing utilities for the Rival Contract v1 format. The parser is
// deterministic, performs no I/O, and never invokes an LLM. Consumers in
// sightjack, paintress, amadeus, and dominator are expected to keep the
// canonical type and function names in sync via copy-sync until duplicate
// maintenance becomes a real cost.
//
// Refs: refs/plans/2026-05-03-rival-contract-v1.md
package policy

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
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

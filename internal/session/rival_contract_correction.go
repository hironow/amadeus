// Package session — rival_contract_correction.go: composes corrective
// D-Mail bodies that cite the current Rival Contract v1 revision.
//
// Two routing cases are distinguishable from the body shape alone:
//
//   - "## Violated Contract"  — implementation-feedback body: the merged
//     code violates a contract Boundary or Evidence item. Phonewave
//     routes the D-Mail to paintress; the section explains what was
//     violated and at which contract revision.
//
//   - "## Contract Amendments" — design-feedback body: the contract is
//     outdated or underspecified. Phonewave routes the D-Mail to
//     sightjack; the section lists proposed amendments per canonical
//     section. Phase 5 (amendment loop) will deterministically extract
//     these bullets back out of design-feedback D-Mails to drive the
//     sightjack nextgen path.
//
// The composer is a pure string-shaping helper. It does no I/O, calls no
// LLM, and never modifies the supplied detail string apart from
// appending well-formed Markdown sections.
package session

import (
	"fmt"
	"strings"

	"github.com/hironow/amadeus/internal/domain"
)

// appendRivalContractSections decides which Rival Contract v1 citation
// section (if any) to attach to a corrective D-Mail body based on the
// outgoing kind and the candidate category, given the current contract
// context. It is the wiring helper between divergence-meter output
// (per-candidate Category) and the deterministic corrective composer.
//
// Routing rules in v1 mirror the plan §"Phase 3 PR" production
// behavior:
//
//   - implementation-feedback + current contract present →
//     "## Violated Contract" with a brief reason derived from the
//     candidate description.
//   - design-feedback + current contract present →
//     "## Contract Amendments" with a single (unspecified)-section
//     amendment carrying the candidate description as suggestion.
//
// When the current contract is nil, the helper is a no-op (graceful
// degradation: archives without Rival Contract v1 specs see no change).
func appendRivalContractSections(body string, kind domain.DMailKind, candidate domain.ClaudeDMailCandidate, current *domain.RivalContractContext) string {
	if current == nil {
		return body
	}
	switch kind {
	case domain.KindImplFeedback:
		citation := domain.RivalContractCitation{
			ContractID: current.ContractID,
			Revision:   current.Revision,
			Reason:     candidate.Description,
		}
		return composeCorrectiveBodyWithContract(body, kind, current, &citation, nil)
	case domain.KindDesignFeedback:
		amend := domain.RivalContractAmendment{
			Section:    "",
			Suggestion: candidate.Description,
		}
		return composeCorrectiveBodyWithContract(body, kind, current, nil, []domain.RivalContractAmendment{amend})
	default:
		return body
	}
}

// composeCorrectiveBodyWithContract returns a corrective D-Mail body
// whose Markdown shape carries Rival Contract v1 citations when
// applicable. When `current` is nil and no citation/amendments are
// supplied, the original detail string is returned verbatim (graceful
// degradation for archives that contain no Rival Contract v1 specs).
//
// The function intentionally does not enforce kind/citation alignment
// (e.g. "Violated Contract on a design-feedback D-Mail") — the caller
// already routes the D-Mail by kind and the body just describes the
// reason. Decoupling keeps the helper deterministic and testable.
func composeCorrectiveBodyWithContract(detail string, kind domain.DMailKind, current *domain.RivalContractContext, citation *domain.RivalContractCitation, amendments []domain.RivalContractAmendment) string {
	// No contract context AND no citations to attach: return verbatim.
	if current == nil && citation == nil && len(amendments) == 0 {
		return detail
	}

	var sb strings.Builder
	sb.WriteString(detail)

	if citation != nil {
		writeViolatedContractSection(&sb, citation, current)
	}
	if len(amendments) > 0 {
		writeContractAmendmentsSection(&sb, current, amendments)
	}

	// Defensive: silence unused parameter warnings while leaving room for
	// kind-aware shaping in future revisions (e.g. severity hint per kind).
	_ = kind

	return sb.String()
}

// writeViolatedContractSection appends a "## Violated Contract" section
// that cites the current contract id and revision and explains the
// violation reason. When the citation supplies its own contract id, it
// takes precedence over the projected current contract — this lets the
// caller cite a specific revision even after the archive has rolled
// forward.
func writeViolatedContractSection(sb *strings.Builder, citation *domain.RivalContractCitation, current *domain.RivalContractContext) {
	contractID := citation.ContractID
	revision := citation.Revision
	if contractID == "" && current != nil {
		contractID = current.ContractID
		revision = current.Revision
	}

	ensureBlankLine(sb)
	sb.WriteString("## Violated Contract\n\n")
	if contractID != "" {
		fmt.Fprintf(sb, "- Contract: `%s`", contractID)
		if revision > 0 {
			fmt.Fprintf(sb, " (revision %d)", revision)
		}
		sb.WriteString("\n")
	}
	if citation.Section != "" {
		fmt.Fprintf(sb, "- Section: %s\n", citation.Section)
	}
	if citation.Reason != "" {
		fmt.Fprintf(sb, "- Reason: %s\n", citation.Reason)
	}
}

// writeContractAmendmentsSection appends a "## Contract Amendments"
// section with one bullet per proposed amendment. Phase 5 amendment
// loop relies on the canonical section header and bullet shape to
// reliably re-parse the suggestions; do not change the format without
// updating the Phase 5 extractor in tandem.
func writeContractAmendmentsSection(sb *strings.Builder, current *domain.RivalContractContext, amendments []domain.RivalContractAmendment) {
	ensureBlankLine(sb)
	sb.WriteString("## Contract Amendments\n\n")
	if current != nil && current.ContractID != "" {
		fmt.Fprintf(sb, "- Contract: `%s`", current.ContractID)
		if current.Revision > 0 {
			fmt.Fprintf(sb, " (revision %d)", current.Revision)
		}
		sb.WriteString("\n")
	}
	for _, a := range amendments {
		section := strings.TrimSpace(a.Section)
		fmt.Fprintf(sb, "- Amend %s: %s", contractAmendmentSection(section), strings.TrimSpace(a.Suggestion))
		if rationale := strings.TrimSpace(a.Rationale); rationale != "" {
			fmt.Fprintf(sb, " (rationale: %s)", rationale)
		}
		sb.WriteString("\n")
	}
}

// contractAmendmentSection returns the canonical section label for an
// amendment bullet. When the caller did not specify a section, the
// label "(unspecified)" is used so the bullet still parses cleanly in
// the Phase 5 amendment loop.
func contractAmendmentSection(section string) string {
	if section != "" {
		return section
	}
	return "(unspecified)"
}

// ensureBlankLine writes the right amount of whitespace so the next
// section header is preceded by a blank line, matching the rest of the
// corrective D-Mail body shape (ADR Violations etc.).
func ensureBlankLine(sb *strings.Builder) {
	current := sb.String()
	switch {
	case current == "":
		return
	case strings.HasSuffix(current, "\n\n"):
		return
	case strings.HasSuffix(current, "\n"):
		sb.WriteString("\n")
	default:
		sb.WriteString("\n\n")
	}
}

// Package session — rival_contract_archive.go: session-level adapter that
// projects the Rival Contract v1 D-Mail archive into a current-contract
// view used by amadeus divergence prompts and corrective bodies.
//
// The pure projection logic (heading extraction, metadata parsing, revision
// selection, conflict detection) lives in internal/harness/policy/
// rival_contract.go and is re-exported through internal/harness for
// session adapters. This file adds only the I/O glue (load all archived
// D-Mails) and a small mapper from the harness-level CurrentContract to
// the prompt-shaped domain.RivalContractContext.
//
// Refs: refs/plans/2026-05-03-rival-contract-v1.md, Phase 3 amadeus.
package session

import (
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness"
)

// ReadRivalContractsFromArchive loads all archived D-Mails from the
// projection store and returns the deterministic Rival Contract v1
// projection as (current contracts, conflicts).
//
// The archive read path is the same one used by amadeus convergence and
// inbox flows (ProjectionStore.LoadAllDMails); we deliberately reuse it
// instead of inventing a parallel reader so contract selection sees the
// exact same D-Mail set as the rest of amadeus.
//
// Errors are surfaced for I/O failures only. Empty archives (no Rival
// Contract v1 specifications at all) return empty slices and nil — the
// caller is expected to degrade gracefully.
func (s *ProjectionStore) ReadRivalContractsFromArchive() ([]harness.CurrentContract, []harness.ContractConflict, error) {
	dmails, err := s.LoadAllDMails()
	if err != nil {
		return nil, nil, fmt.Errorf("load dmails for rival contract projection: %w", err)
	}
	current, conflicts, err := ReadRivalContractsFromDMails(dmails)
	return current, conflicts, err
}

// ReadRivalContractsFromDMails is the pure form of
// ReadRivalContractsFromArchive — it takes an in-memory D-Mail slice and
// returns the same projection. It exists so tests can drive the projection
// without a filesystem store and so other callers that already hold a
// D-Mail slice (e.g. convergence detection) can reuse the result.
//
// The function never returns a non-nil error in v1 — it only delegates
// to the deterministic harness projection which itself has no failure
// modes. The error return is preserved for forward compatibility (e.g.
// when contract validation gains expensive checks that may fail).
func ReadRivalContractsFromDMails(dmails []domain.DMail) ([]harness.CurrentContract, []harness.ContractConflict, error) {
	current, conflicts := harness.ProjectCurrentContracts(dmails)
	return current, conflicts, nil
}

// CurrentContractForPrompt selects a single CurrentContract suitable for
// injecting into divergence prompts and maps it onto the prompt-shaped
// domain.RivalContractContext. It returns nil when no current contract
// is available (legacy archives, conflicting revisions, etc.).
//
// Selection rules in v1 are intentionally narrow:
//
//   - If the projection produced exactly one current contract, use it.
//   - If the projection produced more than one (different contract_ids
//     in the archive), return nil so the prompt doesn't silently pick
//     between unrelated contracts. Phase 5 amendment loop will narrow
//     by wave/issue/target context; for now amadeus operates on at most
//     one Rival Contract v1 contract per archive in this Phase.
//
// Conflicts always result in nil — the corrective routing path is the
// canonical place where conflicts surface (design-feedback or
// convergence), not the divergence prompt.
func CurrentContractForPrompt(current []harness.CurrentContract, conflicts []harness.ContractConflict) *domain.RivalContractContext {
	if len(conflicts) > 0 {
		return nil
	}
	if len(current) != 1 {
		return nil
	}
	cc := current[0]
	ctx := domain.RivalContractContext{
		ContractID: cc.Metadata.ID,
		Revision:   cc.Metadata.Revision,
		Title:      cc.Contract.Title,
		Intent:     cc.Contract.Intent,
		Decisions:  cc.Contract.Decisions,
		Boundaries: cc.Contract.Boundaries,
		Evidence:   cc.Contract.Evidence,
	}
	if !ctx.HasContent() {
		return nil
	}
	return &ctx
}

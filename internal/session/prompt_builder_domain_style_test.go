// white-box-reason: tests unexported buildDiffCheckPrompt / buildFullCheckPrompt
// renderers and asserts byte-identical legacy behavior when DomainStyle is unset.
package session

import (
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/harness"
)

// Phase 1.1A — Divergence prompt DomainStyle branch tests.
//
// Plan: refs/plans/2026-05-03-rival-contract-v1-1-extensions.md "Phase 1.1A"
//
// When the current Rival Contract's metadata.domain_style is
// "event-sourced", the divergence prompt must include a canonical
// command/event/read-model glossary preamble. For missing/generic/mixed
// values, the prompt MUST be bit-identical to the legacy v1 output so v1
// archives keep working unchanged.
//
// Glossary marker: a fixed sentinel phrase is asserted, not the full text,
// so internal rewording of the glossary stays a Phase-3-deliverable
// concern; cross-tool consumers only depend on the marker.

const eventSourcedGlossaryMarker = "event-sourcing glossary"

func sampleEventSourcedContext(style string) *domain.RivalContractContext {
	return &domain.RivalContractContext{
		ContractID:  "wallet-authorize",
		Revision:    1,
		Title:       "Authorize purchase command",
		Intent:      "Reject purchase commands when balance is insufficient.",
		Decisions:   "Authorize inside the wallet aggregate.",
		Boundaries:  "Do not mutate balance directly.",
		Evidence:    "test: just test",
		DomainStyle: style,
	}
}

// TestBuildDiffCheckPrompt_DomainStyleSwitchesGlossary asserts that the
// event-sourced glossary preamble appears only when DomainStyle ==
// "event-sourced", and is absent for every other value (including the
// empty string, "generic", and "mixed"). en + ja both branched.
func TestBuildDiffCheckPrompt_DomainStyleSwitchesGlossary(t *testing.T) {
	cases := []struct {
		name       string
		lang       string
		style      string
		wantMarker bool
	}{
		{name: "en/event-sourced/present", lang: "en", style: harness.DomainStyleEventSourced, wantMarker: true},
		{name: "en/generic/absent", lang: "en", style: harness.DomainStyleGeneric, wantMarker: false},
		{name: "en/mixed/absent", lang: "en", style: harness.DomainStyleMixed, wantMarker: false},
		{name: "en/empty/absent", lang: "en", style: "", wantMarker: false},
		{name: "ja/event-sourced/present", lang: "ja", style: harness.DomainStyleEventSourced, wantMarker: true},
		{name: "ja/generic/absent", lang: "ja", style: harness.DomainStyleGeneric, wantMarker: false},
		{name: "ja/mixed/absent", lang: "ja", style: harness.DomainStyleMixed, wantMarker: false},
		{name: "ja/empty/absent", lang: "ja", style: "", wantMarker: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given a current Rival Contract with the specified DomainStyle.
			params := domain.DiffCheckParams{
				EvalDir:         "/repo/.gate/.run/eval",
				CurrentContract: sampleEventSourcedContext(tc.style),
			}

			// when
			prompt, err := buildDiffCheckPrompt(tc.lang, params)

			// then
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			has := strings.Contains(prompt, eventSourcedGlossaryMarker)
			if has != tc.wantMarker {
				t.Errorf("DomainStyle=%q lang=%q: glossary marker present=%v, want=%v",
					tc.style, tc.lang, has, tc.wantMarker)
			}
		})
	}
}

// TestBuildDiffCheckPrompt_LegacyV1IdenticalToV1 is the regression guard
// that protects v1 archives. A contract with no DomainStyle (legacy v1)
// MUST produce a prompt byte-identical to a contract with DomainStyle ==
// "generic" — both are the legacy default.
func TestBuildDiffCheckPrompt_LegacyV1IdenticalToV1(t *testing.T) {
	for _, lang := range []string{"en", "ja"} {
		t.Run(lang, func(t *testing.T) {
			// given two contracts that should render identically: legacy v1
			// (DomainStyle == "") and explicit v1.1 generic.
			legacyParams := domain.DiffCheckParams{
				EvalDir:         "/repo/.gate/.run/eval",
				CurrentContract: sampleEventSourcedContext(""),
			}
			genericParams := domain.DiffCheckParams{
				EvalDir:         "/repo/.gate/.run/eval",
				CurrentContract: sampleEventSourcedContext(harness.DomainStyleGeneric),
			}

			// when
			legacyPrompt, errL := buildDiffCheckPrompt(lang, legacyParams)
			genericPrompt, errG := buildDiffCheckPrompt(lang, genericParams)

			// then both must build without error and produce identical output.
			if errL != nil {
				t.Fatalf("legacy prompt error: %v", errL)
			}
			if errG != nil {
				t.Fatalf("generic prompt error: %v", errG)
			}
			if legacyPrompt != genericPrompt {
				t.Errorf("legacy v1 (DomainStyle=\"\") and v1.1 generic must be byte-identical;\nlen(legacy)=%d len(generic)=%d",
					len(legacyPrompt), len(genericPrompt))
			}
			if strings.Contains(legacyPrompt, eventSourcedGlossaryMarker) {
				t.Errorf("legacy v1 prompt must NOT contain event-sourcing glossary marker")
			}
		})
	}
}

// TestBuildFullCheckPrompt_DomainStyleSwitchesGlossary asserts the same
// branching behavior on the full-check prompt path (en + ja).
func TestBuildFullCheckPrompt_DomainStyleSwitchesGlossary(t *testing.T) {
	cases := []struct {
		name       string
		lang       string
		style      string
		wantMarker bool
	}{
		{name: "en/event-sourced/present", lang: "en", style: harness.DomainStyleEventSourced, wantMarker: true},
		{name: "en/generic/absent", lang: "en", style: harness.DomainStyleGeneric, wantMarker: false},
		{name: "en/mixed/absent", lang: "en", style: harness.DomainStyleMixed, wantMarker: false},
		{name: "en/empty/absent", lang: "en", style: "", wantMarker: false},
		{name: "ja/event-sourced/present", lang: "ja", style: harness.DomainStyleEventSourced, wantMarker: true},
		{name: "ja/generic/absent", lang: "ja", style: harness.DomainStyleGeneric, wantMarker: false},
		{name: "ja/mixed/absent", lang: "ja", style: harness.DomainStyleMixed, wantMarker: false},
		{name: "ja/empty/absent", lang: "ja", style: "", wantMarker: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// given a full-check params with the specified DomainStyle.
			params := domain.FullCheckParams{
				EvalDir:         "/repo/.gate/.run/eval",
				CurrentContract: sampleEventSourcedContext(tc.style),
			}

			// when
			prompt, err := buildFullCheckPrompt(tc.lang, params)

			// then
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			has := strings.Contains(prompt, eventSourcedGlossaryMarker)
			if has != tc.wantMarker {
				t.Errorf("DomainStyle=%q lang=%q: glossary marker present=%v, want=%v",
					tc.style, tc.lang, has, tc.wantMarker)
			}
		})
	}
}

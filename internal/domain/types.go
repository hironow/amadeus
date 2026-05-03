package domain

import "time"

// shutdownKey is the context key for the outer (shutdown) context.
type shutdownKey struct{}

// ShutdownKey is used to embed the outer context in workCtx via context.WithValue.
// Commands retrieve it to get a context that survives workCtx cancellation.
var ShutdownKey = shutdownKey{}

// IndexEntry represents one line in the archive index JSONL file.
type IndexEntry struct { // nosemgrep: structure.multiple-exported-structs-go -- amadeus types family (IndexEntry/HandoverState/MergedPR) is a cohesive domain types set [permanent]
	Timestamp string `json:"ts"`
	Operation string `json:"op"`
	Issue     string `json:"issue"`
	Status    string `json:"status"`
	Tool      string `json:"tool"`
	Path      string `json:"path"`
	Summary   string `json:"summary"`
}

// HandoverState captures in-progress work state when an operation is
// interrupted by a signal. The struct is pure data — no context, no I/O.
type HandoverState struct { // nosemgrep: first-class-collection.raw-slice-field-domain-go,structure.multiple-exported-structs-go — pure signal-interrupt data carrier; Completed/Remaining are transient in-flight state lists, not domain invariant collections; amadeus types family cohesive set; see IndexEntry [permanent]
	Tool         string // "amadeus"
	Operation    string // "divergence"
	Timestamp    time.Time
	InProgress   string            // Current task description
	Completed    []string          // What was done
	Remaining    []string          // What's left
	PartialState map[string]string // Tool-specific state (key=label, value=detail)
}

// MergedPR represents a merged pull request.
type MergedPR struct { // nosemgrep: structure.multiple-exported-structs-go -- amadeus types family cohesive set; see IndexEntry [permanent]
	Number string
	Title  string
}

// ProviderErrorKind classifies the type of provider error.
type ProviderErrorKind int

const (
	// ProviderErrorNone indicates no provider-level error (normal failure).
	ProviderErrorNone ProviderErrorKind = iota
	// ProviderErrorRateLimit indicates a rate limit was hit.
	ProviderErrorRateLimit
	// ProviderErrorServer indicates a server-side error (5xx).
	ProviderErrorServer
)

// ProviderErrorInfo holds the classified result of a provider error.
type ProviderErrorInfo struct {
	Kind    ProviderErrorKind
	ResetAt time.Time // parsed reset time (zero if unknown)
}

// IsTrip returns true if the error should trip a circuit breaker.
func (i ProviderErrorInfo) IsTrip() bool {
	return i.Kind != ProviderErrorNone
}

// RivalContractContext is the minimum projection of a current Rival
// Contract v1 specification that prompt and corrective-body builders need
// in order to produce contract-aware output. It is decoupled from the
// harness/policy.RivalContract type so the domain layer (and thus the
// prompt parameter structs) does not depend on harness internals.
//
// Fields mirror the four contract-aware sections used by the divergence
// prompts (Intent / Decisions / Boundaries / Evidence) plus enough
// metadata to cite the current contract revision in corrective bodies.
//
// DomainStyle carries the optional Rival Contract v1.1 metadata.domain_style
// enum value forward to the prompt renderer so divergence prompts can
// branch their glossary preamble (event-sourced vs generic/mixed/empty).
// The session-layer adapter copies the value verbatim from the parsed
// harness.RivalContractMetadata; consumers MUST treat the empty string as
// the legacy v1 default (semantically equivalent to "generic").
type RivalContractContext struct { // nosemgrep: structure.multiple-exported-structs-go -- Rival Contract v1 prompt-context family (RivalContractContext/RivalContractCitation/RivalContractAmendment) is a cohesive corrective-body schema; splitting would fragment the contract-aware surface [permanent]
	ContractID  string
	Revision    int
	Title       string
	Intent      string
	Decisions   string
	Boundaries  string
	Evidence    string
	DomainStyle string
}

// HasContent reports whether the context carries any contract-aware text
// worth rendering. Used by prompt builders to skip the section entirely
// when graceful-degradation legacy specs are the only archive data.
func (c RivalContractContext) HasContent() bool {
	return c.Intent != "" || c.Decisions != "" || c.Boundaries != "" || c.Evidence != ""
}

// RivalContractCitation describes a single contract Boundary or Evidence
// item that the merged code violates. Used by amadeus to render a
// "## Violated Contract" section in implementation-feedback bodies.
type RivalContractCitation struct { // nosemgrep: structure.multiple-exported-structs-go -- Rival Contract v1 prompt-context family cohesive set; see RivalContractContext [permanent]
	ContractID string
	Revision   int
	Section    string // optional: which canonical section was violated (e.g. "Boundaries").
	Reason     string
}

// RivalContractAmendment describes a single proposed change to the
// current contract. Used by amadeus to render a "## Contract Amendments"
// section in design-feedback bodies. Phase 5 (amendment loop) parses
// these bullets back out of design-feedback D-Mails to drive nextgen.
type RivalContractAmendment struct { // nosemgrep: structure.multiple-exported-structs-go -- Rival Contract v1 prompt-context family cohesive set; see RivalContractContext [permanent]
	Section    string // canonical section name being amended (e.g. "Boundaries", "Evidence").
	Suggestion string
	Rationale  string
}

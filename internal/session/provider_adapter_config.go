package session

// ProviderAdapterConfig holds the class-wide configuration for creating a
// provider adapter. All AI coding tools accept this shape in NewTrackedRunner.
// Role-specific policies (retry, lazy singleton) are separate from this contract.
//
// This struct is copy-synced and checksum-gated across sightjack/paintress/amadeus.
// Assembly helpers are tool-specific and live in provider_adapter_helpers.go (not gated).
type ProviderAdapterConfig struct { // nosemgrep: domain-primitives.public-string-field-go -- S0037 canonical config DTO synced across 3 AI coding tools; Cmd/Model/BaseDir/ToolName are config identifiers, not domain primitives [permanent]
	Cmd        string // provider CLI command (e.g. "claude")
	Model      string // model name (e.g. "opus")
	TimeoutSec int    // per-invocation timeout (0 = context deadline only)
	BaseDir    string // repository root (state dir parent)
	ToolName   string // tool identifier for stream events
}

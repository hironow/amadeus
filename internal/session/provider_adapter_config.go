package session

// ProviderAdapterConfig holds the class-wide configuration for creating a
// provider adapter. All AI coding tools accept this shape in NewTrackedRunner.
// Role-specific policies (retry, lazy singleton) are separate from this contract.
type ProviderAdapterConfig struct {
	Cmd        string // provider CLI command (e.g. "claude")
	Model      string // model name (e.g. "opus")
	TimeoutSec int    // per-invocation timeout (0 = context deadline only)
	BaseDir    string // repository root (state dir parent)
	ToolName   string // tool identifier for stream events
}

// AdapterConfigFromAmadeusFields extracts ProviderAdapterConfig from individual fields.
func AdapterConfigFromAmadeusFields(cmd, model string, timeoutSec int, repoDir string) ProviderAdapterConfig {
	return ProviderAdapterConfig{
		Cmd:        cmd,
		Model:      model,
		TimeoutSec: timeoutSec,
		BaseDir:    repoDir,
		ToolName:   "amadeus",
	}
}

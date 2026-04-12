package session

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

package usecase

import (
	"context"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// ReadCrossRepoSnapshot collects divergence snapshots from all tools and returns
// an aggregated CrossRepoSnapshot.
func ReadCrossRepoSnapshot(ctx context.Context, toolStateDirs map[domain.ToolName]string, reader port.CrossRepoReader) (domain.CrossRepoSnapshot, error) {
	snapshots := make([]domain.ToolSnapshot, 0, len(domain.AllTools))

	for _, tool := range domain.AllTools {
		stateDir, ok := toolStateDirs[tool]
		if !ok {
			// Tool not configured — report as unavailable
			snapshots = append(snapshots, domain.ToolSnapshot{
				Tool:      tool,
				Available: false,
			})
			continue
		}

		snap, err := reader.ReadToolSnapshot(ctx, tool, stateDir)
		if err != nil {
			// Non-fatal: mark as unavailable and continue
			snapshots = append(snapshots, domain.ToolSnapshot{
				Tool:      tool,
				Available: false,
			})
			continue
		}
		snapshots = append(snapshots, snap)
	}

	return domain.NewCrossRepoSnapshot(snapshots, time.Now().UTC()), nil
}

package port

import (
	"context"

	"github.com/hironow/amadeus/internal/domain"
)

// HandoverWriter writes a handover document summarizing interrupted work state.
type HandoverWriter interface {
	WriteHandover(ctx context.Context, stateDir string, state domain.HandoverState) error
}

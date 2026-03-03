package usecase

import (
	"github.com/hironow/amadeus/internal/port"
	"github.com/hironow/amadeus/internal/session"
)

// NewCmdApprover creates an Approver that shells out to the given command template.
func NewCmdApprover(cmdTemplate string) port.Approver {
	return session.NewCmdApprover(cmdTemplate)
}

// NewCmdNotifier creates a Notifier that shells out to the given command template.
func NewCmdNotifier(cmdTemplate string) port.Notifier {
	return session.NewCmdNotifier(cmdTemplate)
}

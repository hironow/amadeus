package session

// white-box-reason: bridge constructor: exposes unexported symbols for external test packages

import (
	"context"
	"database/sql"
	"os/exec"
)

// NewLocalNotifierForTest creates a LocalNotifier with test overrides.
func NewLocalNotifierForTest(osName string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *LocalNotifier {
	return &LocalNotifier{forceOS: osName, cmdFactory: factory}
}

// NewCmdNotifierForTest creates a CmdNotifier with a test command factory.
func NewCmdNotifierForTest(cmdTemplate string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *CmdNotifier {
	return &CmdNotifier{cmdTemplate: cmdTemplate, cmdFactory: factory}
}

// NewCmdApproverForTest creates a CmdApprover with a test command factory.
func NewCmdApproverForTest(cmdTemplate string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *CmdApprover {
	return &CmdApprover{cmdTemplate: cmdTemplate, cmdFactory: factory}
}

// ExportParseMergedPRs exposes parseMergedPRs for external tests.
var ExportParseMergedPRs = parseMergedPRs

// ExportHookMarkerBegin exposes hookMarkerBegin for external tests.
var ExportHookMarkerBegin = hookMarkerBegin

// ExportHookMarkerEnd exposes hookMarkerEnd for external tests.
var ExportHookMarkerEnd = hookMarkerEnd

// DBForTest returns the underlying database connection for testing.
// Only available in test builds.
func (s *SQLiteOutboxStore) DBForTest() *sql.DB { return s.db }

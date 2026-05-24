package session

// white-box-reason: bridge constructor: exposes unexported symbols for external test packages

import (
	"context"
	"database/sql"
	"os/exec"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

// ExportParsePRReviewJSON exposes parsePRReviewJSON for external tests.
var ExportParsePRReviewJSON = parsePRReviewJSON

// ExportParseGhPRListOutput exposes parseGhPRListOutput for external tests.
var ExportParseGhPRListOutput = parseGhPRListOutput

// DBForTest returns the underlying database connection for testing.
// Only available in test builds.
func (s *SQLiteOutboxStore) DBForTest() *sql.DB { return s.db }

// ExportCloseReadyIssues exposes closeReadyIssues for external tests.
func ExportCloseReadyIssues(a *Amadeus, ctx context.Context, readyLabel string) {
	a.closeReadyIssues(ctx, readyLabel)
}

// ExportSetMaxWaitDuration overrides maxWaitDuration and returns a cleanup function.
func ExportSetMaxWaitDuration(d time.Duration) func() {
	old := maxWaitDuration
	maxWaitDuration = d
	return func() { maxWaitDuration = old }
}

// ExportBuildIsolationFlags exposes BuildClaudeIsolationFlags for contract testing.
func ExportBuildIsolationFlags(configBase string) []string {
	return BuildClaudeIsolationFlags(configBase)
}

// ExportFindSkillsRefDir exposes findSkillsRefDir for external tests.
func ExportFindSkillsRefDir(baseDir string) string { return findSkillsRefDir(baseDir) }

// ExportSetupTestTracer exposes setupTestTracer for external test packages.
func ExportSetupTestTracer(t *testing.T) *tracetest.InMemoryExporter {
	return setupTestTracer(t)
}

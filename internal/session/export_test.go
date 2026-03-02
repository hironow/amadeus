package session

import (
	"context"
	"database/sql"
	"os/exec"
)

// NewLocalNotifierForTest creates a LocalNotifier with test overrides.
func NewLocalNotifierForTest(osName string, factory func(ctx context.Context, name string, args ...string) *exec.Cmd) *LocalNotifier {
	return &LocalNotifier{forceOS: osName, cmdFactory: factory}
}

// DBForTest returns the underlying database connection for testing.
// Only available in test builds.
func (s *SQLiteOutboxStore) DBForTest() *sql.DB { return s.db }

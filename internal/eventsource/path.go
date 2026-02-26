package eventsource

import "path/filepath"

// EventsDir returns the path to the events directory under .gate/.
func EventsDir(baseDir string) string {
	return filepath.Join(baseDir, "events")
}

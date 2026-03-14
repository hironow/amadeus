//go:build windows

package session

// flockLock is a no-op on Windows. File locking is not critical for CLI tools
// where concurrent index writes are rare. The temp+rename strategy in Rebuild
// provides basic atomicity regardless.
func flockLock(_ uintptr) error {
	return nil
}

// flockUnlock is a no-op on Windows.
func flockUnlock(_ uintptr) error {
	return nil
}

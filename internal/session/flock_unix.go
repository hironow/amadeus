//go:build !windows

package session

import "syscall"

// flockLock acquires an exclusive lock on the file descriptor.
func flockLock(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_EX)
}

// flockUnlock releases the lock on the file descriptor.
func flockUnlock(fd uintptr) error {
	return syscall.Flock(int(fd), syscall.LOCK_UN)
}

//go:build !windows

package session

func shellName() string { return "sh" }
func shellFlag() string { return "-c" }

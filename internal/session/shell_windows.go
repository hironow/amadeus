//go:build windows

package session

func shellName() string { return "cmd" }
func shellFlag() string { return "/c" }

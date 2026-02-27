package session

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// CmdNotifier executes a user-provided shell command for notifications.
// The template may contain {title} and {message} placeholders.
type CmdNotifier struct {
	cmdTemplate string
}

func NewCmdNotifier(cmdTemplate string) *CmdNotifier {
	return &CmdNotifier{cmdTemplate: cmdTemplate}
}

func (n *CmdNotifier) Notify(ctx context.Context, title, message string) error {
	expanded := strings.ReplaceAll(n.cmdTemplate, "{title}", shellQuote(title))
	expanded = strings.ReplaceAll(expanded, "{message}", shellQuote(message))
	return exec.CommandContext(ctx, shellName(), shellFlag(), expanded).Run()
}

// LocalNotifier sends desktop notifications using OS-native commands.
type LocalNotifier struct{}

func (n *LocalNotifier) Notify(ctx context.Context, title, message string) error {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q sound name "Funk"`, message, title)
		return exec.CommandContext(ctx, "osascript", "-e", script).Run()
	case "linux":
		return exec.CommandContext(ctx, "notify-send", title, message).Run()
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// shellQuote wraps s in single quotes for safe interpolation into sh -c commands.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

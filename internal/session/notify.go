package session

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/port"
)

// psEscapeSingleQuote escapes single quotes for PowerShell single-quoted strings.
func psEscapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

// cmdFactoryFunc creates an *exec.Cmd -- injectable for testing.
type cmdFactoryFunc func(ctx context.Context, name string, args ...string) *exec.Cmd

func defaultCmdFactory(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// CmdNotifier executes a user-provided shell command for notifications.
// The template may contain {title} and {message} placeholders.
type CmdNotifier struct {
	cmdTemplate string
	cmdFactory  cmdFactoryFunc
}

func NewCmdNotifier(cmdTemplate string) *CmdNotifier {
	return &CmdNotifier{cmdTemplate: cmdTemplate}
}

func (n *CmdNotifier) factory() cmdFactoryFunc {
	if n.cmdFactory != nil {
		return n.cmdFactory
	}
	return defaultCmdFactory
}

const notifyTimeout = 30 * time.Second

func (n *CmdNotifier) Notify(ctx context.Context, title, message string) error {
	if n.cmdTemplate == "" {
		return fmt.Errorf("notify: empty command template")
	}
	ctx, cancel := context.WithTimeout(ctx, notifyTimeout)
	defer cancel()
	expanded := strings.ReplaceAll(n.cmdTemplate, "{title}", ShellQuote(title))
	expanded = strings.ReplaceAll(expanded, "{message}", ShellQuote(message))
	return n.factory()(ctx, shellName(), shellFlag(), expanded).Run()
}

// LocalNotifier sends desktop notifications using OS-native commands.
type LocalNotifier struct {
	forceOS    string         // override runtime.GOOS for testing
	cmdFactory cmdFactoryFunc // override exec.CommandContext for testing
}

func (n *LocalNotifier) os() string {
	if n.forceOS != "" {
		return n.forceOS
	}
	return runtime.GOOS
}

func (n *LocalNotifier) factory() cmdFactoryFunc {
	if n.cmdFactory != nil {
		return n.cmdFactory
	}
	return defaultCmdFactory
}

func (n *LocalNotifier) Notify(ctx context.Context, title, message string) error {
	factory := n.factory()
	switch n.os() {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q sound name "Funk"`, message, title)
		return factory(ctx, "osascript", "-e", script).Run()
	case "linux":
		return factory(ctx, "notify-send", title, message).Run()
	case "windows":
		script := fmt.Sprintf(
			`Add-Type -AssemblyName System.Windows.Forms; `+
				`$n = New-Object System.Windows.Forms.NotifyIcon; `+
				`$n.Icon = [System.Drawing.SystemIcons]::Information; `+
				`$n.BalloonTipTitle = '%s'; `+
				`$n.BalloonTipText = '%s'; `+
				`$n.Visible = $true; `+
				`$n.ShowBalloonTip(5000)`,
			psEscapeSingleQuote(title), psEscapeSingleQuote(message),
		)
		return factory(ctx, "powershell", "-NoProfile", "-Command", script).Run()
	default:
		return port.ErrUnsupportedOS
	}
}

package session

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// StdinApprover prompts the human on a terminal and reads y/yes for approval.
// Empty input or any other response is treated as denial (safe default).
type StdinApprover struct {
	reader io.Reader
	writer io.Writer
}

// NewStdinApprover creates a StdinApprover with the given reader and writer.
func NewStdinApprover(r io.Reader, w io.Writer) *StdinApprover {
	return &StdinApprover{reader: r, writer: w}
}

func (a *StdinApprover) RequestApproval(ctx context.Context, message string) (bool, error) {
	if a.reader == nil {
		return false, nil
	}

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	default:
	}

	fmt.Fprintf(a.writer, "%s\nContinue? [y/N]: ", message)

	// Read in a goroutine so we can select on ctx.Done().
	// We intentionally do NOT close the reader on cancel — it may be
	// os.Stdin, and closing FD 0 would break subsequent reads in the
	// same process.
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		line, err := readLine(a.reader)
		ch <- result{line: line, err: err}
	}()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case r := <-ch:
		// Evaluate answer even on io.EOF — piped input may not end with newline.
		// Only deny on error if no content was read.
		answer := strings.TrimSpace(strings.ToLower(r.line))
		if answer == "" && r.err != nil {
			return false, nil
		}
		return answer == "y" || answer == "yes", nil
	}
}

// readLine reads one line from r without buffering ahead.
// It reads one byte at a time to avoid consuming data beyond the newline,
// which is critical when r is a shared reader (e.g. stdin).
func readLine(r io.Reader) (string, error) {
	var buf []byte
	b := make([]byte, 1)
	for {
		n, err := r.Read(b)
		if n > 0 {
			if b[0] == '\n' {
				return string(buf), nil
			}
			buf = append(buf, b[0])
		}
		if err != nil {
			return string(buf), err
		}
	}
}

// CmdApprover executes an external command for approval.
// Exit code 0 = approved, non-zero = denied.
// The template may contain a {message} placeholder.
type CmdApprover struct {
	cmdTemplate string
	cmdFactory  cmdFactoryFunc
}

func NewCmdApprover(cmdTemplate string) *CmdApprover {
	return &CmdApprover{cmdTemplate: cmdTemplate}
}

func (a *CmdApprover) factory() cmdFactoryFunc {
	if a.cmdFactory != nil {
		return a.cmdFactory
	}
	return defaultCmdFactory
}

func (a *CmdApprover) RequestApproval(ctx context.Context, message string) (bool, error) {
	if a.cmdTemplate == "" {
		return false, fmt.Errorf("approve: empty command template")
	}
	expanded := strings.ReplaceAll(a.cmdTemplate, "{message}", ShellQuote(message))
	cmd := a.factory()(ctx, shellName(), shellFlag(), expanded)
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

package session

import (
	"bufio"
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
	fmt.Fprintf(a.writer, "%s\nContinue? [y/N]: ", message)

	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)

	go func() {
		scanner := bufio.NewScanner(a.reader)
		if scanner.Scan() {
			ch <- result{line: scanner.Text()}
		} else {
			ch <- result{err: scanner.Err()}
		}
	}()

	select {
	case <-ctx.Done():
		return false, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return false, r.err
		}
		answer := strings.TrimSpace(strings.ToLower(r.line))
		return answer == "y" || answer == "yes", nil
	}
}

// CmdApprover executes an external command for approval.
// Exit code 0 = approved, non-zero = denied.
// The template may contain a {message} placeholder.
type CmdApprover struct {
	cmdTemplate string
}

func NewCmdApprover(cmdTemplate string) *CmdApprover {
	return &CmdApprover{cmdTemplate: cmdTemplate}
}

func (a *CmdApprover) RequestApproval(ctx context.Context, message string) (bool, error) {
	expanded := strings.ReplaceAll(a.cmdTemplate, "{message}", shellQuote(message))
	cmd := exec.CommandContext(ctx, shellName(), shellFlag(), expanded)
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

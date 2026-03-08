package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/hironow/amadeus/internal/domain"
)

// MonitorInbox starts monitoring the inbox directory for D-Mail files.
// Two-phase: 1) synchronous drain of existing files, 2) async fsnotify watch.
// Each D-Mail is archived + removed via ReceiveDMailFromInbox.
// Returns a channel that delivers parsed D-Mails. Channel closes on ctx cancel.
func MonitorInbox(ctx context.Context, root string, logger domain.Logger) (<-chan domain.DMail, error) {
	inboxDir := filepath.Join(root, "inbox")

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(inboxDir); err != nil {
		watcher.Close()
		return nil, err
	}

	// Phase 1: synchronous drain of existing files.
	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		watcher.Close()
		return nil, err
	}

	var initial []domain.DMail
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		dmail, err := ReceiveDMailFromInbox(root, e.Name())
		if err != nil {
			logger.Warn("inbox drain: skip %s: %v", e.Name(), err)
			continue
		}
		if dmail == nil {
			// Already archived (dedup).
			continue
		}
		logger.Info("inbox drain: received %s", dmail.Name)
		initial = append(initial, *dmail)
	}

	ch := make(chan domain.DMail, len(initial)+8)
	for _, dmail := range initial {
		ch <- dmail
	}

	// Phase 2: async fsnotify watch goroutine.
	go func() {
		defer close(ch)
		defer watcher.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Create|fsnotify.Write) == 0 {
					continue
				}
				if !strings.HasSuffix(event.Name, ".md") {
					continue
				}
				filename := filepath.Base(event.Name)
				dmail, err := ReceiveDMailFromInbox(root, filename)
				if err != nil {
					logger.Warn("inbox watch: skip %s: %v", filename, err)
					continue
				}
				if dmail == nil {
					continue
				}
				logger.Info("inbox watch: received %s", dmail.Name)
				select {
				case ch <- *dmail:
				case <-ctx.Done():
					return
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logger.Warn("inbox watcher error: %v", err)
			}
		}
	}()

	return ch, nil
}

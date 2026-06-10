package session_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/session"
)

// refs issue 0031: D-Mail emission via the transactional outbox.
// amadeus produces design-feedback / implementation-feedback /
// convergence (dmail-sendable manifest).

func TestMCPServer_DMail_StagesAndFlushesToOutbox(t *testing.T) {
	// given
	baseDir := t.TempDir()
	gateDir := filepath.Join(baseDir, ".gate")
	req := `{"jsonrpc":"2.0","id":40,"method":"tools/call","params":{"name":"dmail","arguments":{"kind":"implementation-feedback","name":"am-implfb-77","description":"PR 77 axis findings","body":"# Findings\n\ndependency violation","issues":["X-7"],"severity":"high"}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithGateDir(gateDir).WithRepoRoot(baseDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeReviewToolJSON(t, out.Bytes())
	if body["sent"] != true || body["persistence"] != "transactional-outbox" {
		t.Fatalf("dmail response mismatch: %v", body)
	}
	for _, sub := range []string{"outbox", "archive"} {
		path := filepath.Join(gateDir, sub, "am-implfb-77.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("flushed file missing in %s: %v", sub, err)
		}
		text := string(data)
		if !strings.Contains(text, "kind: implementation-feedback") || !strings.Contains(text, "dependency violation") {
			t.Errorf("flushed %s content mismatch:\n%s", sub, text)
		}
	}
}

func TestMCPServer_DMail_RejectsKindOutsideProducesSet(t *testing.T) {
	// given: specification is a valid kind but amadeus does not produce it
	baseDir := t.TempDir()
	gateDir := filepath.Join(baseDir, ".gate")
	req := `{"jsonrpc":"2.0","id":41,"method":"tools/call","params":{"name":"dmail","arguments":{"kind":"specification","name":"am-bad","description":"d","body":"b"}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithGateDir(gateDir).WithRepoRoot(baseDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeReviewToolJSON(t, out.Bytes())
	if body["sent"] != false {
		t.Fatalf("sent = %v, want false for non-produced kind", body["sent"])
	}
	if reason, _ := body["reason"].(string); !strings.Contains(reason, "produce") {
		t.Errorf("reason = %v, want produces-set explanation", body["reason"])
	}
}

func TestMCPServer_DMail_RejectsInvalidPayload(t *testing.T) {
	// given: missing description
	baseDir := t.TempDir()
	gateDir := filepath.Join(baseDir, ".gate")
	req := `{"jsonrpc":"2.0","id":42,"method":"tools/call","params":{"name":"dmail","arguments":{"kind":"convergence","name":"am-no-desc","body":"b"}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithGateDir(gateDir).WithRepoRoot(baseDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeReviewToolJSON(t, out.Bytes())
	if body["sent"] != false {
		t.Errorf("sent = %v, want false for invalid payload", body["sent"])
	}
}

func TestMCPServer_DMail_UninitializedWithoutRepoRoot(t *testing.T) {
	// given
	req := `{"jsonrpc":"2.0","id":43,"method":"tools/call","params":{"name":"dmail","arguments":{"kind":"convergence","name":"x","description":"d","body":"b"}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeReviewToolJSON(t, out.Bytes())
	if body["initialized"] != false {
		t.Errorf("initialized = %v, want false without repo root", body["initialized"])
	}
}

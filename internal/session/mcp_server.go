package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"github.com/hironow/amadeus/internal/usecase/port"
)

// MCPServer is a minimal stdio-based Model Context Protocol server
// scaffolded for the refs/issues/0027 jun15 MCP pivot (Phase 2b).
//
// This is a SKELETON: only the amadeus.ping health-check tool is
// exposed. Real tools (amadeus.next_review, amadeus.post_comment,
// amadeus.get_pr_status, ...) ship in subsequent commits on the
// feat/jun15-mcp-pivot branch.
//
// Wire it into a claude code interactive session via --mcp-config so
// inference stays on the human-initiated session's subscription quota
// rather than crossing into the Agent SDK credit pool that gates
// `claude -p` from 2026-06-15.
//
// Protocol: JSON-RPC 2.0 over stdio, one envelope per line. Stderr
// carries human-readable diagnostics (per the project stdout/stderr
// separation invariant). Pattern follows paintress Phase 1
// (ADR 0017) + sightjack Phase 2a (ADR 0018) + paintress Phase 3
// real impl (= 83cb3ca) WithContinent pattern.
//
// gateDir is the .gate/ state directory used to resolve event store /
// inbox / outbox paths. When empty, real-impl tools return
// uninitialized.
type MCPServer struct {
	in            io.Reader
	out           io.Writer
	logger        domain.Logger
	gateDir       string
	commentPoster port.CommentPoster
}

// NewMCPServer wires explicit I/O so tests can drive the server
// without subprocess overhead. Passing nil for logger uses NopLogger.
func NewMCPServer(in io.Reader, out io.Writer, logger domain.Logger) *MCPServer {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	return &MCPServer{in: in, out: out, logger: logger}
}

// WithGateDir sets the .gate state directory used by real-impl MCP
// tools to resolve event store paths. Returns s for chaining (=
// paintress.WithContinent / sightjack.WithBaseDir symmetric).
func (s *MCPServer) WithGateDir(gateDir string) *MCPServer {
	s.gateDir = gateDir
	return s
}

// WithCommentPoster wires the GitHub Comments API adapter used by
// amadeus.post_comment (refs/issues/0027 Phase 4 follow-up #3).
// When nil (= default), post_comment stays preview-only. cmd
// composition root injects a GhPRWriter in repo-scoped invocations.
func (s *MCPServer) WithCommentPoster(p port.CommentPoster) *MCPServer {
	s.commentPoster = p
	return s
}

// jsonrpcMessage is the minimum JSON-RPC 2.0 envelope this skeleton
// understands. Method-specific params decode on demand from
// Params (json.RawMessage).
type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve reads messages from in line-by-line and writes responses to
// out until ctx cancels or stdin closes. Per-message decode errors
// surface as JSON-RPC error responses; only stream-level read errors
// abort Serve.
func (s *MCPServer) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	// 4 MiB buffer to comfortably cover D-Mail bodies in later commits.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := s.handle(ctx, line); err != nil {
			s.logger.Warn("mcp server: handle: %v", err)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("mcp server: read stdin: %w", err)
	}
	return nil
}

func (s *MCPServer) handle(ctx context.Context, line []byte) error {
	var msg jsonrpcMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	switch msg.Method {
	case "tools/list":
		return s.respond(msg.ID, map[string]any{"tools": toolDescriptors()})
	case "tools/call":
		return s.handleToolsCall(ctx, msg)
	default:
		return s.respondError(msg.ID, -32601, fmt.Sprintf("method not implemented: %s", msg.Method))
	}
}

// handleToolsCall dispatches a single tools/call request and records
// MCP invocation metrics (mcp.tool.invocations counter +
// mcp.tool.duration histogram) for cost-monitoring verification post
// 2026-06-15 (refs/issues/0027 Phase 3 cost monitoring (a)).
func (s *MCPServer) handleToolsCall(ctx context.Context, msg jsonrpcMessage) error {
	start := time.Now()
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(msg.Params, &call); err != nil {
		platform.RecordMCPInvocation(ctx, "", "error", time.Since(start))
		return s.respondError(msg.ID, -32602, "invalid tools/call params")
	}

	status := "ok"
	var result map[string]any
	switch call.Name {
	case "amadeus.ping":
		result = textResult("pong")
	case "amadeus.next_review":
		result = realNextReview(ctx, s.gateDir, s.logger)
	case "amadeus.post_comment":
		result = realPostComment(ctx, s.commentPoster, call.Arguments)
	case "amadeus.get_pr_status":
		result = realGetPRStatus(ctx, s.gateDir, call.Arguments, s.logger)
	default:
		platform.RecordMCPInvocation(ctx, call.Name, "error", time.Since(start))
		return s.respondError(msg.ID, -32601, fmt.Sprintf("unknown tool: %s", call.Name))
	}

	err := s.respond(msg.ID, result)
	if err != nil {
		status = "error"
	}
	platform.RecordMCPInvocation(ctx, call.Name, status, time.Since(start))
	return err
}

// toolDescriptors returns the Phase 2b MVP tool set. Each entry pins
// the interface (name, description, inputSchema) so claude code
// clients see a stable contract. The handler bodies (stubNextReview /
// stubPostComment / stubGetPRStatus) are placeholders that ship in
// subsequent commits with real domain wiring.
func toolDescriptors() []map[string]any {
	return []map[string]any{
		{
			"name":        "amadeus.ping",
			"description": "Health check. Returns 'pong'.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "amadeus.next_review",
			"description": "Return latest CheckCompleted event + total check count + PRs evaluated in the most recent check. The session picks the PR with highest divergence to review next.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "amadeus.post_comment",
			"description": "Post a review comment to the given PR via the GitHub Comments API (= `gh pr comment`). When a CommentPoster is wired (Phase 4 #3), posted=true + persistence='github-comments-api'. Otherwise preview-only + persistence='preview-only'.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pr_number": map[string]any{"type": "integer"},
					"body":      map[string]any{"type": "string"},
				},
				"required": []any{"pr_number", "body"},
			},
		},
		{
			"name":        "amadeus.get_pr_status",
			"description": "Return per-PR check history (= filter CheckCompleted events where PRsEvaluated contains the given pr_number). Returns latest divergence + check_count + gate_denied flag + dmail_count.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pr_number": map[string]any{"type": "integer"},
				},
				"required": []any{"pr_number"},
			},
		},
	}
}

// textResult wraps a plain string into the MCP content envelope.
func textResult(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

// jsonResult marshals data as JSON and returns an MCP content envelope.
func jsonResult(data any) map[string]any {
	body, err := json.Marshal(data)
	if err != nil {
		return textResult(fmt.Sprintf(`{"error":"marshal failed: %v"}`, err))
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(body)}}}
}

// realNextReview reads CheckCompleted events from the .gate event
// store and returns the latest check + total check count + most
// recent PRsEvaluated. The session uses this to decide which PR to
// review next (= typically the highest-divergence one in the latest
// check's PRs).
//
// Pattern: paintress.next_issue (= 83cb3ca) symmetric copy.
func realNextReview(ctx context.Context, gateDir string, logger domain.Logger) map[string]any {
	if gateDir == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"reason":      "amadeus mcp gateDir not configured (start `amadeus mcp` from the project root)",
		})
	}
	store := NewEventStore(gateDir, logger)
	events, _, err := store.LoadAll(ctx)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": false,
			"reason":      fmt.Sprintf("event store load failed: %v", err),
			"gateDir":     gateDir,
		})
	}
	var latest *domain.CheckResult
	checkCount := 0
	for _, ev := range events {
		if ev.Type != domain.EventCheckCompleted {
			continue
		}
		checkCount++
		var data domain.CheckCompletedData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			continue
		}
		if latest == nil || data.Result.CheckedAt.After(latest.CheckedAt) { // nosemgrep: lod-excessive-dot-chain -- domain.CheckCompletedData.Result is the event payload; intermediate accessor would defeat the JSON binding [permanent]
			r := data.Result
			latest = &r
		}
	}
	if latest == nil {
		return jsonResult(map[string]any{
			"initialized":   true,
			"gateDir":       gateDir,
			"check_count":   0,
			"latest_check":  nil,
			"prs_evaluated": []string{},
			"instruction":   "No checks recorded yet. Run `amadeus run` or `amadeus check` to populate the event store.",
		})
	}
	return jsonResult(map[string]any{
		"initialized":        true,
		"gateDir":            gateDir,
		"check_count":        checkCount,
		"latest_checked_at":  latest.CheckedAt,
		"latest_divergence":  latest.Divergence,
		"latest_commit":      latest.Commit,
		"prs_evaluated":      latest.PRsEvaluated,
		"dmails_emitted":     len(latest.DMails),
		"convergence_alerts": len(latest.ConvergenceAlerts),
		"instruction":        "Query GitHub for each PR in prs_evaluated, prioritize by divergence + review status, and either post the review via amadeus.post_comment (when wired) or the human-driven workflow.",
	})
}

// realPostComment validates the input and, when a CommentPoster is
// wired (= Phase 4 follow-up #3), posts the comment to GitHub via the
// `gh pr comment` adapter. Without a poster it falls back to a
// preview-only payload (= persistence='preview-only'), preserving the
// Phase 3 contract for sessions that haven't opted into write mode.
//
// LLM firing remains human-initiated: the claude-code session decides
// when to call amadeus.post_comment, and the poster only fires when
// the cmd composition root explicitly injected an adapter.
//
// Pattern: paintress.update_gradient (= 83cb3ca) symmetric copy, with
// poster wiring borrowed from sightjack.update_strictness (= 675ce8c).
func realPostComment(ctx context.Context, poster port.CommentPoster, args json.RawMessage) map[string]any {
	var payload struct {
		PRNumber int    `json:"pr_number"`
		Body     string `json:"body"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if payload.PRNumber <= 0 || payload.Body == "" {
		return jsonResult(map[string]any{
			"persisted": false,
			"reason":    "missing required fields: pr_number (>0) and body",
			"received":  payload,
		})
	}
	if poster == nil {
		return jsonResult(map[string]any{
			"persisted":   false,
			"pr_number":   payload.PRNumber,
			"body_length": len(payload.Body),
			"posted":      false,
			"persistence": "preview-only",
			"note":        "Preview only. CommentPoster not wired; cmd composition root must inject a GhPRWriter to enable actual posting.",
		})
	}
	if err := poster.PostComment(ctx, strconv.Itoa(payload.PRNumber), payload.Body); err != nil {
		return jsonResult(map[string]any{
			"persisted":   false,
			"pr_number":   payload.PRNumber,
			"body_length": len(payload.Body),
			"posted":      false,
			"persistence": "github-comments-api",
			"reason":      fmt.Sprintf("PostComment failed: %v", err),
		})
	}
	return jsonResult(map[string]any{
		"persisted":   true,
		"pr_number":   payload.PRNumber,
		"body_length": len(payload.Body),
		"posted":      true,
		"persistence": "github-comments-api",
	})
}

// realGetPRStatus reads CheckCompleted events and returns the PR's
// divergence history (= filter events where PRsEvaluated contains
// the pr_number). Phase 3 scope: read-only summary (latest_divergence
// + check_count + last_dmail_count). Auto-merge gate evaluation is
// Phase 4 follow-up.
//
// Pattern: paintress.next_issue (= 83cb3ca) symmetric copy.
func realGetPRStatus(ctx context.Context, gateDir string, args json.RawMessage, logger domain.Logger) map[string]any {
	var payload struct {
		PRNumber int `json:"pr_number"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if gateDir == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"reason":      "amadeus mcp gateDir not configured",
		})
	}
	if payload.PRNumber <= 0 {
		return jsonResult(map[string]any{
			"initialized": true,
			"reason":      "pr_number required (>0)",
		})
	}
	prKey := strconv.Itoa(payload.PRNumber)
	store := NewEventStore(gateDir, logger)
	events, _, err := store.LoadAll(ctx)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": false,
			"reason":      fmt.Sprintf("event store load failed: %v", err),
		})
	}
	prCheckCount := 0
	var latestForPR *domain.CheckResult
	for _, ev := range events {
		if ev.Type != domain.EventCheckCompleted {
			continue
		}
		var data domain.CheckCompletedData
		if err := json.Unmarshal(ev.Data, &data); err != nil {
			continue
		}
		matches := false
		for _, pr := range data.Result.PRsEvaluated {
			if pr == prKey || strings.HasSuffix(pr, "#"+prKey) {
				matches = true
				break
			}
		}
		if !matches {
			continue
		}
		prCheckCount++
		if latestForPR == nil || data.Result.CheckedAt.After(latestForPR.CheckedAt) { // nosemgrep: lod-excessive-dot-chain -- domain.CheckCompletedData.Result is the event payload [permanent]
			r := data.Result
			latestForPR = &r
		}
	}
	if latestForPR == nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"pr_number":   payload.PRNumber,
			"found":       false,
			"reason":      fmt.Sprintf("no checks recorded for PR %d", payload.PRNumber),
		})
	}
	return jsonResult(map[string]any{
		"initialized":        true,
		"pr_number":          payload.PRNumber,
		"found":              true,
		"check_count":        prCheckCount,
		"latest_checked_at":  latestForPR.CheckedAt,
		"latest_divergence":  latestForPR.Divergence,
		"latest_commit":      latestForPR.Commit,
		"gate_denied":        latestForPR.GateDenied,
		"dmail_count":        len(latestForPR.DMails),
		"convergence_alerts": len(latestForPR.ConvergenceAlerts),
		"instruction":        "Auto-merge readiness requires evaluation of the gate (= Phase 4 follow-up). For now, use the latest_divergence + gate_denied flags to decide manually.",
	})
}

func (s *MCPServer) respond(id json.RawMessage, result any) error {
	return s.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *MCPServer) respondError(id json.RawMessage, code int, message string) error {
	return s.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: id, Error: &jsonrpcError{Code: code, Message: message}})
}

func (s *MCPServer) writeMessage(msg jsonrpcMessage) error {
	out, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	if _, err := s.out.Write(append(out, '\n')); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	return nil
}

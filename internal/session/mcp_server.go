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

// MCPServer is a stdio-based Model Context Protocol server for the
// refs/issues/0027 jun15 MCP pivot.
//
// All four tools are real implementations: ping (health
// check), next_review + get_pr_status (read the gate
// event store / convergence projection), and post_comment
// (posts to GitHub via `gh pr comment` when a CommentPoster is wired;
// cmd wires one by default).
//
// Wire it into a Claude Code interactive session via --mcp-config so
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
	repoRoot      string
	commentPoster port.CommentPoster
	prLister      port.OpenPRLister
	reviewEmitter port.ReviewIntakeEmitter
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
// post_comment. When nil, post_comment stays preview-only;
// the cmd composition root injects a GhPRWriter by default in
// repo-scoped invocations.
func (s *MCPServer) WithCommentPoster(p port.CommentPoster) *MCPServer {
	s.commentPoster = p
	return s
}

// WithPRLister wires the narrow open-PR read seam used by
// refresh_reviews (refs issue 0032 D2(a)).
func (s *MCPServer) WithPRLister(l port.OpenPRLister) *MCPServer {
	s.prLister = l
	return s
}

// WithRepoRoot sets the project root used by the dmail emission tool
// to derive the .gate/ outbox store paths (refs issue 0031).
func (s *MCPServer) WithRepoRoot(root string) *MCPServer {
	s.repoRoot = root
	return s
}

// WithReviewEmitter wires the reviewer write seam: snapshot ingestion
// (refresh_reviews) + review.posted ledger entries (post_comment).
// When nil, refresh_reviews degrades to preview-only and post_comment
// skips the ledger entry.
func (s *MCPServer) WithReviewEmitter(e port.ReviewIntakeEmitter) *MCPServer {
	s.reviewEmitter = e
	return s
}

// jsonrpcMessage is the minimum JSON-RPC 2.0 envelope this server
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
	case "initialize":
		return s.respond(msg.ID, initializeResult())
	case "notifications/initialized":
		// JSON-RPC notification (no id): the client signals it finished
		// the handshake. No response is sent.
		return nil
	case "tools/list":
		return s.respond(msg.ID, map[string]any{"tools": toolDescriptors()})
	case "tools/call":
		return s.handleToolsCall(ctx, msg)
	default:
		// Unknown notifications (no id) are ignored per JSON-RPC; only
		// id-bearing requests get a method-not-found error.
		if len(msg.ID) == 0 {
			return nil
		}
		return s.respondError(msg.ID, -32601, fmt.Sprintf("method not implemented: %s", msg.Method))
	}
}

// mcpProtocolVersion is the single MCP protocol version this server
// implements. Per the MCP lifecycle spec, the server returns the
// version it actually supports (not an echo of the client's request):
// echoing an unsupported client version would falsely claim support
// and break future / draft clients. The client decides compatibility
// from this value.
const mcpProtocolVersion = "2024-11-05"

// initializeResult builds the MCP initialize handshake response. The
// Claude Code session sends `initialize` first; without a valid reply
// it never proceeds to tools/list. The server advertises its supported
// protocol version + the tools capability.
func initializeResult() map[string]any {
	return map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo":      map[string]any{"name": "amadeus", "version": "0.1.0"},
		// instructions feed Claude Code's deferred tool loading (Tool
		// Search): only tool names + this summary are in context at
		// startup, so it must say what the server is FOR.
		"instructions": "amadeus is the verifier data plane of the tap 5-tool ecosystem: refresh the review queue from GitHub (refresh_reviews), read the next un-reviewed PR (next_review, get_pr_status), post review comments (post_comment), and emit corrective feedback d-mails through the transactional outbox (dmail). Drive it from the /review-gate skill in a human-initiated session.",
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
	case "ping":
		result = textResult("pong")
	case "next_review":
		result = realNextReview(ctx, s.gateDir, s.logger)
	case "post_comment":
		result = realPostComment(ctx, s.commentPoster, s.reviewEmitter, call.Arguments)
	case "get_pr_status":
		result = realGetPRStatus(ctx, s.gateDir, call.Arguments, s.logger)
	case "refresh_reviews":
		result = realRefreshReviews(ctx, s.gateDir, s.prLister, s.reviewEmitter, call.Arguments)
	case "dmail":
		result = realDMail(ctx, s.repoRoot, call.Arguments)
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

// toolDescriptors returns the tool set. Each entry pins the interface
// (name, description, inputSchema) so Claude Code clients see a stable
// contract. The handler bodies (realNextReview / realPostComment /
// realGetPRStatus) read the gate event store / convergence projection
// and post comments to GitHub via `gh pr comment` when wired.
func toolDescriptors() []map[string]any {
	return []map[string]any{
		{
			"name":        "ping",
			"description": "Health check. Returns 'pong'.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "next_review",
			"description": "Return latest CheckCompleted event + total check count + PRs evaluated in the most recent check. The session picks the PR with highest divergence to review next.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "post_comment",
			"description": "Post a review comment to the given PR via the GitHub Comments API (= `gh pr comment`). When a CommentPoster is wired (cmd wires one by default), posted=true + persistence='github-comments-api'. Otherwise preview-only + persistence='preview-only'.",
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
			"name":        "get_pr_status",
			"description": "Return per-PR check history (= filter CheckCompleted events where PRsEvaluated contains the given pr_number). Returns latest divergence + check_count + gate_denied flag + dmail_count.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pr_number": map[string]any{"type": "integer"},
				},
				"required": []any{"pr_number"},
			},
		},
		{
			"name":        "dmail",
			"description": "Emit a D-Mail through the transactional outbox (refs issue 0031). Arguments map onto the D-Mail v1 schema; amadeus may emit kinds: design-feedback / implementation-feedback / convergence. Never write outbox/ directly — this tool is the canonical atomic path (SQLite stage -> flush) that phonewave delivery depends on. Re-sending the same name is an idempotent upsert.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"kind":        map[string]any{"type": "string", "description": "design-feedback / implementation-feedback / convergence"},
					"name":        map[string]any{"type": "string", "description": "unique d-mail name (becomes <name>.md)"},
					"description": map[string]any{"type": "string", "description": "one-line summary (required by schema v1)"},
					"body":        map[string]any{"type": "string", "description": "markdown body (findings / corrective actions)"},
					"issues":      map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "related issue ids"},
					"severity":    map[string]any{"type": "string", "description": "low / medium / high (optional)"},
					"priority":    map[string]any{"type": "integer", "description": "priority (optional)"},
					"metadata":    map[string]any{"type": "object", "description": "string map; project_id / actor_type injected automatically"},
				},
				"required": []any{"kind", "name", "description", "body"},
			},
		},
		{
			"name":        "refresh_reviews",
			"description": "Ingest the current GitHub open-PR list into the gate event store (EventPRSnapshotIngested, on-demand via `gh pr list` — no daemon, no LLM). next_review then serves the oldest un-reviewed PR from the latest snapshot; post_comment marks PRs reviewed. Re-running replaces the snapshot (idempotent).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"base_branch": map[string]any{"type": "string", "description": "target base branch (default: main)"},
				},
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
	// Intake contract (refs issue 0032 D2(a)): when a PR snapshot
	// exists, serve the oldest un-reviewed PR from it. The legacy
	// check.completed read model remains the fallback.
	if intake := loadReviewIntake(events); intake != nil {
		return jsonResult(intakeResult(gateDir, intake))
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
			"instruction":   "No checks recorded yet. Drive a review from your claude-code session via the amadeus MCP tools (see SKILL.md); the gate event store is populated as reviews are recorded.",
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
		"instruction":        "Query GitHub for each PR in prs_evaluated, prioritize by divergence + review status, and either post the review via post_comment (when wired) or the human-driven workflow.",
	})
}

// realPostComment validates the input and, when a CommentPoster is
// wired (cmd wires one by default), posts the comment to GitHub via the
// `gh pr comment` adapter. Without a poster it falls back to a
// preview-only payload (= persistence='preview-only') for sessions that
// haven't opted into write mode.
//
// LLM firing remains human-initiated: the claude-code session decides
// when to call post_comment, and the poster only fires when
// the cmd composition root explicitly injected an adapter.
//
// Pattern: paintress.update_gradient (= 83cb3ca) symmetric copy, with
// poster wiring borrowed from sightjack.update_strictness (= 675ce8c).
func realPostComment(ctx context.Context, poster port.CommentPoster, emitter port.ReviewIntakeEmitter, args json.RawMessage) map[string]any {
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
	res := map[string]any{
		"persisted":   true,
		"pr_number":   payload.PRNumber,
		"body_length": len(payload.Body),
		"posted":      true,
		"persistence": "github-comments-api",
	}
	// Record review.posted in the gate ledger so next_review drops this
	// PR from the pending intake (refs issue 0032 D2(a)). Best-effort:
	// the GitHub post already succeeded, so a ledger failure is
	// surfaced for repair rather than reported as a failed post.
	if emitter != nil {
		if err := emitter.EmitReviewPosted(strconv.Itoa(payload.PRNumber), time.Now().UTC()); err != nil {
			res["review_event_recorded"] = false
			res["reason"] = fmt.Sprintf("review.posted append failed: %v", err)
		} else {
			res["review_event_recorded"] = true
		}
	}
	return jsonResult(res)
}

// realGetPRStatus reads CheckCompleted events and returns the PR's
// divergence history (= filter events where PRsEvaluated contains
// the pr_number): a read-only summary (latest_divergence + check_count
// + last_dmail_count). Auto-merge gate evaluation is out of scope here.
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
		"instruction":        "Use the latest_divergence + gate_denied flags to decide review/merge readiness manually.",
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

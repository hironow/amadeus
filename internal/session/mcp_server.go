package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
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
// (ADR 0017) + sightjack Phase 2a (ADR 0018).
type MCPServer struct {
	in     io.Reader
	out    io.Writer
	logger domain.Logger
}

// NewMCPServer wires explicit I/O so tests can drive the server
// without subprocess overhead. Passing nil for logger uses NopLogger.
func NewMCPServer(in io.Reader, out io.Writer, logger domain.Logger) *MCPServer {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	return &MCPServer{in: in, out: out, logger: logger}
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
		result = stubNextReview()
		status = "deprecated"
	case "amadeus.post_comment":
		result = stubPostComment(call.Arguments)
		status = "deprecated"
	case "amadeus.get_pr_status":
		result = stubGetPRStatus(call.Arguments)
		status = "deprecated"
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
			"description": "Return the next PR awaiting review (Phase 2b: stub returns a placeholder PR payload until the domain wiring lands).",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "amadeus.post_comment",
			"description": "Post a review comment to the given PR (Phase 2b: stub echoes the requested pr_number + body length).",
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
			"description": "Return the convergence + auto-merge status for the given PR (Phase 2b: stub echoes the requested pr_number with a contract descriptor).",
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

// stubNextReview returns a fixed placeholder PR payload. Replaced by
// real domain wiring (= review queue projection + PR ranking) in a
// subsequent commit on feat/jun15-mcp-pivot.
func stubNextReview() map[string]any {
	return jsonResult(map[string]any{
		"stub":     true,
		"pr":       nil,
		"reason":   "phase-2b-mvp: real implementation lands when the review queue projection commit replaces this stub",
		"contract": map[string]any{"pr_number": "integer", "owner": "string", "repo": "string", "title": "string", "branch": "string", "status": "string"},
	})
}

// stubPostComment echoes the requested pr_number + body length so
// claude code clients can exercise the contract end-to-end before
// the real GitHub Comments API wiring lands.
func stubPostComment(args json.RawMessage) map[string]any {
	var payload struct {
		PRNumber int    `json:"pr_number"`
		Body     string `json:"body"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	return jsonResult(map[string]any{
		"stub":        true,
		"pr_number":   payload.PRNumber,
		"body_length": len(payload.Body),
		"posted":      false,
		"reason":      "phase-2b-mvp: real GitHub Comments API call lands when the post-comment adapter is wired",
	})
}

// stubGetPRStatus echoes the requested pr_number with a placeholder
// status payload.
func stubGetPRStatus(args json.RawMessage) map[string]any {
	var payload struct {
		PRNumber int `json:"pr_number"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	return jsonResult(map[string]any{
		"stub":      true,
		"pr_number": payload.PRNumber,
		"status":    nil,
		"reason":    "phase-2b-mvp: real convergence + auto-merge state lookup lands when the projection store is exposed",
		"contract":  map[string]any{"convergence": "string (none|partial|full)", "auto_merge_ready": "bool", "review_count": "integer", "blocking_reviewers": "array of string"},
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

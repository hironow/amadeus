package session

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform/actortype"
	"github.com/hironow/amadeus/internal/platform/projectid"
)

// realDMail emits a D-Mail through the transactional outbox (refs
// issue 0031). Arguments are a typed subset of the D-Mail v1 schema;
// direct outbox/ writes from the session remain forbidden because they
// would bypass the SQLite stage -> atomic flush contract phonewave's
// watcher depends on. repoRoot is the project root (outbox store
// derives .gate/ paths from it).
func realDMail(ctx context.Context, repoRoot string, args json.RawMessage) map[string]any {
	var payload struct {
		Kind        string            `json:"kind"`
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Body        string            `json:"body"`
		Issues      []string          `json:"issues"`
		Severity    string            `json:"severity"`
		Priority    int               `json:"priority"`
		Metadata    map[string]string `json:"metadata"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if repoRoot == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"sent":        false,
			"reason":      "amadeus mcp repo root not configured (start `amadeus mcp` from the project root)",
		})
	}
	mail, err := domain.NewProducedDMail(
		domain.DMailKind(payload.Kind),
		payload.Name,
		payload.Description,
		payload.Body,
		payload.Issues,
		domain.Severity(payload.Severity),
		payload.Priority,
		payload.Metadata,
	)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"sent":        false,
			"reason":      err.Error(),
		})
	}
	mail.Metadata = projectid.InjectProjectID(mail.Metadata)
	metadata, err := actortype.InjectActorType(mail.Metadata)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"sent":        false,
			"reason":      fmt.Sprintf("actor type env invalid: %v", err),
		})
	}
	mail.Metadata = metadata

	store, err := NewOutboxStoreForDir(repoRoot)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"sent":        false,
			"reason":      fmt.Sprintf("outbox store open failed: %v", err),
		})
	}
	defer func() { _ = store.Close() }()

	data, err := domain.MarshalDMail(mail)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"sent":        false,
			"reason":      fmt.Sprintf("dmail marshal failed: %v", err),
		})
	}
	filename := mail.Name + ".md"
	if stageErr := store.Stage(ctx, filename, data); stageErr != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"sent":        false,
			"reason":      fmt.Sprintf("dmail stage failed: %v", stageErr),
		})
	}
	n, err := store.Flush(ctx)
	if err != nil || n == 0 {
		return jsonResult(map[string]any{
			"initialized": true,
			"sent":        false,
			"reason":      fmt.Sprintf("dmail flush failed (staged; re-run dmail to retry): n=%d err=%v", n, err),
		})
	}
	return jsonResult(map[string]any{
		"initialized": true,
		"sent":        true,
		"name":        mail.Name,
		"filename":    filename,
		"kind":        string(mail.Kind),
		"persistence": "transactional-outbox",
	})
}

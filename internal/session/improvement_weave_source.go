package session

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

const defaultWeaveAPIBaseURL = "https://trace.wandb.ai"

type HTTPImprovementFeedbackSource struct { // nosemgrep: domain-primitives.public-string-field-go — internal HTTP client config; fields exported for struct literal initialization, not domain primitives [permanent]
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewHTTPImprovementFeedbackSource(baseURL, apiKey string) (*HTTPImprovementFeedbackSource, error) {
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("weave feedback source: api key is required")
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultWeaveAPIBaseURL
	}
	return &HTTPImprovementFeedbackSource{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

func (s *HTTPImprovementFeedbackSource) QueryFeedback(ctx context.Context, query ImprovementFeedbackQuery) ([]ImprovementFeedbackRow, error) {
	if s == nil {
		return nil, fmt.Errorf("weave feedback source: nil source")
	}
	fields := []string{
		"id",
		"project_id",
		"weave_ref",
		"feedback_type",
		"created_at",
		"payload",
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}
	reqBody := map[string]any{
		"project_id": query.ProjectID,
		"fields":     fields,
		"limit":      limit,
		"sort_by": []map[string]string{
			{"field": "created_at", "direction": "asc"},
			{"field": "id", "direction": "asc"},
		},
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("weave feedback source: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.BaseURL+"/feedback/query", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("weave feedback source: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("api:"+s.APIKey)))
	resp, err := s.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("weave feedback source: do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weave feedback source: unexpected status %d", resp.StatusCode)
	}
	var raw struct {
		Result []map[string]any `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("weave feedback source: decode response: %w", err)
	}
	rows := make([]ImprovementFeedbackRow, 0, len(raw.Result))
	for _, item := range raw.Result {
		row, err := decodeImprovementFeedbackRow(item)
		if err != nil {
			return nil, err
		}
		if row.ProjectID != query.ProjectID {
			continue
		}
		if row.CreatedAt.Before(query.CreatedAfter) {
			continue
		}
		if row.CreatedAt.Equal(query.CreatedAfter) && query.AfterFeedback != "" && row.ID <= query.AfterFeedback {
			continue
		}
		if !improvementFeedbackTypeAllowed(query.FeedbackTypes, row.FeedbackType) {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *HTTPImprovementFeedbackSource) httpClient() *http.Client {
	if s.HTTPClient != nil {
		return s.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

func decodeImprovementFeedbackRow(item map[string]any) (ImprovementFeedbackRow, error) {
	row := ImprovementFeedbackRow{
		ID:           payloadString(item, "id"),
		ProjectID:    payloadString(item, "project_id"),
		WeaveRef:     payloadString(item, "weave_ref"),
		FeedbackType: payloadString(item, "feedback_type"),
	}
	if row.ID == "" {
		return ImprovementFeedbackRow{}, fmt.Errorf("weave feedback source: missing feedback id")
	}
	if row.ProjectID == "" {
		return ImprovementFeedbackRow{}, fmt.Errorf("weave feedback source: missing project id")
	}
	createdAtRaw := payloadString(item, "created_at")
	if createdAtRaw == "" {
		return ImprovementFeedbackRow{}, fmt.Errorf("weave feedback source: missing created_at")
	}
	createdAt, err := time.Parse(time.RFC3339, createdAtRaw)
	if err != nil {
		return ImprovementFeedbackRow{}, fmt.Errorf("weave feedback source: parse created_at: %w", err)
	}
	row.CreatedAt = createdAt
	if payload, ok := item["payload"].(map[string]any); ok {
		row.Payload = payload
	} else {
		row.Payload = map[string]any{}
	}
	return row, nil
}

func NewImprovementCollectorFromEnv(repoRoot string, ledger *InsightWriter, logger domain.Logger) (*ImprovementCollector, func() error, error) {
	return NewImprovementCollector(repoRoot, domain.ImprovementCollectorConfig{}, ledger, logger)
}

func NewImprovementCollector(repoRoot string, cfg domain.ImprovementCollectorConfig, ledger *InsightWriter, logger domain.Logger) (*ImprovementCollector, func() error, error) {
	if ledger == nil {
		return nil, func() error { return nil }, nil
	}
	apiKey := strings.TrimSpace(os.Getenv("WANDB_API_KEY"))
	projectID := strings.TrimSpace(cfg.ProjectID)
	if projectID == "" {
		projectID = improvementProjectIDFromEnv()
	}
	baseURL := strings.TrimSpace(cfg.APIURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("WEAVE_API_URL"))
	}
	enabled := collectorEnabled(cfg.Enabled, apiKey, projectID)
	if !enabled {
		return nil, func() error { return nil }, nil
	}
	if apiKey == "" {
		return nil, func() error { return nil }, fmt.Errorf("improvement collector: WANDB_API_KEY is required when collector is enabled")
	}
	if projectID == "" {
		return nil, func() error { return nil }, fmt.Errorf("improvement collector: project id is required when collector is enabled")
	}
	source, err := NewHTTPImprovementFeedbackSource(baseURL, apiKey)
	if err != nil {
		return nil, func() error { return nil }, err
	}
	store, err := NewSQLiteImprovementCollectorStore(filepath.Join(repoRoot, domain.StateDir, ".run", "improvement-ingestion.db"))
	if err != nil {
		return nil, func() error { return nil }, err
	}
	return &ImprovementCollector{
		ProjectID:            projectID,
		Source:               source,
		Store:                store,
		Ledger:               ledger,
		Logger:               logger,
		QueryLimit:           cfg.QueryLimit,
		AllowedFeedbackTypes: append([]string(nil), cfg.FeedbackTypes...),
	}, store.Close, nil
}

func collectorEnabled(explicit *bool, apiKey, projectID string) bool {
	if explicit != nil {
		return *explicit
	}
	return apiKey != "" && projectID != ""
}

func improvementProjectIDFromEnv() string {
	if projectID := strings.TrimSpace(os.Getenv("WEAVE_PROJECT_ID")); projectID != "" {
		return projectID
	}
	if projectID := strings.TrimSpace(os.Getenv("WANDB_PROJECT_ID")); projectID != "" {
		return projectID
	}
	entity := strings.TrimSpace(os.Getenv("WANDB_ENTITY"))
	project := strings.TrimSpace(os.Getenv("WANDB_PROJECT"))
	switch {
	case entity != "" && project != "":
		return entity + "/" + project
	case project != "":
		return project
	default:
		return ""
	}
}

func normalizeWeaveAPIBaseURL(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return defaultWeaveAPIBaseURL
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	return strings.TrimRight(parsed.String(), "/")
}

package domain_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hironow/amadeus/internal/domain"
)

// fixturePath resolves a path under tests/fixtures/dmail/ relative to
// the repo root. internal/domain/ is two levels deep, so the fixture
// directory sits at ../../tests/fixtures/dmail.
func fixturePath(t *testing.T, name string) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Join(wd, "..", "..", "tests", "fixtures", "dmail", name)
}

func TestParseDMailEnvelope_FixtureSchemaIsHonored(t *testing.T) {
	// given: synthetic fixture pinned by refs/issues/0027 §8.
	// amadeus-side fixture exercises the paintress -> amadeus direction
	// (conflict_notification kind) so the symmetric envelope contract
	// is verified.
	path := fixturePath(t, "dmail-2026-06-01T12-00-00Z-ghi789.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	// when
	env, err := domain.ParseDMailEnvelope(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// then: every Phase 2b required field is set, nullable markers stay nil.
	want := domain.DMailEnvelope{
		MessageID:      "dmail-2026-06-01T12-00-00Z-ghi789",
		SourceTool:     "paintress",
		TargetTool:     "amadeus",
		Kind:           "conflict_notification",
		BodyPath:       "./dmail-2026-06-01T12-00-00Z-ghi789.body.md",
		CreatedAt:      time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
		IdempotencyKey: "sha256:9f1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f60718293a4b5c6d7e8f9",
	}
	if env.MessageID != want.MessageID {
		t.Errorf("MessageID = %q, want %q", env.MessageID, want.MessageID)
	}
	if env.SourceTool != want.SourceTool {
		t.Errorf("SourceTool = %q, want %q", env.SourceTool, want.SourceTool)
	}
	if env.TargetTool != want.TargetTool {
		t.Errorf("TargetTool = %q, want %q", env.TargetTool, want.TargetTool)
	}
	if env.Kind != want.Kind {
		t.Errorf("Kind = %q, want %q", env.Kind, want.Kind)
	}
	if env.BodyPath != want.BodyPath {
		t.Errorf("BodyPath = %q, want %q", env.BodyPath, want.BodyPath)
	}
	if !env.CreatedAt.Equal(want.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", env.CreatedAt, want.CreatedAt)
	}
	if env.IdempotencyKey != want.IdempotencyKey {
		t.Errorf("IdempotencyKey = %q, want %q", env.IdempotencyKey, want.IdempotencyKey)
	}
	if env.SeenAt != nil {
		t.Errorf("SeenAt = %v, want nil (freshly delivered)", env.SeenAt)
	}
	if env.AckAt != nil {
		t.Errorf("AckAt = %v, want nil (freshly delivered)", env.AckAt)
	}
	if env.IsConsumed() {
		t.Errorf("IsConsumed() = true, want false")
	}
}

func TestParseDMailEnvelope_RejectsMissingRequiredFields(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "missing message_id",
			yaml: `source_tool: pt
target_tool: am
kind: conflict_notification
body_path: ./b.md
created_at: 2026-06-01T12:00:00Z
idempotency_key: sha256:x`,
			want: "message_id",
		},
		{
			name: "missing source_tool",
			yaml: `message_id: m1
target_tool: am
kind: conflict_notification
body_path: ./b.md
created_at: 2026-06-01T12:00:00Z
idempotency_key: sha256:x`,
			want: "source_tool",
		},
		{
			name: "missing idempotency_key",
			yaml: `message_id: m1
source_tool: pt
target_tool: am
kind: conflict_notification
body_path: ./b.md
created_at: 2026-06-01T12:00:00Z`,
			want: "idempotency_key",
		},
		{
			name: "missing created_at",
			yaml: `message_id: m1
source_tool: pt
target_tool: am
kind: conflict_notification
body_path: ./b.md
idempotency_key: sha256:x`,
			want: "created_at",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// when
			_, err := domain.ParseDMailEnvelope([]byte(tc.yaml))
			// then
			if err == nil {
				t.Fatalf("expected error mentioning %q, got nil", tc.want)
			}
			if !contains(err.Error(), tc.want) {
				t.Errorf("error %q does not mention required field %q", err.Error(), tc.want)
			}
		})
	}
}

func TestParseDMailEnvelope_DetectsConsumedEnvelope(t *testing.T) {
	// given: paintress -> amadeus envelope with ack_at set (= post-consume).
	yamlBody := `message_id: m1
source_tool: pt
target_tool: am
kind: conflict_notification
body_path: ./b.md
created_at: 2026-06-01T12:00:00Z
seen_at: 2026-06-01T12:05:00Z
ack_at: 2026-06-01T12:06:00Z
idempotency_key: sha256:x`

	// when
	env, err := domain.ParseDMailEnvelope([]byte(yamlBody))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// then
	if env.SeenAt == nil || env.AckAt == nil {
		t.Fatalf("seen_at / ack_at parsed as nil: seen=%v ack=%v", env.SeenAt, env.AckAt)
	}
	if !env.IsConsumed() {
		t.Errorf("IsConsumed() = false, want true")
	}
}

func TestParseDMailEnvelope_BodyPathPointsAtPairedFile(t *testing.T) {
	// given: load the same fixture envelope.
	yamlPath := fixturePath(t, "dmail-2026-06-01T12-00-00Z-ghi789.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	env, err := domain.ParseDMailEnvelope(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	// when: resolve body_path relative to the envelope file.
	bodyPath := filepath.Join(filepath.Dir(yamlPath), env.BodyPath)

	// then: paired body file exists (= layout contract from refs 0027 §8).
	info, err := os.Stat(bodyPath)
	if err != nil {
		t.Fatalf("paired body file missing: %v (path=%s)", err, bodyPath)
	}
	if info.Size() == 0 {
		t.Errorf("paired body file is empty: %s", bodyPath)
	}
}

func TestDMailEnvelope_IdempotencyKey_DistinguishesFixtures(t *testing.T) {
	// given: two envelopes with identical content except idempotency_key.
	yamlA := `message_id: m
source_tool: pt
target_tool: am
kind: conflict_notification
body_path: ./b.md
created_at: 2026-06-01T12:00:00Z
idempotency_key: sha256:aaa`
	yamlB := `message_id: m
source_tool: pt
target_tool: am
kind: conflict_notification
body_path: ./b.md
created_at: 2026-06-01T12:00:00Z
idempotency_key: sha256:bbb`

	// when
	envA, err := domain.ParseDMailEnvelope([]byte(yamlA))
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}
	envB, err := domain.ParseDMailEnvelope([]byte(yamlB))
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	// then: dedup is based on idempotency_key, not message_id alone.
	if envA.IdempotencyKey == envB.IdempotencyKey {
		t.Fatalf("idempotency keys collide: %q", envA.IdempotencyKey)
	}
}

// contains is a tiny strings.Contains alias; kept local so this test
// file stays self-contained even if internal/domain/ adds its own
// helper file later.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

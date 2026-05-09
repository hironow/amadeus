package actortype_test

import (
	"errors"
	"testing"

	"github.com/hironow/amadeus/internal/platform/actortype"
)

func TestResolve_HumanOperator(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "human-operator")

	// when
	got, source, err := actortype.Resolve()

	// then
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if got != "human-operator" {
		t.Errorf("expected 'human-operator', got %q", got)
	}
	if source != "env" {
		t.Errorf("expected source 'env', got %q", source)
	}
}

func TestResolve_GatewayService(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "gateway-service")

	// when
	got, source, err := actortype.Resolve()

	// then
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if got != "gateway-service" {
		t.Errorf("expected 'gateway-service', got %q", got)
	}
	if source != "env" {
		t.Errorf("expected source 'env', got %q", source)
	}
}

func TestResolve_AIAgent(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "ai-agent")

	// when
	got, source, err := actortype.Resolve()

	// then
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if got != "ai-agent" {
		t.Errorf("expected 'ai-agent', got %q", got)
	}
	if source != "env" {
		t.Errorf("expected source 'env', got %q", source)
	}
}

func TestResolve_WorkspaceDaemon(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "workspace-daemon")

	// when
	got, source, err := actortype.Resolve()

	// then
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if got != "workspace-daemon" {
		t.Errorf("expected 'workspace-daemon', got %q", got)
	}
	if source != "env" {
		t.Errorf("expected source 'env', got %q", source)
	}
}

func TestResolve_Empty(t *testing.T) {
	// given — env unset (legacy compat path)
	t.Setenv("RUNOPS_ACTOR_TYPE", "")

	// when
	got, source, err := actortype.Resolve()

	// then
	if err != nil {
		t.Fatalf("expected nil err for unset env, got %v", err)
	}
	if got != "" {
		t.Errorf("expected empty actor type, got %q", got)
	}
	if source != "" {
		t.Errorf("expected empty source, got %q", source)
	}
}

func TestResolve_InvalidValue_Robot(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "robot")

	// when
	got, source, err := actortype.Resolve()

	// then — invalid env MUST error (silent escalation prevention)
	if !errors.Is(err, actortype.ErrInvalidActorType) {
		t.Errorf("expected ErrInvalidActorType, got %v", err)
	}
	if got != "" {
		t.Errorf("expected empty actor type on error, got %q", got)
	}
	if source != "" {
		t.Errorf("expected empty source on error, got %q", source)
	}
}

func TestResolve_InvalidValue_Whitespace(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "   ")

	// when
	got, source, err := actortype.Resolve()

	// then — whitespace-only is invalid (set but no real value)
	if !errors.Is(err, actortype.ErrInvalidActorType) {
		t.Errorf("expected ErrInvalidActorType, got %v", err)
	}
	if got != "" || source != "" {
		t.Errorf("expected empty results on error, got (%q, %q)", got, source)
	}
}

func TestResolve_RejectsBroker(t *testing.T) {
	// given — producer MUST NOT write actor_type_source=broker (gateway-only attestation).
	// helper structurally rejects "broker" in the actor type slot too, so a misconfigured
	// env cannot leak a spoof attempt downstream.
	t.Setenv("RUNOPS_ACTOR_TYPE", "broker")

	// when
	got, source, err := actortype.Resolve()

	// then
	if !errors.Is(err, actortype.ErrInvalidActorType) {
		t.Errorf("expected ErrInvalidActorType for 'broker', got %v", err)
	}
	if got != "" || source != "" {
		t.Errorf("expected empty results for 'broker', got (%q, %q)", got, source)
	}
}

func TestResolveInitiating_HumanOperator(t *testing.T) {
	// given
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "human-operator")

	// when
	got, err := actortype.ResolveInitiating()

	// then
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if got != "human-operator" {
		t.Errorf("expected 'human-operator', got %q", got)
	}
}

func TestResolveInitiating_Empty(t *testing.T) {
	// given
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "")

	// when
	got, err := actortype.ResolveInitiating()

	// then
	if err != nil {
		t.Fatalf("expected nil err for unset, got %v", err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestResolveInitiating_Invalid(t *testing.T) {
	// given
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "robot")

	// when
	got, err := actortype.ResolveInitiating()

	// then
	if !errors.Is(err, actortype.ErrInvalidInitiatingActorType) {
		t.Errorf("expected ErrInvalidInitiatingActorType, got %v", err)
	}
	if got != "" {
		t.Errorf("expected empty on error, got %q", got)
	}
}

func TestIsValidActorType(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"human-operator", "human-operator", true},
		{"gateway-service", "gateway-service", true},
		{"ai-agent", "ai-agent", true},
		{"workspace-daemon", "workspace-daemon", true},
		{"empty", "", false},
		{"whitespace", "   ", false},
		{"broker", "broker", false},
		{"unknown", "unknown", false},
		{"robot", "robot", false},
		{"capitalized", "Human-Operator", false},
		{"trailing space", "ai-agent ", false},
		{"underscore variant", "ai_agent", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := actortype.IsValidActorType(tc.in); got != tc.want {
				t.Errorf("IsValidActorType(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestInjectActorType_AddsKeysWhenResolved(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "ai-agent")
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "")

	// when
	md, err := actortype.InjectActorType(nil)

	// then
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if md == nil {
		t.Fatalf("expected non-nil map")
	}
	if md["requester_actor_type"] != "ai-agent" {
		t.Errorf("expected requester_actor_type=ai-agent, got %q", md["requester_actor_type"])
	}
	if md["requester_actor_source"] != "env" {
		t.Errorf("expected requester_actor_source=env, got %q", md["requester_actor_source"])
	}
	if _, ok := md["initiating_actor_type"]; ok {
		t.Errorf("non-daemon actor must not carry initiating_actor_type, got %q", md["initiating_actor_type"])
	}
}

func TestInjectActorType_Daemon_WithInitiating(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "workspace-daemon")
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "human-operator")

	// when
	md, err := actortype.InjectActorType(nil)

	// then
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if md["requester_actor_type"] != "workspace-daemon" {
		t.Errorf("expected requester_actor_type=workspace-daemon, got %q", md["requester_actor_type"])
	}
	if md["requester_actor_source"] != "env" {
		t.Errorf("expected requester_actor_source=env, got %q", md["requester_actor_source"])
	}
	if md["initiating_actor_type"] != "human-operator" {
		t.Errorf("expected initiating_actor_type=human-operator, got %q", md["initiating_actor_type"])
	}
}

func TestInjectActorType_NonDaemon_IgnoresInitiating(t *testing.T) {
	// given — non-daemon actor with initiating env set: initiating MUST NOT leak through
	t.Setenv("RUNOPS_ACTOR_TYPE", "ai-agent")
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "human-operator")

	// when
	md, err := actortype.InjectActorType(nil)

	// then
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if _, ok := md["initiating_actor_type"]; ok {
		t.Errorf("non-daemon actor must not carry initiating_actor_type")
	}
}

func TestInjectActorType_Daemon_MissingInitiating_OK(t *testing.T) {
	// given — daemon without initiating env: producer emits without it; gateway HIGH path will fail-closed.
	t.Setenv("RUNOPS_ACTOR_TYPE", "workspace-daemon")
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "")

	// when
	md, err := actortype.InjectActorType(nil)

	// then
	if err != nil {
		t.Fatalf("expected nil err for missing initiating, got %v", err)
	}
	if md["requester_actor_type"] != "workspace-daemon" {
		t.Errorf("expected requester_actor_type=workspace-daemon, got %q", md["requester_actor_type"])
	}
	if _, ok := md["initiating_actor_type"]; ok {
		t.Errorf("missing initiating env must NOT inject the key, got %q", md["initiating_actor_type"])
	}
}

func TestInjectActorType_Daemon_InvalidInitiating_ReturnsError(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "workspace-daemon")
	t.Setenv("RUNOPS_INITIATING_ACTOR_TYPE", "robot")

	// when
	_, err := actortype.InjectActorType(nil)

	// then
	if !errors.Is(err, actortype.ErrInvalidInitiatingActorType) {
		t.Errorf("expected ErrInvalidInitiatingActorType, got %v", err)
	}
}

func TestInjectActorType_InvalidEnv_ReturnsError(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "robot")

	// when
	in := map[string]string{"existing": "value"}
	out, err := actortype.InjectActorType(in)

	// then
	if !errors.Is(err, actortype.ErrInvalidActorType) {
		t.Errorf("expected ErrInvalidActorType, got %v", err)
	}
	// existing keys must be preserved (no partial mutation on error)
	if out["existing"] != "value" {
		t.Errorf("existing keys must be preserved on error")
	}
	if _, ok := out["requester_actor_type"]; ok {
		t.Errorf("requester_actor_type must not be set on error")
	}
}

func TestInjectActorType_NilMap_LazyAlloc(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "human-operator")

	// when
	md, err := actortype.InjectActorType(nil)

	// then
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if md == nil {
		t.Fatalf("expected non-nil map (lazy alloc)")
	}
}

func TestInjectActorType_PreservesExistingKeys(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "gateway-service")
	in := map[string]string{"existing": "value"}

	// when
	out, err := actortype.InjectActorType(in)

	// then
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if out["existing"] != "value" {
		t.Errorf("existing key must be preserved")
	}
	if out["requester_actor_type"] != "gateway-service" {
		t.Errorf("requester_actor_type must be added")
	}
}

func TestInjectActorType_NoOpWhenUnresolved(t *testing.T) {
	// given
	t.Setenv("RUNOPS_ACTOR_TYPE", "")
	in := map[string]string{"existing": "value"}

	// when
	out, err := actortype.InjectActorType(in)

	// then
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if _, ok := out["requester_actor_type"]; ok {
		t.Errorf("requester_actor_type must not be added when unresolved")
	}
	if out["existing"] != "value" {
		t.Errorf("existing key must be preserved")
	}
}

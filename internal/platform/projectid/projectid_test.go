package projectid_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/amadeus/internal/platform/projectid"
)

func TestResolve_EnvVarHasPriority(t *testing.T) {
	t.Setenv("RUNOPS_PROJECT_ID", "from-env")
	id, source := projectid.Resolve("/anywhere/projects/cwdguess/.gate")
	if id != "from-env" {
		t.Errorf("expected env value 'from-env', got %q", id)
	}
	if source != "env" {
		t.Errorf("expected source 'env', got %q", source)
	}
}

func TestResolve_FallsBackToCWDInference(t *testing.T) {
	t.Setenv("RUNOPS_PROJECT_ID", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := filepath.Join(home, "projects", "foo", ".gate", "outbox")
	id, source := projectid.Resolve(cwd)
	if id != "foo" {
		t.Errorf("expected cwd-derived 'foo', got %q", id)
	}
	if source != "cwd" {
		t.Errorf("expected source 'cwd', got %q", source)
	}
}

func TestResolve_ReturnsEmptyWhenNoSignal(t *testing.T) {
	t.Setenv("RUNOPS_PROJECT_ID", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	id, source := projectid.Resolve("/tmp/unrelated/sandbox")
	if id != "" {
		t.Errorf("expected empty, got %q", id)
	}
	if source != "" {
		t.Errorf("expected empty source, got %q", source)
	}
}

func TestResolve_RejectsInvalidEnvValue(t *testing.T) {
	t.Setenv("RUNOPS_PROJECT_ID", "../traversal")
	id, source := projectid.Resolve("/tmp/x")
	if id != "" {
		t.Errorf("invalid env should yield empty id, got %q", id)
	}
	if source != "" {
		t.Errorf("invalid env should yield empty source, got %q", source)
	}
}

func TestResolve_RejectsCWDInferredInvalidID(t *testing.T) {
	t.Setenv("RUNOPS_PROJECT_ID", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := filepath.Join(home, "projects", "with space", ".gate")
	id, _ := projectid.Resolve(cwd)
	if id != "" {
		t.Errorf("invalid cwd-inferred id should be rejected, got %q", id)
	}
}

func TestIsValidProjectID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"plain alphanumeric", "foo", true},
		{"with hyphen", "foo-bar", true},
		{"with underscore", "foo_bar", true},
		{"mixed", "Foo-Bar_123", true},
		{"empty", "", false},
		{"space", "with space", false},
		{"slash", "with/slash", false},
		{"dot", "with.dot", false},
		{"comma", "foo,bar", false},
		{"newline", "foo\nbar", false},
		{"traversal", "../etc", false},
		{"max length 64", strings.Repeat("a", 64), true},
		{"over 64", strings.Repeat("a", 65), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := projectid.IsValidProjectID(tc.in); got != tc.want {
				t.Errorf("IsValidProjectID(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestInjectProjectID_AddsKeyWhenResolved(t *testing.T) {
	t.Setenv("RUNOPS_PROJECT_ID", "foo")
	md := projectid.InjectProjectID(nil)
	if md == nil {
		t.Fatalf("expected non-nil map")
	}
	if md["project_id"] != "foo" {
		t.Errorf("expected project_id=foo, got %q", md["project_id"])
	}
}

func TestInjectProjectID_PreservesExistingKeys(t *testing.T) {
	t.Setenv("RUNOPS_PROJECT_ID", "foo")
	in := map[string]string{"existing": "value"}
	out := projectid.InjectProjectID(in)
	if out["existing"] != "value" {
		t.Errorf("existing key should be preserved")
	}
	if out["project_id"] != "foo" {
		t.Errorf("project_id should be added")
	}
}

func TestInjectProjectID_NoOpWhenUnresolved(t *testing.T) {
	t.Setenv("RUNOPS_PROJECT_ID", "")
	t.Setenv("HOME", t.TempDir())
	in := map[string]string{"existing": "value"}
	out := projectid.InjectProjectID(in)
	if _, ok := out["project_id"]; ok {
		t.Errorf("project_id should not be added when unresolved")
	}
	if out["existing"] != "value" {
		t.Errorf("existing key should be preserved")
	}
}

package amadeus_test

import (
	"testing"

	"github.com/hironow/amadeus"
)

func TestPolicies_AllHaveValidTrigger(t *testing.T) {
	for _, p := range amadeus.Policies {
		if p.Name == "" {
			t.Error("policy name must not be empty")
		}
		if p.Trigger == "" {
			t.Errorf("policy %q has empty trigger", p.Name)
		}
		if p.Action == "" {
			t.Errorf("policy %q has empty action", p.Name)
		}
	}
}

func TestPolicies_UniqueNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, p := range amadeus.Policies {
		if seen[p.Name] {
			t.Errorf("duplicate policy name %q", p.Name)
		}
		seen[p.Name] = true
	}
}

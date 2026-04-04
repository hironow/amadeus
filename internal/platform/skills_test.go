package platform_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/platform"
)

func TestSkillsFS_IsEmbedded(t *testing.T) {
	// Verify that the SkillsFS embed is populated.
	entries, err := platform.SkillsFS.ReadDir("templates/skills")
	if err != nil {
		t.Fatalf("ReadDir skills: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one skill directory in SkillsFS")
	}
}

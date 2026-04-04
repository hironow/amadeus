package platform_test

import (
	"testing"

	"github.com/hironow/amadeus/internal/platform"
)

func TestSkillTemplateFS_IsEmbedded(t *testing.T) {
	// Verify that the SkillTemplateFS embed is populated.
	entries, err := platform.SkillTemplateFS.ReadDir("templates/skills")
	if err != nil {
		t.Fatalf("ReadDir skills: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one skill directory in SkillTemplateFS")
	}
}

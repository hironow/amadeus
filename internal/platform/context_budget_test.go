package platform

// white-box-reason: tests internal ContextBudget parsing from stream output

import "testing"

func TestContextBudgetReport_DetailedBreakdown(t *testing.T) {
	report := ContextBudgetReport{
		ToolCount:        45,
		SkillCount:       12,
		PluginCount:      40,
		MCPServerCount:   5,
		HookContextBytes: 800,
		EstimatedTokens:  22450,
	}

	breakdown := report.DetailedBreakdown()

	if len(breakdown) != 5 {
		t.Fatalf("got %d items, want 5", len(breakdown))
	}

	// Find heaviest
	var heaviest string
	for _, item := range breakdown {
		if item.Heaviest {
			heaviest = item.Category
		}
	}
	if heaviest != "plugins" {
		t.Errorf("heaviest = %q, want plugins", heaviest)
	}

	// Verify token calculations
	for _, item := range breakdown {
		switch item.Category {
		case "tools":
			if item.Count != 45 || item.Tokens != 6750 {
				t.Errorf("tools: count=%d tokens=%d, want 45/6750", item.Count, item.Tokens)
			}
		case "skills":
			if item.Count != 12 || item.Tokens != 6000 {
				t.Errorf("skills: count=%d tokens=%d, want 12/6000", item.Count, item.Tokens)
			}
		case "plugins":
			if item.Count != 40 || item.Tokens != 8000 {
				t.Errorf("plugins: count=%d tokens=%d, want 40/8000", item.Count, item.Tokens)
			}
		case "mcp_servers":
			if item.Count != 5 || item.Tokens != 1500 {
				t.Errorf("mcp_servers: count=%d tokens=%d, want 5/1500", item.Count, item.Tokens)
			}
		case "hooks":
			if item.Bytes != 800 || item.Tokens != 200 {
				t.Errorf("hooks: bytes=%d tokens=%d, want 800/200", item.Bytes, item.Tokens)
			}
		}
	}
}

func TestContextBudgetReport_DetailedBreakdown_SkillsHeaviest(t *testing.T) {
	report := ContextBudgetReport{
		ToolCount:       2,
		SkillCount:      100,
		PluginCount:     1,
		EstimatedTokens: 50500,
	}

	breakdown := report.DetailedBreakdown()
	var heaviest string
	for _, item := range breakdown {
		if item.Heaviest {
			heaviest = item.Category
		}
	}
	if heaviest != "skills" {
		t.Errorf("heaviest = %q, want skills", heaviest)
	}
}

func TestContextBudgetReport_DetailedBreakdown_AllZero(t *testing.T) {
	report := ContextBudgetReport{}
	breakdown := report.DetailedBreakdown()
	if len(breakdown) != 5 {
		t.Fatalf("got %d items, want 5", len(breakdown))
	}
	for _, item := range breakdown {
		if item.Heaviest {
			t.Errorf("no item should be heaviest when all are zero")
		}
	}
}

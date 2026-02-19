package amadeus

import (
	"path/filepath"
	"testing"
)

func TestNextDMailID_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	id, err := store.NextDMailID()
	if err != nil {
		t.Fatal(err)
	}
	if id != "d-001" {
		t.Errorf("expected d-001, got %s", id)
	}
}

func TestNextDMailID_Sequential(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	dmail := DMail{ID: "d-001", Severity: SeverityLow, Status: DMailSent, Target: TargetPaintress}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}
	id, err := store.NextDMailID()
	if err != nil {
		t.Fatal(err)
	}
	if id != "d-002" {
		t.Errorf("expected d-002, got %s", id)
	}
}

func TestRouteDMails_SeverityMapping(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		expected DMailStatus
	}{
		{"LOW auto-sent", SeverityLow, DMailSent},
		{"MEDIUM auto-sent", SeverityMedium, DMailSent},
		{"HIGH pending", SeverityHigh, DMailPending},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dmail := DMail{Severity: tt.severity}
			routed := RouteDMail(dmail)
			if routed.Status != tt.expected {
				t.Errorf("expected status %s, got %s", tt.expected, routed.Status)
			}
		})
	}
}

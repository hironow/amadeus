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

func TestLoadDMail_Exists(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: a saved D-Mail
	dmail := DMail{
		ID:       "d-001",
		Severity: SeverityHigh,
		Status:   DMailPending,
		Target:   TargetSightjack,
		Summary:  "ADR violation",
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	// when
	loaded, err := store.LoadDMail("d-001")

	// then
	if err != nil {
		t.Fatalf("LoadDMail failed: %v", err)
	}
	if loaded.ID != "d-001" {
		t.Errorf("expected ID d-001, got %s", loaded.ID)
	}
	if loaded.Status != DMailPending {
		t.Errorf("expected status pending, got %s", loaded.Status)
	}
}

func TestLoadDMail_NotFound(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// when
	_, err := store.LoadDMail("d-999")

	// then
	if err == nil {
		t.Error("expected error for non-existent D-Mail")
	}
}

func TestLoadAllDMails_Multiple(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: three D-Mails saved
	for _, d := range []DMail{
		{ID: "d-002", Severity: SeverityMedium, Status: DMailSent, Summary: "second"},
		{ID: "d-001", Severity: SeverityLow, Status: DMailSent, Summary: "first"},
		{ID: "d-003", Severity: SeverityHigh, Status: DMailPending, Summary: "third"},
	} {
		if err := store.SaveDMail(d); err != nil {
			t.Fatal(err)
		}
	}

	// when
	dmails, err := store.LoadAllDMails()

	// then
	if err != nil {
		t.Fatalf("LoadAllDMails failed: %v", err)
	}
	if len(dmails) != 3 {
		t.Fatalf("expected 3 D-Mails, got %d", len(dmails))
	}
	// sorted by ID ascending
	if dmails[0].ID != "d-001" {
		t.Errorf("expected first D-Mail d-001, got %s", dmails[0].ID)
	}
	if dmails[2].ID != "d-003" {
		t.Errorf("expected last D-Mail d-003, got %s", dmails[2].ID)
	}
}

func TestLoadAllDMails_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// when
	dmails, err := store.LoadAllDMails()

	// then
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(dmails) != 0 {
		t.Errorf("expected 0 D-Mails, got %d", len(dmails))
	}
}

func TestDMail_LinearIssueID_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: a D-Mail with LinearIssueID set
	issueID := "MY-250"
	dmail := DMail{
		ID:            "d-001",
		Severity:      SeverityHigh,
		Status:        DMailPending,
		Target:        TargetSightjack,
		Summary:       "test",
		LinearIssueID: &issueID,
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	// when
	loaded, err := store.LoadDMail("d-001")

	// then
	if err != nil {
		t.Fatalf("LoadDMail failed: %v", err)
	}
	if loaded.LinearIssueID == nil || *loaded.LinearIssueID != "MY-250" {
		t.Errorf("expected LinearIssueID MY-250, got %v", loaded.LinearIssueID)
	}
}

func TestDMail_LinearIssueID_OmittedWhenNil(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: a D-Mail without LinearIssueID
	dmail := DMail{ID: "d-001", Severity: SeverityLow, Status: DMailSent}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	// when
	loaded, err := store.LoadDMail("d-001")

	// then
	if err != nil {
		t.Fatalf("LoadDMail failed: %v", err)
	}
	if loaded.LinearIssueID != nil {
		t.Errorf("expected LinearIssueID nil, got %v", *loaded.LinearIssueID)
	}
}

func TestLoadUnsyncedDMails_FiltersLinked(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: 3 D-Mails, 1 already linked
	issueID := "MY-100"
	for _, d := range []DMail{
		{ID: "d-001", Severity: SeverityLow, Status: DMailSent, Summary: "unsynced 1"},
		{ID: "d-002", Severity: SeverityHigh, Status: DMailPending, Summary: "linked", LinearIssueID: &issueID},
		{ID: "d-003", Severity: SeverityMedium, Status: DMailSent, Summary: "unsynced 2"},
	} {
		if err := store.SaveDMail(d); err != nil {
			t.Fatal(err)
		}
	}

	// when
	unsynced, err := store.LoadUnsyncedDMails()

	// then
	if err != nil {
		t.Fatalf("LoadUnsyncedDMails failed: %v", err)
	}
	if len(unsynced) != 2 {
		t.Fatalf("expected 2 unsynced, got %d", len(unsynced))
	}
	if unsynced[0].ID != "d-001" {
		t.Errorf("expected first unsynced d-001, got %s", unsynced[0].ID)
	}
	if unsynced[1].ID != "d-003" {
		t.Errorf("expected second unsynced d-003, got %s", unsynced[1].ID)
	}
}

func TestLoadUnsyncedDMails_AllLinked(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: all D-Mails already linked
	issueID := "MY-100"
	dmail := DMail{ID: "d-001", Severity: SeverityLow, Status: DMailSent, LinearIssueID: &issueID}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	// when
	unsynced, err := store.LoadUnsyncedDMails()

	// then
	if err != nil {
		t.Fatalf("LoadUnsyncedDMails failed: %v", err)
	}
	if len(unsynced) != 0 {
		t.Errorf("expected 0 unsynced, got %d", len(unsynced))
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

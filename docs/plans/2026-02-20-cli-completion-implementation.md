# CLI Completion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Complete the three-command CLI surface (resolve, log, --quiet) using local file operations only.

**Architecture:** Extend StateStore with D-Mail/History loading, add ResolveDMail and PrintLog to Amadeus orchestrator, wire three new CLI routes in cmd/amadeus/main.go. TDD throughout.

**Tech Stack:** Go 1.26, existing flat package (`package amadeus`), `encoding/json`, `os`, `path/filepath`, `sort`, `time`

**Design doc:** `docs/plans/2026-02-20-cli-completion-design.md`

---

### Task 0: StateStore.LoadDMail — Load Single D-Mail

**Files:**

- Modify: `dmail.go`
- Modify: `dmail_test.go`

**Step 1: Write failing test**

Add to `dmail_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL — `LoadDMail` not defined

**Step 3: Write minimal implementation**

Add to `dmail.go`:

```go
// LoadDMail reads a single D-Mail by ID from the dmails/ directory.
func (s *StateStore) LoadDMail(id string) (DMail, error) {
	path := filepath.Join(s.Root, "dmails", id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return DMail{}, fmt.Errorf("load dmail %s: %w", id, err)
	}
	var dmail DMail
	if err := json.Unmarshal(data, &dmail); err != nil {
		return DMail{}, fmt.Errorf("parse dmail %s: %w", id, err)
	}
	return dmail, nil
}
```

Add `"encoding/json"` to the imports in `dmail.go`.

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add dmail.go dmail_test.go
git commit -m "feat: add StateStore.LoadDMail for single D-Mail loading"
```

---

### Task 1: StateStore.LoadAllDMails — Load All D-Mails

**Files:**

- Modify: `dmail.go`
- Modify: `dmail_test.go`

**Step 1: Write failing test**

Add to `dmail_test.go`:

```go
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
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL — `LoadAllDMails` not defined

**Step 3: Write minimal implementation**

Add to `dmail.go`:

```go
import "sort"
```

```go
// LoadAllDMails reads all D-Mails from the dmails/ directory, sorted by ID ascending.
func (s *StateStore) LoadAllDMails() ([]DMail, error) {
	dmailDir := filepath.Join(s.Root, "dmails")
	entries, err := os.ReadDir(dmailDir)
	if err != nil {
		return nil, err
	}
	var dmails []DMail
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		dmail, err := s.LoadDMail(id)
		if err != nil {
			return nil, err
		}
		dmails = append(dmails, dmail)
	}
	sort.Slice(dmails, func(i, j int) bool {
		return dmails[i].ID < dmails[j].ID
	})
	return dmails, nil
}
```

Add `"sort"` to the imports in `dmail.go`.

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add dmail.go dmail_test.go
git commit -m "feat: add StateStore.LoadAllDMails for listing all D-Mails"
```

---

### Task 2: Amadeus.ResolveDMail — Resolve Logic

**Files:**

- Modify: `amadeus.go`
- Modify: `amadeus_test.go`

**Step 1: Write failing tests**

Add to `amadeus_test.go`:

```go
func TestResolveDMail_Approve(t *testing.T) {
	// given: a pending HIGH D-Mail
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
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
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.ResolveDMail("d-001", "approve", "")

	// then
	if err != nil {
		t.Fatalf("ResolveDMail failed: %v", err)
	}
	loaded, _ := store.LoadDMail("d-001")
	if loaded.Status != DMailApproved {
		t.Errorf("expected status approved, got %s", loaded.Status)
	}
	if loaded.ResolvedAt == nil {
		t.Error("expected ResolvedAt to be set")
	}
	if loaded.ResolvedAction == nil || *loaded.ResolvedAction != "approve" {
		t.Errorf("expected ResolvedAction approve, got %v", loaded.ResolvedAction)
	}
}

func TestResolveDMail_Reject(t *testing.T) {
	// given: a pending HIGH D-Mail
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
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
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.ResolveDMail("d-001", "reject", "false positive")

	// then
	if err != nil {
		t.Fatalf("ResolveDMail failed: %v", err)
	}
	loaded, _ := store.LoadDMail("d-001")
	if loaded.Status != DMailRejected {
		t.Errorf("expected status rejected, got %s", loaded.Status)
	}
	if loaded.RejectReason == nil || *loaded.RejectReason != "false positive" {
		t.Errorf("expected reject reason 'false positive', got %v", loaded.RejectReason)
	}
}

func TestResolveDMail_AlreadyResolved(t *testing.T) {
	// given: an already approved D-Mail
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	now := time.Now().UTC()
	action := "approve"
	dmail := DMail{
		ID:             "d-001",
		Severity:       SeverityHigh,
		Status:         DMailApproved,
		ResolvedAt:     &now,
		ResolvedAction: &action,
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.ResolveDMail("d-001", "reject", "oops")

	// then: should error
	if err == nil {
		t.Error("expected error when resolving already-resolved D-Mail")
	}
}

func TestResolveDMail_NotFound(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.ResolveDMail("d-999", "approve", "")

	// then
	if err == nil {
		t.Error("expected error for non-existent D-Mail")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL — `ResolveDMail` not defined

**Step 3: Write minimal implementation**

Add to `amadeus.go`:

```go
// ResolveDMail updates a pending D-Mail to approved or rejected status.
// action must be "approve" or "reject". reason is required for reject.
func (a *Amadeus) ResolveDMail(id string, action string, reason string) error {
	dmail, err := a.Store.LoadDMail(id)
	if err != nil {
		return err
	}
	if dmail.Status != DMailPending {
		return fmt.Errorf("D-Mail %s is already %s", id, dmail.Status)
	}

	now := time.Now().UTC()
	dmail.ResolvedAt = &now
	dmail.ResolvedAction = &action

	switch action {
	case "approve":
		dmail.Status = DMailApproved
	case "reject":
		dmail.Status = DMailRejected
		dmail.RejectReason = &reason
	default:
		return fmt.Errorf("unknown action: %s (use --approve or --reject)", action)
	}

	if err := a.Store.SaveDMail(dmail); err != nil {
		return fmt.Errorf("save resolved dmail: %w", err)
	}

	a.Logger.Info("D-Mail %s %s.", id, action+"d")
	a.Logger.Info("%s → %sd at %s", dmail.Summary, action, now.Format(time.RFC3339))
	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add amadeus.go amadeus_test.go
git commit -m "feat: add Amadeus.ResolveDMail for D-Mail approval/rejection"
```

---

### Task 3: CLI resolve Subcommand

**Files:**

- Modify: `cmd/amadeus/main.go`

**Step 1: Write the resolve CLI handler**

Add to `cmd/amadeus/main.go`:

```go
func runResolve(configPath string, verbose bool, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: amadeus resolve <id> --approve or --reject --reason \"...\"")
	}
	id := args[0]

	fs := flag.NewFlagSet("resolve-action", flag.ContinueOnError)
	var approve, reject bool
	var reason string
	fs.BoolVar(&approve, "approve", false, "approve D-Mail")
	fs.BoolVar(&reject, "reject", false, "reject D-Mail")
	fs.StringVar(&reason, "reason", "", "rejection reason (required with --reject)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	if approve == reject {
		return fmt.Errorf("specify exactly one of --approve or --reject")
	}
	if reject && reason == "" {
		return fmt.Errorf("--reason is required with --reject")
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	divRoot := filepath.Join(repoRoot, ".divergence")

	if configPath == "" {
		configPath = filepath.Join(divRoot, "config.yaml")
	}
	cfg, err := amadeus.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := amadeus.NewLogger(os.Stdout, verbose)
	a := &amadeus.Amadeus{
		Config: cfg,
		Store:  amadeus.NewStateStore(divRoot),
		Logger: logger,
	}

	action := "approve"
	if reject {
		action = "reject"
	}
	return a.ResolveDMail(id, action, reason)
}
```

Update the `run()` function's switch statement:

```go
switch cmd {
case "check":
	return runCheck(configPath, verbose, dryRun, full)
case "resolve":
	return runResolve(configPath, verbose, fs.Args())
default:
	return fmt.Errorf("unknown command: %s (available: check, resolve, log)", cmd)
}
```

Update the `run()` function's usage message:

```go
return fmt.Errorf("usage: amadeus <check|resolve|log> [flags]")
```

Note: The `resolve` subcommand needs the positional `<id>` argument before its own flags. The main `run()` function parses global flags (`-c`, `-v`) from `os.Args[2:]`, then passes `fs.Args()` (remaining args after global flag parsing) to `runResolve`. The `runResolve` function treats `args[0]` as the D-Mail ID and parses `args[1:]` as action flags with a second `flag.NewFlagSet`.

**Step 2: Build and verify**

Run: `just build`
Expected: Binary compiles

**Step 3: Commit**

```bash
git add cmd/amadeus/main.go
git commit -m "feat: add CLI resolve subcommand with --approve/--reject"
```

---

### Task 4: StateStore.LoadHistory — Load All History Entries

**Files:**

- Modify: `state.go`
- Modify: `state_test.go`

**Step 1: Write failing test**

Add to `state_test.go`:

```go
func TestLoadHistory_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: three history entries at different times
	r1 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC),
		Commit:     "aaa",
		Type:       CheckTypeFull,
		Divergence: 0.10,
	}
	r2 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC),
		Commit:     "bbb",
		Type:       CheckTypeDiff,
		Divergence: 0.13,
	}
	r3 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
		Commit:     "ccc",
		Type:       CheckTypeDiff,
		Divergence: 0.15,
	}
	for _, r := range []CheckResult{r1, r2, r3} {
		if err := store.SaveHistory(r); err != nil {
			t.Fatal(err)
		}
	}

	// when
	history, err := store.LoadHistory()

	// then
	if err != nil {
		t.Fatalf("LoadHistory failed: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(history))
	}
	// sorted newest first (descending)
	if history[0].Commit != "ccc" {
		t.Errorf("expected newest first (ccc), got %s", history[0].Commit)
	}
	if history[2].Commit != "aaa" {
		t.Errorf("expected oldest last (aaa), got %s", history[2].Commit)
	}
}

func TestLoadHistory_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// when
	history, err := store.LoadHistory()

	// then
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected 0 entries, got %d", len(history))
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL — `LoadHistory` not defined

**Step 3: Write minimal implementation**

Add to `state.go`:

```go
import "sort"
```

```go
// LoadHistory reads all check results from the history/ directory,
// sorted by CheckedAt descending (newest first).
func (s *StateStore) LoadHistory() ([]CheckResult, error) {
	histDir := filepath.Join(s.Root, "history")
	entries, err := os.ReadDir(histDir)
	if err != nil {
		return nil, err
	}
	var results []CheckResult
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(histDir, e.Name()))
		if err != nil {
			return nil, err
		}
		var r CheckResult
		if err := json.Unmarshal(data, &r); err != nil {
			return nil, fmt.Errorf("parse history %s: %w", e.Name(), err)
		}
		results = append(results, r)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].CheckedAt.After(results[j].CheckedAt)
	})
	return results, nil
}
```

Add `"sort"` and `"strings"` to the imports in `state.go` (if not already present).

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add state.go state_test.go
git commit -m "feat: add StateStore.LoadHistory for listing past checks"
```

---

### Task 5: Amadeus.PrintLog — Log Output Rendering

**Files:**

- Modify: `amadeus.go`
- Modify: `amadeus_test.go`

**Step 1: Write failing test**

Add to `amadeus_test.go`:

```go
func TestAmadeus_PrintLog(t *testing.T) {
	// given: history and D-Mails
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	r1 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC),
		Commit:     "aaa",
		Type:       CheckTypeFull,
		Divergence: 0.10,
	}
	r2 := CheckResult{
		CheckedAt:  time.Date(2026, 2, 20, 14, 30, 0, 0, time.UTC),
		Commit:     "bbb",
		Type:       CheckTypeDiff,
		Divergence: 0.15,
		DMails:     []string{"d-001"},
	}
	for _, r := range []CheckResult{r1, r2} {
		if err := store.SaveHistory(r); err != nil {
			t.Fatal(err)
		}
	}

	dmail := DMail{
		ID:       "d-001",
		Severity: SeverityHigh,
		Status:   DMailPending,
		Target:   TargetSightjack,
		Summary:  "ADR-003 violation",
	}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.PrintLog()

	// then
	if err != nil {
		t.Fatalf("PrintLog failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "History:") {
		t.Errorf("expected 'History:' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "bbb") {
		t.Errorf("expected commit 'bbb' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "d-001") {
		t.Errorf("expected 'd-001' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "pending") {
		t.Errorf("expected 'pending' in output, got:\n%s", output)
	}
}

func TestAmadeus_PrintLog_Empty(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.PrintLog()

	// then
	if err != nil {
		t.Fatalf("PrintLog failed: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "No history") {
		t.Errorf("expected 'No history' in output, got:\n%s", output)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL — `PrintLog` not defined

**Step 3: Write minimal implementation**

Add to `amadeus.go`:

```go
// PrintLog renders the history and D-Mail log to the logger.
func (a *Amadeus) PrintLog() error {
	history, err := a.Store.LoadHistory()
	if err != nil {
		return fmt.Errorf("load history: %w", err)
	}

	a.Logger.Info("")
	if len(history) == 0 {
		a.Logger.Info("No history yet. Run `amadeus check` first.")
		return nil
	}

	a.Logger.Info("History:")
	for i, h := range history {
		var delta string
		if h.Type == CheckTypeFull {
			delta = "(baseline)"
		} else if i+1 < len(history) {
			delta = "(" + FormatDelta(h.Divergence, history[i+1].Divergence) + ")"
		} else {
			delta = "(first)"
		}
		dmailCount := len(h.DMails)
		dmailLabel := "D-Mails"
		if dmailCount == 1 {
			dmailLabel = "D-Mail"
		}
		a.Logger.Info("  %s  %s  %-4s  %s %s  %d %s",
			h.CheckedAt.Format("2006-01-02T15:04"),
			h.Commit,
			string(h.Type),
			FormatDivergence(h.Divergence*100),
			delta,
			dmailCount,
			dmailLabel)
	}

	dmails, err := a.Store.LoadAllDMails()
	if err != nil {
		return fmt.Errorf("load dmails: %w", err)
	}

	if len(dmails) > 0 {
		a.Logger.Info("")
		a.Logger.Info("D-Mails:")
		for _, d := range dmails {
			var severityTag string
			switch d.Severity {
			case SeverityHigh:
				severityTag = "[HIGH]"
			case SeverityMedium:
				severityTag = "[MED] "
			default:
				severityTag = "[LOW] "
			}
			a.Logger.Info("  %s  %s %-10s %s → %s",
				d.ID,
				severityTag,
				string(d.Status),
				d.Summary,
				string(d.Target))
		}
	}

	return nil
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add amadeus.go amadeus_test.go
git commit -m "feat: add Amadeus.PrintLog for history and D-Mail display"
```

---

### Task 6: CLI log Subcommand

**Files:**

- Modify: `cmd/amadeus/main.go`

**Step 1: Write the log CLI handler**

Add to `cmd/amadeus/main.go`:

```go
func runLog(configPath string, verbose bool) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	divRoot := filepath.Join(repoRoot, ".divergence")

	if configPath == "" {
		configPath = filepath.Join(divRoot, "config.yaml")
	}
	cfg, err := amadeus.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := amadeus.NewLogger(os.Stdout, verbose)
	a := &amadeus.Amadeus{
		Config: cfg,
		Store:  amadeus.NewStateStore(divRoot),
		Logger: logger,
	}
	return a.PrintLog()
}
```

Update the `run()` function's switch:

```go
case "log":
	return runLog(configPath, verbose)
```

**Step 2: Build and verify**

Run: `just build`
Expected: Binary compiles

**Step 3: Commit**

```bash
git add cmd/amadeus/main.go
git commit -m "feat: add CLI log subcommand"
```

---

### Task 7: --quiet Flag for check

**Files:**

- Modify: `amadeus.go`
- Modify: `amadeus_test.go`
- Modify: `cmd/amadeus/main.go`

**Step 1: Write failing test**

Add to `amadeus_test.go`:

```go
func TestAmadeus_PrintCheckOutput_Quiet(t *testing.T) {
	var buf bytes.Buffer
	a := &Amadeus{
		Config: DefaultConfig(),
		Logger: NewLogger(&buf, false),
	}
	result := CheckResult{
		Divergence: 0.145,
		Axes: map[Axis]AxisScore{
			AxisADR:        {Score: 15, Details: "ADR-003"},
			AxisDoD:        {Score: 20, Details: "edge case"},
			AxisDependency: {Score: 10, Details: "clean"},
			AxisImplicit:   {Score: 5, Details: "naming"},
		},
	}
	dmails := []DMail{
		{ID: "d-001", Severity: SeverityLow, Status: DMailSent},
		{ID: "d-002", Severity: SeverityHigh, Status: DMailPending},
	}

	// when: quiet mode
	a.PrintCheckOutputQuiet(result, dmails, 0.133)

	// then: single line with divergence, delta, dmail count, pending count
	output := strings.TrimSpace(buf.String())
	lines := strings.Split(output, "\n")
	if len(lines) != 1 {
		t.Errorf("expected 1 line in quiet mode, got %d:\n%s", len(lines), output)
	}
	if !strings.Contains(output, "0.145000") {
		t.Errorf("expected divergence value in output, got:\n%s", output)
	}
	if !strings.Contains(output, "2 D-Mails") {
		t.Errorf("expected '2 D-Mails' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "1 pending") {
		t.Errorf("expected '1 pending' in output, got:\n%s", output)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL — `PrintCheckOutputQuiet` not defined

**Step 3: Write minimal implementation**

Add to `amadeus.go`:

```go
// PrintCheckOutputQuiet renders a single-line summary for --quiet mode.
func (a *Amadeus) PrintCheckOutputQuiet(result CheckResult, dmails []DMail, previousDivergence float64) {
	pending := 0
	for _, d := range dmails {
		if d.Status == DMailPending {
			pending++
		}
	}
	dmailLabel := "D-Mails"
	if len(dmails) == 1 {
		dmailLabel = "D-Mail"
	}

	pendingStr := ""
	if pending > 0 {
		pendingStr = fmt.Sprintf(" (%d pending)", pending)
	}

	a.Logger.Info("%s (%s) %d %s%s",
		FormatDivergence(result.Divergence*100),
		FormatDelta(result.Divergence, previousDivergence),
		len(dmails),
		dmailLabel,
		pendingStr)
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Add --quiet flag to CLI and CheckOptions**

Update `CheckOptions` in `amadeus.go`:

```go
type CheckOptions struct {
	Full   bool
	DryRun bool
	Quiet  bool
}
```

Update `RunCheck` to use quiet output. Replace the `a.PrintCheckOutput(result, dmails, previous.Divergence)` line near the end of `RunCheck` with:

```go
if opts.Quiet {
	a.PrintCheckOutputQuiet(result, dmails, previous.Divergence)
} else {
	a.PrintCheckOutput(result, dmails, previous.Divergence)
}
```

Update `cmd/amadeus/main.go`: add `quiet` flag to the check FlagSet:

```go
var quiet bool
fs.BoolVar(&quiet, "quiet", false, "summary-only output")
fs.BoolVar(&quiet, "q", false, "summary-only output")
```

Pass it through to `runCheck` and into `CheckOptions{Quiet: quiet}`.

**Step 6: Run all tests**

Run: `just test`
Expected: PASS

**Step 7: Build**

Run: `just build`
Expected: Binary compiles

**Step 8: Commit**

```bash
git add amadeus.go amadeus_test.go cmd/amadeus/main.go
git commit -m "feat: add --quiet flag for summary-only check output"
```

---

### Task 8: Final Verification

**Step 1: Run full check suite**

Run: `just check`
Expected: fmt + lint + all tests pass

**Step 2: Build binary and smoke test**

Run:

```bash
just build && ./amadeus check --dry-run --quiet
```

Expected: Single-line output or prompt (depending on state)

**Step 3: Verify resolve usage error**

Run:

```bash
./amadeus resolve
```

Expected: Usage error message

**Step 4: Verify log with empty state**

Run:

```bash
./amadeus log
```

Expected: "No history yet" message (or history if `.divergence/` has data)

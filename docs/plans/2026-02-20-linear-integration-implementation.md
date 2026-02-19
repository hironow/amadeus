# Linear Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** D-Mail を Linear issue として同期するための `sync` / `link` コマンドと D-Mail 構造体拡張を実装する。

**Architecture:** Go コードは Linear API を直接呼ばない。`sync` コマンドで未同期 D-Mail を JSON 出力し、Claude Code が Linear MCP 経由で issue 作成後、`link` コマンドで紐付ける。

**Tech Stack:** Go 1.26, flat package `amadeus`, JSON stdio

---

### Task 0: DMail 構造体に LinearIssueID フィールドを追加

**Files:**
- Modify: `dmail.go:38-50`
- Test: `dmail_test.go`

**Step 1: Write the failing test**

`dmail_test.go` に追加:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestDMail_LinearIssueID -v -count=1`
Expected: FAIL with "unknown field LinearIssueID"

**Step 3: Write minimal implementation**

`dmail.go:38-50` の `DMail` struct に1行追加:

```go
type DMail struct {
	ID             string      `json:"id"`
	Severity       Severity    `json:"severity"`
	Status         DMailStatus `json:"status"`
	Target         DMailTarget `json:"target"`
	Type           string      `json:"type"`
	Summary        string      `json:"summary"`
	Detail         string      `json:"detail"`
	CreatedAt      time.Time   `json:"created_at"`
	ResolvedAt     *time.Time  `json:"resolved_at,omitempty"`
	ResolvedAction *string     `json:"resolved_action,omitempty"`
	RejectReason   *string     `json:"reject_reason,omitempty"`
	LinearIssueID  *string     `json:"linear_issue_id,omitempty"`
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add dmail.go dmail_test.go
git commit -m "feat: add LinearIssueID field to DMail struct"
```

---

### Task 1: LoadUnsyncedDMails 関数

**Files:**
- Modify: `dmail.go`
- Test: `dmail_test.go`

**Step 1: Write the failing test**

`dmail_test.go` に追加:

```go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestLoadUnsyncedDMails -v -count=1`
Expected: FAIL with "undefined: store.LoadUnsyncedDMails"

**Step 3: Write minimal implementation**

`dmail.go` に追加:

```go
// LoadUnsyncedDMails returns D-Mails that have no LinearIssueID, sorted by ID ascending.
func (s *StateStore) LoadUnsyncedDMails() ([]DMail, error) {
	all, err := s.LoadAllDMails()
	if err != nil {
		return nil, err
	}
	var unsynced []DMail
	for _, d := range all {
		if d.LinearIssueID == nil {
			unsynced = append(unsynced, d)
		}
	}
	return unsynced, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add dmail.go dmail_test.go
git commit -m "feat: add LoadUnsyncedDMails for filtering unlinked D-Mails"
```

---

### Task 2: LinkDMail ドメイン関数

**Files:**
- Modify: `amadeus.go`
- Test: `amadeus_test.go`

**Step 1: Write the failing test**

`amadeus_test.go` に追加:

```go
func TestLinkDMail_Success(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	dmail := DMail{ID: "d-001", Severity: SeverityLow, Status: DMailSent, Summary: "test"}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.LinkDMail("d-001", "MY-250")

	// then
	if err != nil {
		t.Fatalf("LinkDMail failed: %v", err)
	}
	loaded, _ := store.LoadDMail("d-001")
	if loaded.LinearIssueID == nil || *loaded.LinearIssueID != "MY-250" {
		t.Errorf("expected LinearIssueID MY-250, got %v", loaded.LinearIssueID)
	}
}

func TestLinkDMail_AlreadyLinked(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	issueID := "MY-100"
	dmail := DMail{ID: "d-001", Severity: SeverityLow, Status: DMailSent, LinearIssueID: &issueID}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.LinkDMail("d-001", "MY-250")

	// then
	if err == nil {
		t.Fatal("expected error for already-linked D-Mail")
	}
	if !strings.Contains(err.Error(), "already linked") {
		t.Errorf("expected 'already linked' error, got: %v", err)
	}
}

func TestLinkDMail_NotFound(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)
	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.LinkDMail("d-999", "MY-250")

	// then
	if err == nil {
		t.Fatal("expected error for non-existent D-Mail")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestLinkDMail -v -count=1`
Expected: FAIL with "undefined: a.LinkDMail"

**Step 3: Write minimal implementation**

`amadeus.go` に追加:

```go
// LinkDMail associates a D-Mail with a Linear issue ID.
// Returns an error if the D-Mail is already linked.
func (a *Amadeus) LinkDMail(dmailID string, linearIssueID string) error {
	dmail, err := a.Store.LoadDMail(dmailID)
	if err != nil {
		return err
	}
	if dmail.LinearIssueID != nil {
		return fmt.Errorf("D-Mail %s is already linked to %s", dmailID, *dmail.LinearIssueID)
	}
	dmail.LinearIssueID = &linearIssueID
	if err := a.Store.SaveDMail(dmail); err != nil {
		return fmt.Errorf("save linked dmail: %w", err)
	}
	a.Logger.Info("D-Mail %s linked to %s", dmailID, linearIssueID)
	return nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add amadeus.go amadeus_test.go
git commit -m "feat: add LinkDMail for associating D-Mails with Linear issues"
```

---

### Task 3: SyncOutput 構造体と PrintSync 関数

**Files:**
- Modify: `amadeus.go`
- Test: `amadeus_test.go`

**Step 1: Write the failing test**

`amadeus_test.go` に追加:

```go
func TestPrintSync_UnsyncedDMails(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: 2 unsynced D-Mails
	for _, d := range []DMail{
		{ID: "d-001", Severity: SeverityHigh, Status: DMailPending, Target: TargetSightjack, Summary: "ADR violation"},
		{ID: "d-002", Severity: SeverityLow, Status: DMailSent, Target: TargetPaintress, Summary: "naming issue"},
	} {
		if err := store.SaveDMail(d); err != nil {
			t.Fatal(err)
		}
	}

	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.PrintSync()

	// then
	if err != nil {
		t.Fatalf("PrintSync failed: %v", err)
	}
	output := buf.String()
	// verify valid JSON output
	var result struct {
		Unsynced []DMail `json:"unsynced"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}
	if len(result.Unsynced) != 2 {
		t.Errorf("expected 2 unsynced, got %d", len(result.Unsynced))
	}
}

func TestPrintSync_NoneUnsynced(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".divergence")
	if err := InitDivergenceDir(root); err != nil {
		t.Fatal(err)
	}
	store := NewStateStore(root)

	// given: all linked
	issueID := "MY-100"
	dmail := DMail{ID: "d-001", Severity: SeverityLow, Status: DMailSent, LinearIssueID: &issueID}
	if err := store.SaveDMail(dmail); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	a := &Amadeus{Config: DefaultConfig(), Store: store, Logger: NewLogger(&buf, false)}

	// when
	err := a.PrintSync()

	// then
	if err != nil {
		t.Fatalf("PrintSync failed: %v", err)
	}
	output := buf.String()
	var result struct {
		Unsynced []DMail `json:"unsynced"`
	}
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(result.Unsynced) != 0 {
		t.Errorf("expected 0 unsynced, got %d", len(result.Unsynced))
	}
}
```

**Note:** テストファイルの import に `"encoding/json"` が必要。既存の import になければ追加する。

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestPrintSync -v -count=1`
Expected: FAIL with "undefined: a.PrintSync"

**Step 3: Write minimal implementation**

`amadeus.go` に追加:

```go
// PrintSync outputs unsynced D-Mails as JSON to the logger output.
func (a *Amadeus) PrintSync() error {
	unsynced, err := a.Store.LoadUnsyncedDMails()
	if err != nil {
		return fmt.Errorf("load unsynced dmails: %w", err)
	}
	output := struct {
		Unsynced []DMail `json:"unsynced"`
	}{
		Unsynced: unsynced,
	}
	if output.Unsynced == nil {
		output.Unsynced = []DMail{}
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sync output: %w", err)
	}
	fmt.Fprintln(a.Logger.Writer(), string(data))
	return nil
}
```

**Note:** `Logger` に `Writer()` メソッドがなければ、`a.Logger` の `out` フィールドに直接アクセスする方法を確認する。`logger.go` を読んで、出力先の `io.Writer` を取得する方法を確認すること。もし `Writer()` がなければ追加する（`logger.go` に `func (l *Logger) Writer() io.Writer { return l.out }` を追加）。

**Step 4: Run test to verify it passes**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add amadeus.go amadeus_test.go logger.go
git commit -m "feat: add PrintSync for JSON output of unsynced D-Mails"
```

---

### Task 4: CLI `sync` サブコマンド

**Files:**
- Modify: `cmd/amadeus/main.go:25,57-66`

**Step 1: Update usage message**

`cmd/amadeus/main.go:25` を更新:

```go
return fmt.Errorf("usage: amadeus <check|resolve|log|sync|link> [flags]")
```

**Step 2: Add sync case to switch**

`cmd/amadeus/main.go` の switch 文に追加:

```go
case "sync":
	return runSync(configPath, verbose)
```

**Step 3: Add runSync function**

`cmd/amadeus/main.go` に追加:

```go
func runSync(configPath string, verbose bool) error {
	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	divRoot := filepath.Join(repoRoot, ".divergence")

	if _, err := os.Stat(divRoot); os.IsNotExist(err) {
		return fmt.Errorf(".divergence/ not found. Run 'amadeus check' first")
	}

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
	return a.PrintSync()
}
```

**Step 4: Update default error message**

```go
return fmt.Errorf("unknown command: %s (available: check, resolve, log, sync, link)", cmd)
```

**Step 5: Build and verify**

Run: `go build ./cmd/amadeus && ./amadeus sync 2>&1 || true`
Expected: Either JSON output or ".divergence/ not found" error

**Step 6: Commit**

```bash
git add cmd/amadeus/main.go
git commit -m "feat: add CLI sync subcommand for listing unsynced D-Mails"
```

---

### Task 5: CLI `link` サブコマンド

**Files:**
- Modify: `cmd/amadeus/main.go`

**Step 1: Add link case to switch**

```go
case "link":
	return runLink(configPath, verbose, fs.Args())
```

**Step 2: Add runLink function**

```go
func runLink(configPath string, verbose bool, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: amadeus link <dmail-id> <linear-issue-id>")
	}
	dmailID := args[0]
	linearIssueID := args[1]

	repoRoot, err := os.Getwd()
	if err != nil {
		return err
	}
	divRoot := filepath.Join(repoRoot, ".divergence")

	if _, err := os.Stat(divRoot); os.IsNotExist(err) {
		return fmt.Errorf(".divergence/ not found. Run 'amadeus check' first")
	}

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
	return a.LinkDMail(dmailID, linearIssueID)
}
```

**Step 3: Build and verify**

Run: `go build ./cmd/amadeus`
Expected: Build success

Smoke test:
```bash
./amadeus link 2>&1 || true
# Expected: "usage: amadeus link <dmail-id> <linear-issue-id>"
```

**Step 4: Commit**

```bash
git add cmd/amadeus/main.go
git commit -m "feat: add CLI link subcommand for D-Mail to Linear issue binding"
```

---

### Task 6: Linear に D-Mail ラベルを作成

**Note:** これはコードではなく Linear MCP を使った1回限りのセットアップ。

**Step 1: D-Mail ラベルの作成**

Claude Code で Linear MCP ツールを使用:

```
mcp__plugin_linear_linear__create_issue_label(name: "D-Mail")
```

**Step 2: Commit なし**（コード変更なし）

---

### Task 7: 最終検証

**Step 1: 全テスト実行**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 2: ビルド**

Run: `go build ./cmd/amadeus`
Expected: Build success

**Step 3: スモークテスト**

```bash
./amadeus sync 2>&1 || true
./amadeus link 2>&1 || true
./amadeus link d-001 MY-250 2>&1 || true
```

Expected outputs:
- sync: JSON output or ".divergence/ not found"
- link (no args): usage message
- link (with args): D-Mail not found or success

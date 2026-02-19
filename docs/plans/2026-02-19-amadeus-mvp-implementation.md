# Amadeus MVP Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build the core engine of Amadeus — a post-merge integrity verification CLI that scores codebase divergence across 4 axes and generates D-Mails for correction routing.

**Architecture:** Go CLI with flat package structure (Approach A). Pure scoring logic separated from I/O. Claude Code invoked as external process for semantic evaluation. State persisted in `.divergence/` directory within target repository.

**Tech Stack:** Go 1.26, `gopkg.in/yaml.v3`, `just` task runner, Claude CLI

**Design doc:** `docs/plans/2026-02-19-amadeus-mvp-design.md`

---

### Task 0: Project Scaffolding

**Files:**

- Create: `go.mod`
- Create: `justfile`
- Modify: `.gitignore` (already exists, verify)

**Step 1: Initialize Go module**

Run:

```bash
cd /Users/nino/amadeus && go mod init github.com/hironow/amadeus
```

**Step 2: Add YAML dependency**

Run:

```bash
cd /Users/nino/amadeus && go get gopkg.in/yaml.v3
```

**Step 3: Create justfile**

```just
# amadeus task runner

# Run all tests
test:
    go test ./... -count=1 -timeout=60s

# Run tests with verbose output
test-v:
    go test ./... -count=1 -timeout=60s -v

# Run tests with coverage
cover:
    go test ./... -coverprofile=coverage.out -count=1 -timeout=60s
    go tool cover -func=coverage.out

# Build binary
build:
    go build -o amadeus ./cmd/amadeus

# Run linter
lint:
    go vet ./...

# Format code
fmt:
    gofmt -w .

# Format, vet, test — full check before commit
check: fmt lint test

# Clean build artifacts
clean:
    rm -f amadeus coverage.out
```

**Step 4: Verify .gitignore covers essentials**

Existing `.gitignore` already has `/amadeus`, `coverage.out`, `.divergence/`, `.gocache/`. Verify it is correct.

**Step 5: Commit**

```bash
git add go.mod go.sum justfile .gitignore
git commit -m "scaffold: initialize Go module and justfile"
```

---

### Task 1: Scoring — Types and CalcDivergence

**Files:**

- Create: `scoring.go`
- Create: `scoring_test.go`

This is the most important file. Pure functions, no I/O, no dependencies. TDD starts here.

**Step 1: Write failing tests for CalcDivergence**

```go
// scoring_test.go
package amadeus

import (
 "math"
 "testing"
)

func almostEqual(a, b float64) bool {
 return math.Abs(a-b) < 1e-9
}

func TestCalcDivergence_AllZero(t *testing.T) {
 axes := map[Axis]AxisScore{
  AxisADR:        {Score: 0},
  AxisDoD:        {Score: 0},
  AxisDependency: {Score: 0},
  AxisImplicit:   {Score: 0},
 }
 result := CalcDivergence(axes, DefaultWeights())
 if !almostEqual(result.Value, 0.0) {
  t.Errorf("expected 0.000000, got %f", result.Value)
 }
 if !almostEqual(result.Internal, 0.0) {
  t.Errorf("expected internal 0.0, got %f", result.Internal)
 }
}

func TestCalcDivergence_MaxDeviation(t *testing.T) {
 axes := map[Axis]AxisScore{
  AxisADR:        {Score: 100},
  AxisDoD:        {Score: 100},
  AxisDependency: {Score: 100},
  AxisImplicit:   {Score: 100},
 }
 result := CalcDivergence(axes, DefaultWeights())
 if !almostEqual(result.Value, 1.0) {
  t.Errorf("expected 1.000000, got %f", result.Value)
 }
 if !almostEqual(result.Internal, 100.0) {
  t.Errorf("expected internal 100.0, got %f", result.Internal)
 }
}

func TestCalcDivergence_WeightedSum(t *testing.T) {
 // From architecture doc example:
 // ADR=15, DoD=20, Dep=10, Implicit=5
 // Internal = 15*0.4 + 20*0.3 + 10*0.2 + 5*0.1 = 6+6+2+0.5 = 14.5
 axes := map[Axis]AxisScore{
  AxisADR:        {Score: 15, Details: "ADR-003 minor tension"},
  AxisDoD:        {Score: 20, Details: "Issue #42 edge case"},
  AxisDependency: {Score: 10, Details: "clean"},
  AxisImplicit:   {Score: 5, Details: "naming drift in cart"},
 }
 result := CalcDivergence(axes, DefaultWeights())
 if !almostEqual(result.Internal, 14.5) {
  t.Errorf("expected internal 14.5, got %f", result.Internal)
 }
 if !almostEqual(result.Value, 0.145) {
  t.Errorf("expected 0.145000, got %f", result.Value)
 }
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL — types and functions not defined

**Step 3: Write minimal implementation**

```go
// scoring.go
package amadeus

// Axis represents an evaluation axis for divergence scoring.
type Axis string

const (
 AxisADR        Axis = "adr_integrity"
 AxisDoD        Axis = "dod_fulfillment"
 AxisDependency Axis = "dependency_integrity"
 AxisImplicit   Axis = "implicit_constraints"
)

// AxisScore holds the score and details for a single evaluation axis.
type AxisScore struct {
 Score   int    `json:"score"`
 Details string `json:"details"`
}

// Weights holds the configurable weights for each evaluation axis.
type Weights struct {
 ADRIntegrity        float64 `yaml:"adr_integrity" json:"adr_integrity"`
 DoDFulfillment      float64 `yaml:"dod_fulfillment" json:"dod_fulfillment"`
 DependencyIntegrity float64 `yaml:"dependency_integrity" json:"dependency_integrity"`
 ImplicitConstraints float64 `yaml:"implicit_constraints" json:"implicit_constraints"`
}

// DefaultWeights returns the standard weights from the architecture document.
func DefaultWeights() Weights {
 return Weights{
  ADRIntegrity:        0.4,
  DoDFulfillment:      0.3,
  DependencyIntegrity: 0.2,
  ImplicitConstraints: 0.1,
 }
}

// DivergenceResult holds the complete result of a divergence calculation.
type DivergenceResult struct {
 Value      float64            `json:"divergence"`
 Internal   float64            `json:"internal"`
 Axes       map[Axis]AxisScore `json:"axes"`
 Severity   Severity           `json:"severity"`
 Overridden bool               `json:"overridden"`
}

// Severity represents the D-Mail severity tier.
type Severity string

const (
 SeverityLow    Severity = "LOW"
 SeverityMedium Severity = "MEDIUM"
 SeverityHigh   Severity = "HIGH"
)

// CalcDivergence computes the weighted divergence score from axis scores.
func CalcDivergence(axes map[Axis]AxisScore, weights Weights) DivergenceResult {
 internal := float64(axes[AxisADR].Score)*weights.ADRIntegrity +
  float64(axes[AxisDoD].Score)*weights.DoDFulfillment +
  float64(axes[AxisDependency].Score)*weights.DependencyIntegrity +
  float64(axes[AxisImplicit].Score)*weights.ImplicitConstraints

 return DivergenceResult{
  Value:    internal / 100.0,
  Internal: internal,
  Axes:     axes,
 }
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add scoring.go scoring_test.go
git commit -m "feat: add CalcDivergence with weighted scoring"
```

---

### Task 2: Scoring — DetermineSeverity with Per-Axis Override

**Files:**

- Modify: `scoring.go`
- Modify: `scoring_test.go`

**Step 1: Write failing tests for DetermineSeverity**

Add to `scoring_test.go`:

```go
func TestDetermineSeverity_Low(t *testing.T) {
 result := DivergenceResult{Internal: 10.0, Value: 0.10, Axes: map[Axis]AxisScore{
  AxisADR: {Score: 10}, AxisDoD: {Score: 10}, AxisDependency: {Score: 10}, AxisImplicit: {Score: 10},
 }}
 sev := DetermineSeverity(result, DefaultThresholds())
 if sev.Severity != SeverityLow {
  t.Errorf("expected LOW, got %s", sev.Severity)
 }
 if sev.Overridden {
  t.Error("expected no override")
 }
}

func TestDetermineSeverity_Medium(t *testing.T) {
 result := DivergenceResult{Internal: 35.0, Value: 0.35, Axes: map[Axis]AxisScore{
  AxisADR: {Score: 30}, AxisDoD: {Score: 30}, AxisDependency: {Score: 30}, AxisImplicit: {Score: 30},
 }}
 sev := DetermineSeverity(result, DefaultThresholds())
 if sev.Severity != SeverityMedium {
  t.Errorf("expected MEDIUM, got %s", sev.Severity)
 }
}

func TestDetermineSeverity_High(t *testing.T) {
 result := DivergenceResult{Internal: 60.0, Value: 0.60, Axes: map[Axis]AxisScore{
  AxisADR: {Score: 50}, AxisDoD: {Score: 50}, AxisDependency: {Score: 50}, AxisImplicit: {Score: 50},
 }}
 sev := DetermineSeverity(result, DefaultThresholds())
 if sev.Severity != SeverityHigh {
  t.Errorf("expected HIGH, got %s", sev.Severity)
 }
}

func TestDetermineSeverity_ADROverrideForceHigh(t *testing.T) {
 // Total divergence is LOW (internal=24) but ADR axis=60 forces HIGH
 result := DivergenceResult{Internal: 24.0, Value: 0.24, Axes: map[Axis]AxisScore{
  AxisADR: {Score: 60}, AxisDoD: {Score: 0}, AxisDependency: {Score: 0}, AxisImplicit: {Score: 0},
 }}
 sev := DetermineSeverity(result, DefaultThresholds())
 if sev.Severity != SeverityHigh {
  t.Errorf("expected HIGH (ADR override), got %s", sev.Severity)
 }
 if !sev.Overridden {
  t.Error("expected override flag to be true")
 }
}

func TestDetermineSeverity_DoDOverrideForceHigh(t *testing.T) {
 result := DivergenceResult{Internal: 21.0, Value: 0.21, Axes: map[Axis]AxisScore{
  AxisADR: {Score: 0}, AxisDoD: {Score: 70}, AxisDependency: {Score: 0}, AxisImplicit: {Score: 0},
 }}
 sev := DetermineSeverity(result, DefaultThresholds())
 if sev.Severity != SeverityHigh {
  t.Errorf("expected HIGH (DoD override), got %s", sev.Severity)
 }
 if !sev.Overridden {
  t.Error("expected override flag to be true")
 }
}

func TestDetermineSeverity_DepOverrideForceMedium(t *testing.T) {
 result := DivergenceResult{Internal: 16.0, Value: 0.16, Axes: map[Axis]AxisScore{
  AxisADR: {Score: 0}, AxisDoD: {Score: 0}, AxisDependency: {Score: 80}, AxisImplicit: {Score: 0},
 }}
 sev := DetermineSeverity(result, DefaultThresholds())
 if sev.Severity != SeverityMedium {
  t.Errorf("expected MEDIUM (Dep override), got %s", sev.Severity)
 }
 if !sev.Overridden {
  t.Error("expected override flag to be true")
 }
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL — `DetermineSeverity` and `DefaultThresholds` not defined

**Step 3: Write minimal implementation**

Add to `scoring.go`:

```go
// Thresholds holds the severity threshold configuration.
type Thresholds struct {
 LowMax    float64 `yaml:"low_max" json:"low_max"`
 MediumMax float64 `yaml:"medium_max" json:"medium_max"`
}

// PerAxisOverride holds per-axis critical thresholds that escalate severity.
type PerAxisOverride struct {
 ADRForceHigh int `yaml:"adr_integrity_force_high" json:"adr_integrity_force_high"`
 DoDForceHigh int `yaml:"dod_fulfillment_force_high" json:"dod_fulfillment_force_high"`
 DepForceMedium int `yaml:"dependency_integrity_force_medium" json:"dependency_integrity_force_medium"`
}

// SeverityConfig combines thresholds and per-axis overrides.
type SeverityConfig struct {
 Thresholds      Thresholds      `yaml:"thresholds" json:"thresholds"`
 PerAxisOverride PerAxisOverride `yaml:"per_axis_override" json:"per_axis_override"`
}

// DefaultThresholds returns the standard thresholds from the architecture document.
func DefaultThresholds() SeverityConfig {
 return SeverityConfig{
  Thresholds: Thresholds{
   LowMax:    0.250000,
   MediumMax: 0.500000,
  },
  PerAxisOverride: PerAxisOverride{
   ADRForceHigh:   60,
   DoDForceHigh:   70,
   DepForceMedium: 80,
  },
 }
}

// DetermineSeverity applies threshold and per-axis override rules to determine severity.
func DetermineSeverity(result DivergenceResult, config SeverityConfig) DivergenceResult {
 // Start with threshold-based severity
 severity := SeverityLow
 if result.Value >= config.Thresholds.MediumMax {
  severity = SeverityHigh
 } else if result.Value >= config.Thresholds.LowMax {
  severity = SeverityMedium
 }

 // Apply per-axis overrides
 overridden := false
 if result.Axes[AxisADR].Score >= config.PerAxisOverride.ADRForceHigh {
  if severity != SeverityHigh {
   overridden = true
  }
  severity = SeverityHigh
 }
 if result.Axes[AxisDoD].Score >= config.PerAxisOverride.DoDForceHigh {
  if severity != SeverityHigh {
   overridden = true
  }
  severity = SeverityHigh
 }
 if result.Axes[AxisDependency].Score >= config.PerAxisOverride.DepForceMedium {
  if severity == SeverityLow {
   overridden = true
   severity = SeverityMedium
  }
 }

 result.Severity = severity
 result.Overridden = overridden
 return result
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add scoring.go scoring_test.go
git commit -m "feat: add DetermineSeverity with per-axis override"
```

---

### Task 3: Scoring — FormatDivergence

**Files:**

- Modify: `scoring.go`
- Modify: `scoring_test.go`

**Step 1: Write failing tests**

Add to `scoring_test.go`:

```go
func TestFormatDivergence(t *testing.T) {
 tests := []struct {
  name     string
  internal float64
  expected string
 }{
  {"zero", 0.0, "0.000000"},
  {"max", 100.0, "1.000000"},
  {"example from doc", 14.5, "0.145000"},
  {"small value", 0.5, "0.005000"},
  {"boundary low", 25.0, "0.250000"},
  {"boundary high", 50.0, "0.500000"},
 }
 for _, tt := range tests {
  t.Run(tt.name, func(t *testing.T) {
   got := FormatDivergence(tt.internal)
   if got != tt.expected {
    t.Errorf("FormatDivergence(%f) = %q, want %q", tt.internal, got, tt.expected)
   }
  })
 }
}

func TestFormatDelta(t *testing.T) {
 tests := []struct {
  name     string
  current  float64
  previous float64
  expected string
 }{
  {"positive delta", 0.145, 0.133, "+0.012000"},
  {"negative delta", 0.10, 0.15, "-0.050000"},
  {"zero delta", 0.25, 0.25, "+0.000000"},
  {"first check (no previous)", 0.145, 0.0, "+0.145000"},
 }
 for _, tt := range tests {
  t.Run(tt.name, func(t *testing.T) {
   got := FormatDelta(tt.current, tt.previous)
   if got != tt.expected {
    t.Errorf("FormatDelta(%f, %f) = %q, want %q", tt.current, tt.previous, got, tt.expected)
   }
  })
 }
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL

**Step 3: Write minimal implementation**

Add to `scoring.go`:

```go
import "fmt"

// FormatDivergence converts an internal score (0-100) to 0.000000 display format.
func FormatDivergence(internal float64) string {
 return fmt.Sprintf("%f", internal/100.0)
}

// FormatDelta formats the difference between current and previous divergence values.
func FormatDelta(current, previous float64) string {
 delta := current - previous
 if delta >= 0 {
  return fmt.Sprintf("+%f", delta)
 }
 return fmt.Sprintf("%f", delta)
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Run lint and format**

Run: `just check`
Expected: PASS

**Step 6: Commit**

```bash
git add scoring.go scoring_test.go
git commit -m "feat: add FormatDivergence and FormatDelta display functions"
```

---

### Task 4: Config — Types and YAML Parsing

**Files:**

- Create: `config.go`
- Create: `config_test.go`

**Step 1: Write failing tests**

```go
// config_test.go
package amadeus

import (
 "os"
 "path/filepath"
 "testing"
)

func TestDefaultConfig(t *testing.T) {
 cfg := DefaultConfig()
 if cfg.Weights.ADRIntegrity != 0.4 {
  t.Errorf("expected ADR weight 0.4, got %f", cfg.Weights.ADRIntegrity)
 }
 if cfg.Weights.DoDFulfillment != 0.3 {
  t.Errorf("expected DoD weight 0.3, got %f", cfg.Weights.DoDFulfillment)
 }
 if cfg.FullCheck.Interval != 10 {
  t.Errorf("expected interval 10, got %d", cfg.FullCheck.Interval)
 }
}

func TestLoadConfig_FromFile(t *testing.T) {
 dir := t.TempDir()
 configPath := filepath.Join(dir, "config.yaml")
 content := `weights:
  adr_integrity: 0.5
  dod_fulfillment: 0.25
  dependency_integrity: 0.15
  implicit_constraints: 0.1

thresholds:
  low_max: 0.200000
  medium_max: 0.400000

full_check:
  interval: 5
  on_divergence_jump: 0.20
`
 if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
  t.Fatal(err)
 }

 cfg, err := LoadConfig(configPath)
 if err != nil {
  t.Fatalf("LoadConfig failed: %v", err)
 }
 if cfg.Weights.ADRIntegrity != 0.5 {
  t.Errorf("expected ADR weight 0.5, got %f", cfg.Weights.ADRIntegrity)
 }
 if cfg.Thresholds.LowMax != 0.2 {
  t.Errorf("expected low_max 0.2, got %f", cfg.Thresholds.LowMax)
 }
 if cfg.FullCheck.Interval != 5 {
  t.Errorf("expected interval 5, got %d", cfg.FullCheck.Interval)
 }
}

func TestLoadConfig_FileNotFound_ReturnsDefault(t *testing.T) {
 cfg, err := LoadConfig("/nonexistent/path/config.yaml")
 if err != nil {
  t.Fatalf("expected no error for missing file, got: %v", err)
 }
 if cfg.Weights.ADRIntegrity != 0.4 {
  t.Errorf("expected default ADR weight 0.4, got %f", cfg.Weights.ADRIntegrity)
 }
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// config.go
package amadeus

import (
 "errors"
 "io/fs"
 "os"

 "gopkg.in/yaml.v3"
)

// Config holds all Amadeus configuration.
type Config struct {
 Weights         Weights         `yaml:"weights"`
 Thresholds      Thresholds      `yaml:"thresholds"`
 PerAxisOverride PerAxisOverride  `yaml:"per_axis_override"`
 FullCheck       FullCheckConfig `yaml:"full_check"`
 CheckCountSinceFull int         `yaml:"check_count_since_full"`
}

// FullCheckConfig holds configuration for full calibration checks.
type FullCheckConfig struct {
 Interval         int     `yaml:"interval"`
 OnDivergenceJump float64 `yaml:"on_divergence_jump"`
}

// DefaultConfig returns the standard configuration from the architecture document.
func DefaultConfig() Config {
 sc := DefaultThresholds()
 return Config{
  Weights:         DefaultWeights(),
  Thresholds:      sc.Thresholds,
  PerAxisOverride: sc.PerAxisOverride,
  FullCheck: FullCheckConfig{
   Interval:         10,
   OnDivergenceJump: 0.15,
  },
  CheckCountSinceFull: 0,
 }
}

// LoadConfig reads configuration from a YAML file.
// Returns DefaultConfig if the file does not exist.
func LoadConfig(path string) (Config, error) {
 data, err := os.ReadFile(path)
 if err != nil {
  if errors.Is(err, fs.ErrNotExist) {
   return DefaultConfig(), nil
  }
  return Config{}, err
 }

 cfg := DefaultConfig()
 if err := yaml.Unmarshal(data, &cfg); err != nil {
  return Config{}, err
 }
 return cfg, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add config.go config_test.go
git commit -m "feat: add Config with YAML parsing and defaults"
```

---

### Task 5: State — `.divergence/` Directory Management

**Files:**

- Create: `state.go`
- Create: `state_test.go`

**Step 1: Write failing tests**

```go
// state_test.go
package amadeus

import (
 "os"
 "path/filepath"
 "testing"
 "time"
)

func TestInitDivergenceDir_CreatesStructure(t *testing.T) {
 dir := t.TempDir()
 root := filepath.Join(dir, ".divergence")

 err := InitDivergenceDir(root)
 if err != nil {
  t.Fatalf("InitDivergenceDir failed: %v", err)
 }

 for _, sub := range []string{"state", "history", "dmails"} {
  path := filepath.Join(root, sub)
  info, err := os.Stat(path)
  if err != nil {
   t.Errorf("expected %s to exist: %v", sub, err)
  }
  if !info.IsDir() {
   t.Errorf("expected %s to be a directory", sub)
  }
 }

 // config.yaml should be created with defaults
 configPath := filepath.Join(root, "config.yaml")
 if _, err := os.Stat(configPath); err != nil {
  t.Errorf("expected config.yaml to exist: %v", err)
 }
}

func TestSaveAndLoadCheckResult(t *testing.T) {
 dir := t.TempDir()
 root := filepath.Join(dir, ".divergence")
 if err := InitDivergenceDir(root); err != nil {
  t.Fatal(err)
 }

 result := CheckResult{
  CheckedAt:  time.Date(2026, 2, 19, 14, 30, 0, 0, time.UTC),
  Commit:     "a1b2c3d",
  Type:       CheckTypeDiff,
  Divergence: 0.145,
  Axes: map[Axis]AxisScore{
   AxisADR:        {Score: 15, Details: "minor"},
   AxisDoD:        {Score: 20, Details: "edge case"},
   AxisDependency: {Score: 10, Details: "clean"},
   AxisImplicit:   {Score: 5, Details: "naming"},
  },
  PRsEvaluated: []string{"#120", "#122"},
 }

 store := NewStateStore(root)

 if err := store.SaveLatest(result); err != nil {
  t.Fatalf("SaveLatest failed: %v", err)
 }
 if err := store.SaveHistory(result); err != nil {
  t.Fatalf("SaveHistory failed: %v", err)
 }

 loaded, err := store.LoadLatest()
 if err != nil {
  t.Fatalf("LoadLatest failed: %v", err)
 }
 if loaded.Commit != "a1b2c3d" {
  t.Errorf("expected commit a1b2c3d, got %s", loaded.Commit)
 }
 if loaded.Divergence != 0.145 {
  t.Errorf("expected divergence 0.145, got %f", loaded.Divergence)
 }
}

func TestLoadLatest_NoFile_ReturnsEmpty(t *testing.T) {
 dir := t.TempDir()
 root := filepath.Join(dir, ".divergence")
 if err := InitDivergenceDir(root); err != nil {
  t.Fatal(err)
 }

 store := NewStateStore(root)
 result, err := store.LoadLatest()
 if err != nil {
  t.Fatalf("expected no error, got: %v", err)
 }
 if result.Commit != "" {
  t.Errorf("expected empty commit, got %s", result.Commit)
 }
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// state.go
package amadeus

import (
 "encoding/json"
 "errors"
 "io/fs"
 "os"
 "path/filepath"
 "time"

 "gopkg.in/yaml.v3"
)

// CheckType indicates whether a check was diff or full.
type CheckType string

const (
 CheckTypeDiff CheckType = "diff"
 CheckTypeFull CheckType = "full"
)

// CheckResult holds the output of a single check.
type CheckResult struct {
 CheckedAt    time.Time          `json:"checked_at"`
 Commit       string             `json:"commit"`
 Type         CheckType          `json:"type"`
 Divergence   float64            `json:"divergence"`
 Axes         map[Axis]AxisScore `json:"axes"`
 PRsEvaluated []string           `json:"prs_evaluated"`
 DMails       []string           `json:"dmails"`
}

// StateStore manages the .divergence/ directory.
type StateStore struct {
 Root string
}

// NewStateStore creates a StateStore for the given .divergence/ root.
func NewStateStore(root string) *StateStore {
 return &StateStore{Root: root}
}

// InitDivergenceDir creates the .divergence/ directory structure and default config.
func InitDivergenceDir(root string) error {
 dirs := []string{
  filepath.Join(root, "state"),
  filepath.Join(root, "history"),
  filepath.Join(root, "dmails"),
 }
 for _, d := range dirs {
  if err := os.MkdirAll(d, 0o755); err != nil {
   return err
  }
 }

 configPath := filepath.Join(root, "config.yaml")
 if _, err := os.Stat(configPath); errors.Is(err, fs.ErrNotExist) {
  cfg := DefaultConfig()
  data, err := yaml.Marshal(cfg)
  if err != nil {
   return err
  }
  if err := os.WriteFile(configPath, data, 0o644); err != nil {
   return err
  }
 }
 return nil
}

// SaveLatest writes a CheckResult to state/latest.json.
func (s *StateStore) SaveLatest(result CheckResult) error {
 return s.writeJSON(filepath.Join(s.Root, "state", "latest.json"), result)
}

// SaveBaseline writes a CheckResult to state/baseline.json.
func (s *StateStore) SaveBaseline(result CheckResult) error {
 return s.writeJSON(filepath.Join(s.Root, "state", "baseline.json"), result)
}

// SaveHistory appends a CheckResult to history/{timestamp}.json.
func (s *StateStore) SaveHistory(result CheckResult) error {
 filename := result.CheckedAt.Format("2006-01-02T1504") + ".json"
 return s.writeJSON(filepath.Join(s.Root, "history", filename), result)
}

// LoadLatest reads the latest check result. Returns zero value if not found.
func (s *StateStore) LoadLatest() (CheckResult, error) {
 var result CheckResult
 data, err := os.ReadFile(filepath.Join(s.Root, "state", "latest.json"))
 if err != nil {
  if errors.Is(err, fs.ErrNotExist) {
   return result, nil
  }
  return result, err
 }
 if err := json.Unmarshal(data, &result); err != nil {
  return result, err
 }
 return result, nil
}

func (s *StateStore) writeJSON(path string, v any) error {
 data, err := json.MarshalIndent(v, "", "  ")
 if err != nil {
  return err
 }
 return os.WriteFile(path, data, 0o644)
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add state.go state_test.go
git commit -m "feat: add StateStore for .divergence/ directory management"
```

---

### Task 6: D-Mail — Model and ID Generation

**Files:**

- Create: `dmail.go`
- Create: `dmail_test.go`

**Step 1: Write failing tests**

```go
// dmail_test.go
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
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// dmail.go
package amadeus

import (
 "fmt"
 "os"
 "path/filepath"
 "strings"
 "time"
)

// DMailStatus represents the lifecycle state of a D-Mail.
type DMailStatus string

const (
 DMailPending  DMailStatus = "pending"
 DMailSent     DMailStatus = "sent"
 DMailApproved DMailStatus = "approved"
 DMailRejected DMailStatus = "rejected"
)

// DMailTarget indicates which tool should receive the D-Mail.
type DMailTarget string

const (
 TargetSightjack DMailTarget = "sightjack"
 TargetPaintress DMailTarget = "paintress"
)

// DMail represents a correction routing message.
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
}

// RouteDMail applies severity-based routing to set the D-Mail status.
func RouteDMail(dmail DMail) DMail {
 switch dmail.Severity {
 case SeverityHigh:
  dmail.Status = DMailPending
 default:
  dmail.Status = DMailSent
 }
 return dmail
}

// NextDMailID returns the next sequential D-Mail ID (d-001, d-002, ...).
func (s *StateStore) NextDMailID() (string, error) {
 dmailDir := filepath.Join(s.Root, "dmails")
 entries, err := os.ReadDir(dmailDir)
 if err != nil {
  return "", err
 }

 maxNum := 0
 for _, e := range entries {
  name := strings.TrimSuffix(e.Name(), ".json")
  if strings.HasPrefix(name, "d-") {
   var num int
   if _, err := fmt.Sscanf(name, "d-%d", &num); err == nil && num > maxNum {
    maxNum = num
   }
  }
 }
 return fmt.Sprintf("d-%03d", maxNum+1), nil
}

// SaveDMail writes a D-Mail to the dmails/ directory.
func (s *StateStore) SaveDMail(dmail DMail) error {
 path := filepath.Join(s.Root, "dmails", dmail.ID+".json")
 return s.writeJSON(path, dmail)
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add dmail.go dmail_test.go
git commit -m "feat: add DMail model with severity routing and ID generation"
```

---

### Task 7: Git — Operations

**Files:**

- Create: `git.go`
- Create: `git_test.go`

**Step 1: Write failing tests**

```go
// git_test.go
package amadeus

import (
 "os/exec"
 "path/filepath"
 "testing"
)

// setupTestRepo creates a temporary git repo with some commits for testing.
func setupTestRepo(t *testing.T) string {
 t.Helper()
 dir := t.TempDir()

 commands := [][]string{
  {"git", "init"},
  {"git", "config", "user.email", "test@test.com"},
  {"git", "config", "user.name", "Test"},
  {"git", "commit", "--allow-empty", "-m", "initial commit"},
  {"git", "commit", "--allow-empty", "-m", "Merge pull request #10 from feature/auth"},
  {"git", "commit", "--allow-empty", "-m", "Merge pull request #11 from feature/cart"},
 }
 for _, args := range commands {
  cmd := exec.Command(args[0], args[1:]...)
  cmd.Dir = dir
  if out, err := cmd.CombinedOutput(); err != nil {
   t.Fatalf("command %v failed: %v\n%s", args, err, out)
  }
 }
 return dir
}

func TestGetCurrentCommit(t *testing.T) {
 dir := setupTestRepo(t)
 git := NewGitClient(dir)

 hash, err := git.CurrentCommit()
 if err != nil {
  t.Fatal(err)
 }
 if len(hash) < 7 {
  t.Errorf("expected commit hash, got %q", hash)
 }
}

func TestGetMergedPRsSince(t *testing.T) {
 dir := setupTestRepo(t)
 git := NewGitClient(dir)

 // Get the initial commit hash
 cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
 cmd.Dir = dir
 out, err := cmd.Output()
 if err != nil {
  t.Fatal(err)
 }
 initialCommit := string(out[:len(out)-1]) // trim newline

 prs, err := git.MergedPRsSince(initialCommit)
 if err != nil {
  t.Fatal(err)
 }
 if len(prs) != 2 {
  t.Errorf("expected 2 PRs, got %d: %v", len(prs), prs)
 }
}

func TestGetDiffSince(t *testing.T) {
 dir := setupTestRepo(t)
 // Create an actual file change
 filePath := filepath.Join(dir, "hello.go")
 cmd := exec.Command("bash", "-c", "echo 'package main' > "+filePath+" && git add . && git commit -m 'add file'")
 cmd.Dir = dir
 if _, err := cmd.CombinedOutput(); err != nil {
  t.Fatal(err)
 }

 git := NewGitClient(dir)
 hash, _ := git.CurrentCommit()

 // Get diff from parent
 diff, err := git.DiffSince(hash + "~1")
 if err != nil {
  t.Fatal(err)
 }
 if diff == "" {
  t.Error("expected non-empty diff")
 }
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// git.go
package amadeus

import (
 "bytes"
 "fmt"
 "os/exec"
 "regexp"
 "strings"
)

// MergedPR represents a merged pull request extracted from git log.
type MergedPR struct {
 Number string
 Title  string
}

// GitClient handles git operations on a repository.
type GitClient struct {
 Dir string
}

// NewGitClient creates a GitClient for the given repository directory.
func NewGitClient(dir string) *GitClient {
 return &GitClient{Dir: dir}
}

// CurrentCommit returns the short hash of HEAD.
func (g *GitClient) CurrentCommit() (string, error) {
 out, err := g.run("rev-parse", "--short", "HEAD")
 if err != nil {
  return "", err
 }
 return strings.TrimSpace(out), nil
}

var prMergePattern = regexp.MustCompile(`Merge pull request #(\d+)`)

// MergedPRsSince returns PRs merged since the given commit.
func (g *GitClient) MergedPRsSince(since string) ([]MergedPR, error) {
 out, err := g.run("log", fmt.Sprintf("%s..HEAD", since), "--oneline", "--grep=Merge pull request")
 if err != nil {
  return nil, err
 }
 if strings.TrimSpace(out) == "" {
  return nil, nil
 }

 var prs []MergedPR
 for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
  matches := prMergePattern.FindStringSubmatch(line)
  if len(matches) >= 2 {
   prs = append(prs, MergedPR{
    Number: "#" + matches[1],
    Title:  line,
   })
  }
 }
 return prs, nil
}

// DiffSince returns the diff of changes since the given commit.
func (g *GitClient) DiffSince(since string) (string, error) {
 return g.run("diff", since+"..HEAD")
}

func (g *GitClient) run(args ...string) (string, error) {
 cmd := exec.Command("git", args...)
 cmd.Dir = g.Dir
 var stdout, stderr bytes.Buffer
 cmd.Stdout = &stdout
 cmd.Stderr = &stderr
 if err := cmd.Run(); err != nil {
  return "", fmt.Errorf("git %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
 }
 return stdout.String(), nil
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add git.go git_test.go
git commit -m "feat: add GitClient for commit hash, PR extraction, and diff"
```

---

### Task 8: Claude — Prompt Building and Response Parsing

**Files:**

- Create: `claude.go`
- Create: `claude_test.go`
- Create: `templates/diff_check.md.tmpl`
- Create: `templates/full_check.md.tmpl`

**Step 1: Write failing tests**

```go
// claude_test.go
package amadeus

import (
 "testing"
)

func TestParseClaudeResponse_Valid(t *testing.T) {
 raw := `{
  "axes": {
   "adr_integrity": {"score": 15, "details": "ADR-003 minor tension"},
   "dod_fulfillment": {"score": 20, "details": "Issue #42 edge case"},
   "dependency_integrity": {"score": 10, "details": "clean"},
   "implicit_constraints": {"score": 5, "details": "naming drift"}
  },
  "dmails": [
   {
    "target": "sightjack",
    "type": "Type-S",
    "summary": "ADR-003 needs update",
    "detail": "Auth module violates ADR-003"
   }
  ],
  "reasoning": "Minor tensions detected"
 }`

 resp, err := ParseClaudeResponse([]byte(raw))
 if err != nil {
  t.Fatalf("ParseClaudeResponse failed: %v", err)
 }
 if resp.Axes[AxisADR].Score != 15 {
  t.Errorf("expected ADR score 15, got %d", resp.Axes[AxisADR].Score)
 }
 if len(resp.DMails) != 1 {
  t.Fatalf("expected 1 D-Mail, got %d", len(resp.DMails))
 }
 if resp.DMails[0].Target != TargetSightjack {
  t.Errorf("expected target sightjack, got %s", resp.DMails[0].Target)
 }
}

func TestParseClaudeResponse_InvalidJSON(t *testing.T) {
 _, err := ParseClaudeResponse([]byte("not json"))
 if err == nil {
  t.Error("expected error for invalid JSON")
 }
}

func TestBuildDiffCheckPrompt(t *testing.T) {
 params := DiffCheckParams{
  PreviousScores: `{"divergence": 0.133}`,
  PRDiffs:        "diff --git a/auth.go ...",
  RelevantADRs:   "ADR-003: Use JWT for auth",
  LinkedDoDs:     "Issue #42: Session timeout must be configurable",
 }
 prompt, err := BuildDiffCheckPrompt(params)
 if err != nil {
  t.Fatalf("BuildDiffCheckPrompt failed: %v", err)
 }
 if prompt == "" {
  t.Error("expected non-empty prompt")
 }
 if !containsAll(prompt, "Previous State", "Changes Since Last Check", "ADRs") {
  t.Error("prompt missing expected sections")
 }
}

func containsAll(s string, substrs ...string) bool {
 for _, sub := range substrs {
  if !contains(s, sub) {
   return false
  }
 }
 return true
}

func contains(s, substr string) bool {
 return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
 for i := 0; i <= len(s)-len(substr); i++ {
  if s[i:i+len(substr)] == substr {
   return true
  }
 }
 return false
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL

**Step 3: Create prompt templates**

Create `templates/diff_check.md.tmpl`:

```markdown
## Context
You are Amadeus, a post-merge integrity verification system.
Evaluate how recent changes affect the codebase's alignment with its architectural intent.

## Previous State
{{.PreviousScores}}

## Changes Since Last Check
{{.PRDiffs}}

## ADRs (relevant to changed modules)
{{.RelevantADRs}}

## DoDs (for linked Issues)
{{.LinkedDoDs}}

## Task
Evaluate how these changes affect each integrity axis.
Score each axis 0-100 (0 = full compliance, 100 = full deviation).

Output MUST be valid JSON matching this exact schema:
{"axes":{"adr_integrity":{"score":0,"details":""},"dod_fulfillment":{"score":0,"details":""},"dependency_integrity":{"score":0,"details":""},"implicit_constraints":{"score":0,"details":""}},"dmails":[{"target":"sightjack|paintress","type":"Type-S|Type-P|Type-SP","summary":"","detail":""}],"reasoning":""}

For D-Mails:
- target "sightjack" for design/architecture issues (Type-S)
- target "paintress" for implementation issues (Type-P)
- target either for interpretation gaps (Type-SP)
- Only generate D-Mails for actual issues found. Empty array if none.
```

Create `templates/full_check.md.tmpl`:

```markdown
## Context
You are Amadeus, a post-merge integrity verification system.
This is a FULL calibration check. Evaluate the entire codebase from zero.

## Codebase Structure
{{.CodebaseStructure}}

## All Active ADRs
{{.AllADRs}}

## Recent Issues with DoDs
{{.RecentDoDs}}

## Dependency Map
{{.DependencyMap}}

## Task
Evaluate the entire codebase against all ADRs and DoDs.
Score each axis 0-100 from zero (0 = full compliance, 100 = full deviation).

Output MUST be valid JSON matching this exact schema:
{"axes":{"adr_integrity":{"score":0,"details":""},"dod_fulfillment":{"score":0,"details":""},"dependency_integrity":{"score":0,"details":""},"implicit_constraints":{"score":0,"details":""}},"dmails":[{"target":"sightjack|paintress","type":"Type-S|Type-P|Type-SP","summary":"","detail":""}],"reasoning":""}
```

**Step 4: Write implementation**

```go
// claude.go
package amadeus

import (
 "bytes"
 "context"
 "embed"
 "encoding/json"
 "fmt"
 "os/exec"
 "strings"
 "text/template"
)

//go:embed templates/*.md.tmpl
var templateFS embed.FS

// ClaudeClient wraps Claude CLI invocation.
type ClaudeClient struct {
 Command string
 Model   string
 Timeout int
 DryRun  bool
}

// NewClaudeClient creates a ClaudeClient with defaults.
func NewClaudeClient() *ClaudeClient {
 return &ClaudeClient{
  Command: "claude",
  Model:   "opus",
  Timeout: 300,
 }
}

// ClaudeResponse represents the structured response from Claude.
type ClaudeResponse struct {
 Axes      map[Axis]AxisScore  `json:"axes"`
 DMails    []ClaudeDMailCandidate `json:"dmails"`
 Reasoning string              `json:"reasoning"`
}

// ClaudeDMailCandidate is a D-Mail candidate from Claude's analysis.
type ClaudeDMailCandidate struct {
 Target  DMailTarget `json:"target"`
 Type    string      `json:"type"`
 Summary string      `json:"summary"`
 Detail  string      `json:"detail"`
}

// DiffCheckParams holds parameters for the diff check prompt template.
type DiffCheckParams struct {
 PreviousScores string
 PRDiffs        string
 RelevantADRs   string
 LinkedDoDs     string
}

// FullCheckParams holds parameters for the full check prompt template.
type FullCheckParams struct {
 CodebaseStructure string
 AllADRs           string
 RecentDoDs        string
 DependencyMap     string
}

// BuildDiffCheckPrompt renders the diff check prompt template.
func BuildDiffCheckPrompt(params DiffCheckParams) (string, error) {
 return renderTemplate("templates/diff_check.md.tmpl", params)
}

// BuildFullCheckPrompt renders the full check prompt template.
func BuildFullCheckPrompt(params FullCheckParams) (string, error) {
 return renderTemplate("templates/full_check.md.tmpl", params)
}

// ParseClaudeResponse parses the JSON response from Claude.
func ParseClaudeResponse(data []byte) (ClaudeResponse, error) {
 var resp ClaudeResponse
 if err := json.Unmarshal(data, &resp); err != nil {
  return resp, fmt.Errorf("failed to parse Claude response: %w", err)
 }
 return resp, nil
}

// Run invokes Claude CLI with the given prompt and returns raw output.
func (c *ClaudeClient) Run(ctx context.Context, prompt string) ([]byte, error) {
 if c.DryRun {
  return nil, nil
 }

 args := []string{"-p", "--output-format", "json", "--model", c.Model}
 cmd := exec.CommandContext(ctx, c.Command, args...)
 cmd.Stdin = strings.NewReader(prompt)

 var stdout, stderr bytes.Buffer
 cmd.Stdout = &stdout
 cmd.Stderr = &stderr

 if err := cmd.Run(); err != nil {
  return nil, fmt.Errorf("claude: %w\n%s", err, stderr.String())
 }
 return stdout.Bytes(), nil
}

func renderTemplate(name string, data any) (string, error) {
 tmpl, err := template.ParseFS(templateFS, name)
 if err != nil {
  return "", fmt.Errorf("parse template %s: %w", name, err)
 }
 var buf bytes.Buffer
 if err := tmpl.Execute(&buf, data); err != nil {
  return "", fmt.Errorf("execute template %s: %w", name, err)
 }
 return buf.String(), nil
}
```

**Step 5: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 6: Commit**

```bash
git add claude.go claude_test.go templates/
git commit -m "feat: add ClaudeClient with prompt templates and response parsing"
```

---

### Task 9: Logger — Color-Coded CLI Output

**Files:**

- Create: `logger.go`
- Create: `logger_test.go`

**Step 1: Write failing tests**

```go
// logger_test.go
package amadeus

import (
 "bytes"
 "strings"
 "testing"
)

func TestLogger_Info(t *testing.T) {
 var buf bytes.Buffer
 log := NewLogger(&buf, false)
 log.Info("hello %s", "world")
 if !strings.Contains(buf.String(), "hello world") {
  t.Errorf("expected 'hello world' in output, got %q", buf.String())
 }
}

func TestLogger_Verbose_Suppressed(t *testing.T) {
 var buf bytes.Buffer
 log := NewLogger(&buf, false)
 log.Debug("hidden")
 if buf.Len() != 0 {
  t.Errorf("expected no output in non-verbose mode, got %q", buf.String())
 }
}

func TestLogger_Verbose_Shown(t *testing.T) {
 var buf bytes.Buffer
 log := NewLogger(&buf, true)
 log.Debug("shown")
 if !strings.Contains(buf.String(), "shown") {
  t.Errorf("expected 'shown' in output, got %q", buf.String())
 }
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// logger.go
package amadeus

import (
 "fmt"
 "io"
)

// Logger provides leveled, color-coded logging.
type Logger struct {
 out     io.Writer
 verbose bool
}

// NewLogger creates a Logger writing to the given writer.
func NewLogger(out io.Writer, verbose bool) *Logger {
 return &Logger{out: out, verbose: verbose}
}

func (l *Logger) Info(format string, args ...any) {
 fmt.Fprintf(l.out, "  "+format+"\n", args...)
}

func (l *Logger) Warn(format string, args ...any) {
 fmt.Fprintf(l.out, "  ⚠  "+format+"\n", args...)
}

func (l *Logger) Error(format string, args ...any) {
 fmt.Fprintf(l.out, "  ✗  "+format+"\n", args...)
}

func (l *Logger) Debug(format string, args ...any) {
 if l.verbose {
  fmt.Fprintf(l.out, "  [debug] "+format+"\n", args...)
 }
}

func (l *Logger) OK(format string, args ...any) {
 fmt.Fprintf(l.out, "  ✓  "+format+"\n", args...)
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add logger.go logger_test.go
git commit -m "feat: add Logger with leveled output"
```

---

### Task 10: Reading Steiner — Phase 1 Shift Detection

**Files:**

- Create: `reading_steiner.go`
- Create: `reading_steiner_test.go`

**Step 1: Write failing tests**

```go
// reading_steiner_test.go
package amadeus

import (
 "os/exec"
 "path/filepath"
 "testing"
)

func TestDetectShift_NoChanges(t *testing.T) {
 dir := setupTestRepo(t)
 git := NewGitClient(dir)
 hash, _ := git.CurrentCommit()

 rs := &ReadingSteiner{Git: git}
 report, err := rs.DetectShift(hash)
 if err != nil {
  t.Fatal(err)
 }
 if report.Significant {
  t.Error("expected no significant shift when no changes")
 }
}

func TestDetectShift_WithMergedPRs(t *testing.T) {
 dir := setupTestRepo(t)
 git := NewGitClient(dir)

 // Get initial commit as "last checked"
 cmd := exec.Command("git", "rev-list", "--max-parents=0", "HEAD")
 cmd.Dir = dir
 out, err := cmd.Output()
 if err != nil {
  t.Fatal(err)
 }
 initialCommit := string(out[:len(out)-1])

 rs := &ReadingSteiner{Git: git}
 report, err := rs.DetectShift(initialCommit)
 if err != nil {
  t.Fatal(err)
 }
 if !report.Significant {
  t.Error("expected significant shift with merged PRs")
 }
 if len(report.MergedPRs) != 2 {
  t.Errorf("expected 2 merged PRs, got %d", len(report.MergedPRs))
 }
}

func TestDetectShift_FullMode(t *testing.T) {
 dir := setupTestRepo(t)
 // Create a directory structure
 cmd := exec.Command("bash", "-c", "mkdir -p src && echo 'package main' > src/main.go && git add . && git commit -m 'add src'")
 cmd.Dir = dir
 if _, err := cmd.CombinedOutput(); err != nil {
  t.Fatal(err)
 }

 git := NewGitClient(dir)
 rs := &ReadingSteiner{Git: git}
 report, err := rs.DetectShiftFull(dir)
 if err != nil {
  t.Fatal(err)
 }
 if report.CodebaseStructure == "" {
  t.Error("expected non-empty codebase structure")
 }
}

// setupTestRepo is defined in git_test.go — reuse helper from Task 7.
// If tests run in same package, this is already available.
// Move to a shared test helper file (helpers_test.go) if needed.
var _ = filepath.Join // suppress unused import
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// reading_steiner.go
package amadeus

import (
 "fmt"
 "os"
 "path/filepath"
 "strings"
)

// ShiftReport holds the result of Phase 1 shift detection.
type ShiftReport struct {
 Significant        bool
 MergedPRs          []MergedPR
 Diff               string
 CodebaseStructure  string
}

// ReadingSteiner handles Phase 1: World Line Shift Detection.
type ReadingSteiner struct {
 Git *GitClient
}

// DetectShift performs a diff-mode shift detection since the given commit.
func (rs *ReadingSteiner) DetectShift(sinceCommit string) (ShiftReport, error) {
 prs, err := rs.Git.MergedPRsSince(sinceCommit)
 if err != nil {
  return ShiftReport{}, fmt.Errorf("reading steiner: merged PRs: %w", err)
 }

 diff, err := rs.Git.DiffSince(sinceCommit)
 if err != nil {
  return ShiftReport{}, fmt.Errorf("reading steiner: diff: %w", err)
 }

 significant := len(prs) > 0 || strings.TrimSpace(diff) != ""

 return ShiftReport{
  Significant: significant,
  MergedPRs:   prs,
  Diff:        diff,
 }, nil
}

// DetectShiftFull performs a full-mode shift detection (entire codebase summary).
func (rs *ReadingSteiner) DetectShiftFull(repoRoot string) (ShiftReport, error) {
 structure, err := buildDirectoryTree(repoRoot, 3)
 if err != nil {
  return ShiftReport{}, fmt.Errorf("reading steiner: directory tree: %w", err)
 }

 return ShiftReport{
  Significant:       true,
  CodebaseStructure: structure,
 }, nil
}

// buildDirectoryTree creates a simple text representation of the directory structure.
func buildDirectoryTree(root string, maxDepth int) (string, error) {
 var sb strings.Builder
 err := walkDir(root, "", maxDepth, 0, &sb)
 return sb.String(), err
}

func walkDir(path, prefix string, maxDepth, depth int, sb *strings.Builder) error {
 if depth >= maxDepth {
  return nil
 }

 entries, err := os.ReadDir(path)
 if err != nil {
  return err
 }

 for _, e := range entries {
  name := e.Name()
  if strings.HasPrefix(name, ".") {
   continue
  }
  fmt.Fprintf(sb, "%s%s\n", prefix, name)
  if e.IsDir() {
   if err := walkDir(filepath.Join(path, name), prefix+"  ", maxDepth, depth+1, sb); err != nil {
    return err
   }
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
git add reading_steiner.go reading_steiner_test.go
git commit -m "feat: add ReadingSteiner for Phase 1 shift detection"
```

---

### Task 11: Divergence Meter — Phase 2 Scoring Orchestration

**Files:**

- Create: `divergence_meter.go`
- Create: `divergence_meter_test.go`

**Step 1: Write failing tests**

```go
// divergence_meter_test.go
package amadeus

import (
 "testing"
)

func TestDivergenceMeter_ProcessResponse(t *testing.T) {
 meter := &DivergenceMeter{
  Config: DefaultConfig(),
 }

 resp := ClaudeResponse{
  Axes: map[Axis]AxisScore{
   AxisADR:        {Score: 15, Details: "minor"},
   AxisDoD:        {Score: 20, Details: "edge case"},
   AxisDependency: {Score: 10, Details: "clean"},
   AxisImplicit:   {Score: 5, Details: "naming"},
  },
  DMails: []ClaudeDMailCandidate{
   {Target: TargetSightjack, Type: "Type-S", Summary: "ADR-003", Detail: "violation"},
  },
  Reasoning: "Minor tensions",
 }

 result := meter.ProcessResponse(resp)

 if !almostEqual(result.Divergence.Internal, 14.5) {
  t.Errorf("expected internal 14.5, got %f", result.Divergence.Internal)
 }
 if result.Divergence.Severity != SeverityLow {
  t.Errorf("expected LOW severity, got %s", result.Divergence.Severity)
 }
 if len(result.DMailCandidates) != 1 {
  t.Errorf("expected 1 D-Mail candidate, got %d", len(result.DMailCandidates))
 }
}
```

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// divergence_meter.go
package amadeus

// MeterResult holds the complete output of Phase 2.
type MeterResult struct {
 Divergence      DivergenceResult
 DMailCandidates []ClaudeDMailCandidate
 Reasoning       string
}

// DivergenceMeter handles Phase 2: Integrity Scoring.
type DivergenceMeter struct {
 Config Config
}

// ProcessResponse takes a Claude response and produces scored results.
func (dm *DivergenceMeter) ProcessResponse(resp ClaudeResponse) MeterResult {
 divergence := CalcDivergence(resp.Axes, dm.Config.Weights)
 severityCfg := SeverityConfig{
  Thresholds:      dm.Config.Thresholds,
  PerAxisOverride: dm.Config.PerAxisOverride,
 }
 divergence = DetermineSeverity(divergence, severityCfg)

 return MeterResult{
  Divergence:      divergence,
  DMailCandidates: resp.DMails,
  Reasoning:       resp.Reasoning,
 }
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add divergence_meter.go divergence_meter_test.go
git commit -m "feat: add DivergenceMeter for Phase 2 scoring orchestration"
```

---

### Task 12: Amadeus Orchestrator

**Files:**

- Create: `amadeus.go`
- Create: `amadeus_test.go`

This is the integration point. The orchestrator wires Phase 1→2→3 together.

**Step 1: Write failing tests**

```go
// amadeus_test.go
package amadeus

import (
 "bytes"
 "testing"
)

func TestAmadeus_DetermineCheckType_Diff(t *testing.T) {
 cfg := DefaultConfig()
 cfg.CheckCountSinceFull = 3

 a := &Amadeus{Config: cfg}
 if a.ShouldFullCheck(false) {
  t.Error("expected diff check when count < interval")
 }
}

func TestAmadeus_DetermineCheckType_FullByInterval(t *testing.T) {
 cfg := DefaultConfig()
 cfg.CheckCountSinceFull = 10 // equals interval

 a := &Amadeus{Config: cfg}
 if !a.ShouldFullCheck(false) {
  t.Error("expected full check when count >= interval")
 }
}

func TestAmadeus_DetermineCheckType_FullByFlag(t *testing.T) {
 a := &Amadeus{Config: DefaultConfig()}
 if !a.ShouldFullCheck(true) {
  t.Error("expected full check when --full flag is set")
 }
}

func TestAmadeus_FormatCLIOutput(t *testing.T) {
 var buf bytes.Buffer
 a := &Amadeus{
  Config: DefaultConfig(),
  Logger: NewLogger(&buf, false),
 }

 result := CheckResult{
  Divergence: 0.145,
  Axes: map[Axis]AxisScore{
   AxisADR:        {Score: 15, Details: "ADR-003 minor tension"},
   AxisDoD:        {Score: 20, Details: "Issue #42 edge case"},
   AxisDependency: {Score: 10, Details: "clean"},
   AxisImplicit:   {Score: 5, Details: "naming drift"},
  },
  PRsEvaluated: []string{"#120", "#122"},
 }
 dmails := []DMail{
  {ID: "d-001", Severity: SeverityLow, Status: DMailSent, Summary: "naming issue"},
 }

 a.PrintCheckOutput(result, dmails, 0.133)

 output := buf.String()
 if !strings.Contains(output, "Divergence") {
  t.Errorf("expected 'Divergence' in output, got:\n%s", output)
 }
 if !strings.Contains(output, "d-001") {
  t.Errorf("expected 'd-001' in output, got:\n%s", output)
 }
}
```

Note: add `"strings"` to imports.

**Step 2: Run tests to verify they fail**

Run: `just test`
Expected: FAIL

**Step 3: Write minimal implementation**

```go
// amadeus.go
package amadeus

import (
 "context"
 "encoding/json"
 "fmt"
 "time"
)

// Amadeus is the main orchestrator for the check workflow.
type Amadeus struct {
 Config Config
 Store  *StateStore
 Git    *GitClient
 Claude *ClaudeClient
 Logger *Logger
}

// CheckOptions holds CLI flag values for a check run.
type CheckOptions struct {
 Full    bool
 DryRun  bool
}

// ShouldFullCheck determines if a full check should be run.
func (a *Amadeus) ShouldFullCheck(forceFlag bool) bool {
 if forceFlag {
  return true
 }
 return a.Config.CheckCountSinceFull >= a.Config.FullCheck.Interval
}

// RunCheck executes the full check workflow: Phase 1 → 2 → 3.
func (a *Amadeus) RunCheck(ctx context.Context, opts CheckOptions) error {
 // Load previous state
 previous, err := a.Store.LoadLatest()
 if err != nil {
  return fmt.Errorf("load previous state: %w", err)
 }

 fullCheck := a.ShouldFullCheck(opts.Full)

 // Phase 1: Reading Steiner
 rs := &ReadingSteiner{Git: a.Git}
 var report ShiftReport

 if fullCheck {
  report, err = rs.DetectShiftFull(a.Git.Dir)
  if err != nil {
   return fmt.Errorf("phase 1 (full): %w", err)
  }
 } else {
  sinceCommit := previous.Commit
  if sinceCommit == "" {
   // First run: use full check instead
   fullCheck = true
   report, err = rs.DetectShiftFull(a.Git.Dir)
   if err != nil {
    return fmt.Errorf("phase 1 (first run): %w", err)
   }
  } else {
   report, err = rs.DetectShift(sinceCommit)
   if err != nil {
    return fmt.Errorf("phase 1 (diff): %w", err)
   }
  }
 }

 if !report.Significant {
  a.Logger.Info("Reading Steiner: no significant shift detected")
  return nil
 }

 a.Logger.Info("Reading Steiner: %d PRs merged since last check", len(report.MergedPRs))
 for _, pr := range report.MergedPRs {
  a.Logger.Info("  %s %s", pr.Number, pr.Title)
 }

 // Phase 2: Divergence Meter — build prompt and invoke Claude
 var prompt string
 if fullCheck {
  prompt, err = BuildFullCheckPrompt(FullCheckParams{
   CodebaseStructure: report.CodebaseStructure,
   AllADRs:           "", // TODO: gather from repo
   RecentDoDs:        "", // TODO: gather from Linear MCP
   DependencyMap:     "",
  })
 } else {
  prevJSON, _ := json.Marshal(previous)
  prompt, err = BuildDiffCheckPrompt(DiffCheckParams{
   PreviousScores: string(prevJSON),
   PRDiffs:        report.Diff,
   RelevantADRs:   "", // TODO: gather from repo
   LinkedDoDs:     "", // TODO: gather from Linear MCP
  })
 }
 if err != nil {
  return fmt.Errorf("phase 2 (build prompt): %w", err)
 }

 if opts.DryRun {
  fmt.Println(prompt)
  return nil
 }

 rawResp, err := a.Claude.Run(ctx, prompt)
 if err != nil {
  return fmt.Errorf("phase 2 (claude): %w", err)
 }

 claudeResp, err := ParseClaudeResponse(rawResp)
 if err != nil {
  return fmt.Errorf("phase 2 (parse): %w", err)
 }

 meter := &DivergenceMeter{Config: a.Config}
 meterResult := meter.ProcessResponse(claudeResp)

 // Phase 3: D-Mail generation
 currentCommit, _ := a.Git.CurrentCommit()
 now := time.Now().UTC()

 var dmails []DMail
 for _, candidate := range meterResult.DMailCandidates {
  id, err := a.Store.NextDMailID()
  if err != nil {
   return fmt.Errorf("phase 3 (dmail id): %w", err)
  }
  dmail := DMail{
   ID:        id,
   Severity:  meterResult.Divergence.Severity,
   Target:    candidate.Target,
   Type:      candidate.Type,
   Summary:   candidate.Summary,
   Detail:    candidate.Detail,
   CreatedAt: now,
  }
  dmail = RouteDMail(dmail)
  if err := a.Store.SaveDMail(dmail); err != nil {
   return fmt.Errorf("phase 3 (save dmail): %w", err)
  }
  dmails = append(dmails, dmail)
 }

 // Save check result
 var prNumbers []string
 for _, pr := range report.MergedPRs {
  prNumbers = append(prNumbers, pr.Number)
 }
 var dmailIDs []string
 for _, d := range dmails {
  dmailIDs = append(dmailIDs, d.ID)
 }

 checkType := CheckTypeDiff
 if fullCheck {
  checkType = CheckTypeFull
 }

 result := CheckResult{
  CheckedAt:    now,
  Commit:       currentCommit,
  Type:         checkType,
  Divergence:   meterResult.Divergence.Value,
  Axes:         meterResult.Divergence.Axes,
  PRsEvaluated: prNumbers,
  DMails:       dmailIDs,
 }

 if err := a.Store.SaveLatest(result); err != nil {
  return fmt.Errorf("save latest: %w", err)
 }
 if fullCheck {
  if err := a.Store.SaveBaseline(result); err != nil {
   return fmt.Errorf("save baseline: %w", err)
  }
 }
 if err := a.Store.SaveHistory(result); err != nil {
  return fmt.Errorf("save history: %w", err)
 }

 // Print CLI output
 a.PrintCheckOutput(result, dmails, previous.Divergence)

 return nil
}

// PrintCheckOutput renders the CLI output for a check result.
func (a *Amadeus) PrintCheckOutput(result CheckResult, dmails []DMail, previousDivergence float64) {
 a.Logger.Info("")
 a.Logger.Info("Divergence: %s (%s)",
  FormatDivergence(result.Divergence*100),
  FormatDelta(result.Divergence, previousDivergence))

 axisOrder := []Axis{AxisADR, AxisDoD, AxisDependency, AxisImplicit}
 axisNames := map[Axis]string{
  AxisADR:        "ADR Integrity",
  AxisDoD:        "DoD Fulfillment",
  AxisDependency: "Dependency Integrity",
  AxisImplicit:   "Implicit Constraints",
 }

 for _, axis := range axisOrder {
  if score, ok := result.Axes[axis]; ok {
   a.Logger.Info("  %-22s %s — %s",
    axisNames[axis]+":",
    FormatDivergence(float64(score.Score)*weightForAxis(axis, a.Config.Weights)*100),
    score.Details)
  }
 }

 if len(dmails) > 0 {
  a.Logger.Info("")
  a.Logger.Info("D-Mails:")
  pending := 0
  for _, d := range dmails {
   prefix := "[LOW] "
   if d.Severity == SeverityMedium {
    prefix = "[MED] "
   } else if d.Severity == SeverityHigh {
    prefix = "[HIGH]"
    pending++
   }
   status := "sent"
   if d.Status == DMailPending {
    status = "awaiting approval"
   }
   a.Logger.Info("  %s %s %s → %s to %s",
    prefix, d.ID, d.Summary, status, d.Target)
  }
  if pending > 0 {
   a.Logger.Info("")
   a.Logger.Info("%d pending. Run `amadeus resolve <id> --approve` or `--reject`", pending)
  }
 }
}

func weightForAxis(axis Axis, w Weights) float64 {
 switch axis {
 case AxisADR:
  return w.ADRIntegrity
 case AxisDoD:
  return w.DoDFulfillment
 case AxisDependency:
  return w.DependencyIntegrity
 case AxisImplicit:
  return w.ImplicitConstraints
 default:
  return 0
 }
}
```

**Step 4: Run tests to verify they pass**

Run: `just test`
Expected: PASS

**Step 5: Commit**

```bash
git add amadeus.go amadeus_test.go
git commit -m "feat: add Amadeus orchestrator with Phase 1→2→3 workflow"
```

---

### Task 13: CLI Entry Point

**Files:**

- Create: `cmd/amadeus/main.go`

**Step 1: Write the CLI entry point**

```go
// cmd/amadeus/main.go
package main

import (
 "context"
 "flag"
 "fmt"
 "os"
 "path/filepath"

 "github.com/hironow/amadeus"
)

var version = "dev"

func main() {
 if err := run(); err != nil {
  fmt.Fprintf(os.Stderr, "error: %v\n", err)
  os.Exit(1)
 }
}

func run() error {
 var (
  configPath string
  verbose    bool
  dryRun     bool
  full       bool
  showVer    bool
 )

 flag.StringVar(&configPath, "c", "", "config file path")
 flag.StringVar(&configPath, "config", "", "config file path")
 flag.BoolVar(&verbose, "v", false, "verbose output")
 flag.BoolVar(&verbose, "verbose", false, "verbose output")
 flag.BoolVar(&dryRun, "dry-run", false, "generate prompt only")
 flag.BoolVar(&full, "full", false, "force full calibration check")
 flag.BoolVar(&showVer, "version", false, "show version")
 flag.Parse()

 if showVer {
  fmt.Printf("amadeus %s\n", version)
  return nil
 }

 args := flag.Args()
 if len(args) == 0 {
  return fmt.Errorf("usage: amadeus <check|resolve|log> [flags]")
 }

 cmd := args[0]

 switch cmd {
 case "check":
  return runCheck(configPath, verbose, dryRun, full)
 default:
  return fmt.Errorf("unknown command: %s (available: check)", cmd)
 }
}

func runCheck(configPath string, verbose, dryRun, full bool) error {
 // Determine repository root (current directory)
 repoRoot, err := os.Getwd()
 if err != nil {
  return err
 }

 divRoot := filepath.Join(repoRoot, ".divergence")

 // Initialize .divergence/ if it doesn't exist
 if err := amadeus.InitDivergenceDir(divRoot); err != nil {
  return fmt.Errorf("init .divergence: %w", err)
 }

 // Load config
 if configPath == "" {
  configPath = filepath.Join(divRoot, "config.yaml")
 }
 cfg, err := amadeus.LoadConfig(configPath)
 if err != nil {
  return fmt.Errorf("load config: %w", err)
 }

 logger := amadeus.NewLogger(os.Stdout, verbose)
 claude := amadeus.NewClaudeClient()
 claude.DryRun = dryRun

 a := &amadeus.Amadeus{
  Config: cfg,
  Store:  amadeus.NewStateStore(divRoot),
  Git:    amadeus.NewGitClient(repoRoot),
  Claude: claude,
  Logger: logger,
 }

 return a.RunCheck(context.Background(), amadeus.CheckOptions{
  Full:   full,
  DryRun: dryRun,
 })
}
```

**Step 2: Build and verify**

Run: `just build`
Expected: Binary compiled successfully

**Step 3: Run full check suite**

Run: `just check`
Expected: All format, lint, and tests pass

**Step 4: Commit**

```bash
git add cmd/amadeus/main.go
git commit -m "feat: add CLI entry point with check command"
```

---

### Task 14: Integration Smoke Test

**Step 1: Build the binary**

Run: `just build`

**Step 2: Test dry-run in the amadeus repo itself**

Run:

```bash
cd /Users/nino/amadeus && ./amadeus check --dry-run
```

Expected: Prompt printed to stdout (or initial `.divergence/` created + prompt output)

**Step 3: Verify .divergence/ structure was created**

Run:

```bash
ls -la /Users/nino/amadeus/.divergence/
```

Expected: `config.yaml`, `state/`, `history/`, `dmails/` directories exist

**Step 4: Final check**

Run: `just check`
Expected: All tests pass, code formatted, no vet warnings

**Step 5: Commit any fixups, tag as v0.1.0**

```bash
git tag v0.1.0
```

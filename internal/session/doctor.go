package session

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gopkg.in/yaml.v3"
)

// newShellCmd is a package-level variable for creating shell-aware exec.Cmd.
// Override in tests via OverrideShellCmd.
var newShellCmd = platform.NewShellCmd

// OverrideShellCmd replaces the command constructor for testing and returns a
// cleanup function.
func OverrideShellCmd(fn func(ctx context.Context, name string, args ...string) *exec.Cmd) func() {
	old := newShellCmd
	newShellCmd = fn
	return func() { newShellCmd = old }
}

// lookPathShell is a package-level variable for shell-aware LookPath.
// Override in tests via OverrideLookPath.
var lookPathShell = platform.LookPathShell

// OverrideLookPath replaces the path lookup function for testing and returns a
// cleanup function.
func OverrideLookPath(fn func(cmd string) (string, error)) func() {
	old := lookPathShell
	lookPathShell = fn
	return func() { lookPathShell = old }
}

// installSkillsRefFn runs "uv tool install skills-ref". Injectable for testing.
var installSkillsRefFn = func() error {
	cmd := exec.Command("uv", "tool", "install", "skills-ref")
	return cmd.Run()
}

// findSkillsRefDirFn searches for skills-ref submodule directory relative to baseDir.
var findSkillsRefDirFn = findSkillsRefDir

// generateSkillsFn regenerates SKILL.md files. Injectable for testing.
var generateSkillsFn = func(repoRoot string, logger domain.Logger) error {
	_, err := InitGateDir(filepath.Join(repoRoot, domain.StateDir), logger, "")
	return err
}

func findSkillsRefDir(baseDir string) string {
	candidates := []string{
		filepath.Join(baseDir, "..", "skills-ref"),
		filepath.Join(baseDir, "..", "..", "skills-ref"),
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			return c
		}
	}
	return ""
}

// OverrideInstallSkillsRef replaces the skills-ref installer for testing and
// returns a cleanup function.
func OverrideInstallSkillsRef(fn func() error) func() {
	old := installSkillsRefFn
	installSkillsRefFn = fn
	return func() { installSkillsRefFn = old }
}

// OverrideFindSkillsRefDir replaces the skills-ref directory finder for testing
// and returns a cleanup function.
func OverrideFindSkillsRefDir(fn func(string) string) func() {
	old := findSkillsRefDirFn
	findSkillsRefDirFn = fn
	return func() { findSkillsRefDirFn = old }
}

// OverrideGenerateSkills replaces the skill generator for testing and returns a
// cleanup function.
func OverrideGenerateSkills(fn func(string, domain.Logger) error) func() {
	old := generateSkillsFn
	generateSkillsFn = fn
	return func() { generateSkillsFn = old }
}

// SuccessRateChecker computes the success-rate doctor check.
// cmd layer injects the implementation that calls usecase.ComputeSuccessRate,
// keeping session free from usecase imports (semgrep layer rule).
type SuccessRateChecker func(gateDir string, logger domain.Logger) domain.DoctorCheck

// CheckTool verifies that a CLI tool is installed and executable.
// Supports shell-like command strings with leading KEY=VALUE env vars and tilde paths.
func CheckTool(ctx context.Context, name string) domain.DoctorCheck {
	path, err := lookPathShell(name)
	if err != nil {
		return domain.DoctorCheck{
			Name:    name,
			Status:  domain.CheckFail,
			Message: "command not found",
			Hint:    fmt.Sprintf("install %s and ensure it is in PATH", name),
		}
	}

	out, err := newShellCmd(ctx, name, "--version").Output()
	if err != nil {
		return domain.DoctorCheck{
			Name:    name,
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("found at %s but --version failed: %v", path, err),
			Hint:    fmt.Sprintf("%s may be corrupted; reinstall it", name),
		}
	}

	version := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	return domain.DoctorCheck{
		Name:    name,
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s (%s)", path, version),
	}
}

// CheckGitRepo verifies the given directory is inside a git repository.
// Uses exec.Command directly (not newShellCmd) because cmd.Dir must be set,
// and tests use real git repos via git init.
func CheckGitRepo(dir string) domain.DoctorCheck {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		return domain.DoctorCheck{
			Name:    "Git Repository",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s is not a git repository", dir),
			Hint:    `run "git init" or navigate to a git repository`,
		}
	}
	return domain.DoctorCheck{
		Name:    "Git Repository",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s is a git repository", dir),
	}
}

// CheckGitRemote verifies that at least one git remote is configured.
// amadeus reads Pull Requests for divergence checks, so a remote is required.
func CheckGitRemote(dir string) domain.DoctorCheck {
	cmd := exec.Command("git", "remote")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return domain.DoctorCheck{
			Name:    "Git Remote",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("failed to check git remote in %s", dir),
			Hint:    "ensure the directory is a git repository",
		}
	}
	if strings.TrimSpace(string(out)) == "" {
		return domain.DoctorCheck{
			Name:    "Git Remote",
			Status:  domain.CheckFail,
			Message: "no remote configured",
			Hint:    `amadeus reads Pull Requests for divergence checks — run "git remote add origin <url>" to connect to GitHub`,
		}
	}
	remotes := strings.Fields(strings.TrimSpace(string(out)))
	return domain.DoctorCheck{
		Name:    "Git Remote",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%d remote(s): %s", len(remotes), strings.Join(remotes, ", ")),
	}
}

// CheckGateDir verifies .gate/ directory exists and is writable.
// When repair is true and the directory is missing, it creates it.
func CheckGateDir(repoRoot string, repair bool) domain.DoctorCheck {
	dir := filepath.Join(repoRoot, domain.StateDir)
	info, err := os.Stat(dir)
	if err != nil {
		if !repair {
			return domain.DoctorCheck{
				Name:    ".gate/",
				Status:  domain.CheckFail,
				Message: "not found — run 'amadeus init' first",
				Hint:    `run "amadeus init" or "amadeus doctor --repair"`,
			}
		}
		if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
			return domain.DoctorCheck{
				Name:    ".gate/",
				Status:  domain.CheckFail,
				Message: fmt.Sprintf("cannot create %s: %v", dir, mkErr),
				Hint:    `check directory permissions or run "amadeus init"`,
			}
		}
		return domain.DoctorCheck{
			Name:    ".gate/",
			Status:  domain.CheckFixed,
			Message: fmt.Sprintf("created %s", dir),
		}
	}
	if !info.IsDir() {
		return domain.DoctorCheck{
			Name:    ".gate/",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s exists but is not a directory", dir),
			Hint:    `remove the .gate file and run "amadeus init"`,
		}
	}
	probe := filepath.Join(dir, ".doctor_probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return domain.DoctorCheck{
			Name:    ".gate/",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("not writable: %v", err),
			Hint:    "check file permissions on the .gate/ directory",
		}
	}
	if err := os.Remove(probe); err != nil {
		return domain.DoctorCheck{
			Name:    ".gate/",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("probe cleanup failed: %v", err),
			Hint:    "check file permissions on the .gate/ directory",
		}
	}
	return domain.DoctorCheck{
		Name:    ".gate/",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s writable", dir),
	}
}

// checkClaudeAuth determines if the Claude CLI is authenticated by
// interpreting the result of running `claude mcp list`. A successful
// command execution (no error) indicates the CLI is authenticated.
// claudeCmd is the configured command string (may include env prefix).
func RunDoctorWithClaudeCmd(ctx context.Context, configPath string, repoRoot string, claudeCmd string, logger domain.Logger, repair bool, mode domain.TrackingMode, successRateCheck SuccessRateChecker) []domain.DoctorCheck {
	_, span := platform.Tracer.Start(ctx, "domain.doctor")
	defer span.End()

	var results []domain.DoctorCheck

	// --- Binaries ---
	results = append(results, CheckTool(ctx, "git"))
	claudeResult := CheckTool(ctx, claudeCmd)
	results = append(results, claudeResult)
	ghResult := CheckTool(ctx, "gh")
	results = append(results, ghResult)
	if ghResult.Status == domain.CheckOK {
		results = append(results, checkGHAuth(ctx))
	} else {
		results = append(results, domain.DoctorCheck{
			Name:    "gh-auth",
			Status:  domain.CheckSkip,
			Message: "skipped (gh not available)",
		})
	}

	// --- Repository ---
	results = append(results, CheckGitRepo(repoRoot))
	results = append(results, CheckGitRemote(repoRoot))

	// --- State ---
	gateDirResult := CheckGateDir(repoRoot, repair)
	results = append(results, gateDirResult)

	// When .gate/ was just created by repair, regenerate skills/config before
	// checking config so that config.yaml exists by the time CheckConfig runs.
	if gateDirResult.Status == domain.CheckFixed {
		if err := generateSkillsFn(repoRoot, logger); err != nil {
			results = append(results, domain.DoctorCheck{
				Name: "Config", Status: domain.CheckFail,
				Message: fmt.Sprintf("generateSkills after .gate/ repair failed: %v", err),
				Hint:    `run "amadeus init" manually`,
			})
		} else {
			results = append(results, CheckConfig(configPath))
		}
	} else {
		results = append(results, CheckConfig(configPath))
	}

	// --- Data ---
	skillResult := checkSkillMD(repoRoot)
	if repair && skillResult.Status == domain.CheckFail {
		if err := generateSkillsFn(repoRoot, logger); err == nil {
			recheck := checkSkillMD(repoRoot)
			if recheck.Status == domain.CheckOK {
				results = append(results, domain.DoctorCheck{
					Name: "SKILL.md", Status: domain.CheckFixed,
					Message: "regenerated SKILL.md files",
				})
			} else {
				results = append(results, skillResult)
			}
		} else {
			results = append(results, skillResult)
		}
	} else {
		results = append(results, skillResult)
	}
	results = append(results, CheckEventStore(filepath.Join(repoRoot, domain.StateDir)))
	results = append(results, CheckDMailSchema(filepath.Join(repoRoot, domain.StateDir)))
	results = append(results, CheckDeadLetters(ctx, repoRoot))
	results = append(results, CheckFsnotify())

	// --- Connectivity ---
	if claudeResult.Status != domain.CheckOK {
		results = append(results, domain.DoctorCheck{
			Name:    "claude-auth",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "linear-mcp",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "claude-inference",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
		results = append(results, domain.DoctorCheck{
			Name:    "context-budget",
			Status:  domain.CheckSkip,
			Message: "skipped (claude not available)",
		})
	} else {
		// Run mcp list (may fail due to auth or broken MCP config)
		mcpCtx, mcpCancel := context.WithTimeout(ctx, 10*time.Second)
		cmd := newShellCmd(mcpCtx, claudeCmd, "mcp", "list")
		out, mcpErr := cmd.Output()
		mcpCancel()
		mcpOutput := string(out)

		authResult := checkClaudeAuth(mcpOutput, mcpErr, claudeCmd)
		results = append(results, authResult)

		// Linear MCP: skip if auth failed (mcp list output unreliable)
		if authResult.Status != domain.CheckOK {
			results = append(results, domain.DoctorCheck{
				Name:    "linear-mcp",
				Status:  domain.CheckSkip,
				Message: "skipped (auth failed)",
			})
		} else if mode.IsLinear() {
			results = append(results, checkLinearMCP(mcpOutput, mcpErr))
		} else {
			results = append(results, domain.DoctorCheck{
				Name:    "linear-mcp",
				Status:  domain.CheckSkip,
				Message: "skipped (wave mode)",
			})
		}

		// Inference: run independently of mcp list result (only needs claude binary)
		inferCtx, inferCancel := context.WithTimeout(ctx, 3*time.Minute)
		inferCmd := newShellCmd(inferCtx, claudeCmd, "--print", "--verbose", "--output-format", "stream-json", "--max-turns", "1", "1+1=")
		// Filter CLAUDECODE only for the doctor inference probe to prevent
		// nested-session errors. Other subprocesses must preserve CLAUDECODE.
		if inferCmd.Env != nil {
			inferCmd.Env = platform.FilterEnv(inferCmd.Env, "CLAUDECODE")
		} else {
			inferCmd.Env = platform.FilterEnv(os.Environ(), "CLAUDECODE")
		}
		inferOut, inferErr := inferCmd.Output()
		inferCancel()
		inferOutput := string(inferOut)
		inferResult := checkClaudeInference(strings.TrimSpace(extractStreamResult(inferOutput)), inferErr)
		results = append(results, inferResult)

		// Context budget check: skip if inference failed
		if inferResult.Status != domain.CheckOK {
			results = append(results, domain.DoctorCheck{
				Name:    "context-budget",
				Status:  domain.CheckSkip,
				Message: "skipped (inference failed)",
			})
		} else {
			results = append(results, checkContextBudget(inferOutput, repoRoot))
		}
	}

	// --- skills-ref toolchain ---
	results = append(results, CheckSkillsRefToolchain(repoRoot, repair)...)

	// --- Metrics ---
	results = append(results, successRateCheck(filepath.Join(repoRoot, domain.StateDir), logger))

	// --- Repair: stale PID cleanup ---
	if repair {
		pidPath := filepath.Join(repoRoot, domain.StateDir, "watch.pid")
		if data, err := os.ReadFile(pidPath); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(data))) // nosemgrep: ignored-error-go,ignored-error-short-go -- parse failure yields 0; pid>0 guard below safely rejects non-numeric data
			if pid > 0 {
				if !platform.IsProcessAlive(pid) {
					os.Remove(pidPath)
					results = append(results, domain.DoctorCheck{
						Name: "stale-pid", Status: domain.CheckFixed,
						Message: "removed stale PID file",
					})
				}
			}
		}
	}

	for _, r := range results {
		span.AddEvent("doctor.check", trace.WithAttributes(
			attribute.String("check.name", platform.SanitizeUTF8(r.Name)),
			attribute.String("check.status", platform.SanitizeUTF8(r.Status.StatusLabel())),
		))
	}

	return results
}


// CheckFsnotify verifies that the OS file watcher is available.
// On Linux, inotify limits can prevent watcher creation.
func CheckFsnotify() domain.DoctorCheck {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return domain.DoctorCheck{
			Name:    "fsnotify",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("cannot create file watcher: %v", err),
			Hint:    "on Linux, increase inotify limit: sysctl fs.inotify.max_user_watches=524288",
		}
	}
	defer w.Close()
	return domain.DoctorCheck{
		Name:    "fsnotify",
		Status:  domain.CheckOK,
		Message: "file watcher available",
	}
}

// skillsRefBinNames lists possible binary names for the skills-ref package.
// "uv tool install skills-ref" installs as "agentskills", not "skills-ref".
var skillsRefBinNames = []string{"skills-ref", "agentskills"}

func findSkillsRefBin() (string, error) {
	for _, name := range skillsRefBinNames {
		if path, err := lookPathShell(name); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("none of %v found on PATH", skillsRefBinNames)
}

// CheckSkillsRefToolchain verifies that the skills-ref tool is available.
func CheckSkillsRefToolchain(repoRoot string, repair bool) []domain.DoctorCheck {
	if path, err := findSkillsRefBin(); err == nil {
		return []domain.DoctorCheck{{
			Name: "skills-ref", Status: domain.CheckOK,
			Message: fmt.Sprintf("skills-ref found on PATH (%s)", filepath.Base(path)),
		}}
	}
	_, uvErr := lookPathShell("uv")
	if uvErr != nil {
		return []domain.DoctorCheck{{
			Name: "skills-ref", Status: domain.CheckWarn,
			Message: "uv not found on PATH: SKILL.md spec validation is unavailable",
			Hint:    `install uv (https://docs.astral.sh/uv/) or "uv tool install skills-ref"`,
		}}
	}
	subDir := findSkillsRefDirFn(repoRoot)
	if subDir != "" {
		return []domain.DoctorCheck{{
			Name: "skills-ref", Status: domain.CheckOK,
			Message: "uv + submodule ready",
		}}
	}
	if repair {
		if err := installSkillsRefFn(); err != nil {
			return []domain.DoctorCheck{{
				Name: "skills-ref", Status: domain.CheckWarn,
				Message: fmt.Sprintf("uv tool install skills-ref failed: %v", err),
				Hint:    `try manually: "uv tool install skills-ref"`,
			}}
		}
		return []domain.DoctorCheck{{
			Name: "skills-ref", Status: domain.CheckFixed,
			Message: "installed skills-ref via uv tool install",
		}}
	}
	return []domain.DoctorCheck{{
		Name: "skills-ref", Status: domain.CheckWarn,
		Message: "uv found but skills-ref not installed",
		Hint:    `run "amadeus doctor --repair" or "uv tool install skills-ref"`,
	}}
}

// CheckConfig validates that config.yaml exists and can be loaded.
func CheckConfig(path string) domain.DoctorCheck {
	if _, err := os.Stat(path); err != nil {
		return domain.DoctorCheck{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
			Hint:    `run "amadeus init" to create config.yaml`,
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return domain.DoctorCheck{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
			Hint:    "check file permissions on config.yaml",
		}
	}
	cfg := domain.DefaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return domain.DoctorCheck{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %v", path, err),
			Hint:    "fix YAML syntax in config.yaml",
		}
	}
	if errs := domain.ValidateConfig(cfg); len(errs) > 0 {
		return domain.DoctorCheck{
			Name:    "Config",
			Status:  domain.CheckFail,
			Message: fmt.Sprintf("%s: %s", path, strings.Join(errs, "; ")),
			Hint:    `check config.yaml values; run "amadeus init" to regenerate`,
		}
	}
	return domain.DoctorCheck{
		Name:    "Config",
		Status:  domain.CheckOK,
		Message: fmt.Sprintf("%s loaded and validated", path),
	}
}

// extractStreamResult parses stream-json output and returns the "result" field
// from the result message. Used to reuse login check output for inference check.

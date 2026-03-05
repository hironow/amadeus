package testdata

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ==========================================================================
// layers.yaml test fixture
// Covers all rules in layers.yaml (dependency inversion: cmd→session ALLOWED, usecase→session PROHIBITED)
//
// semgrep --test ignores path filters. Each import statement matches ALL
// rules whose regex matches the import path. Comma-separated IDs are used
// in annotations to cover all matching rules per line.
// ==========================================================================

// --- Import-based rules: each import path with ALL matching rule IDs ---

// ruleid: layer-root-no-import-internal, layer-session-no-import-cmd, layer-domain-no-import-upper, layer-eventsource-no-import-upper, layer-usecase-no-import-cmd, layer-platform-no-import-upper, layer-port-no-import-upper
import "github.com/hironow/amadeus/internal/cmd"

// ruleid: layer-root-no-import-internal, layer-domain-no-import-upper, layer-eventsource-no-import-upper, layer-session-no-import-usecase, layer-platform-no-import-upper, layer-port-no-import-upper
import "github.com/hironow/amadeus/internal/usecase"

// ruleid: layer-root-no-import-internal, layer-usecase-no-import-session, layer-domain-no-import-upper, layer-eventsource-no-import-upper, layer-platform-no-import-upper, layer-port-no-import-upper
import "github.com/hironow/amadeus/internal/session"

// ruleid: layer-root-no-import-internal, layer-domain-no-import-upper, s0008-cmd-no-import-eventsource, layer-usecase-no-import-eventsource, layer-platform-no-import-upper, layer-port-no-import-upper
import "github.com/hironow/amadeus/internal/eventsource"

// ruleid: layer-root-no-import-internal, no-import-old-port
import "github.com/hironow/amadeus/internal/port"

// ruleid: layer-root-no-import-internal, layer-domain-no-import-upper, layer-eventsource-no-import-upper, layer-platform-no-import-upper, layer-port-no-import-upper
import "github.com/hironow/amadeus/internal/usecase/port"

// ruleid: layer-root-no-import-internal, layer-domain-no-import-upper, layer-eventsource-no-import-upper, layer-port-no-import-upper
import "github.com/hironow/amadeus/internal/platform"

// ruleid: layer-root-no-import-internal
import "github.com/hironow/amadeus/internal/domain"

// ruleid: layer-no-root-package-import
import "github.com/hironow/amadeus"

// ruleid: layer-no-root-package-import
import pw "github.com/hironow/amadeus"

// --- stdlib I/O imports: layer-domain-no-stdlib-io ---

// ruleid: layer-domain-no-stdlib-io
import "os"

// ruleid: layer-domain-no-stdlib-io
import "io"

// ruleid: layer-domain-no-stdlib-io
import "os/exec"

// ruleid: layer-domain-no-stdlib-io
import "net"

// ruleid: layer-domain-no-stdlib-io
import "net/http"

// ruleid: layer-domain-no-stdlib-io
import "database/sql"

// ruleid: layer-domain-no-stdlib-io
import "context"

// --- Known-good imports (should not trigger) ---

// ok: layer-root-no-import-internal
import "github.com/hironow/amadeus/pkg/something"

// ok: layer-domain-no-stdlib-io
import "fmt"

// ok: layer-domain-no-stdlib-io
import "strings"

// --- Rule: no-direct-linear-api (pattern-regex) ---

func badLinearAPI() {
	// ruleid: no-direct-linear-api
	url := "https://api.linear.app/graphql"
	_ = url
}

func goodNoLinearAPI() {
	// ok: no-direct-linear-api
	url := "https://example.com/api"
	_ = url
}

// --- Rule: domain-no-vendor-vocabulary (pattern-regex) ---

// ruleid: domain-no-vendor-vocabulary
type LinearConfig struct{}

// ruleid: domain-no-vendor-vocabulary
type ClaudeConfig struct{}

// ruleid: domain-no-vendor-vocabulary
type WandbTracker struct{}

// ruleid: domain-no-vendor-vocabulary
type JaegerExporter struct{}

// ok: domain-no-vendor-vocabulary
type IssueTrackerConfig struct{}

// ok: domain-no-vendor-vocabulary
type AIAssistantConfig struct{}

// --- Rule: session-no-direct-new-event (pattern) ---

func badSessionNewEvent() {
	// ruleid: session-no-direct-new-event
	domain.NewEvent("test", nil)
}

// --- Rule: session-no-direct-new-aggregate (pattern-regex) ---

func badSessionNewAggregate() {
	// ruleid: session-no-direct-new-aggregate
	_ = domain.NewReviewAggregate()
}

// ok: session-no-direct-new-aggregate
func goodSessionNoAggregate() {
	_ = "no aggregate creation here"
}

// --- Rule: domain-no-io-contract-interface (pattern-regex) ---

// ruleid: domain-no-io-contract-interface
type EventStore interface {
	Save() error
}

// ruleid: domain-no-io-contract-interface
type FileReader interface {
	Read() ([]byte, error)
}

// ruleid: domain-no-io-contract-interface
type DataRepository interface {
	Find() error
}

// ruleid: domain-no-io-contract-interface
type EventRecorder interface {
	Record() error
}

// ruleid: domain-no-io-contract-interface
type LogWriter interface {
	Write() error
}

// ruleid: domain-no-io-contract-interface
type APIGateway interface {
	Call() error
}

// ok: domain-no-io-contract-interface
type EventType int

// ok: domain-no-io-contract-interface
type ReviewStatus string

// --- Rule: no-shell-exec-command (pattern) ---

func badShellExec() {
	// ruleid: no-shell-exec-command
	exec.Command("sh", "-c", "echo hello")
}

func badBashExec() {
	// ruleid: no-shell-exec-command
	exec.Command("bash", "-c", "echo hello")
}

func badShellExecContext() {
	ctx := context.Background()
	// ruleid: no-shell-exec-command
	exec.CommandContext(ctx, "sh", "-c", "echo hello")
}

func badBashExecContext() {
	ctx := context.Background()
	// ruleid: no-shell-exec-command
	exec.CommandContext(ctx, "bash", "-c", "echo hello")
}

func goodDirectExec() {
	// ok: no-shell-exec-command
	exec.Command("git", "status")
}

// --- Rule: no-ambiguous-type-names (pattern-regex) ---

// ruleid: no-ambiguous-type-names
type TaskManager struct{}

// ruleid: no-ambiguous-type-names
type ReviewService struct{}

// ruleid: no-ambiguous-type-names
type FileHelper struct{}

// ruleid: no-ambiguous-type-names
type StringUtil struct{}

// ruleid: no-ambiguous-type-names
type BaseImpl struct{}

// ruleid: no-ambiguous-type-names
type APIFacade struct{}

// ok: no-ambiguous-type-names
type TaskOrchestrator struct{}

// ok: no-ambiguous-type-names
type ReviewEngine struct{}

// suppress unused import warnings for test fixture
var (
	_ = fmt.Sprintf
	_ = strings.TrimSpace
	_ = json.Marshal
)

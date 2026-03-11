package testdata

import (
	"database/sql"
	"os"
	"os/signal"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// Stubs for semgrep test (not real code)
var platform struct{ SanitizeUTF8 func(string) string }

func SanitizeUTF8(s string) string { return s }

// === ADR 0001: cobra.Command must use RunE, not Run ===

// ruleid: adr0001-cobra-run-without-rune
var badRunCmd = &cobra.Command{
	Use: "bad",
	Run: func(cmd *cobra.Command, args []string) {},
}

// ok: adr0001-cobra-run-without-rune
var goodRunECmd = &cobra.Command{
	Use:  "good",
	RunE: func(cmd *cobra.Command, args []string) error { return nil },
}

// === ADR 0003: OTel span lifecycle ===

func badSpanNoEnd() {
	tracer := otel.Tracer("test")
	// ruleid: adr0003-otel-span-without-defer-end
	ctx, span := tracer.Start(nil, "op")
	_ = ctx
	_ = span
}

func goodSpanDeferred() {
	tracer := otel.Tracer("test")
	// ok: adr0003-otel-span-without-defer-end
	ctx, span := tracer.Start(nil, "op")
	defer span.End()
	_ = ctx
}

// === ADR 0003: SetTracerProvider/SetMeterProvider outside InitTracer ===

func badSetTracerProvider() {
	// ruleid: adr0003-otel-set-tracer-provider-outside-init
	otel.SetTracerProvider(nil)
}

func badSetMeterProvider() {
	// ruleid: adr0003-otel-set-meter-provider-outside-init
	otel.SetMeterProvider(nil)
}

// === ADR 0005: fsnotify watcher lifecycle ===

func badWatcher() {
	// ruleid: adr0005-fsnotify-watcher-without-close
	w, err := fsnotify.NewWatcher()
	_ = w
	_ = err
}

func goodWatcher() {
	// ok: adr0005-fsnotify-watcher-without-close
	w, err := fsnotify.NewWatcher()
	defer w.Close()
	_ = err
}

// === ADR 0005: mutex lock/unlock pairing ===

func badMutex() {
	var mu sync.Mutex
	// ruleid: adr0005-mutex-lock-without-defer-unlock
	mu.Lock()
}

func goodMutex() {
	var mu sync.Mutex
	// ok: adr0005-mutex-lock-without-defer-unlock
	mu.Lock()
	defer mu.Unlock()
}

// === ADR 0008: signal.Notify → NotifyContext ===

func badSignalNotifyADR() {
	ch := make(chan os.Signal, 1)
	// ruleid: adr0008-signal-notify-instead-of-notifycontext
	signal.Notify(ch, os.Interrupt)
}

// === ADR 0008: Execute → ExecuteContext ===

func badExecute() {
	cmd := &cobra.Command{}
	// ruleid: adr0008-execute-without-context
	cmd.Execute()
}

// === ADR 0009: os.Getwd in cobra command ===

var cmdBadGetwd = &cobra.Command{
	RunE: func(cmd *cobra.Command, args []string) error {
		// ruleid: adr0009-os-getwd-in-cobra-cmd
		_, _ = os.Getwd()
		return nil
	},
}

// === OTel UTF-8 safety ===

func badAttributeStringUnsanitized(externalVal string) {
	// ruleid: otel-attribute-string-unsanitized
	attribute.String("key", externalVal)
}

func goodAttributeStringLiteral() {
	// ok: otel-attribute-string-unsanitized
	attribute.String("key", "literal-value")
}

func goodAttributeStringSanitized(externalVal string) {
	// ok: otel-attribute-string-unsanitized
	attribute.String("key", platform.SanitizeUTF8(externalVal))
}

func goodAttributeStringSanitizedLocal(externalVal string) {
	// ok: otel-attribute-string-unsanitized
	attribute.String("key", SanitizeUTF8(externalVal))
}

func badAttributeStringSlice(vals []string) {
	// ruleid: otel-attribute-stringslice-unsanitized
	attribute.StringSlice("key", vals)
}

// === D4: sql.Open without defer Close ===

func badSQLOpen() {
	// ruleid: d4-sql-open-without-defer-close
	db, err := sql.Open("sqlite3", ":memory:")
	_ = db
	_ = err
}

func goodSQLOpen() {
	// ok: d4-sql-open-without-defer-close
	db, err := sql.Open("sqlite3", ":memory:")
	defer db.Close()
	_ = err
}

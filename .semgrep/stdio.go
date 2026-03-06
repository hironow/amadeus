package testdata

import (
	"fmt"
	"os"
)

// ==========================================================================
// stdio.yaml test fixture
// Covers all 6 rules in stdio.yaml
//
// Path filters are ignored in test mode, so fmt.Print* matches all three
// print rules (session, eventsource, root) simultaneously.
// ==========================================================================

// --- fmt.Print* rules (3 rules share the same pattern) ---

func badFmtPrint() {
	// ruleid: adr0002-no-fmt-print-in-session, adr0002-no-fmt-print-in-eventsource, adr0002-no-fmt-print-in-root
	fmt.Print("hello")
}

func badFmtPrintln() {
	// ruleid: adr0002-no-fmt-print-in-session, adr0002-no-fmt-print-in-eventsource, adr0002-no-fmt-print-in-root
	fmt.Println("hello")
}

func badFmtPrintf() {
	// ruleid: adr0002-no-fmt-print-in-session, adr0002-no-fmt-print-in-eventsource, adr0002-no-fmt-print-in-root
	fmt.Printf("hello %s", "world")
}

// ok: adr0002-no-fmt-print-in-session, adr0002-no-fmt-print-in-eventsource, adr0002-no-fmt-print-in-root
func goodFmtSprintf() {
	_ = fmt.Sprintf("hello %s", "world")
}

// --- os.Stdout (unique to adr0002-no-os-stdout-in-internal) ---

func badOsStdout() {
	// ruleid: adr0002-no-os-stdout-in-internal
	_ = os.Stdout
}

// --- os.Stderr (unique to adr0002-no-os-stderr-in-internal) ---

func badOsStderr() {
	// ruleid: adr0002-no-os-stderr-in-internal
	_ = os.Stderr
}

// --- os.Stdin (unique to adr0002-no-os-stdin-in-internal) ---

func badOsStdin() {
	// ruleid: adr0002-no-os-stdin-in-internal
	_ = os.Stdin
}

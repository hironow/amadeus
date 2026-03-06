// Package usecase orchestrates COMMAND → Aggregate → EVENT flows
// and dispatches events through the PolicyEngine.
//
// Layer rules (enforced by semgrep):
//   - usecase MAY import usecase/port and domain
//   - usecase MUST NOT import cmd, session, or eventsource directly
//   - I/O delegation uses usecase/port output interfaces (dependency inversion)
package usecase

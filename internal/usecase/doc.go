// Package usecase orchestrates domain aggregates and external systems.
// It sits between cmd (CLI) and session (I/O adapters), owning the
// COMMAND → Aggregate → EVENT flow and POLICY dispatch.
//
// Layer rules (enforced by semgrep):
//   - usecase MAY import root package (amadeus) and session
//   - usecase MUST NOT import cmd or eventsource directly
//   - session MUST NOT import usecase (no reverse dependency)
package usecase

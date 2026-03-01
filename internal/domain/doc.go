// Package domain contains pure functions with no I/O and no context.Context.
// All functions in this package must be deterministic and side-effect free.
// I/O operations belong in the session layer; orchestration belongs in usecase.
package domain

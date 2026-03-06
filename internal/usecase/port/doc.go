// Package port defines context-aware interface contracts and trivial default
// implementations (null objects) for the port-adapter pattern.
// Concrete I/O implementations live in session and platform layers.
// Port may only import domain (+ stdlib such as context, errors).
// No imports of upper internal layers (cmd, usecase root, session, eventsource, platform).
//
// This package lives under usecase/ to express that ports are owned by the
// usecase layer (Output Port in hexagonal architecture). Session and cmd
// layers consume these interfaces; only domain may be imported here.
package port

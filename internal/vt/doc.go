// Package vt contains Virginia Tech registration protocol primitives and safe
// HTTP client methods.
//
// It owns request construction and response parsing details that are specific to
// classes.vt.edu. Registration workflows, persistence, scheduling, and user
// interaction live in higher-level packages.
//
// VT exposes three API "pages" behind the same /api/ path:
//   - fose: public course search, plain JSON, no authentication.
//   - sisproxy: authenticated student/cart/preflight reads, JSONP responses.
//   - shockabsorber: registration queue, intentionally not implemented yet.
//
// Most names in this package mirror VT's API names rather than inventing nicer
// ones. That makes it easier to compare code against browser network traces and
// docs/VT_API_REFERENCE.md when the protocol behaves unexpectedly.
package vt

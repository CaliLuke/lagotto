// Lagotto sniffs out Go layout smells — structural problems that the
// language's specific rules (methods bound to receiver-defining
// packages, package = directory, build tags, internal/ visibility)
// produce. It loads workspaces with golang.org/x/tools/go/packages,
// walks the type graph with go/types, and flags structural
// anti-patterns that filesystem-only linters cannot see.
//
// # Smells
//
// See README.md and docs/patterns/ for full descriptions, examples,
// and remediation guidance. The catalog at a glance:
//
//   - G1  — Receiver Monolith
//   - G1B — Decomposition Theatre
//   - G1C — Aggregate Holder
//   - G2  — Stutter Names
//   - G3  — Build-Tag Pair Sprawl
//   - G4  — God Dependency Bag
//   - G5  — Mixed-Concern File
//   - G6  — Facade Method
//   - G7  — Init Coupling
//   - G8  — Internal Re-Export Tunnel
//   - G9  — Prefix Cluster
//   - G10 — Shadow Suffix
//   - G11 — Junk Drawer
//   - G12 — Premature Package
//
// # Output contract
//
// Every detector returns a slice of [Finding] values. The CLI
// aggregates them into a [Report] and serializes JSON (default) or
// text. JSON is the stable contract; downstream tooling depends on
// the field names.
//
// # Architecture
//
// Each detector lives in its own file (receivers.go, deps.go,
// mixed.go, etc.) and exports a scan*() entry point used by both
// the audit subcommand and direct CLI subcommands. Helpers shared
// across detectors live in loader.go (package loading, exclusion),
// emit.go (output), severity.go (severity ranks), and report.go
// (the wire types).
package main

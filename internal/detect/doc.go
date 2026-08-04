// Package detect contains every lagotto layout-smell detector and the
// cobra subcommand factories that expose them.
//
// # Smells
//
// See README.md and docs/patterns/ for full descriptions, examples,
// and remediation guidance. The catalog at a glance:
//
//   - G1  — Receiver Monolith
//   - G1B — Decomposition Theatre
//   - G1C — Aggregate Holder
//   - G1D — Hidden Holder via Registry
//   - G1E — Foreign Holder
//   - G2  — Stutter Names
//   - G3  — Build-Tag Pair Sprawl
//   - G4  — God Dependency Bag
//   - G5  — Disconnected File Concerns
//   - G6  — Facade Method
//   - G7  — Init Coupling
//   - G8  — Internal Re-Export Tunnel
//   - G9  — Prefix Cluster
//   - G10 — Shadow Suffix
//   - G11 — Generic Filename
//   - G12 — Premature Package
//   - G13 — Large Cohesive File
//   - G14 — Cross-Layer Orchestration
//   - G15 — Materialized Result Pipeline
//
// # Output contract
//
// Every detector returns a slice of [audit.Finding] values. The CLI
// aggregates them into an [audit.Report] and serializes JSON
// (default) or text. JSON is the stable contract; downstream tooling
// depends on the field names.
//
// # Architecture
//
// One file per detector (receivers.go, deps.go, mixed.go, …) with
// each detector exposing a Scan*() entry point. Cross-detector
// helpers live in support.go and concerns.go. The cobra subcommand
// factories — AuditCmd, MonolithsCmd, StutterCmd, FacadesCmd,
// DepsCmd, MixedCmd, FSCmd, InitsCmd, TunnelCmd, LayersCmd, ResultsCmd — are defined in
// cmd.go and wired up by cmd/lagotto/main.go.
package detect

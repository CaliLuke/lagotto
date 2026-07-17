# Pattern catalog

Each file in this directory describes one smell lagotto detects: what
it is, why it matters, what triggers it, what doesn't, and how to fix
it. Read these to understand a finding before acting on it.

The smells are not all equally bad. Use the severity in the
[finding](../../README.md#severity-guide) as a triage cue, and the
order below as the suggested remediation order when a single package
trips multiple detectors.

## Type-boundary signals

Type-design review signals. File reshuffling cannot resolve them; act when the
evidence shows independently changing responsibilities or consumer coupling,
and suppress intentionally broad APIs.

- [G1 — Receiver Monolith](g1-receiver-monolith.md): one named type
  owns too many methods spanning too many concerns.
- [G1B — Decomposition Theatre](g1b-decomposition-theatre.md): a
  cluster of type aliases pretending to split a god type.
- [G1C — Aggregate Holder](g1c-aggregate-holder.md): a struct that
  collects same-package sub-services so callers still pass one
  handle around.
- [G1D — Hidden Holder via Registry](g1d-hidden-holder.md): a
  thin holder paired with package-level `sync.Map` registries
  keyed by the holder's pointer.
- [G1E — Foreign Holder](g1e-foreign-holder.md): a broad holder that
  remains in production signatures after its services moved out.
- [G4 — God Dependency Bag](g4-god-dependency-bag.md): a `Deps`
  struct that mixes types from many unrelated packages.

## Repository layer policy

Opt-in, typed rules for architecture boundaries specific to a repository.

- [G14 — Cross-Layer Orchestration](g14-cross-layer-orchestration.md): a
  configured boundary method coordinates several configured services or stores
  while mapping configured generated types.

## File and package shape

Mid-cost cleanup. Triggers usually point at a file that should be
split or a directory that wants to be a subpackage.

- [G2 — Stutter Names](g2-stutter-names.md): exported names repeat
  the package name (`lanes.LaneConfig`).
- [G3 — Build-Tag Pair Sprawl](g3-build-tag-pair-sprawl.md): too
  many `*_stub.go` / `*.go` paired files in one directory.
- [G5 — Disconnected File Concerns](g5-mixed-concern-file.md): a 600+ line
  file contains two or more substantial disconnected declaration-reference
  clusters.
- [G9 — Prefix Cluster](g9-prefix-cluster.md): three or more files
  share a name prefix.
- [G10 — Shadow Suffix](g10-shadow-suffix.md): file names ending in
  `_helpers`, `_utils`, `_handlers`, etc.
- [G11 — Generic Filename](g11-junk-drawer.md): a file literally named
  `helpers.go`, `utils.go`, `common.go`, …
- [G12 — Premature Package](g12-premature-package.md): a directory
  with one source file.
- [G13 — Large Cohesive File](g13-large-cohesive-file.md): a very large file
  whose reference graph remains cohesive, reported only as a navigation or
  layer-policy review signal.

## Code-smell adjacent

Lower-priority polish, but each finding is concrete.

- [G6 — Facade Method](g6-facade-method.md): a method whose body
  is a thin pass-through to another package.
- [G7 — Init Coupling](g7-init-coupling.md): multiple `init()`
  funcs across files with cross-file ordering surface.
- [G8 — Internal Re-Export Tunnel](g8-internal-re-export-tunnel.md):
  a package whose exported surface is dominated by a deeper package.

# Changelog

All notable changes to lagotto will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.2.5] - 2026-07-16

### Fixed

- Repository configuration now requires the declared `version: 1`; empty,
  omitted, zero, and unsupported future versions fail before package loading.
- Interior `**/` layer-policy globs match zero or more path segments, so a
  pattern such as `internal/**/handler.go` covers both
  `internal/handler.go` and deeper handlers.

### Changed

- Tests now require the expected G1 finding, use severity constants, and lock
  in the intended implicit-interface and minor-component cohesion behavior.
- G13 documentation clarifies that complexity-rejected components are a subset
  of the reported minor components.

## [0.2.4] - 2026-07-16

### Added

- G14 Cross-Layer Orchestration adds opt-in, repository-owned layer policies
  that identify boundary methods coordinating multiple configured services or
  stores while mapping configured generated types.
- `.lagotto.yaml` provides durable suppressions and detector thresholds;
  reports now record the Lagotto version and fully resolved configuration.
- G13 Large Cohesive File preserves a LOW navigation and architecture-review
  signal for very large files that the stricter cohesion detector correctly
  leaves alone.

### Changed

- G5 now measures disconnected declaration-reference clusters instead of
  declaration categories, recognizes cohesive interface families, and reports
  actionable component members and source ranges.
- Cyclomatic complexity is applied only after structural candidate selection:
  it filters trivial single-callable islands and ranks
  `prioritization_hotspots`, but never creates findings by itself.
- Receiver findings use cross-package coupling evidence for severity. Prefix
  clusters and foreign-holder findings are calibrated lower, and single-file
  packages are consolidated into one repository-pattern observation.
- G11 generic-filename findings now consider declaration and file size, so a
  one-function `helpers.go` is treated as a naming nit rather than evidence of
  mixed responsibilities.

### Fixed

- G13 evidence now reports actual substantial and ignored graph-component
  counts.
- G14 ignores constructor wiring and avoids double-counting a package helper
  alongside a receiver call from the same dependency package.

## [0.2.1] - 2026-07-16

### Changed

- Release artifacts are now built with Go 1.26 instead of the unsupported
  Go 1.23 toolchain. CI continues to verify the declared Go 1.23 compatibility
  floor alongside Go 1.26.

## [0.2.0] - 2026-07-16

### Added

- G1E Foreign Holder detects decomposed holder types that remain in production
  signatures in downstream packages, closing the caller-view gap. (#5)
- `--fail-on=<severity>` flag: exit 2 when any finding at or above
  the threshold (`critical|high|medium|low`) exists, so CI and
  tooling get a real exit-code signal instead of grepping the text
  output. `check.sh` and the CI self-audit now use it. (#36)

### Fixed

- Filesystem detectors now inspect actual directory entries, including
  build-constrained files, and correctly exclude generated code, `doc.go`,
  commands, and platform filename suites. Intentional one-file visibility
  packages can be recorded via `--exclude`. (#2, #3, #12, #16, #41)
- Receiver detectors share a method-set cache, unwrap type aliases, count only
  locally declared pointee methods, apply fixture filters consistently, and
  produce deterministic, accurate messages and locations. (#13, #15, #21,
  #28, #37–#40)
- Mixed-file, facade, init, dependency-bag, re-export, stutter, source filename,
  and package-loading analysis now match their documented semantics and avoid
  the reviewed false-positive and performance paths. (#4, #14, #17–#20,
  #26, #29–#33)
- The source-install command points at the actual main package. (#25)
- Runtime errors print once (`lagotto: <error>`) instead of twice
  with a full usage dump; `--tags` values are validated up front so
  raw `go list` plumbing no longer leaks. (#22)
- `lagotto audit ./...` and other `dir/...` arguments now work — the
  `...` suffix is accepted and recursion remains implicit. Missing
  paths and file arguments get clear one-line errors instead of
  cryptic chdir/fork failures. (#23)
- `--format` is validated before packages are loaded, so a typo
  fails in milliseconds instead of after a full typecheck. (#24)
- The text emitter propagates write errors: a truncated report
  (full disk, closed pipe) now fails the run instead of exiting 0.
  (#34)
- The CI self-audit step uses `pipefail`, so a lagotto crash can no
  longer be masked by the pipe into `tee`. (#35)

- G6 (facades) no longer panics on a method declared with an empty
  receiver list (`func () F() ...`), which `go/parser` accepts with
  only a soft type error. Such methods are skipped. (#11)
- Per-package load and type errors are no longer swallowed. They are
  printed to stderr (`lagotto: load error: ...`) with a summary
  warning, and echoed in a new `load_errors` array in the JSON report
  envelope, so a broken package is distinguishable from a clean one.
  A path containing no Go packages also warns instead of silently
  emitting an empty report. (#9)
- `--exclude` patterns now match whole path segments instead of
  substrings: the default `gen` no longer silently drops packages
  like `agent`, `engine`, or `legend`, and `vendor` no longer drops
  `vendorfeed`. Multi-segment patterns (`design/generated`) match a
  consecutive segment run. (#7, #10)
- Empty reports emit `"findings": []` instead of `"findings": null`,
  so JSON consumers can iterate findings without a null special
  case. (#6, #27)

## [0.1.2] - 2026-05-21

### Fixed

- Prevented G2, G4, G5, G6, G7, and G8 detectors from panicking when
  `go/packages` returns package metadata where `Syntax` and `GoFiles`
  have different lengths. Detectors now resolve source filenames
  defensively and continue auditing instead of crashing.

## [0.1.1] - 2026-05-11

### Fixed

- Tightened G8 function re-export detection so generated
  service-specific factory functions returning framework types are
  not classified as Internal Re-Export Tunnels. Transparent
  same-name wrappers and alias/value re-exports still count.

### Changed

- Restructured the repo into `cmd/lagotto/` + `internal/` layout
  (`internal/audit`, `internal/pkgload`, `internal/version`,
  `internal/detect`). No behavior change; everything compiles and
  audits identically. The release ldflag target moves from
  `main.version` to
  `github.com/CaliLuke/lagotto/internal/version.Version`.

## [0.1.0] - 2026-04-26

### Added

- Initial public release.
- Detectors for fourteen Go layout smells:
  - `G1` — Receiver Monolith (effective method-set count via
    `types.NewMethodSet`, sees through embedding)
  - `G1B` — Decomposition Theatre (alias clusters that pretend to
    split a god type)
  - `G1C` — Aggregate Holder (same-package sub-services on a
    holder struct)
  - `G1D` — Hidden Holder via Registry (thin holder paired with
    package-level pointer-keyed registry maps and accessor
    functions)
  - `G2` — Stutter Names
  - `G3` — Build-Tag Pair Sprawl
  - `G4` — God Dependency Bag (filtered by distinct external
    packages, not field count alone)
  - `G5` — Mixed-Concern File
  - `G6` — Facade Method
  - `G7` — Init Coupling
  - `G8` — Internal Re-Export Tunnel
  - `G9` — Prefix Cluster
  - `G10` — Shadow Suffix
  - `G11` — Junk Drawer
  - `G12` — Premature Package
- Test-double filter: `Fake*`, `Mock*`, `Stub*`, `Spy*` types and
  `testutil`/`testing`-style packages are skipped by G1.
- `--version` flag, populated by goreleaser ldflags or
  `runtime/debug.ReadBuildInfo` fallback.
- JSON and text output formats.
- Pattern documentation at `docs/patterns/`.
- Skill bundle for use with Claude Code at `skill/`.
- Strict `golangci-lint` configuration and `check.sh` quality
  gate that runs build, vet, lint, race tests, and a lagotto
  self-audit.
- Goreleaser configuration with multi-arch darwin/linux builds and
  Homebrew tap publish.

[Unreleased]: https://github.com/CaliLuke/lagotto/compare/v0.2.5...HEAD
[0.2.5]: https://github.com/CaliLuke/lagotto/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/CaliLuke/lagotto/compare/v0.2.3...v0.2.4
[0.2.1]: https://github.com/CaliLuke/lagotto/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/CaliLuke/lagotto/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/CaliLuke/lagotto/releases/tag/v0.1.2
[0.1.1]: https://github.com/CaliLuke/lagotto/releases/tag/v0.1.1
[0.1.0]: https://github.com/CaliLuke/lagotto/releases/tag/v0.1.0

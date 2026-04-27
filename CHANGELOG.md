# Changelog

All notable changes to lagotto will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

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

[Unreleased]: https://github.com/CaliLuke/lagotto/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/CaliLuke/lagotto/releases/tag/v0.1.0

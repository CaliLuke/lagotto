---
id: feat-cmd-internal-layout
name: Restructure repo into cmd/ + internal/ layout
status: planned
type: refactor
risk: medium (large diff, no behavior change, no public API yet)
---

# Restructure repo into cmd/ + internal/ layout

Move every Go source file out of the repo root into the standard
`cmd/<binary>/` + `internal/<package>/` Go layout. Everything compiles
and behaves identically; the diff is structural only.

## Why

The repo currently keeps all code at the root as `package main`. That
shape is normal in Go for tiny tools, but lagotto is past the line
where the layout starts to feel like a code smell on its own — and
this is a code-quality auditor, so the layout itself reads as a
statement about the project's standards.

Concretely:

- A reviewer who clones the repo sees ~14 `.go` source files plus
  `_test.go` siblings at the root. The producer-package directory
  for the `mixed-concern file` and `prefix cluster` smells looks
  uncomfortably similar to lagotto's own root.
- Cross-detector helpers (`shouldExclude`, `sortedKeys`,
  `embeddedFieldType`, `Finding`, `Severity`, `emit`) are currently
  package-private only because everything shares `package main`.
  That hides actual coupling.
- New detectors (G1E and beyond) will keep growing the root.

This is a pre-v0.1.0 cleanup. There are no external importers, so it
is not a breaking change.

## Target layout

```text
lagotto/
├── cmd/lagotto/                    package main, ~30 lines
│   └── main.go                     cobra root + command wiring
├── internal/
│   ├── audit/                      package audit
│   │   ├── finding.go              Finding, Report (the wire types)
│   │   ├── severity.go             Severity, sevRank, SevCritical/...
│   │   └── emit.go                 Emit(), emitText (renamed exported)
│   ├── pkgload/                    package pkgload
│   │   └── load.go                 LoadPackages, ShouldExclude
│   ├── version/                    package version
│   │   └── version.go              version var, String() func
│   └── detect/                     package detect — all 9 detector files
│       ├── doc.go                  package overview, smell catalog
│       ├── concerns.go             detectConcerns (shared verb buckets)
│       ├── support.go              isTestDouble, isTestPackage,
│       │                           sortedKeys, sortedCopy, firstFilename,
│       │                           embeddedFieldType, receiverTypeName,
│       │                           astReceiverFallback (cross-detector
│       │                           helpers, kept package-private)
│       ├── receivers.go            G1, G1B, G1C, G1D + their helpers
│       ├── stutter.go              G2
│       ├── facades.go              G6
│       ├── deps.go                 G4
│       ├── mixed.go                + mixed_classify.go merged
│       ├── inits.go                G7
│       ├── tunnel.go               G8
│       └── fs.go                   G3, G9–G12
└── (everything else unchanged: docs/, roadmap/, skill/, .github/,
   .goreleaser.yaml, .golangci.yml, README.md, CHANGELOG.md,
   LICENSE, check.sh, go.mod, go.sum)
```

Reasoning for each subpackage:

- **`cmd/lagotto/main.go`** — only the cobra `rootCmd`, persistent
  flags, and `main()`. Each subcommand is registered by calling
  `detect.AuditCmd()`, `detect.MonolithsCmd()`, etc. — the
  subcommand factories live with the detectors that own them.
- **`internal/audit`** — the wire format. `Finding`, `Report`,
  `Severity`, and `Emit(*Report) error`. This is the public
  contract: every detector returns `[]audit.Finding`. Importing
  packages: `cmd/lagotto`, every `detect/*` file.
- **`internal/pkgload`** — `go/packages` loading and the
  `--exclude` filter. Importing packages: every detector.
- **`internal/version`** — the `--version` plumbing.
- **`internal/detect`** — every detector. Stays one package
  (rather than one-package-per-detector) because the detectors
  share a substantial helper surface (`detectConcerns`,
  `isTestDouble`, `sortedKeys`, etc.) and splitting one-per-pkg
  would either duplicate those or require a fourth helper package.
  One package per detector _file_, multiple detectors per _package_,
  is the right granularity.

## Identifier visibility changes

The package-private shortcut goes away. Each existing identifier
falls into one of three buckets:

| Identifier                                          | Now (root)  | After (internal/...)                                                                         |
| --------------------------------------------------- | ----------- | -------------------------------------------------------------------------------------------- |
| `Finding`, `Report`, `Severity`, `Sev*` consts      | exported    | `audit.Finding`, `audit.Report`, `audit.Severity`, `audit.SevCritical`, ...                  |
| `emit`, `emitText`, `resolvedTags`                  | private     | `audit.Emit` (exported), `emitText` (private), `audit.ResolvedTags`                          |
| `loadPackages`, `shouldExclude`                     | private     | `pkgload.Load`, `pkgload.ShouldExclude`                                                      |
| `versionString`, `version`                          | private     | `version.String`, `version.Version`                                                          |
| `auditCmd`, `monolithsCmd`, ...                     | package var | factory funcs returning `*cobra.Command`: `detect.AuditCmd()`, `detect.MonolithsCmd()`, etc. |
| `scanReceivers`, `scanFacades`, `scanFS`, ...       | private     | `detect.ScanReceivers`, `detect.ScanFacades`, ...                                            |
| `detectConcerns`, `isTestDouble`, `sortedKeys`, ... | private     | private to `detect` package                                                                  |

Test files keep their current package and move with their detector.

## Migration steps

Order matters; do not parallelize the moves.

1. **Create the new directories.**

   ```bash
   mkdir -p cmd/lagotto internal/audit internal/pkgload internal/version internal/detect
   ```

2. **Move and rename the wire types** (lowest dependency layer).
   - `severity.go` → `internal/audit/severity.go` (package audit)
   - `report.go` → `internal/audit/finding.go` (package audit; rename
     mirrors the dominant type)
   - `emit.go` → `internal/audit/emit.go` (package audit; export
     `Emit`, keep `emitText` private, export `ResolvedTags`)
   - Update flag references: `flagFormat` and `flagTags` are
     defined in cobra setup; `audit.Emit` and `audit.ResolvedTags`
     take their values as parameters (do not import the cobra-flag
     globals from `audit`).

3. **Move the loader.**
   - `loader.go` → `internal/pkgload/load.go` (package pkgload).
   - `Load(root string, tags []string, exclude []string)` —
     parameters replace globals.
   - `ShouldExclude(path string, exclude []string) bool` — same.
   - Update every caller in `internal/detect/*` to pass tags and
     exclude lists (read once at command-construction time from
     cobra flags, passed down).

4. **Move version plumbing.**
   - `version.go` → `internal/version/version.go` (package version).
   - Export the `Version` var (the `-X main.version=...` ldflag
     target becomes `-X github.com/CaliLuke/lagotto/internal/version.Version=...`).
   - Update `.goreleaser.yaml` ldflags accordingly.

5. **Move detector files into `internal/detect`.**
   - Files: `receivers.go`, `stutter.go`, `facades.go`, `deps.go`,
     `mixed.go`, `mixed_classify.go` (merge with `mixed.go` if it
     keeps below the G5 threshold; otherwise keep separate),
     `inits.go`, `tunnel.go`, `fs.go`.
   - Plus their `_test.go` siblings.
   - Plus `helpers_test.go` (the test fixture builder).
   - All change to `package detect`.
   - Replace each `var auditCmd = &cobra.Command{...}` with a
     factory function `func AuditCmd(deps Deps) *cobra.Command`
     where `Deps` carries flag values and emit/load helpers.
   - Pull cross-detector helpers (`detectConcerns`, `isTestDouble`,
     `isTestPackage`, `sortedKeys`, `sortedCopy`, `firstFilename`,
     `embeddedFieldType`, `receiverTypeName`, `astReceiverFallback`)
     into `internal/detect/support.go`.
   - Pull `detectConcerns` into its own file (`concerns.go`) since
     it has the verb-bucket data table and is non-trivial.

6. **Create `cmd/lagotto/main.go`.**
   - Imports: `internal/audit`, `internal/detect`, `internal/version`,
     `cobra`.
   - Defines the cobra root, persistent flags, and registers each
     subcommand by calling the matching factory: `rootCmd.AddCommand(detect.AuditCmd(deps))`,
     etc.
   - Sets `rootCmd.Version = version.String()`.
   - `main()` calls `rootCmd.Execute()`.
   - Total: ~50 lines.

7. **Delete the old root files.**
   - `main.go`, `doc.go`, `severity.go`, `report.go`, `emit.go`,
     `loader.go`, `version.go`, all detector source files, and
     their `_test.go` siblings.
   - Keep `check.sh`, `README.md`, `CHANGELOG.md`, `LICENSE`,
     `.github/`, `.goreleaser.yaml`, `.golangci.yml`, `.gitignore`,
     `go.mod`, `go.sum`, `docs/`, `roadmap/`, `skill/`.

8. **Move the package-level godoc.**
   - The current `doc.go` at the root explains the smell catalog.
     Move it to `internal/detect/doc.go` and update the
     "Architecture" section to describe the new layout.
   - Add a one-paragraph `internal/audit/doc.go` and
     `internal/pkgload/doc.go` so each subpackage has a real
     package comment (revive's `package-comments` rule will
     enforce this once it sees them).

9. **Fix the `.goreleaser.yaml` build target.**
   - `main: .` becomes `main: ./cmd/lagotto`.
   - `binary: lagotto` unchanged.
   - The `-X` ldflag path updates: `main.version` →
     `github.com/CaliLuke/lagotto/internal/version.Version`.

10. **Fix `.golangci.yml`** if any path-scoped rule references
    `_test\.go$` — the absolute paths change but the regex still
    matches.

11. **Fix `check.sh`** if the self-audit invocation hard-codes
    `.` — it does, and `.` still works because the audit runs
    against the whole module, but `go build -o /tmp/lagotto-selfcheck .`
    becomes `go build -o /tmp/lagotto-selfcheck ./cmd/lagotto`.

12. **Update README.md** if any link points at root-level source
    paths. Scan with `grep -nE '\b(main\.go|receivers\.go|deps\.go)' README.md`.

13. **Run the gates.**

    ```bash
    ./check.sh
    ```

    All five must be green: build, vet, lint, race tests,
    self-audit. The self-audit is the meaningful one — restructured
    code that introduces a G1/G5/G9/G11 finding fails the gate.

14. **Update CHANGELOG.md.**

    ```markdown
    ### Changed

    - Restructured the repo into cmd/lagotto/ + internal/ layout.
      No behavior change; everything compiles and audits identically.
    ```

15. **Commit and remove this task file.**
    `git rm roadmap/feat-cmd-internal-layout.md` and update
    `roadmap/roadmap.md` to drop the row.

## Definition of Done

- `./check.sh` reports `5 passed  0 failed`.
- `go test -race -count=1 ./...` is green; all 38 existing tests
  pass without modification of test logic (only package paths and
  identifier exports change).
- `lagotto audit --tags=cgo,typedb,typedb_prebuilt /Users/luca/code/autok/auto-k-server/internal`
  produces the same findings as before the restructure (modulo
  ordering).
- `lagotto --version` works (`v0.0.0-...+dirty` for an uncommitted
  build, or the ldflag value for a tagged build).
- The repo root contains zero `.go` files. (Self-audit must NOT
  flag the root as a "premature package" — `cmd/lagotto/` is the
  binary's package, not the root.)
- `go doc github.com/CaliLuke/lagotto/internal/detect` and
  `go doc github.com/CaliLuke/lagotto/internal/audit` both render
  meaningful package comments.
- Every `detect.Scan*` function still returns `[]audit.Finding`.
- `roadmap/roadmap.md` no longer lists this task.

## Open questions

1. **Per-detector subpackages?** Whether to split `internal/detect/`
   into `internal/detect/receivers/`, `internal/detect/facades/`,
   etc. The default in this spec is one `detect` package because of
   shared helpers; if those helpers become a real cross-package API
   (G1E will likely need new shared helpers for cross-package
   resolution) it may be worth revisiting in a follow-up task.
2. **Where does the G1E "holder candidate" detection helper live?**
   It's plausibly shared between G1E and a future "verify"
   subcommand, suggesting it wants to be in `internal/detect/holder.go`
   alongside the existing helpers, or even promoted to
   `internal/holder/`. Defer the decision to the G1E task.
3. **Backwards compatibility for ldflag injection.** The
   goreleaser ldflag path changes from `main.version` to the
   internal package path. If any external tooling or release script
   references the old path, it needs updating in lockstep. Check
   `.github/workflows/release.yml` and any scripts under `scripts/`
   if they exist.
4. **Subpackage doc.go comments.** revive's `package-comments`
   rule will require comments on every new package. Plan ~3 lines
   each: `audit`, `pkgload`, `detect`, `version`. Pre-write them as
   part of step 8 above.

## Risks

- **Test fixture import paths.** `helpers_test.go` defines
  `fakeModule` which builds synthetic Go modules. The fakeModule
  helper itself doesn't reference any internal packages of lagotto,
  but if the test fixtures end up needing access to `detect`'s
  unexported helpers, the test file must live in `package detect`,
  not `package detect_test`. The existing tests are already in
  `package main`, so they keep that style — they'll be in
  `package detect`.
- **Lint regression.** revive's `exported` rule fires on every
  newly-exported identifier without a godoc. Audit each new export
  as part of the move; do not let the rule autofix with empty
  comments.
- **Self-audit regression.** If the new `internal/version/`
  package is one file, G12 (Premature Package) will flag it. The
  current rule already exempts `doc.go`-only directories; we may
  need to either add a `version.go` companion (e.g., a small `var
Tag string` for goreleaser-injected tags) or accept the LOW
  finding. Note: this is the right kind of conversation for the
  detector to force.

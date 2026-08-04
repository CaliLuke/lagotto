# lagotto

A Go AST/types-based auditor for **Go layout smells** — structural problems
that the language's specific rules (methods bound to receiver-defining
packages, package = directory, build tags, `internal/` visibility) produce.

Polyglot layout linters miss these because they reason about filesystem
patterns alone. In Go, **filesystem layout is a consequence of type
design**: a directory's organization cannot be cleaner than the types it
contains admit. lagotto loads your packages with `go/packages`, walks the
type graph with `go/types`, and flags the structural anti-patterns that
matter.

## Why "lagotto"

Lagotto Romagnolo, a truffle-hunting dog. Same job: sniff out the
high-value rotten things hiding underground.

## Install

### Homebrew

```bash
brew install caliluke/tap/lagotto
```

### From source

```bash
go install github.com/CaliLuke/lagotto/cmd/lagotto@latest
```

Building from source requires Go 1.26 or later.

### Pre-built binary

Grab a release from <https://github.com/CaliLuke/lagotto/releases>.

## Usage

```bash
# Audit everything under ./internal with the project's build tags
lagotto audit --tags=cgo,typedb --format=json ./internal > findings.json

# Just the receiver-monolith / decomposition-theatre detectors
lagotto monoliths ./internal

# Human-readable output
lagotto audit --format=text ./internal

# Gate CI on findings: exit 2 if anything MEDIUM or worse is found
lagotto audit --fail-on=medium ./internal

# Suppress one accepted finding by its stable smell ID and location
lagotto audit --suppress=G5@cmd/server/cli_perf.go .

# Tune G5 for a one-off review
lagotto mixed --min-lines=800 --min-component-members=3 --min-single-component-complexity=7 --severity=low .

# Run only repository-defined cross-layer orchestration policies
lagotto layers .

# Review materialized raw-result pipelines
lagotto results .
```

JSON output is the default contract for tooling. Each finding has
`smell`, `smell_id`, `severity`, `location`, `message`, `evidence`, and
`suggestion`. Findings are pre-sorted CRITICAL → LOW. When packages
fail to load or typecheck, the report carries a `load_errors` array
(and the errors are printed to stderr), and lagotto exits 1 after
emitting the partial report. A degraded audit therefore cannot pass
as a clean CI result. Lagotto also exits 1 before loading packages when
the target module's `go` directive is newer than the toolchain embedded
in the Lagotto binary.

Exit codes: `0` clean run, `1` run failed, `2` findings at or above
the `--fail-on` threshold (`critical|high|medium|low`). Incomplete
package loading takes precedence over `--fail-on` and exits 1.

`--suppress` is repeatable. Use `SMELL_ID` to suppress a detector
globally or `SMELL_ID@LOCATION` to suppress only findings whose stable
location starts with that value. JSON reports include
`suppressed_findings` when any matches were removed.

### Repository configuration

Lagotto automatically loads `.lagotto.yaml` from the audit root. Check it into
the repository to make accepted findings and detector policy durable and
reviewable:

```yaml
version: 1

suppress:
  - G5@tqlgen/parser.go

mixed:
  min_lines: 600
  min_component_members: 2
  min_component_lines: 40
  min_single_component_complexity: 5
  severity: medium
  cohesive_min_lines: 1200

layer_policy:
  - name: thin-transport
    paths:
      - internal/transport/**
    dependencies:
      - internal/service/**
      - internal/storage/**
    generated_types:
      - gen/**
    max_coordinated_dependencies: 1
    severity: medium
```

The `mixed` settings apply to both `lagotto audit` and `lagotto mixed`.
One-off `mixed` flags override the config values. Use `--config=path/to/file`
to load an explicit file instead; unknown keys and invalid thresholds fail
before packages are loaded. A one-callable component that qualifies only by
the 40-line rule must also meet cyclomatic complexity 5; set
`min_single_component_complexity: 0` to disable that post-candidate check. Set
`cohesive_min_lines: 0` to disable G13. JSON
output includes the exact Lagotto `version`, loaded config path, and fully
resolved `configuration` (defaults plus repository and CLI overrides).
Complexity values and `prioritization_hotspots` rank already-structural
candidates; complexity never creates a finding by itself.

`layer_policy` enables the opt-in G14 detector. A scoped function or method is
reported only when it both calls more than the configured number of distinct
dependency receiver types (or dependency package-function groups) and maps a
configured generated type. Patterns accept module-relative or full import
paths with `*`, `**`, and `?`. G14 does not enforce import allow/deny rules;
keep strict dependency boundaries in a tool such as depguard.

## Smell catalog

| ID  | Smell                     | What it catches                                                                              |
| --- | ------------------------- | -------------------------------------------------------------------------------------------- |
| G1  | Receiver Monolith         | A non-fluent named type has ≥15 effective methods across ≥3 supported concern groups         |
| G1B | Decomposition Theatre     | 3+ type aliases in one package resolving to the same exact named type or instantiation       |
| G1C | Aggregate Holder          | A struct with 5+ distinct same-package pointer sub-service types totaling ≥25 methods        |
| G1D | Hidden Holder             | Thin holder + ≥3 pointer-keyed registry maps + ≥5 exported `*Holder` accessors               |
| G1E | Foreign Holder            | A decomposed holder still appears in production signatures in downstream packages           |
| G2  | Stutter Names             | Exported type/function repeats the package name (`lanes.LaneConfig`)                         |
| G3  | Build-Tag Pair Sprawl     | ≥3 paired files conditioned by build tags (`*_stub.go` / `*_cgo.go`) in one dir              |
| G4  | God Dependency Bag        | A `Deps`/`Container` has ≥8 fields drawn from ≥5 distinct external packages                  |
| G5  | Disconnected File Concerns | A 600+ line file contains 2+ substantial disconnected declaration-reference clusters       |
| G6  | Facade Method             | A method whose body is a thin pass-through (≤3 lines) to a function in another package       |
| G7  | Init Coupling             | Multiple `func init()` in a package with cross-file ordering dependencies                    |
| G8  | Internal Re-Export Tunnel | ≥50% of exports tunnel to one deeper package, excluding public facades over `internal`       |
| G9  | Prefix Cluster            | 3+ files share a ≥2-character name prefix in a flat directory                                |
| G10 | Shadow Suffix             | File names ending in `_helpers`, `_utils`, `_handlers`, `_actions`, `_responses`             |
| G11 | Generic Filename          | Generic filename weighted by its top-level declaration count and physical line count         |
| G12 | Premature Package         | A one-source-file package without shared-test or repeated production reuse                   |
| G13 | Large Cohesive File       | A 1200+ line cohesive file worth a LOW navigation or layer-policy review                     |
| G14 | Cross-Layer Orchestration | A configured layer coordinates too many services/stores while mapping generated boundary types |
| G15 | Materialized Result Pipeline | A complete `[]map[string]any` result set is transformed into a second typed slice             |

## The headline detector: Receiver Monolith and its disguises

In Go, a method must be defined in the package that owns its receiver
type. So if `*TypeDB` has 100 methods, all 100 files must live in the
same directory. No file rename, no convention can change that — the
receiver IS the layout boundary. When a layout problem hits this wall,
the cure is type-level decomposition, not file-level reshuffling.

Two patterns recur whenever someone tries to silence a Receiver Monolith
warning without doing the structural work. lagotto catches the producer-side
disguises and the downstream caller-side escape:

### G1B — Decomposition Theatre (alias cluster)

```go
type graphOps struct{ conn *Conn }

type Mutator     = graphOps
type Searcher    = graphOps
type Threads     = graphOps
type CheckRunner = graphOps
// ... 9 aliases total
```

All "concerns" are the same struct. Receivers are written as `(t *Mutator)`,
`(t *Searcher)`, etc., so a source-AST receiver-name counter sees small
per-receiver counts. The type checker collapses the aliases — every
method is reachable through every alias and remains on the underlying
struct. lagotto flags any package with 3+ aliases pointing at one struct.

### G1D — Hidden Holder via Registry

```go
type TypeDB struct{ conn *Conn }

var (
    nodeReg   sync.Map // map[*TypeDB]*Mutator
    edgeReg   sync.Map // ...
    searchReg sync.Map
    threadReg sync.Map
    promoReg  sync.Map
)

func Nodes(t *TypeDB) *Mutator   { v, _ := nodeReg.Load(t); return v.(*Mutator) }
func Edges(t *TypeDB) *Mutator   { v, _ := edgeReg.Load(t); return v.(*Mutator) }
// ... etc.
```

The third disguise. The holder is "narrow" (no methods, one field)
but the package-level registries do the job that struct fields would
have done — invisibly. Every caller still receives `*TypeDB`, so the
chokepoint is unchanged. lagotto's G1D detector flags any package
with ≥3 pointer-keyed registry maps, ≥5 exported accessors taking
`*Holder` as their first argument, and a holder type with ≤2 of its
own methods.

The remediation is the same as G1C: replace the registries with
typed fields on the holder where the field types live in
**subpackages** (cross-package fields are the goal, not same-package
ones); update callers to take only the narrow sub-service.

### G1C — Aggregate Holder

```go
type TypeDB struct {
    Nodes      *Mutator
    Edges      *EdgeMutator
    Search     *Searcher
    Threads    *Threads
    Promotions *Promotions
    Checks     *CheckRunner
}
```

The sub-services are real distinct types — but they all live in the same
package as the holder, and every caller still receives one `*TypeDB` and
reaches into `t.Nodes.CreateNodes(...)`. The decomposition isn't real
until the sub-services move into their own subpackages and callers take
only the narrow service they need.

### G1E — Foreign Holder

Even after sub-services move into real subpackages, a decomposition remains
cosmetic if downstream functions and structs still accept the broad holder and
reach through it. G1E identifies holders with at least three subpackage service
fields or accessors, then reports production signatures in other packages that
still mention `*Holder`. Constructors and test-fixture packages are exempt.
G1E is MEDIUM: it is concrete architectural evidence, not a correctness risk.

### Embedding theatre

```go
type TypeDB struct {
    *graphOps  // 87 methods promoted onto *TypeDB
}
```

A single embedded same-package struct contributes most of the outer
type's method set. lagotto's G1 detector counts the **effective method
set** via `types.NewMethodSet(*types.Pointer(named))`, so promoted
methods land back on the embedding type. The finding includes
`evidence.promoted_from` so reviewers know the structural fix is to
remove the embedding, not move files.

## Verifying a true decomposition

Before accepting "the receiver is decomposed", confirm:

1. **Effective method set shrinks.** `lagotto monoliths` no longer
   reports the original type under G1, G1B, G1C, G1D, or G1E.
2. **No alias cluster.** `grep -nE '^type \w+ = ' pkg/` shows fewer
   than 3 aliases pointing at one type.
3. **No same-package aggregate holder.** The old type, if it still
   exists, has fewer than 5 pointer fields to types defined in the
   same package.
4. **Callers migrated.** `grep -rn 'OldType' --include='*.go'` shows
   callers importing the new subpackages and taking the narrow type.
5. **The legacy interface narrowed.** If the god type satisfied an
   omnibus interface, that interface has been split into per-concern
   interfaces and consumers updated.

## How it works

`lagotto` loads packages with `golang.org/x/tools/go/packages`, then
runs each detector against the loaded type graph. Detection is based on
the type checker's view, not source-AST string matching, so it sees
through type aliases, generics, embedding, and build tags accurately.
The default audit performs receiver analysis from declaration-only type
metadata, then loads statement-level type information in bounded package
batches to keep peak memory proportional to a slice of the module.

## Severity guide

- **CRITICAL** — Receiver Monolith with a large method set dominated by
  same-package embedding (structural decomposition-theatre evidence); Aggregate
  Holder with ≥50 pointee methods or ≥7 sub-services; Decomposition
  Theatre ≥6 aliases;
  God Dependency Bag ≥12 fields.
- **HIGH** — Receiver Monolith with at least three cross-package operational
  API sites, or an operational concrete-type escape plus at least four total
  operational/state sites; stored DI fields alone remain MEDIUM;
  Aggregate Holder 5–6
  sub-services with ≥25 pointee methods; Decomposition Theatre 3–5
  aliases;
  God Dependency Bag 8–11 fields.
- **MEDIUM** — Receiver Monolith without demonstrated cross-package coupling;
  Foreign Holder; Stutter Names; Build-Tag Pair Sprawl; Disconnected File
  Concerns; Internal Re-Export Tunnel; a generic filename with ≥10 declarations
  over ≥200 lines; Cross-Layer Orchestration by default (repository-configurable).
- **LOW** — Prefix Cluster; Premature Package; Large Cohesive File; Shadow
  Suffix; Init Coupling; Generic Filename.

## What lagotto does NOT flag

- A `Conn` struct with many low-level methods at one abstraction level
  (`Read`, `Write`, `Close`...). Method count alone isn't the smell;
  spanning ≥3 distinct concerns is.
- `package main` with many files — entry-point packages are exempt.
- Test doubles (`Fake*`, `Mock*`, `Stub*`, `Spy*`) and `testutil`
  packages, which legitimately implement wide interfaces.
- Generated files carrying the standard `// Code generated ... DO NOT EDIT.`
  header.

## Contributing

Issues and PRs welcome at <https://github.com/CaliLuke/lagotto>.

## License

MIT — see [LICENSE](./LICENSE).

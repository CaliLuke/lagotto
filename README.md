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
go install github.com/CaliLuke/lagotto@latest
```

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
```

JSON output is the default contract for tooling. Each finding has
`smell`, `smell_id`, `severity`, `location`, `message`, `evidence`, and
`suggestion`. Findings are pre-sorted CRITICAL → LOW. When packages
fail to load or typecheck, the report carries a `load_errors` array
(and the errors are printed to stderr) so a degraded audit is
distinguishable from a clean one.

Exit codes: `0` clean run, `1` run failed, `2` findings at or above
the `--fail-on` threshold (`critical|high|medium|low`; without the
flag lagotto always exits 0 when the audit runs).

## Smell catalog

| ID  | Smell                     | What it catches                                                                              |
| --- | ------------------------- | -------------------------------------------------------------------------------------------- |
| G1  | Receiver Monolith         | A named type's effective method set (incl. promoted via embedding) is ≥15 across ≥3 concerns |
| G1B | Decomposition Theatre     | 3+ type aliases in one package all resolving to a single underlying struct                   |
| G1C | Aggregate Holder          | A struct with 5+ same-package sub-service fields whose pointee method counts total ≥25       |
| G1D | Hidden Holder             | Thin holder + ≥3 pointer-keyed registry maps + ≥5 exported `*Holder` accessors               |
| G2  | Stutter Names             | Exported type/function repeats the package name (`lanes.LaneConfig`)                         |
| G3  | Build-Tag Pair Sprawl     | >2 paired files conditioned by build tags (`*_stub.go` / `*_cgo.go`) in one dir              |
| G4  | God Dependency Bag        | A `Deps`/`Container` struct mixes >8 dependency types from unrelated packages                |
| G5  | Mixed-Concern File        | A single file holds 3+ unrelated decl groups (types + validation + utilities)                |
| G6  | Facade Method             | A method whose body is a thin pass-through (≤3 lines) to a function in another package       |
| G7  | Init Coupling             | Multiple `func init()` in a package with cross-file ordering dependencies                    |
| G8  | Internal Re-Export Tunnel | A package whose only role is to re-export from a deeper package                              |
| G9  | Prefix Cluster            | 3+ files share a name prefix in a flat directory                                             |
| G10 | Shadow Suffix             | File names ending in `_helpers`, `_utils`, `_handlers`, `_actions`, `_responses`             |
| G11 | Junk Drawer               | File named `helpers.go` / `utils.go` / `common.go` / `misc.go` with mixed contents           |
| G12 | Premature Package         | A directory containing only 1 source file (excluding tests, doc, generated)                  |

## The headline detector: Receiver Monolith and its disguises

In Go, a method must be defined in the package that owns its receiver
type. So if `*TypeDB` has 100 methods, all 100 files must live in the
same directory. No file rename, no convention can change that — the
receiver IS the layout boundary. When a layout problem hits this wall,
the cure is type-level decomposition, not file-level reshuffling.

Two patterns recur whenever someone tries to silence a Receiver Monolith
warning without doing the structural work. lagotto catches both:

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
   reports the original type under G1, G1B, or G1C.
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

## Severity guide

- **CRITICAL** — Receiver Monolith ≥25 methods or ≥7 files; Aggregate
  Holder with ≥50 pointee methods or ≥7 sub-services; Decomposition
  Theatre ≥6 aliases; God Dependency Bag ≥12 fields.
- **HIGH** — Receiver Monolith ≥15 methods; Aggregate Holder 5–6
  sub-services with ≥25 pointee methods; Decomposition Theatre 3–5
  aliases; Mixed-Concern File >300 lines; God Dependency Bag 8–11
  fields.
- **MEDIUM** — Stutter Names; Build-Tag Pair Sprawl; Mixed-Concern File
  100–300 lines; Prefix Cluster of 4+ files; Internal Re-Export Tunnel.
- **LOW** — Premature Package; Shadow Suffix; Init Coupling; Junk
  Drawer <100 lines.

## What lagotto does NOT flag

- A `Conn` struct with many low-level methods at one abstraction level
  (`Read`, `Write`, `Close`...). Method count alone isn't the smell;
  spanning ≥3 distinct concerns is.
- `package main` with many files — entry-point packages are exempt.
- Test doubles (`Fake*`, `Mock*`, `Stub*`, `Spy*`) and `testutil`
  packages, which legitimately implement wide interfaces.
- Generated files (filter via `--exclude` if your generator emits a
  recognizable path fragment).

## Contributing

Issues and PRs welcome at <https://github.com/CaliLuke/lagotto>.

## License

MIT — see [LICENSE](./LICENSE).

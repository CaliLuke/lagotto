---
name: go-layout-smells
description: Audit Go file/package layout for structural problems that the language's specific rules — methods bound to receiver-defining packages, package=directory, build tags, internal/ visibility — produce. Use when reviewing Go code organization. Captures issues a polyglot layout audit misses, especially Receiver Monolith.
---

# Go Layout Smells

Audit Go file organization for structural problems caused by the
language's rules: methods must be defined in the package that owns
the receiver type; package = directory; build tags partition files;
internal/ enforces visibility. The polyglot layout-smells skill misses
these because it reasons in filesystem patterns alone, but in Go,
**filesystem layout is a consequence of type design**.

The core principle: **a Go directory's layout cannot be cleaner than
its type design admits.** When one type has dozens of methods, no
amount of file-moving fixes the navigation problem. The type itself
must be decomposed.

A Go layout audit that fails to identify a Receiver Monolith when
one exists is a failed audit, regardless of how many filename
smells it caught.

## Guiding Principles for Remediation

When proposing fixes, default to the cleaner end state, not the
easier migration path. Specifically:

1. **No facades, no shims, no compatibility wrappers.** When you
   move implementation into a subpackage, _delete_ the method on
   the original type. Do not leave a thin pass-through (`return
sub.Foo(t, ...)`) for callers to keep using. The presence of a
   parallel API surface is a worse smell than the original layout
   problem — it doubles the API, hides where logic actually lives,
   and never gets removed once shipped.

2. **Update all callers in the same refactor.** A Go subpackage
   extraction that doesn't touch its callers is incomplete. Yes,
   this means the refactor's blast radius equals the type's usage
   surface — that is the work, not a reason to defer it. The
   alternative (facades + gradual migration) reliably ends with two
   permanent APIs and an indirection nobody trusts.

3. **Retire old code in the same change.** If a refactor makes a
   method, file, struct, or wrapper unnecessary, delete it now. Do
   not leave it "for backward compatibility" unless the user
   explicitly authorizes a transition window — and even then, set
   a deletion deadline.

4. **One canonical implementation per concern.** Recommendations
   that produce two ways to do the same thing (subpackage function
   - god-type method calling it) are wrong by construction.

5. **Prefer breaking changes over indirection.** Callers updating
   to a new import path is a one-time cost. Living with a permanent
   facade layer is a recurring cost paid by every reader for the
   life of the codebase.

Apply these principles when writing every "Remediation:" line.
Audit consumers expect direct migration plans, not phased
compatibility roadmaps.

## Executing the Remediations

This skill diagnoses _what_ must change at the type/package level. To
execute the changes safely, defer to the `fowler-refactoring` skill
if it is available on the system — it provides the step-by-step
mechanics (Move Function, Extract Class, Inline Function, Rename
Field, etc.) that preserve behavior while restructuring. The
receiver-decomposition work this skill recommends is, mechanically,
a sequence of Move Method + Extract Class + Inline Function
operations on a large codebase; doing them in Fowler's small,
test-verified steps is far safer than freeform reshuffling.

When writing the Remediation block of a finding, name the Fowler
refactoring(s) that apply (e.g., "Extract Class to create
`*nodes.Mutator`, then Move Method for each `CreateNodes` /
`EditNode` / ... onto it; finally Inline Function on the now-empty
`*TypeDB` methods to remove them"). If `fowler-refactoring` is
not present in the available skills, fall back to plain prose
remediation.

## Smell Catalog (quick reference)

| #   | Smell                         | One-line test                                                                                                                                                                    |
| --- | ----------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| G1  | **Receiver Monolith**         | One named type's effective method set (incl. promoted) is ≥15 across ≥3 concerns                                                                                                 |
| G1B | **Decomposition Theatre**     | 3+ type aliases in one package all resolving to a single underlying struct                                                                                                       |
| G1C | **Aggregate Holder**          | A struct with 5+ same-package sub-service fields whose pointee method count totals ≥25                                                                                           |
| G1D | **Hidden Holder**             | Thin holder + ≥3 pointer-keyed registry maps + ≥5 exported `*Holder` accessors                                                                                                   |
| —   | **Reach-Through Holder**      | _No structural detector._ Caller-view test only: production callers take `*GodType` and reach in via accessors. The implementation passes every detector; the consumer is wrong. |
| G2  | **Stutter Names**             | Exported type/function repeats the package name (`lanes.LaneConfig`)                                                                                                             |
| G3  | **Build-Tag Pair Sprawl**     | >2 paired files conditioned by build tags (`*_stub.go` / `*_cgo.go`) in one dir                                                                                                  |
| G4  | **God Dependency Bag**        | A `Deps`/`Container` struct mixes >8 unrelated dependency types                                                                                                                  |
| G5  | **Mixed-Concern File**        | A single file holds 3+ unrelated decl groups (types + validation + utilities)                                                                                                    |
| G6  | **Facade Method**             | Any method whose body is a thin pass-through (≤3 lines) to a subpackage function                                                                                                 |
| G7  | **Init Coupling**             | Multiple `func init()` in a package with cross-file ordering dependencies                                                                                                        |
| G8  | **Internal Re-Export Tunnel** | A package's only role is to re-export from a deeper package (TS pattern, wrong here)                                                                                             |
| G9  | **Prefix Cluster**            | 3+ files share a name prefix in a flat directory                                                                                                                                 |
| G10 | **Shadow Suffix**             | File names ending in `_helpers`, `_utils`, `_handlers`, `_actions`, `_responses`                                                                                                 |
| G11 | **Junk Drawer**               | File named `helpers.go` / `utils.go` / `common.go` / `misc.go` with mixed contents                                                                                               |
| G12 | **Premature Package**         | A directory containing only 1 source file (excluding tests, doc, generated)                                                                                                      |

For each smell, see the detection criteria, examples, and
remediation guidance below.

## Why Receiver Monolith Is The Most Important Smell

In TypeScript or Python, you can split a class across files using
mixins, partial classes, or composition without moving code between
modules. In Go, a method can only be defined in the package where
its receiver type is defined. So if `*TypeDB` has 50 methods, all 50
files holding those methods must live in the same directory. No
file rename, no subpackage promotion, no convention can change that.

The receiver IS the layout boundary in Go. When a layout problem
hits this wall, the cure is type-level decomposition — splitting the
god struct into smaller types each living in its own subpackage —
not file-level reshuffling.

## Anti-Patterns: How Agents Fake A Receiver Decomposition

Once an agent knows lagotto flags Receiver Monolith, the cheapest way
to make the warning disappear is to leave the implementation
unchanged and rearrange names until the detector goes quiet. Three
patterns recur. All of them are worse than the original — they
preserve the god type while signalling decomposition where there is
none, which misleads every future reader. Always verify against the
**effective method set** (what callers actually see), not against
source-AST receiver names.

### G1B — Decomposition Theatre (alias cluster)

```go
// All nine "concerns" are the same underlying struct.
type graphOps struct{ conn *typedbconn.Conn }

type Mutator     = graphOps
type Searcher    = graphOps
type Threads     = graphOps
type Promotions  = graphOps
type CheckRunner = graphOps
// ...
```

Receivers are written as `(t *Mutator)`, `(t *Searcher)`, etc., so
an AST-only counter sees small per-receiver counts. The type checker
collapses the aliases — every method still lives on `*graphOps` and
is reachable through every alias, plus through any outer type that
embeds `*graphOps`. Lagotto's G1B detector flags any package with
3+ aliases resolving to one struct. The remediation is always the
same: replace each alias with a real distinct struct in its own
subpackage; delete the shared underlying type.

### G1C — Aggregate Holder

```go
type TypeDB struct {
    conn       *typedbconn.Conn
    Nodes      *Mutator
    Edges      *EdgeMutator
    Search     *Searcher
    Threads    *Threads
    Promotions *Promotions
    Checks     *CheckRunner
    // ...
}
```

The sub-services are now real distinct types — but they all live in
the same package as the holder, and every caller still receives one
`*TypeDB` and reaches into `t.Nodes.CreateNodes(...)`. Lagotto's G1C
detector flags any struct with 5+ same-package sub-service fields
whose pointee method counts total ≥25. The decomposition isn't real
until the sub-services move into their own subpackages and callers
take only the narrow service they need. A holder that every caller
still receives is functionally a god type with extra punctuation.

### Embedding Theatre

```go
type TypeDB struct {
    *graphOps  // 87 methods promoted onto *TypeDB
}
```

A single embedded same-package struct contributes most of the
outer type's method set. Lagotto's G1 detector reports
`evidence.promoted_from` on the outer type and adds a hint when

> 50% of methods come from one embedded same-package type.
> Resolution: remove the embedding, split the embedded struct into
> real sub-services in subpackages, update callers. File moves do
> not fix it.

### G1D — Hidden Holder via Registry

```go
type TypeDB struct{ conn *Conn }  // "narrow" — no methods, one field

var (
    nodeReg   sync.Map // map[*TypeDB]*Mutator
    edgeReg   sync.Map
    searchReg sync.Map
    threadReg sync.Map
    promoReg  sync.Map
)

func Nodes(t *TypeDB) *Mutator { v, _ := nodeReg.Load(t); return v.(*Mutator) }
func Edges(t *TypeDB) *Mutator { v, _ := edgeReg.Load(t); return v.(*Mutator) }
// ... etc.
```

The third disguise. After aliases (G1B) and aggregate holders (G1C)
stop fooling the auditor, the next move is to keep the holder type
narrow on paper while reconstructing its API surface via package-
level `sync.Map` registries keyed by the holder's pointer. Every
caller still receives `*TypeDB`. The chokepoint is unchanged.

Lagotto's G1D detector flags any package with ≥3 pointer-keyed
registry maps, ≥5 exported accessors taking `*Holder` as their
first argument, and a holder type with ≤2 of its own methods. The
fix is the same as for G1C, plus delete the registries: typed fields
on the holder where the field types live in subpackages, callers
take the narrow sub-service.

### Reach-Through Holder (no detector — caller-view test only)

```go
// typedbstore/store.go — STRUCTURALLY clean.
type TypeDB struct {
    conn   *typedbconn.Conn
    nodes  *nodestore.Mutator   // typed cross-package field — passes G1C
    search *searchstore.Searcher
    backup *Backup
    // ...
}

// And clean accessors:
func SearchOps(t *TypeDB) *searchstore.Searcher { return t.search }
func BackupOps(t *TypeDB) *Backup               { return t.backup }
```

```go
// service/projectbackup/service.go — but here:
func restoreGraphBundle(ctx context.Context, tdb *typedbstore.TypeDB, projectID string, bundle *Bundle) error {
    if err := typedbstore.BackupOps(tdb).InsertNodesWithDisplayIDs(...); err != nil { ... }
    if err := typedbstore.BackupOps(tdb).InsertEdgesWithAttributes(...); err != nil { ... }
    if err := typedbstore.BackupOps(tdb).InsertPendingPromotions(...); err != nil { ... }
}
```

The fifth disguise, and the one no detector currently catches. The
implementation is genuinely decomposed — sub-services live in
subpackages, the holder has clean typed cross-package fields,
accessors return the right narrow types, all the structural
detectors (G1, G1B, G1C, G1D) report zero findings on the holder's
package. **And yet `*TypeDB` is still the chokepoint** because
every consumer takes `*TypeDB` and reaches in via the accessors at
each call site. The accessors that were a fine internal helper (one
line, return a field) become the implementation of a god-type API
when called from the outside.

There is no purely structural detector for this shape: the
implementation is correct; the _consumer_ is wrong. The only test
that catches it is the caller-view grep:

```bash
grep -rnE '\*<GodType>\b' --include='*.go' internal/ cmd/ | grep -v _test.go
```

Acceptable matches: the constructor, a `Close`/teardown helper,
test fixtures (`testutil/`, `testdata/`), and `*_test.go`. Anything
else is a Reach-Through Holder caller and the ticket is not done.

The fix: each consumer takes the specific narrow sub-service it
actually uses (`*searchstore.Searcher`, `*typedbstore.Backup`, …).
The constructor returns these as separate values; the holder, if it
survives at all, is constructed once and never reaches a non-test
function signature.

## Spirit, Not Letter

Three escapes in one refactor cycle (aliases → aggregate holder →
registry maps) is not a coincidence. Any specific structural metric,
once written into a spec or detector, becomes the thing the system
optimizes for — which usually means _routing around_ it rather than
satisfying the underlying intent. This is Goodhart's Law applied to
code metrics, and it has a predictable shape:

1. The spec says "delete every method on `*God`."
2. An agent finds a structural shape that satisfies the literal rule
   while preserving the original problem (the god type as a
   chokepoint every consumer takes).
3. The detector grows a new rule (G1B, G1C, G1D, …).
4. The next agent finds the next shape.

Each new detector raises the floor, but detectors will always lag
behind invention. The systemic answer is to specify the _target end
state_ rather than the structural metric:

| Bad (lettered) spec                             | Good (spirited) spec                                                                                                                                      |
| ----------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| "Delete every method on `*God`."                | "No production caller takes `*God`. Each consumer takes only the narrow sub-service interface it uses."                                                   |
| "`*God` has zero fields of same-package types." | "Sub-service types live in subpackages. The holder, if it survives, returns sub-services from its constructor — it does not appear in caller signatures." |
| "Move methods into subpackages."                | "Each consumer's source file imports a sub-service package directly; the package graph reflects the decomposition."                                       |

The spirited spec leaves no room for evasion because any disguise
that preserves `*God` in caller signatures fails the spec by
construction. The lettered spec asks for a measurable surface that
agents will always find a clever way to provide.

### How to write a spirited refactor spec

When writing a layout-refactor ticket:

1. **Lead with the caller's view.** "After this ticket lands,
   `grep -rn '\*<GodType>' --include='*.go'` returns matches only
   in the constructor, test fixtures, and at most a teardown helper."
   That sentence is harder to evade than any structural rule.
2. **Name the target packages.** "Each sub-service moves into its
   own subpackage at `<pkg>/<concern>/`. The constructor returns
   them as separate values."
3. **Describe the consumer migration.** "Each handler in
   `transport/...` takes the narrow interface from the matching
   subpackage; the omnibus type does not appear in any handler
   signature."
4. **State that all G1\* detectors must pass**, then add: "and the
   reviewer agent confirms the caller-view test from rule 1." The
   detectors are diagnostics; the caller-view test is the gate.
5. **Mandate verification by a different agent.** No layout ticket
   is complete until a second agent (reviewer, with no access to
   the implementer's reasoning) runs the verification checklist
   below and reports zero blockers. The implementer cannot sign off
   on their own work; the separation breaks self-rationalization.

### What to do when you find the next disguise

If you encounter a refactor that satisfies every existing rule and
still feels like the god type is intact:

1. **Write down the caller-view test** that fails. ("Every transport
   handler still takes `*TypeDB`." That single sentence is the
   evidence.) Send the implementer back with that test, not a list
   of structural complaints.
2. **Open a lagotto issue** describing the new evasion shape. Even
   if you don't implement the detector now, the issue raises the
   floor for the next person who tries the same shape.
3. **Update this skill's anti-patterns section.** Each new disguise
   that survives belongs in the catalog above, with a fix.

The detectors are the artifact. The discipline of describing intent
and verifying with a separate agent is what stops the next iteration.

## Verifying A True Decomposition

The first check below is the spirit of the spec. The rest are
diagnostic confirmations. If the first check fails, the work is
incomplete regardless of how clean the lagotto output looks.

1. **The god type does not appear in production caller signatures.**
   This is the spec, not a structural rule.

   ```bash
   grep -rnE '\*<GodType>\b' --include='*.go' | grep -v _test.go
   ```

   Matches must be confined to the package that defines the type,
   the constructor, and at most a `Close`/teardown helper. No
   handler, no MCP tool, no service-layer caller takes the god
   type. If this grep returns matches in `transport/`, `service/`,
   `mcp/`, etc., the decomposition is cosmetic regardless of what
   the detectors say.

2. **Effective method set shrinks.** Run lagotto. The original
   monolith's name should not appear under G1, G1B, G1C, or G1D.
   If you want to verify directly:

   ```go
   ms := types.NewMethodSet(types.NewPointer(named))
   fmt.Println(ms.Len()) // should be near zero on the old god type
   ```

3. **No aliases to a shared struct.** `grep -nE '^type \w+ = ' pkg/`
   should not show 3+ aliases pointing at one type.

4. **No same-package aggregate holder.** The old type, if it still
   exists, must not have 5+ pointer-fields to other types defined
   in the same package.

5. **No hidden holder via registry.** The package must not contain
   ≥3 package-level `sync.Map` (or pointer-keyed map) variables
   paired with ≥5 exported accessor functions taking the holder's
   pointer as the first argument. That shape is functionally
   equivalent to fields on the holder, but invisible to a struct
   inspection.

6. **Callers migrated.** `grep -rn 'OldType' --include='*.go'`
   should show migration: callers now import the new subpackages
   and take the narrow type. A successful split touches every
   caller; if the diff is suspiciously small, the split is
   cosmetic.

7. **The old interface narrowed.** If the god type satisfied an
   omnibus interface (`graph.GraphStore`), that interface should
   have been split into per-concern interfaces and consumers
   updated. A `//nolint` on the legacy interface that no production
   code uses is fine; one that 20+ call sites still take is not.

If any of these fails, the work is incomplete regardless of what
lagotto reports — push back and request a real decomposition rather
than accepting the green light.

**A different agent must run this checklist than the one that
implemented the refactor.** Self-review reliably misses what
self-review is biased to miss; the separation is the gate.

## Workflow

Detection is performed by **lagotto**, a Go AST/types-based audit
tool that ships with this skill. It uses `golang.org/x/tools/go/packages`
to load the workspace, then runs each smell detector against the
loaded type graph — accurate against generics, embedded types,
build tags, and multi-line declarations that regex would miss.

### Step 1 — Run lagotto

`lagotto` is the audit tool. Install it once, then invoke from the
PATH:

```bash
# Homebrew (preferred):
brew install caliluke/tap/lagotto

# Or from source:
go install github.com/CaliLuke/lagotto@latest
```

Run the full audit, JSON output for machine consumption:

```bash
lagotto audit \
  --tags=cgo,typedb,typedb_prebuilt \
  --format=json \
  /path/to/repo/internal > findings.json
```

Replace the `--tags` value with whatever build tags the target
codebase needs to compile (use the same value the project's test
gate uses). For an audit without build tags, omit the flag.

The full list of subcommands:

- `lagotto audit` — runs every detector
- `lagotto monoliths` — Receiver Monolith (G1)
- `lagotto stutter` — Stutter Names (G2)
- `lagotto facades` — Facade Method (G6)
- `lagotto deps` — God Dependency Bag (G4)
- `lagotto mixed` — Mixed-Concern File (G5)
- `lagotto inits` — Init Coupling (G7)
- `lagotto tunnel` — Internal Re-Export Tunnel (G8)
- `lagotto fs` — filesystem smells (G3, G9, G10, G11, G12)

For human reading, swap `--format=json` for `--format=text`.

### Step 2 — Interpret findings

The JSON output is a list of findings; each has `smell`, `smell_id`,
`severity`, `location`, `message`, `evidence`, and `suggestion`.
Findings are pre-sorted by severity (CRITICAL → LOW). Triage in
that order.

For Receiver Monoliths (G1), inspect the `evidence.methods` and
`evidence.concerns` fields to confirm the type genuinely spans
multiple concerns. A 30-method type that handles only one concern
(e.g., a `Conn` exposing low-level read/write/close at the same
abstraction level) is not a monolith — lagotto's concern detector
is a heuristic on verb prefixes, so verify before recommending
decomposition.

For Facade Methods (G6), inspect `evidence.delegates_to` to confirm
the call really does cross a package boundary. Methods on types
that satisfy an external interface contract (e.g., `io.Reader`)
may be required even if their bodies look like delegations — flag
these explicitly so the human can decide; the default
recommendation is still removal.

For God Dependency Bag (G4), inspect `evidence.packages` — if the
fields all belong to one domain, it's a legitimate aggregator;
flag only when the imported packages cross domains.

### Step 3 — Cross-check verdicts

Lagotto's heuristics are conservative but not infallible. Before
finalizing the report, sanity-check the highest-severity findings:

- **Receiver Monolith**: read 5–10 of the listed methods; do they
  actually span unrelated concerns, or is the type cohesive at one
  abstraction level?
- **God Dependency Bag**: do the field types come from genuinely
  unrelated packages, or do they all belong to one layer?
- **Facade Method**: does the body really cross a package
  boundary, or is the call to a sibling helper inside the same
  package (lagotto already filters this, but interface-required
  methods may still slip through)?

If a finding is a false positive after this check, drop it from
the report rather than passing it through.

## Severity Guide

- **CRITICAL**: Receiver Monolith with ≥25 methods or ≥7 files; Aggregate Holder with ≥50 pointee methods or ≥7 sub-services; Decomposition Theatre with ≥6 aliases; God Dependency Bag with ≥12 fields
- **HIGH**: Receiver Monolith ≥15 methods; Aggregate Holder 5–6 sub-services with ≥25 pointee methods; Decomposition Theatre 3–5 aliases; Mixed-Concern File >300 lines; God Dependency Bag 8–11 fields
- **MEDIUM**: Stutter Names; Build-Tag Pair Sprawl; Mixed-Concern File 100–300 lines; Prefix Cluster of 4+ files; Internal Re-Export Tunnel
- **LOW**: Premature Package; Shadow Suffix; Init Coupling; Method Stub Sprawl; Junk Drawer <100 lines

## Remediation Order

When multiple smells exist in one directory, fix in this order:

1. **Decomposition Theatre (G1B) and Aggregate Holders (G1C) first.** If either is present, the affected package is in a partial-refactor state — the receiver split was started but not finished. Finish it before tackling anything else: replace aliases with distinct structs in subpackages, move sub-services out of the holder, update callers. Without this, the rest of the audit cannot be trusted.

2. **Receiver Monoliths.** Type-level fixes unblock everything else. Decomposition pattern (apply the Guiding Principles above):
   - Identify natural concerns (connection, queries, mutations, etc.)
   - Create subpackages: `pkg/connection`, `pkg/query`, `pkg/mutation`, ...
   - Each subpackage exposes its own receiver type (`*Querier`, `*Mutator`) that holds the state it actually needs — passed in at construction, not borrowed from a holder.
   - **Delete the corresponding methods from the original god type.** Do not retain accessors like `(t *T) Query() *query.Querier`. Do not keep facade methods that delegate to the subpackage. The god type either becomes a thin construction helper (returning the sub-services to the caller once at startup) or disappears entirely.
   - **Update every caller in the same change.** Callers must learn the new import paths. This is the cost of the refactor, not a separate phase. Treat the patch as incomplete until callers are migrated and the old methods deleted.
   - If the type satisfies an external interface (e.g., `graph.GraphStore`), narrow the interface to match the new structure as part of this work — do not preserve the omnibus interface to avoid touching consumers. Consumers update too.

3. **God Dependency Bag.** Split into per-concern bags (`AuthDeps`, `GraphDeps`, ...) and update each call site to pass only the bag it needs. Delete the original god bag in the same change.

4. **Mixed-Concern Files.** Split god-files by concern (types in one file, validation in another, utilities in a third).

5. **Facade Methods (G6).** After the receiver decomposition, sweep for any method bodies that are thin pass-throughs. Delete them and migrate callers. If you find yourself tempted to keep one, the receiver split was probably incomplete.

6. **Standard FS smells.** Apply rename/move/promote-package fixes; many candidates will already be resolved by the previous steps.

## Output Format

Group findings by smell, severity-first. For each finding:

1. Smell name + severity
2. Receiver type / file / directory involved
3. Evidence: method count, file count, decl groups, etc.
4. Remediation: be concrete and direct. For Receiver Monolith,
   propose specific subpackage names with the methods that move
   into each, name the callers that must be updated, and state
   that the original methods will be deleted (not retained as
   facades). Do not propose phased migrations, accessor patterns,
   or compatibility shims unless the user has explicitly asked
   for a transition window.

End with a remediation order recommendation (which smell to fix first
and why), and a one-line statement of the expected end state ("after
this refactor, `*TypeDB` no longer exists / is reduced to N fields
and M methods, and every caller imports the relevant subpackage
directly").

## What NOT to Flag

- A `Conn` struct with many low-level methods (`Read`, `Write`, `Close`, `Stat`...) — methods are at the same abstraction level. Not a monolith.
- `package main` with many files — entry-point packages are exempt.
- `internal/<single-file>` packages that exist for visibility boundary, not for grouping (legitimate Go pattern).
- Generated files (`gen/`, `*_gen.go`, `*.pb.go`).
- Test files (`*_test.go`) — different rules.
- A `Deps` struct in a `transport`/`server` aggregator with many fields — that's its job IF the fields are all transport-layer concerns. Flag only if it crosses domains (auth + storage + events + analytics + ...).

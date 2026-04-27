---
id: G1E
name: Foreign Holder
status: planned
severity: HIGH (and CRITICAL above thresholds)
detector_type: structural (cross-package)
---

# G1E — Foreign Holder

A consumer-side detector for the **Reach-Through Holder** pattern
(see `skill/SKILL.md` Anti-Patterns and `docs/patterns/g1d-hidden-holder.md`
for the larger family).

## Why

The four existing receiver-monolith detectors (G1, G1B, G1C, G1D)
inspect the **producer** package — the package that defines the god
type. They cannot catch the disguise where the producer package is
genuinely clean (sub-services in subpackages, holder has typed
cross-package fields, no aliases, no registries) but **every consumer
still takes the holder and reaches in via accessor functions**:

```go
// typedbstore — clean, all G1* detectors return zero findings.
type TypeDB struct {
    conn   *typedbconn.Conn
    nodes  *nodestore.Mutator
    search *searchstore.Searcher
    backup *Backup
}
func SearchOps(t *TypeDB) *searchstore.Searcher { return t.search }
func BackupOps(t *TypeDB) *Backup               { return t.backup }
```

```go
// service/projectbackup — but here:
func restoreGraphBundle(ctx context.Context, tdb *typedbstore.TypeDB, ...) error {
    if err := typedbstore.BackupOps(tdb).InsertNodesWithDisplayIDs(...); err != nil { ... }
    if err := typedbstore.BackupOps(tdb).InsertEdgesWithAttributes(...); err != nil { ... }
    if err := typedbstore.BackupOps(tdb).InsertPendingPromotions(...); err != nil { ... }
}
```

The holder is back to being the chokepoint at every call site.
Currently the only test that catches this is the `grep` in the
verification checklist; reviewers must remember to run it. A
detector turns it into a CI gate.

## Detection signal

For each non-test package `Q` in the workspace, scan every function,
method receiver, and exported struct field. For each occurrence of a
type `*T` (where `T` is defined in some other package `P`), report
**G1E** if all of the following hold:

1. **`*T` looks like a holder.** Either:
   - `T` has ≥3 fields whose types are pointers to named types
     defined in subpackages of `P` (paths beginning with `P/`), OR
   - Package `P` exports ≥3 functions of shape
     `func Name(t *T) *X { ... }` where `*X` is a named type from
     a subpackage of `P`. (These are the "accessors" pattern that
     `BackupOps`, `SearchOps`, etc. follow.)
2. **`Q != P`.** Use within `P` is fine; the smell is escape into
   foreign packages.
3. **`Q` is not a test fixture.** Skip `*_test.go`, `testutil/`,
   `testdata/`, `testing/`, and `Fake*`/`Mock*`/`Stub*`/`Spy*` types.
4. **The use is in a parameter, struct field, or return type.** A
   local variable that shadows a constructor return is fine; the
   smell is the holder appearing in a _signature_ a downstream
   caller has to satisfy.

The detector emits one finding per `(holder type, foreign package)`
pair, listing all sites in that package.

## Severity

| Severity | Condition                                                          |
| -------- | ------------------------------------------------------------------ |
| CRITICAL | Holder appears in ≥5 foreign-package signatures across ≥3 packages |
| HIGH     | Holder appears in ≥1 foreign-package signature                     |

## Negative cases (must NOT fire)

- **Test code.** Test fixtures legitimately take the holder so they
  can construct sub-services for the system under test. Filter on
  filename and on test-double naming.
- **Internal use within `P`.** Construction, teardown, and
  package-level wiring inside `typedbstore/` itself is fine.
- **Types whose fields aren't sibling-subpackage types.** A `*sql.DB`
  has many fields, but they're not pointers to types in `database/sql/foo/`.
  The holder heuristic must look for the cross-package field pattern,
  not just "has many fields".
- **Single-method consumers.** A handler that takes `*T` and calls
  exactly one method on it is borderline; we may want to allow this
  in v1 and tighten later. Initial implementation: fire on any
  occurrence; rely on severity tiers to surface the worst cases.
- **Constructors.** A function whose name matches `^(New|Open|Make|Build|Create)<T>` and which returns `*T` is exempt — it's the
  legitimate construction site.

## Implementation sketch

New file `foreign_holder.go`. Add `scanForeignHolders(pkgs)` to the
`scanReceivers` aggregator in `receivers.go`:

```go
func scanReceivers(pkgs []*packages.Package) []Finding {
    var findings []Finding
    for _, pkg := range pkgs {
        // ... existing per-package scans ...
        findings = append(findings, scanHiddenHolders(pkg)...)
    }
    findings = append(findings, scanForeignHolders(pkgs)...)
    return findings
}
```

Pass 1: identify holder candidates per producer package.
Pass 2: scan all non-producer packages for signatures referencing
each holder.

Two-pass design avoids O(packages²) lookups.

```go
type holderCandidate struct {
    pkg  string  // producer package path
    name string  // type name, e.g. "TypeDB"
}

func collectHolderCandidates(pkgs []*packages.Package) []holderCandidate { ... }
func scanForeignReferences(pkgs []*packages.Package, candidates []holderCandidate) []Finding { ... }
```

`collectHolderCandidates` reuses helpers from `scanAggregateHolders`
and `scanHiddenHolders` for the cross-package field count and the
accessor-function count.

`scanForeignReferences` walks each package's syntax tree, examines
function signatures and struct fields, and resolves type identity
through `pkg.TypesInfo`.

## Tests

`foreign_holder_test.go`, all using `fakeModule`:

1. **TestG1E_HolderInForeignSignature_Fires** — producer package
   `store` defines `*Store` with three sibling subpackage fields;
   foreign package `app` has `func handler(s *store.Store)`. Fires.
2. **TestG1E_HolderInForeignStructField_Fires** — foreign package
   has `type Deps struct { Store *store.Store }`. Fires.
3. **TestG1E_AccessorPattern_Fires** — producer has no struct fields
   but exports three `func Foo(s *Store) *foo.Service` accessors.
   Foreign package takes `*Store`. Fires (the accessor heuristic).
4. **TestG1E_OnlyInProducerPkg_NoFire** — `*Store` appears only in
   `store/`'s own files. No fire.
5. **TestG1E_TestFixture_NoFire** — `*Store` appears in
   `testutil/fixtures.go`. No fire.
6. **TestG1E_RegularType_NoFire** — `*Conn` (no sibling-subpackage
   fields, no accessor pattern) appears in many foreign packages.
   No fire.
7. **TestG1E_Constructor_NoFire** — `func NewStore(...) *Store` in a
   foreign package (a builder library) does not fire on the return
   type alone.

## Documentation updates

- `docs/patterns/g1e-foreign-holder.md` — full pattern doc following
  the same structure as the other `gN-*.md` files.
- `docs/patterns/patterns.md` — add entry to the "big four (always
  fix first)" group; rename to "the big five".
- `README.md` — add G1E row to the smell catalog table; add
  description block in the "headline detector" section.
- `doc.go` — add G1E to the package-godoc smell list.
- `CHANGELOG.md` — entry under `[Unreleased]`.
- `skill/SKILL.md` — promote the existing "Reach-Through Holder"
  block from the "no detector" section into the G1E entry; update
  the catalog table to give it an ID; update verification
  checklist step 2 to include G1E.
- `roadmap/roadmap.md` catalog — remove this row.
- Delete `roadmap/g1e-foreign-holder.md` at merge.

## Definition of Done

- `lagotto monoliths --format=json /path/to/auto-k-server/internal`
  reports a G1E finding for `*typedbstore.TypeDB` referencing the
  three known production sites (`service/projectbackup/service.go:728,762`
  and `cmd/server/project_inspection_runtime.go:192`).
- All seven test cases above pass under `go test -race -count=1`.
- `./check.sh` is green (build, vet, lint, race tests, self-audit).
- Self-audit (`lagotto audit .` against the lagotto repo itself)
  remains zero findings.
- Pattern doc, smell catalog, CHANGELOG, and SKILL updates landed in
  the same commit as the detector code.

## Open questions

1. **Tightness of the holder-candidate heuristic.** Should we
   require _both_ sibling-subpackage fields _and_ the accessor
   pattern, or _either_? Initial implementation: either. Tighten if
   false positives appear.
2. **Allowed foreign-package list.** Some refactors legitimately
   need a brief transitional period where one wrapper package
   takes the holder. Consider a `--allow-holder-in=<pkgPath>`
   flag for this. Defer to v0.2 unless it blocks adoption.
3. **Constructor exemption.** A function returning `*T` is likely a
   constructor, but `*T` may also legitimately appear in return
   types of test helpers. Exempt only the construction-name pattern
   (`^(New|Open|Make|Build|Create)<T>`); flag bare returns of `*T`
   from non-constructor-named functions.

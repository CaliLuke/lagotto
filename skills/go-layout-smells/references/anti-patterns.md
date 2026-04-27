# Anti-Patterns: How Agents Fake A Receiver Decomposition

Once an agent knows lagotto flags Receiver Monolith, the cheapest way
to make the warning disappear is to leave the implementation
unchanged and rearrange names until the detector goes quiet. The
patterns below recur. All of them are worse than the original — they
preserve the god type while signalling decomposition where there is
none, which misleads every future reader. Always verify against the
**effective method set** (what callers actually see), not against
source-AST receiver names.

## G1B — Decomposition Theatre (alias cluster)

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

## G1C — Aggregate Holder

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

## Embedding Theatre

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

## G1D — Hidden Holder via Registry

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

## Reach-Through Holder (no detector — caller-view test only)

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
that catches it is the caller-view grep (replace `<GodType>` with
the actual type name, e.g. `TypeDB`):

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

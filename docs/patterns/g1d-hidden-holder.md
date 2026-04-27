# G1D — Hidden Holder via Registry

A package keeps a "thin" holder type (no methods of its own, one or two
fields) but reconstructs the holder's API surface via package-level
`sync.Map` registries keyed by the holder's pointer, exposed through
exported accessor functions of the form
`func Foo(h *Holder) *Sub { return loadFromMap(&fooReg, h) }`.

Every caller still receives `*Holder` everywhere; the receiver split
is cosmetic.

## Why this matters

This is the third disguise an agent reaches for after
[G1B](g1b-decomposition-theatre.md) (alias clusters) and
[G1C](g1c-aggregate-holder.md) (aggregate holders) stop fooling the
auditor:

1. `*God` has too many methods → flag.
2. Rename receivers via aliases → G1B flag.
3. Make sub-services real types but hold them on a god struct → G1C
   flag.
4. Make the god struct empty, move the sub-services into package-level
   `sync.Map` registries keyed by `*God`, expose via accessor
   functions → looks "narrow" but caller signatures are unchanged.

Each iteration preserves the original problem (the holder is the
chokepoint every consumer takes) while finding a structural shape
that the previous detector didn't catch. G1D is the rule that
catches the registry shape; future evasions will need new rules.

The deeper lesson: lagotto is good at flagging _visible_ god
patterns. It cannot infer intent. A spec that demands "the god type
disappears from production caller signatures" is a stronger gate
than any single structural metric — see the SKILL's "Spirit, not
letter" section.

## What lagotto checks

The detector fires when a single package contains:

1. **≥3 registry-shaped vars at package level.** "Registry-shaped"
   means either a `sync.Map` or a Go map whose key type is a
   pointer.
2. **≥5 exported package-level functions** whose first parameter is
   a pointer to a same-package struct `H`.
3. The holder `H` has **≤2 of its own methods.** If `H` has a real
   method set, [G1](g1-receiver-monolith.md) owns the finding; G1D
   only fires on the _thin_ holder shape.
4. Test doubles (`Fake*`, `Mock*`, …) and `testutil` packages are
   skipped (same filter as G1).

| Severity | Condition                         |
| -------- | --------------------------------- |
| CRITICAL | ≥7 accessors or ≥6 registry vars  |
| HIGH     | ≥5 accessors and ≥3 registry vars |

## Positive example (fires)

```go
package store

import "sync"

// "Thin" holder — no methods, one field.
type DB struct{ conn *Conn }

// Sub-service types in the same package (or, equivalently, a
// subpackage that's still keyed by *DB).
type NodeOps struct{}
type EdgeOps struct{}
type SearchOps struct{}
type ThreadOps struct{}
type PromotionOps struct{}

var (
    nodeReg      sync.Map // map[*DB]*NodeOps
    edgeReg      sync.Map // map[*DB]*EdgeOps
    searchReg    sync.Map // map[*DB]*SearchOps
    threadReg    sync.Map // map[*DB]*ThreadOps
    promotionReg sync.Map // map[*DB]*PromotionOps
)

func Nodes(db *DB) *NodeOps { v, _ := nodeReg.Load(db); return v.(*NodeOps) }
func Edges(db *DB) *EdgeOps { v, _ := edgeReg.Load(db); return v.(*EdgeOps) }
func Search(db *DB) *SearchOps { v, _ := searchReg.Load(db); return v.(*SearchOps) }
func Threads(db *DB) *ThreadOps { v, _ := threadReg.Load(db); return v.(*ThreadOps) }
func Promotions(db *DB) *PromotionOps { v, _ := promotionReg.Load(db); return v.(*PromotionOps) }
```

`*DB` is "narrow" only on paper. Every caller still takes `*DB` and
the package-level functions are equivalent to methods. The
registries are doing the job that struct fields would have done,
but invisibly.

## Negative example (does NOT fire)

```go
package store

import "sync"

type DB struct{
    conn       *Conn
    Nodes      *nodes.Mutator
    Edges      *edges.Mutator
    Search     *search.Searcher
    Threads    *threads.Threads
    Promotions *promotions.Promotions
}

// One legitimate cache; not a "registry" in the smell's sense.
var connCache sync.Map
```

Sub-services are typed fields on the holder, with the field types
living in _other_ packages. Callers can take a `*nodes.Mutator`
directly and skip `*DB` entirely. G1C does not fire (cross-package
fields are exempt) and G1D does not fire (only one `sync.Map`).

This is the target shape after a real receiver decomposition.

## How to fix it

Replace the registries with typed fields on the holder, where the
field types live in subpackages. Then update callers to take only
the narrow service they need:

```go
// Step 1: typed fields on the holder.
type DB struct{
    conn       *Conn
    Nodes      *nodes.Mutator
    // ...
}

// Step 2: callers take the narrow type, not the holder.
func handler(m *nodes.Mutator) { m.CreateNodes(...) }
```

The constructor returns `*DB` once at startup; every other
production code path takes the specific sub-service. Delete the
package-level registries and accessor functions in the same change.

If the goal of the registry was to keep the holder "small" for
mocking or testing, the cleaner answer is a per-concern interface
(narrow `nodes.Mutator` or `nodes.MutatorIface`) that test fixtures
implement directly.

## Related

- [G1 Receiver Monolith](g1-receiver-monolith.md) — the original
  problem.
- [G1B Decomposition Theatre](g1b-decomposition-theatre.md) — first
  evasion.
- [G1C Aggregate Holder](g1c-aggregate-holder.md) — second evasion.
- The SKILL's "Spirit, not letter" guidance — the systemic answer
  for stopping the next evasion before the detector has to.

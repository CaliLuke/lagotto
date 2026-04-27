# G1C — Aggregate Holder

A struct aggregates five or more sub-services from the same package
on named pointer fields, with a combined method count of 25 or more.
Callers still pass one holder handle around and reach into it
(`t.Nodes.CreateNodes(...)`, `t.Search.SearchGraph(...)`), so the
sub-services have not actually been split — they have been reshuffled
inside the same package.

## Why this matters in Go specifically

When [G1B](g1b-decomposition-theatre.md) is fixed (aliases replaced
by real types), the next disguise is to declare named struct types
per concern but keep them all in the same directory and present them
to callers via a holder:

```go
type TypeDB struct {
    conn       *Conn
    Nodes      *Mutator
    Edges      *EdgeMutator
    Search     *Searcher
    Threads    *Threads
    Promotions *Promotions
}
```

`*Mutator`, `*Searcher`, etc. are distinct types now — but every
caller still receives one `*TypeDB` and the import graph is
unchanged. The decomposition isn't real until each sub-service moves
into its own subpackage and callers take only the narrow service
they need.

## What lagotto checks

For every named struct in each package, the detector inspects fields
whose type is a pointer to a same-package named struct. For each such
field it looks up the pointee's effective method set via
`types.NewMethodSet` and sums the counts.

A finding fires when the holder has ≥5 such fields and the total
pointee method count is ≥25.

Cross-package fields (`*othersub.Service`) are deliberately ignored:
that shape IS the target end state.

| Severity | Condition                                 |
| -------- | ----------------------------------------- |
| CRITICAL | ≥50 pointee methods or ≥7 sub-services    |
| HIGH     | 5–6 sub-services with ≥25 pointee methods |

## Positive example (fires)

```go
// All in package typedbstore.
type Mutator    struct{ conn *Conn }
type Searcher   struct{ conn *Conn }
type Threads    struct{ conn *Conn }
type CheckRunner struct{ conn *Conn }
type Promotions struct{ conn *Conn }

// ... 30+ methods across these types ...

type TypeDB struct {
    conn       *Conn
    Nodes      *Mutator
    Search     *Searcher
    Threads    *Threads
    Checks     *CheckRunner
    Promotions *Promotions
}
```

Every `*TypeDB` consumer reaches `t.Search.SearchGraph(...)`. Adding
or removing a sub-service still ripples through every consumer.

## Negative example (does NOT fire)

```go
// pkg/holder
package holder

import (
    "example.com/pkg/nodes"
    "example.com/pkg/search"
    "example.com/pkg/threads"
)

type Holder struct {
    Nodes  *nodes.Mutator
    Search *search.Searcher
    Threads *threads.Threads
}
```

The fields point at types in other packages. Each consumer can
import the specific subpackage it needs and skip the holder entirely.
This is the legitimate end state of a receiver decomposition.

## How to fix it

1. **Move each sub-service struct into its own subpackage.** The
   subpackage owns the type and its methods. Connection state and
   any other shared dependencies are passed in at construction.
2. **Decide whether the holder needs to exist at all.** If callers
   only use it as a way to reach sub-services, delete it. If there's
   genuine setup logic (constructing the connection once, registering
   metrics) move that into a small construction helper that returns
   the sub-services as separate values.
3. **Update every caller** in the same change. Callers import the
   specific subpackage they need; no caller takes the holder.

```go
// Before
func handler(t *typedbstore.TypeDB) {
    nodes := t.Nodes.CreateNodes(...)
    results := t.Search.SearchGraph(...)
}

// After
func handler(m *nodes.Mutator, s *search.Searcher) {
    nodes := m.CreateNodes(...)
    results := s.SearchGraph(...)
}
```

## Related

- [G1 — Receiver Monolith](g1-receiver-monolith.md): the original
  problem.
- [G1B — Decomposition Theatre](g1b-decomposition-theatre.md): the
  earlier-stage disguise.
- [G4 — God Dependency Bag](g4-god-dependency-bag.md): a similar
  smell at the dependency-injection layer.

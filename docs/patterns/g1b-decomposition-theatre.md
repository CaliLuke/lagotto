# G1B — Decomposition Theatre

A package contains three or more type aliases that all resolve to the
same underlying named struct. The type system collapses the aliases:
every alias inherits the same method set on the same struct, so the
receiver remains a monolith no matter how many names point at it.

## Why this matters in Go specifically

Go 1.22+ models type aliases as `*types.Alias` in `go/types`, but the
language semantics are unchanged: a type alias is the same type as
its target. Methods defined on the target are reachable through every
alias name. So a "decomposition" that introduces alias names without
moving any methods is purely cosmetic.

This is the disguise an agent reaches for when [G1](g1-receiver-monolith.md)
fires: rename receivers from `*God` to `*Mutator`, `*Searcher`, etc.,
then declare `type Mutator = God`, `type Searcher = God`, … so a
source-AST receiver-name counter sees small per-receiver counts.

The detector defeats this by counting via `types.NewMethodSet` on the
underlying type, which sees through aliasing — and by also flagging
the alias cluster directly.

## What lagotto checks

For each package's scope, it lists every `*types.TypeName` whose
`IsAlias()` returns true, resolves the target via `types.Unalias`,
and groups aliases by target. A cluster of 3+ aliases pointing at a
single same-package named type fires.

Cross-package re-export aliases (`type A = otherpkg.Real`) do not
fire — that's a legitimate compatibility shape.

| Severity | Condition   |
| -------- | ----------- |
| CRITICAL | ≥6 aliases  |
| HIGH     | 3–5 aliases |

## Positive example (fires)

```go
package typedbstore

type ops struct {
    conn *Conn
}

type Mutator     = ops
type EdgeMutator = ops
type Searcher    = ops
type Threads     = ops
type Promotions  = ops
type CheckRunner = ops
```

Six aliases, all the same struct. A caller writing `mutator.CreateNodes(...)`
and `searcher.SearchGraph(...)` is calling the same struct through
two names. The "decomposition" buys nothing.

## Negative example (does NOT fire)

```go
package compat

import "example.com/v2/types"

// Re-export the v2 names under v1 paths during a migration.
type Foo = types.Foo
type Bar = types.Bar
type Baz = types.Baz
```

Cross-package aliases. The detector skips these — re-export tunnels
have a separate smell ([G8](g8-internal-re-export-tunnel.md)) but
this specific shape is fine when there's a real migration.

## How to fix it

Replace each alias with a real distinct struct in its own subpackage,
holding only the state it needs. The shared underlying type
(`ops` in the example) is deleted; aliasing it under multiple names
does not split the monolith.

```go
// pkg/nodes/mutator.go
package nodes

type Mutator struct{ conn *conn.Conn }
func (m *Mutator) CreateNodes(...) {...}

// pkg/search/searcher.go
package search

type Searcher struct{ conn *conn.Conn }
func (s *Searcher) SearchGraph(...) {...}
```

Now `*Mutator` and `*Searcher` are genuinely different types, each
with a focused surface. Callers update to take the narrow type they
need.

## Related

- [G1 — Receiver Monolith](g1-receiver-monolith.md): the smell this
  pattern disguises.
- [G1C — Aggregate Holder](g1c-aggregate-holder.md): the next
  disguise after aliases stop fooling the linter.

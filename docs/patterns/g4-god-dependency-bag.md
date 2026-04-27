# G4 — God Dependency Bag

A `Deps` / `Container` / `Services` struct has eight or more fields
drawing from five or more distinct external packages. The struct is
acting as a god-injection-point: every consumer takes the whole bag
and reaches into the parts it cares about. The blast radius of
adding a dependency or moving one to a new home is the entire
codebase.

## Why this matters

Dependency injection in Go is usually constructor-based: a function
takes the things it needs as named parameters or a small struct.
When a single struct accumulates everything everyone might need, it
becomes the wiring chokepoint:

- Any test of any consumer has to populate the whole bag (often via
  test doubles for fields the test doesn't use) just to construct
  the consumer.
- Adding a new dependency forces ripple changes through every call
  site that constructs the bag.
- The dependency graph becomes opaque: the import graph says every
  consumer depends on every domain, even when the actual code path
  only touches one or two.

The fix is to split the bag by concern: `AuthDeps`, `GraphDeps`,
`StorageDeps`, etc. Each consumer takes only the bag it needs.

## What lagotto checks

It looks for struct types whose names match a known dependency-bag
pattern (`Deps`, `Dependencies`, `Container`, `Services`, `App`,
`Bag`). For each match, it counts fields and the distinct external
packages those fields' types come from (after stripping pointer,
slice, array, and map wrappers). Local types and the struct's own
package are filtered out — the smell is about cross-domain mixing.

A finding fires when the struct has ≥8 fields drawing from ≥5
distinct external packages.

| Severity | Condition                          |
| -------- | ---------------------------------- |
| CRITICAL | ≥12 fields or ≥8 distinct packages |
| HIGH     | 8–11 fields, 5–7 distinct packages |

## Positive example (fires)

```go
package transport

import (
    "example.com/auth"
    "example.com/events"
    "example.com/graph"
    "example.com/storage"
    "example.com/comments"
    "example.com/search"
    "example.com/source"
    "example.com/version"
    "example.com/project"
    "example.com/audit"
)

type Deps struct {
    Auth     *auth.Service
    Events   *events.Bus
    Graph    *graph.Store
    Storage  *storage.DB
    Comments *comments.Service
    Search   *search.Index
    Source   *source.Service
    Version  *version.Store
    Project  *project.Service
    Audit    *audit.Logger
}
```

Ten fields from ten domains. Every transport handler that takes
`Deps` is reaching into a different subset; nobody uses the whole
thing.

## Negative example (does NOT fire)

```go
package transport

import "example.com/transport/handlers"

type Deps struct {
    Auth      *handlers.AuthHandler
    Health    *handlers.HealthHandler
    Routes    *handlers.RouteRegistry
    Static    *handlers.StaticServer
    Sessions  *handlers.SessionStore
    Templates *handlers.TemplateRenderer
    CSRF      *handlers.CSRFGuard
    Cache     *handlers.CacheControl
    Metrics   *handlers.MetricsCollector
}
```

Nine fields, but they all draw from one package and represent one
layer's wiring — exactly what a transport-aggregator struct is for.
The detector skips this because the distinct-packages count is 1,
under the 5 threshold.

## How to fix it

1. **Cluster the fields by domain.** Auth + sessions + CSRF go
   together; storage + caches + DB connections go together; events
   - analytics go together. Aim for clusters of 2–4 fields.
2. **Define a per-cluster bag**: `AuthDeps`, `StorageDeps`,
   `EventsDeps`. Move the fields into the appropriate bag.
3. **Update each consumer** to take only the bag it actually
   touches.
4. **Delete the original `Deps`.** Don't retain a wrapper that holds
   the smaller bags — that recreates the god type at the next layer.

```go
// After
type AuthDeps struct {
    Auth     *auth.Service
    Sessions *handlers.SessionStore
    CSRF     *handlers.CSRFGuard
}

type StorageDeps struct {
    DB    *storage.DB
    Cache *cache.Layer
}

func (h *AuthHandler) Authenticate(d AuthDeps) {...}
```

## Related

- [G1C — Aggregate Holder](g1c-aggregate-holder.md): the same shape
  at the type-decomposition layer.

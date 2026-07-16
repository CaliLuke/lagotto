# G8 — Internal Re-Export Tunnel

A package's exported surface is dominated by re-exports from a deeper package.
Its types are aliases (`type Foo = inner.Foo`), its variables are
re-bindings (`var Default = inner.Default`), and its functions are
transparent wrappers (`func Hello() string { return inner.Hello() }`).

## Why this matters

This is the TypeScript "barrel" pattern, where index files re-export
everything to flatten import paths. In Go, it's structural noise:

- The package adds a layer to the import graph with no logic.
- Every change to the inner package needs a corresponding change in
  the tunnel.
- Readers see `outer.Foo` and have to chase the alias to learn it's
  really `inner.Foo`.

Go's preferred answer to "I want a flatter import path" is to move
the inner package up, not to wrap it.

## What lagotto checks

For each package, the detector counts how much of the package's
exported surface is a tunnel:

- type aliases that resolve to a named type in another package
- variables initialized to values from another package
- functions whose body directly calls a same-named function in
  another package and forwards the wrapper's parameters unchanged

It fires when at least half of exported declarations are re-exports and one
target accounts for at least half of those re-exports. At 50–79%, remediation
keeps genuine local declarations; at ≥80%, deleting the tunnel is appropriate.

| Severity | Condition                                             |
| -------- | ----------------------------------------------------- |
| HIGH     | ≥80% of declarations are re-exports                    |
| MEDIUM   | 50–79% of declarations are re-exports                  |

## Positive example (fires)

```go
package outer

import "example.com/v2/inner"

type Foo = inner.Foo
type Bar = inner.Bar

var Default = inner.Default

func Hello() string { return inner.Hello() }
func World() string { return inner.World() }
```

Every declaration is a forward to `inner`. The `outer` package adds
nothing.

## Negative example (does NOT fire)

```go
package billing

import "example.com/money"

type Invoice struct {
    Total money.Amount
}

func (i Invoice) IsOverdue() bool { /* logic */ }
```

The package re-uses types from `money`, but it has its own logic
(method on its own type). Not a tunnel.

Generated service helpers that return framework types are also not
tunnels when they add service-specific identity:

```go
func MakeInvalidRequest(err error) *runtime.ServiceError {
    return runtime.NewServiceError(err, "invalid_request")
}
```

That function uses `runtime`, but it does not re-export
`runtime.NewServiceError`.

## How to fix it

The right answer depends on why the tunnel exists.

- **Compatibility shim during migration**: keep it, but set a
  deletion deadline. Once consumers have migrated to the new path,
  delete the shim.
- **"Flatter import paths"** (TypeScript instinct): move the inner
  package's contents up. The tunnel disappears.
- **Selective re-exports** (only some inner names should be public):
  keep the inner package private (`internal/`) and accept the
  re-exports as the public API. Document them with godoc so they
  don't read as a barrel.

## Related

Type aliases at scale within a single package signal
[G1B](g1b-decomposition-theatre.md), not G8. G8 is specifically
about cross-package re-export.

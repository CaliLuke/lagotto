# G6 — Facade Method

A method's body is a thin pass-through to a function in another
package. The method adds no logic, no value translation, no error
wrapping — it just forwards arguments and returns the result.

## Why this matters

Facade methods double the API surface for no benefit:

- Readers see `t.Migrate(...)` and have to follow the indirection
  to discover that the real implementation is `migrate.Run(...)`.
- The method's existence implies it does something on the receiver;
  it doesn't.
- Removing the method is a breaking change, but keeping it
  perpetuates the illusion that the receiver type owns the
  capability.

Facade methods often appear during a partial type decomposition: the
old god type is being split, but its existing methods are kept as
thin wrappers around the new subpackages "for compatibility". That
compatibility layer becomes permanent and the readers of the next
generation pay the cost.

## What lagotto checks

For each method declaration in the workspace, the detector checks
whether the body spans at most three source lines and ends in a single
cross-package call, with any prefix statements limited to trivial
setup (assignment, nil-guard, declaration). If so, it records the
target package and method, and emits a finding.

Type conversions are not calls and are excluded. Methods that bind receiver
state, dispatch through an embedded external interface, or wrap a standard
library boundary are reported at LOW with context-aware guidance because they
may be load-bearing adapters or test seams.

In-package calls are skipped — the smell is specifically about
methods that exist only to bridge a package boundary.

| Severity | Condition                                |
| -------- | ---------------------------------------- |
| MEDIUM   | any thin pass-through to another package |
| LOW      | state/interface/standard-library boundary |

## Positive example (fires)

```go
package typedbstore

import "example.com/migrate"

func (t *TypeDBManager) RunAllTypeDBMigrations(ctx context.Context, dryRun bool) error {
    return migrate.Run(ctx, t.cfg, dryRun)
}

func (t *TypeDBManager) DiagnoseTypeDBMigration(ctx context.Context, name string) (*migrate.DoctorResult, error) {
    return migrate.Diagnose(ctx, t.cfg, name)
}
```

Both methods exist only to forward to `migrate`. Callers writing
`mgr.RunAllTypeDBMigrations(...)` could just as easily write
`migrate.Run(...)` directly.

## Negative example (does NOT fire)

```go
func (s *Service) Authenticate(token string) (*User, error) {
    if token == "" {
        return nil, ErrMissingToken
    }
    decoded, err := jwt.Decode(token)
    if err != nil {
        return nil, fmt.Errorf("decode: %w", err)
    }
    return s.lookupUser(decoded.Subject)
}
```

The method has substantive logic (validation, decoding, error
wrapping, in-package follow-up). Even though it crosses a package
boundary in the middle, it's not a thin pass-through.

Also treated conservatively:

- Methods on types that satisfy an external interface contract
  (e.g., `(c *Conn) Read(p []byte) (int, error)` that calls
  `bufio.Reader.Read`). The interface forces the method to exist;
  removing it would break the contract. lagotto can't tell from the
  embedded external interfaces are detected and downgraded.
- Methods whose body is `return fmt.Sprintf(...)` for an `Error()`
  method on a custom error type. Same reason.

## How to fix it

Inline the call at the call sites. `gopls`'s "inline call"
refactoring handles this: every consumer of `t.RunAllTypeDBMigrations(...)`
becomes `migrate.Run(...)`. Then delete the method.

If the consumer count is large, the refactor is mechanical but
high-touch. That's the cost of having added the facade in the first
place — the cleanup matches the original spread.

## Related

This smell often appears in the wake of a [G1 Receiver
Monolith](g1-receiver-monolith.md) decomposition: methods kept on
the old type as wrappers to "ease migration" turn into permanent
facade methods. Better to update callers in the same change as the
decomposition itself.

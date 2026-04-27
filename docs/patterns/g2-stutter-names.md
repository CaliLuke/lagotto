# G2 — Stutter Names

Exported types or functions in a package repeat the package name in
their identifier (`lanes.LaneConfig`, `events.EventBus`,
`tasks.TaskService`). Go style is `lanes.Config`, `events.Bus`,
`tasks.Service` — the import path provides the namespace, so naming
the type after its package is redundant.

## Why this matters

Stutter names happen most often when a package is extracted from a
monolithic file: the existing identifier carries its old fully-
qualified name forward, even though the new package qualifier makes
half the name redundant. Readers see `tasks.TaskService` and have to
mentally strip the prefix to get to the actual concept; over time
the codebase reads like a stutter chain.

The Go style guide and Go Code Review Comments are explicit that
package qualifiers should not be repeated in identifiers. lagotto
flags any package where the rule is violated.

## What lagotto checks

For each loaded package, it iterates exported types and exported
functions (skipping methods) and checks whether the identifier name
starts with the package name as a CamelCase prefix. The check is
identifier-aware: `lanes.LaneConfig` matches because `Lane` is the
package name's leading word, but `lanes.Landfall` does not because
`Lan` doesn't match a complete word boundary.

Singular/plural tolerance allows `lanes` to match `Lane` (drop the
trailing `s`).

The detector emits one finding per offending package, with the count
and an `offenders` map split by `type` and `func`.

| Severity | Condition     |
| -------- | ------------- |
| MEDIUM   | any offenders |

## Positive example (fires)

```go
package lanes

type LaneConfig struct{} // → Config
type LaneState int       // → State
func LaneNew() *LaneConfig { return nil } // → New
```

## Negative example (does NOT fire)

```go
package lanes

type Config struct{}
type State int
func New() *Config { return nil }
```

Same package, clean identifiers. `lanes.Config` reads naturally.

## How to fix it

`gopls`'s rename refactoring is the safest path: rename
`lanes.LaneConfig` to `lanes.Config`, then let gopls update every
caller. Repeat for each offender; commit as one mechanical change.

If a name truly is the "main type" of the package (the constructor's
return type, not one of many siblings), consider whether the package
itself is well-named. `lanes.Lane` is awkward; the type is probably
just `lanes.Lane` and the package name is misleading — but
`schedule.Lane` would be clean.

## Related

This smell often appears alongside [G12](g12-premature-package.md):
a package containing one type with a stuttering name is usually a
thin wrapper that should either grow into a real package or get
inlined into a parent.

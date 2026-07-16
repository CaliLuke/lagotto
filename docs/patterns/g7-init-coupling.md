# G7 — Init Coupling

A package has multiple `init()` functions spread across two or more files,
and one reads package-level state that another writes. The resulting behavior
depends on cross-file initialization order, which is invisible at call sites.

## Why this matters

`init()` is implicit setup. Within one file, the order is the source
order — easy to read. Across files, the order is alphabetical
filename order, which means renaming `auth.go` to `bootstrap_auth.go`
silently changes when its `init()` runs. If anything in another
file's `init()` depends on that ordering, you've introduced a
non-obvious failure mode.

## What lagotto checks

For each loaded package, lagotto records package-level variables read and
written by every `init()`. It fires only when a write in one file feeds a read
in another. Independent codec/driver registration inits do not fire.

| Severity | Condition                      |
| -------- | ------------------------------ |
| MEDIUM   | ≥3 inits or ≥3 files           |
| LOW      | exactly 2 inits across 2 files |

## Positive example (fires)

```go
// auth.go
package server
var handlersReady bool
func init() { handlersReady = true }
```

```go
// metrics.go
package server
func init() { if handlersReady { registerMetrics() } }
```

The second init reads state written by the first, so their ordering matters.

## Negative example (does NOT fire)

```go
// init.go
package server

func init() {
    registerAuthHandlers()
    registerMetrics()
    setupLogging()
}
```

One file, three inits in source order. Or even simpler: one `init()`
that does everything in a deterministic order.

## How to fix it

1. **Consolidate to one `init()`** in one file when the setup is
   small.
2. **Replace `init()` with explicit setup** called from `main()`
   when ordering matters. An explicit `Initialize()` function makes
   the dependency order a feature of the call site, not a function
   of file naming.

```go
func main() {
    auth.Register()
    metrics.Register()
    logging.Setup()
    // …
}
```

## Related

Init coupling and [G12 Premature Package](g12-premature-package.md)
sometimes co-occur: a package with one file containing only an
`init()` function is doing global side effects from a hidden corner.

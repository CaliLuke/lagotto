# G12 — Premature Package

A directory contains exactly one source file (excluding tests, generated code,
and `doc.go`). The package boundary is providing visibility, not
grouping — and the cost of an extra import path may exceed what
that visibility buys.

## Why this matters in Go specifically

In Go, packages are the unit of encapsulation: a name is exported by
its case and a file's reach stops at its package. Creating a
subpackage for one file means:

- the import path grows
- callers need an extra import line
- the file's exported names form the entire public surface of the
  subpackage, with no siblings to share helpers with

That's worth it when the file legitimately needs to hide
implementation from the parent package. It's not worth it when the
parent could just hold the file directly.

This smell is **LOW** by design — single-file packages are common
and often legitimate. lagotto flags them so reviewers can decide;
many findings will be intentional.

## What lagotto checks

For each directory under the audit root, it counts non-test source
files. A directory with exactly one source file fires, except:

- `doc.go`-only directories (legitimate godoc home)
- The audit root itself (a tool's `main` package is often one file)
- `package main` directories (canonical command entry points)
- generated files with the standard Go generated-code header

| Severity | Condition                              |
| -------- | -------------------------------------- |
| LOW      | exactly one source file (not `doc.go`) |

## Positive example (fires)

```text
internal/
├── slugify/
│   └── slugify.go    // 30 lines, one function
└── graph.go
```

`slugify` is a one-file package. Worth asking: does
`internal.Slugify(s)` work just as well?

## Negative example (does NOT fire)

```text
internal/
├── slugify/
│   ├── slugify.go
│   └── slugify_test.go
├── conn/
│   ├── conn.go
│   └── pool.go
└── graph.go
```

`slugify` has tests; lagotto excludes those from the count and the
package looks like a one-source-file package, but if the file
exposes a clean API and the tests are non-trivial, leave it. `conn`
has multiple source files and is fine.

## How to fix it (when you decide it's wrong)

Two paths:

1. **Inline up.** If the visibility boundary doesn't matter, move
   the file into the parent package. One fewer import path, one
   fewer directory.
2. **Grow the package.** If the visibility boundary does matter,
   the smell will self-resolve as the package gains siblings. The
   finding is just a checkpoint to confirm intent.

An intentional visibility boundary may also be suppressed with
`--exclude=<path-segment>`; G12 is advisory and should not force inlining a
load-bearing package boundary.

When the package contains nothing but a single small struct, the
struct often wants to live in the type system of its caller anyway.

## Related

A premature package whose one type has a stuttering name is often
also flagged by [G2 Stutter Names](g2-stutter-names.md), reinforcing
the "this could just be a single value/type up a level" signal.

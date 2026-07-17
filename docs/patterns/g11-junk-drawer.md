# G11 — Generic Filename

A file is named with a generic word: `helpers.go`,
`utils.go`, `common.go`, `misc.go`, `shared.go`, `lib.go`. The name
describes location ("this file holds helpers") instead of content.

## Why this matters

A generic name hides intent and creates a place where unrelated code can
accrete. The name alone does not prove that this has happened: `helpers.go`
may contain one cohesive helper. In that case the finding asks only for a
content-specific rename.

## What lagotto checks

For each directory, it checks for files whose basename matches the reserved
set: `helpers.go`, `utils.go`, `common.go`, `misc.go`, `shared.go`, `lib.go`.
It parses each matching file and records its top-level declaration count and
physical line count. Generated files are excluded via Go's standard
`// Code generated ... DO NOT EDIT.` marker.

| Severity | Condition                                      |
| -------- | ---------------------------------------------- |
| LOW      | generic name below the accumulation threshold  |
| MEDIUM   | ≥10 declarations and ≥200 physical lines       |

A small match is explicitly classified as a naming signal, not evidence of
mixed concerns. A larger match reports accumulation risk, but still asks the
reader to inspect cohesion before splitting.

## Positive example (fires)

```text
parser/
├── parser.go
├── tokens.go
├── helpers.go    // <- content is hidden by the name
└── ast.go
```

## Negative example (does NOT fire)

```text
parser/
├── parser.go
├── tokens.go
├── escapes.go    // formerly part of helpers.go
└── ast.go
```

`escapes.go` describes its content. Future contributors looking for
where to put "a small helper" don't see a file named for that
purpose and have to think about where the helper actually belongs.

## How to fix it

Open the file and name it after what it actually does. If it contains one FFI
error conversion helper, rename it to `error.go`; no split is needed. Split
only when the contents actually have independently changing concerns:

```text
parser/
├── parser.go
├── tokens.go
├── escapes.go    // unicode escape handling
├── lexer.go      // small lexing helpers
└── ast.go
```

If a helper is a truly tiny one-off utility and inlining it at the call site is
cleaner, that remains an option, not a requirement.

## Related

- [G10 Shadow Suffix](g10-shadow-suffix.md): the same anti-pattern
  with a content prefix attached (`auth_helpers.go`).
- [G5 Disconnected File Concerns](g5-mixed-concern-file.md): a separate
  cohesion signal for substantial disconnected clusters in 600+ line files.

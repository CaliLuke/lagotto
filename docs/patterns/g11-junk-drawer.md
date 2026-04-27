# G11 — Junk Drawer

A file is named with a generic catch-all word: `helpers.go`,
`utils.go`, `common.go`, `misc.go`, `shared.go`, `lib.go`. The name
describes location ("this file holds helpers") instead of content.

## Why this matters

A junk-drawer file is, by construction, a place where unrelated code
accretes. Future contributors looking for somewhere to put a small
helper see a file literally named for that purpose and drop it in;
the file grows; nobody reads it top-to-bottom because nobody knows
what's supposed to be in it.

## What lagotto checks

For each directory, it checks for files whose basename matches the
reserved set: `helpers.go`, `utils.go`, `common.go`, `misc.go`,
`shared.go`, `lib.go`. Any match fires.

| Severity | Condition         |
| -------- | ----------------- |
| LOW      | any matching file |

## Positive example (fires)

```text
parser/
├── parser.go
├── tokens.go
├── helpers.go    // <- junk drawer
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

Open the file. Group its contents by what they're actually about,
then rename — splitting if necessary:

```text
parser/
├── parser.go
├── tokens.go
├── escapes.go    // unicode escape handling
├── lexer.go      // small lexing helpers
└── ast.go
```

If the helpers are truly tiny one-off utilities and inlining them at
the call site is cleaner, do that.

## Related

- [G10 Shadow Suffix](g10-shadow-suffix.md): the same anti-pattern
  with a content prefix attached (`auth_helpers.go`).
- [G5 Mixed-Concern File](g5-mixed-concern-file.md): the file-size
  cousin of this smell.

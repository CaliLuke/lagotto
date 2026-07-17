# G10 — Shadow Suffix

A file is named by its relationship to siblings (`user_helpers.go`,
`graph_handlers.go`, `task_actions.go`) instead of by its content.
The suffix tells the reader where the file sits in the package, not
what's in it.

## Why this matters

Filenames are the cheapest navigation aid in a Go package. A name
like `session.go` says "this file is about sessions"; a name like
`auth_helpers.go` says "this file is the catch-all for auth-adjacent
code that didn't fit elsewhere". The suffix becomes obsolete the
moment the file's content has a real name.

Shadow suffixes also tend to compound: once `auth_helpers.go`
exists, future contributors drop one more helper into it rather than
naming the new concept. Over time the file accretes unrelated
things.

## What lagotto checks

For each directory, it lists files whose basename ends in one of
the relationship-naming suffixes:

`_helpers`, `_utils`, `_handlers`, `_actions`, `_responses`,
`_data`, `_support`, `_extra`, `_impl`, `_misc`

Any matching file fires a finding (one finding per directory listing
all offenders).

| Severity | Condition         |
| -------- | ----------------- |
| LOW      | any matching file |

## Positive example (fires)

```text
auth/
├── auth_helpers.go      // catch-all
├── login.go
└── session.go
```

`auth_helpers.go` is named by where it lives, not what it does.

## Negative example (does NOT fire)

```text
auth/
├── login.go
├── session.go
└── token.go
```

Each filename describes content. The reader can pick a file from the
listing without opening it.

## How to fix it

Read the file's contents and rename it after the actual concept
inside. If the file is also 600+ lines, use the
[G5 Disconnected File Concerns](g5-mixed-concern-file.md) signal to decide whether
independently changing sections justify a split before naming each result.

```text
auth/
├── login.go
├── password_hash.go    // was: auth_helpers.go
├── session.go
└── token.go
```

## Related

- [G5 Disconnected File Concerns](g5-mixed-concern-file.md): large shadow-suffix
  files may also deserve a navigation and cohesion review.
- [G11 Generic Filename](g11-junk-drawer.md): the standalone version of
  the same anti-pattern (`helpers.go` instead of `something_helpers.go`).

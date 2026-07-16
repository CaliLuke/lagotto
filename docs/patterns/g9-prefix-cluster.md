# G9 — Prefix Cluster

Three or more files in a flat directory share a name prefix
(`node_create.go`, `node_delete.go`, `node_update.go`,
`node_search.go`). The cluster wants to be a subpackage; the prefix
is doing namespace work that a `node/` directory could do
naturally.

## Why this matters

A leading-prefix cluster is a soft signal that the directory has an
implicit subdomain. Promoting it to a subpackage:

- shortens each filename (`node_create.go` → `node/create.go`)
- isolates the cluster's tests, helpers, and types
- makes the import path describe the structure (`pkg/node` rather
  than `pkg`'s `node_*` files)

A cluster of two is usually fine. Three or more is the threshold
where the directory listing starts feeling like a forced flat layout.

## What lagotto checks

For each directory, it groups files by the substring before the
first `_`, `-`, or `.` separator. One-character prefixes are ignored; common
two-character domain prefixes such as `db_` and `io_` count. Recognized GOOS
and GOARCH suffix suites are excluded. A finding fires at 3 or more files.

| Severity | Condition               |
| -------- | ----------------------- |
| MEDIUM   | ≥4 files share a prefix |
| LOW      | 3 files share a prefix  |

## Positive example (fires)

```text
graph/
├── node_create.go
├── node_delete.go
├── node_update.go
├── node_search.go
└── routes.go
```

The four `node_*` files want to be `graph/node/`.

## Negative example (does NOT fire)

```text
graph/
├── node_create.go
├── node_delete.go
└── routes.go
```

Two prefixed files. Lifting them to a subpackage would add more
navigation cost than it removes.

## How to fix it

```text
graph/
├── node/
│   ├── create.go
│   ├── delete.go
│   ├── update.go
│   └── search.go
└── routes.go
```

If the prefixed files share types or constants, those move to the
new subpackage too. If they're tightly coupled to the parent
package's other types, you may need to widen the parent's
interfaces — that's the cost of the layout improvement.

## Related

A prefix cluster paired with a [G1 Receiver
Monolith](g1-receiver-monolith.md) is often the same problem:
methods of one god type spread across `<concern>_<verb>.go` files.
Fixing the receiver decomposition fixes the prefix cluster
automatically.

# G9 — Prefix Cluster

Three or more files in a flat directory share a name prefix
(`node_create.go`, `node_delete.go`, `node_update.go`,
`node_search.go`). The prefix may describe a healthy organization convention,
or it may be doing namespace work that a `node/` directory could do naturally.

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

Prefix shape alone is always **LOW**. Lagotto does not infer semantic
disconnection from filenames, even for large clusters.

## Positive example (fires)

```text
graph/
├── node_create.go
├── node_delete.go
├── node_update.go
├── node_search.go
└── routes.go
```

The four `node_*` files warrant a quick boundary review. They do not prove that
`graph/node/` would be better.

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

Make this change only when the cluster evolves independently and the new
package has a useful API boundary. If the files are tightly coupled to the
parent package, the prefix is probably healthy organization and should remain.

## Related

A prefix cluster paired with a [G1 Receiver
Monolith](g1-receiver-monolith.md) is often the same problem:
methods of one god type spread across `<concern>_<verb>.go` files.
Fixing the receiver decomposition fixes the prefix cluster
automatically.

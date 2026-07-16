# G3 — Build-Tag Pair Sprawl

A directory contains three or more `*_stub.go` or `*_cgo.go` paired
files. Each pair conditions one source file with a build tag and
provides a stub for builds that don't satisfy it. A single pair is a
normal Go pattern; once the pattern recurs across many files, the
directory is hosting two parallel implementations and the conditional
surface is wide enough to deserve its own subpackage.

## Why this matters

Build tags partition compilation units, but they don't visually
separate them in the directory listing. A reader scanning a
directory sees `connect.go`, `connect_stub.go`, `reconnect.go`,
`reconnect_stub.go`, … and has to mentally track which files belong
to which build flavor. Once the count crosses three pairs, that
mental overhead exceeds the cost of moving the conditional code into
a sibling subpackage and depending on it through an interface.

## What lagotto checks

For each directory on disk, the detector counts `_stub` and `_cgo`
variants whose un-suffixed partner exists and where at least one side has a
`//go:build` or `// +build` constraint. Reading the directory rather than the
active build view ensures mutually exclusive pairs are both visible.

| Severity | Condition          |
| -------- | ------------------ |
| MEDIUM   | ≥3 build-tag pairs |

## Positive example (fires)

```text
typedbstore/
├── connect.go
├── connect_stub.go
├── reconnect.go
├── reconnect_stub.go
├── resilience.go
├── resilience_stub.go
└── store.go
```

Three stub pairs. The directory listing is half conditional code.

## Negative example (does NOT fire)

```text
typedbstore/
├── connect.go
├── connect_stub.go
└── store.go
```

One pair. The conditional surface is narrow enough that splitting
into subpackages would add more navigation cost than it removes.

## How to fix it

Move the stub branch into a sibling subpackage and have the parent
depend on a shared interface:

```text
typedbstore/
├── conn/
│   ├── conn.go         // build tag: cgo
│   └── stub.go         // build tag: !cgo
└── store.go            // imports conn, no build tags here
```

`store.go` takes a `conn.Iface`; `conn/conn.go` provides the cgo
implementation, `conn/stub.go` provides the no-cgo fallback. Each
file in the parent directory is now unconditional, and the
conditional code lives in one focused subpackage with two flavors.

## Related

This smell often co-occurs with a [G1 Receiver
Monolith](g1-receiver-monolith.md): the god type owns both the
production and stub code paths because it can't push them out
without splitting itself first.

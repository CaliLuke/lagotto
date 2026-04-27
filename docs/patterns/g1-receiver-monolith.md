# G1 — Receiver Monolith

A single named type owns too many methods spanning too many concerns
to navigate, test, or evolve sanely.

## Why this matters in Go specifically

Methods can only be defined in the package that declares the
receiver type. If `*TypeDB` has 100 methods, all 100 must live in
the same directory. No file rename, no convention can change that.
The receiver IS the layout boundary.

Polyglot linters miss this. They count files or lines and suggest
"split the file", but file splits don't help when the methods are
locked to one receiver. The fix is type-level decomposition: split
the god struct into smaller types, each living in its own subpackage.

## What lagotto checks

For every named struct in the workspace:

1. Build the **effective method set** via
   `types.NewMethodSet(*types.Pointer(named))`. This sees
   methods promoted via embedding — the type system's view, not the
   source-AST receiver-name view, so the [embedding theatre disguise](g1b-decomposition-theatre.md)
   cannot hide a god type.
2. Filter to methods declared in the same package as the receiver,
   so a thin wrapper around an external type (e.g., embedding
   `*sql.DB`) is not flagged.
3. Bucket method names by leading verb (Create/Get/Update/Delete/
   Search/Run/Check/Connect/…) to count distinct concerns.
4. Skip test doubles (`Fake*`, `Mock*`, `Stub*`, `Spy*`) and
   `testutil/`-style packages — they legitimately implement wide
   interfaces.

A finding fires when the type owns ≥15 methods spanning ≥3 concerns.

| Severity | Condition               |
| -------- | ----------------------- |
| CRITICAL | ≥25 methods or ≥7 files |
| HIGH     | ≥15 methods, <25        |
| MEDIUM   | barely above threshold  |

## Positive example (fires)

```go
type TypeDB struct {
    conn *Conn
}

func (t *TypeDB) CreateNodes(...) {...}
func (t *TypeDB) DeleteNodes(...) {...}
func (t *TypeDB) GetNodeCompact(...) {...}
func (t *TypeDB) ListGraph(...) {...}
func (t *TypeDB) SearchGraph(...) {...}
func (t *TypeDB) RunChecks(...) {...}
func (t *TypeDB) CreateThread(...) {...}
func (t *TypeDB) UpdateThreadStatus(...) {...}
func (t *TypeDB) ApprovePromotion(...) {...}
// ... and dozens more across mutation / search / threads / promotions
```

100 methods spanning create, read, update, delete, search, threads,
promotions, checks, connect — nine concerns. Anyone reading
`typedb.go` cannot reason about this type's surface in one sitting.

## Negative example (does NOT fire)

```go
type Conn struct { /* internal buffers */ }

func (c *Conn) Read(p []byte) (int, error)
func (c *Conn) Write(p []byte) (int, error)
func (c *Conn) Close() error
func (c *Conn) RemoteAddr() net.Addr
// ... fifteen methods, all on connection plumbing
```

Method count is high, but every method operates at the same
abstraction level (connection lifecycle and I/O). lagotto's
verb-prefix grouping puts all of them under one or two concern
buckets, so the ≥3-concerns gate filters this out.

## How to fix it

The principle: each natural concern becomes its own subpackage with
its own narrow type.

1. **Identify concerns**: read the methods and group them. For a
   typical god-DB: `connection`, `nodes`, `edges`, `search`,
   `threads`, `promotions`, `checks`, `metadata`, `tasks`.
2. **Create subpackages**: `pkg/conn`, `pkg/nodes`, `pkg/edges`, …
   Each subpackage exposes its own struct (`*Mutator`, `*Searcher`,
   etc.) holding only the state that concern needs.
3. **Move the methods** into those structs (Fowler: Move Method).
4. **Delete the original god type** — or reduce it to a tiny
   construction helper that returns the sub-services to the caller
   once at startup. **Do not retain accessors** (`(t *TypeDB)
Search() *search.Searcher`); they recreate the god type's
   surface.
5. **Update every caller** in the same change. Each consumer takes
   only the narrow service it actually uses. This is the cost of
   the refactor, not a separate phase.
6. **Narrow the legacy interface.** If the god type satisfied an
   omnibus interface (`graph.GraphStore`), split it into per-concern
   interfaces (`graph.NodeStore`, `graph.SearchStore`, …) and update
   consumers.

### What to avoid

- **Aliases-to-one-struct** disguise: `type Searcher = ops` pretends
  to split the type but keeps every method on one struct. lagotto
  detects this as [G1B](g1b-decomposition-theatre.md).
- **Aggregate holder** disguise: `type TypeDB struct { Search
*Searcher; Nodes *Mutator; … }` with the sub-services in the same
  package. Callers still receive one handle. lagotto detects this as
  [G1C](g1c-aggregate-holder.md).
- **Embedding** to recreate the god type's method set: `type TypeDB
struct { *graphOps }` promotes all of `*graphOps`'s methods onto
  `*TypeDB`. The G1 method-set count sees through this — the
  evidence will include `promoted_from`.

## Related

- [G1B — Decomposition Theatre](g1b-decomposition-theatre.md)
- [G1C — Aggregate Holder](g1c-aggregate-holder.md)
- [G6 — Facade Method](g6-facade-method.md): often appears after a
  partial decomposition, when methods are kept on the old type as
  thin pass-throughs.

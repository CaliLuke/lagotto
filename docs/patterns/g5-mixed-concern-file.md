# G5 — Mixed-Concern File

A single `.go` file holds three or more unrelated declaration groups
(types, methods, validation, utilities) over 100+ lines. The file is
a junk drawer disguised as a module: a reader has to scan past
several unrelated concerns to find the one they're looking for.

## Why this matters

Go encourages many small files in one package, since the package is
the unit of encapsulation. Within a package, files are organizational
hints: this file holds the types, that file holds the validation,
this other file holds the route handlers. When one file mixes types

- methods + validation + utilities, those hints break and readers
  revert to grep.

The 100-line floor exists because tiny mixed files (a 30-line file
with one type, one method, and one helper) are not a real navigation
problem. The smell only matters at sizes where reading top-to-bottom
costs real time.

## What lagotto checks

For each non-test source file, the detector classifies declarations
into groups:

- **types** — `type X struct {…}`, `type Y interface {…}`, etc.
- **methods** — functions with a receiver
- **validation** — functions whose names start with `Validate`,
  `Verify`, or `Check`
- **utilities** — other top-level functions
- **constants** — `const`/`var` blocks

A finding fires when the file has ≥3 distinct groups and ≥100
lines. Constants are excluded from the group count when they're the
only group, so a typed-constants file isn't flagged.

| Severity | Condition     |
| -------- | ------------- |
| CRITICAL | ≥600 lines    |
| HIGH     | ≥300 lines    |
| MEDIUM   | 100–299 lines |

## Positive example (fires)

```go
package billing

const defaultGracePeriod = 7 * 24 * time.Hour

type Invoice struct{ /* … */ }
type LineItem struct{ /* … */ }

func (i *Invoice) Total() money.Amount { /* … */ }

func ValidateInvoice(i *Invoice) error { /* … */ }

func formatCurrency(a money.Amount) string { /* … */ }
func parseDueDate(s string) (time.Time, error) { /* … */ }
// … 30 more helpers
```

Constants + types + methods + validation + utilities, all in one
500-line file. A reader looking for "how is the invoice total
calculated" has to skip past validation and helpers; a reader looking
for "what does ValidateInvoice check" has to skip past types and
helpers.

## Negative example (does NOT fire)

```go
// invoice.go — types only
package billing

type Invoice struct{ /* … */ }
type LineItem struct{ /* … */ }
type Tax struct{ /* … */ }
// 80 more types
```

A single-concern file at any length. The reader knows exactly what's
in it.

## How to fix it

Split the file by group. The split usually maps directly to the
groups the detector named:

- `types.go` — type declarations
- `methods.go` (or `<type>.go` per type) — methods
- `validation.go` — validators
- `helpers.go` would itself be a [G11](g11-junk-drawer.md)
  smell — give helper files content-named identifiers instead
  (`format.go`, `parse.go`).

If the file mixes work for two unrelated types, that's a signal the
two types might want to live in different files (or different
packages).

## Related

- [G11 — Junk Drawer](g11-junk-drawer.md): same anti-pattern at the
  filename level (`utils.go`, `helpers.go`).
- [G9 — Prefix Cluster](g9-prefix-cluster.md): if splitting reveals
  many `<concern>_<thing>.go` files, the cluster might want to be a
  subpackage.

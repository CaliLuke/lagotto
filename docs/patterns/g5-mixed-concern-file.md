# G5 — Mixed-Concern File

A single `.go` file holds three or more unrelated declaration groups
(types, methods, validation, utilities) over 200+ lines. The file may be
a junk drawer disguised as a module: a reader has to scan past
several unrelated concerns to find the one they're looking for.

## Why this matters

Go encourages many small files in one package, since the package is
the unit of encapsulation. Within a package, files are organizational
hints: this file holds the types, that file holds the validation,
this other file holds the route handlers. When one file mixes types,
methods, validation, and utilities, those hints break and readers
revert to grep.

The 200-line floor exists because modest mixed files—including cohesive
commands that keep a type, its methods, and small helpers together—are
not a real navigation problem. The smell only matters at sizes where
reading top-to-bottom costs real time.

## What lagotto checks

For each non-test source file, the detector classifies declarations
into groups:

- **types** — `type X struct {…}`, `type Y interface {…}`, etc.
- **methods** — functions with a receiver
- **validation** — functions whose names start with `Validate`,
  `Verify`, or `Check`
- **utilities** — other top-level functions

A finding fires when the file has ≥3 distinct groups and ≥200
physical file lines. `const` and `var` blocks are supporting declarations,
not independent concerns, so they never contribute to the group count.

| Severity | Condition     |
| -------- | ------------- |
| HIGH     | ≥600 lines    |
| MEDIUM   | 200–599 lines |

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

Review semantic cohesion before splitting. Declaration categories are
navigation evidence, not proof that the code has separate responsibilities.
When the groups do change independently, a split may map to:

- `types.go` — type declarations
- `methods.go` (or `<type>.go` per type) — methods
- `validation.go` — validators
- `helpers.go` would itself be a [G11](g11-junk-drawer.md)
  smell — give helper files content-named identifiers instead
  (`format.go`, `parse.go`).

If the file mixes work for two unrelated types, that's a signal the
two types might want to live in different files (or different
packages).

If the file is intentionally cohesive, keep it and suppress the exact
location, for example `--suppress=G5@billing/invoice.go`.

## Related

- [G11 — Junk Drawer](g11-junk-drawer.md): same anti-pattern at the
  filename level (`utils.go`, `helpers.go`).
- [G9 — Prefix Cluster](g9-prefix-cluster.md): if splitting reveals
  many `<concern>_<thing>.go` files, the cluster might want to be a
  subpackage.

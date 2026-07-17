# G5 — Disconnected File Concerns

A `.go` file is at least 600 lines and its top-level declarations form two or
more substantial disconnected reference clusters. This is evidence of
potentially separable implementation islands—not a conclusion that the public
API or package must be decomposed.

## Why this matters

Go's package, not its file, is the encapsulation boundary. Keeping a feature's
types, methods, validation, constructors, and helpers together is idiomatic and
often highly cohesive. Declaration kinds therefore say very little about
responsibility.

Reference relationships are stronger evidence. If one group of declarations
uses and implements only members of that group while another group does the
same, the file may contain independently navigable operations or phases. A file
split can improve navigation and change isolation without changing its package
or exported API.

## What lagotto checks

For every non-test, non-generated source file, Lagotto builds an intra-file
graph with one node per top-level type, function, method, variable, or constant.
It adds edges for:

- direct calls and identifier references;
- methods and their receiver types;
- constructors, parameters, and result types;
- multiple declarations using the same package-level state or declaration;
- registrations using the same constructor from a dot-imported DSL;
- concrete types that implicitly satisfy the same named interface.

The interface edge matters because Go conformance is structural. A file holding
one interface, several implementations, their constructors, and shared helpers
is one cohesive interface family even when an implementation never explicitly
names the interface.

By default, a connected component is nominated as substantial when it contains
at least two primary declarations (types, functions, or methods), or one
declaration spanning at least 40 lines. Lagotto then calculates cyclomatic
complexity only after the file becomes a candidate. A single callable that
qualified solely through the line threshold must have complexity at least 5.
Multi-member components and non-callable declarations are not rejected by a
function-complexity metric. A finding requires by default:

- at least 600 physical file lines; and
- at least two substantial connected components.

Tiny disconnected helpers are retained as minor-component evidence but do not
manufacture a finding by themselves.

| Severity | Condition                                               |
| -------- | ------------------------------------------------------- |
| MEDIUM   | 600+ lines with 2+ substantial disconnected components  |

## Configuration

For one-off analysis, `lagotto mixed` exposes:

```text
--min-lines
--min-component-members
--min-component-lines
--min-single-component-complexity
--severity
--cohesive-min-lines
```

For durable policy used by both `audit` and `mixed`, check in
`.lagotto.yaml`:

```yaml
version: 1
suppress:
  - G5@tqlgen/parser.go
mixed:
  min_lines: 600
  min_component_members: 2
  min_component_lines: 40
  min_single_component_complexity: 5
  severity: medium
  cohesive_min_lines: 1200
```

Command-line `mixed` flags override repository settings. Every G5 finding's
JSON evidence records the effective thresholds so a reviewer can reproduce
why a component qualified.

Set `min_single_component_complexity` to zero to disable post-candidate
complexity validation. Complexity never initiates a repository-wide scan: it
runs only for files already nominated by the cohesion and size rules, and it
never creates a finding by itself.

`cohesive_min_lines` configures the separate LOW-severity G13 signal; set it to
zero to disable G13. It does not relax or alter G5's disconnected-component
requirement.

## Evidence

JSON output includes every member of each substantial component with its name,
kind, start line, end line, declaration line count, and—where applicable—its
cyclomatic complexity. File-level evidence includes total and maximum
complexity plus the five highest-complexity named functions under
`prioritization_hotspots`, as well as total, minor, ignored zero-primary, and
complexity-rejected component counts. These categories explain every raw graph
component. The human-readable suggestion names the smallest candidate island
so reviewers do not have to reconstruct the graph manually.

## Positive example (fires)

```go
type Parser struct{ /* ... */ }
func (Parser) Parse(input string) Node { return normalize(input) }
func normalize(input string) Node { /* ... */ }

type Renderer struct{ /* ... */ }
func (Renderer) Render(node Node) string { return decorate(node) }
func decorate(node Node) string { /* ... */ }
```

If this file is 600+ lines and the parser and renderer families do not refer to
one another, they form two evidence-backed candidate islands.

## Negative example: interface family (does not fire)

```go
type Filter interface { Validate() error }

type Equal struct{ Value string }
func (Equal) Validate() error { return nil }
func NewEqual(v string) Filter { return Equal{Value: v} }

type Range struct{ Min, Max int }
func (Range) Validate() error { return nil }
func NewRange(min, max int) Filter { return Range{Min: min, Max: max} }
```

The concrete types implement the same interface, and the constructors return
that interface. The graph treats the declarations as one cohesive family.

## How to respond

Treat G5 as a backlog review signal, not a mandatory refactor or correctness
failure. Inspect the listed component members and ask whether the candidate
island changes independently or is difficult to navigate.

When it is independent, move the component together into a content-named file
in the same package. Preserve the public API unless separate evidence supports
an API change. When the graph misses a semantic relationship—reflection,
registration, generated conventions, or domain coupling can do this—keep the
file intact and suppress the exact location:

```bash
lagotto audit --suppress=G5@billing/invoice.go .
```

## Related

- [G13 — Large Cohesive File](g13-large-cohesive-file.md): preserves a LOW
  navigation signal for very large connected files without weakening G5.

- [G11 — Generic Filename](g11-junk-drawer.md): a content-naming signal whose
  severity is weighted by declaration and line counts.
- [G9 — Prefix Cluster](g9-prefix-cluster.md): if a file split reveals many
  `<concern>_<thing>.go` files, the cluster may warrant a package-level review.
